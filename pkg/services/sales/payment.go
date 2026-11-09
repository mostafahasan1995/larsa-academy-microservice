package sales

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/sales/enums"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/shared"
	"larsa-academy-microservice/pkg/types"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PaymentSvcs interface {
	GetById(ctx context.Context, id string) (*models.Payment, error)
	Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.PaymentPagination, error)
	Add(ctx context.Context, data *models.Payment) (*models.Payment, error)
	Update(ctx context.Context, id string, data *models.Payment) (*models.Payment, error)
	UpdateStatus(ctx context.Context, id string, status enums.PaymentStatus) (*models.Payment, error)
	Delete(ctx context.Context, id string) error
}

type paymentsvcs struct {
	paymentRepo db.MainRepo[models.Payment]
	invoiceRepo db.MainRepo[models.Invoice]
	injector    *do.Injector
}

func NewPaymentSvcs(i *do.Injector) (PaymentSvcs, error) {
	return &paymentsvcs{
		paymentRepo: db.NewRepo[models.Payment](i),
		invoiceRepo: db.NewRepo[models.Invoice](i),
		injector:    i,
	}, nil
}

func (s *paymentsvcs) GetById(ctx context.Context, id string) (*models.Payment, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid payment id")
	}
	return s.paymentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
}

func (s *paymentsvcs) Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.PaymentPagination, error) {
	filter := bson.M{}
	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}
		var err error
		filter, err = conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"_id": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.Payment `bson:"data"`
	}

	err := s.paymentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	payments := []models.Payment{}
	if len(result) > 0 {
		payments = result[0].Data
	}
	return &models.PaymentPagination{
		Payments: payments,
		Pagination: types.Pagination{
			TotalPages: math.Ceil(float64(total) / float64(limit)),
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *paymentsvcs) Add(ctx context.Context, data *models.Payment) (*models.Payment, error) {
	// 1) Validate invoice exists (and not trashed)
	if data.InvoiceID.IsZero() {
		return nil, errors.New("invoiceId is required")
	}
	inv, err := s.invoiceRepo.GetByFilter(ctx, bson.M{"_id": data.InvoiceID, "trash": false})
	if err != nil {
		return nil, errors.New("invoice does not exist")
	}
	// 2) Ensure invoice not canceled and status is Pending (can only add payments to pending invoices)
	if inv.Status == enums.InvoiceStatusCanceled {
		return nil, errors.New("cannot add payment to a canceled invoice")
	}
	if inv.Status != enums.InvoiceStatusPending {
		return nil, errors.New("can only add payment to invoices with Pending status")
	}

	// 2.1) Check if due date has passed
	if !inv.DueDate.IsZero() && time.Now().After(inv.DueDate) {
		return nil, errors.New("cannot pay an overdue invoice")
	}

	// 3) Validate amount does not exceed outstanding balance
	var agg []struct {
		Sum float64 `bson:"sum"`
	}
	err = s.paymentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"invoiceId": data.InvoiceID, "trash": false}},
		{"$group": bson.M{"_id": nil, "sum": bson.M{"$sum": "$amount"}}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &agg)
	})
	if err != nil {
		return nil, err
	}
	paidSoFar := 0.0
	if len(agg) > 0 {
		paidSoFar = agg[0].Sum
	}
	totalDue := inv.TotalAmount // TotalAmount already includes fees and excludes discounts in current logic
	remaining := totalDue - paidSoFar
	if data.Amount > remaining {
		return nil, errors.New("payment exceeds outstanding balance")
	}

	data.ID = primitive.NewObjectID()
	data.InvoiceNum = inv.InvoiceNum
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()

	// Set default status to Pending if not provided
	if data.Status == "" {
		data.Status = enums.PaymentStatusPending
	}

	if err := s.paymentRepo.Add(ctx, data); err != nil {
		return nil, err
	}

	// Update invoice status if payment status is Paid
	if data.Status == enums.PaymentStatusPaid {
		if err := s.updateInvoiceStatusIfPaid(ctx, data.InvoiceID); err != nil {
			// Log error but don't fail the payment creation
			fmt.Printf("Warning: failed to update invoice status: %v\n", err)
		}
		// If linked to an order, notify order service
		if !inv.OrderID.IsZero() {
			go s.notifyOrderService(context.Background(), inv.OrderID.Hex(), data.ID.Hex())
		}
	}

	return data, nil
}

func (s *paymentsvcs) Update(ctx context.Context, id string, data *models.Payment) (*models.Payment, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid payment id")
	}

	// Get existing payment to check status change
	existingPayment, err := s.paymentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("payment not found")
	}

	data.UpdatedAt = time.Now()
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": data}
	result, err := s.paymentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Update invoice status if payment status changed to Paid
	statusChangedToPaid := existingPayment.Status != enums.PaymentStatusPaid && data.Status == enums.PaymentStatusPaid
	if statusChangedToPaid {
		if err := s.updateInvoiceStatusIfPaid(ctx, data.InvoiceID); err != nil {
			// Log error but don't fail the payment update
			fmt.Printf("Warning: failed to update invoice status: %v\n", err)
		}
	}

	return result, nil
}

// UpdateStatus updates only the payment status and reflects changes to invoice status
func (s *paymentsvcs) UpdateStatus(ctx context.Context, id string, status enums.PaymentStatus) (*models.Payment, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid payment id")
	}

	// Get existing payment
	existingPayment, err := s.paymentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("payment not found")
	}

	// Update only the status field
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"status":    status,
			"updatedAt": time.Now(),
		},
	}
	result, err := s.paymentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// If status changed to Paid, update invoice status
	statusChangedToPaid := existingPayment.Status != enums.PaymentStatusPaid && status == enums.PaymentStatusPaid
	if statusChangedToPaid {
		if err := s.updateInvoiceStatusIfPaid(ctx, existingPayment.InvoiceID); err != nil {
			// Log error but don't fail the payment update
			fmt.Printf("Warning: failed to update invoice status: %v\n", err)
		}
	}

	// If status changed from Paid to something else, check if invoice should revert to Pending
	statusChangedFromPaid := existingPayment.Status == enums.PaymentStatusPaid && status != enums.PaymentStatusPaid
	if statusChangedFromPaid {
		if err := s.revertInvoiceStatusIfNotPaid(ctx, existingPayment.InvoiceID); err != nil {
			// Log error but don't fail the payment update
			fmt.Printf("Warning: failed to revert invoice status: %v\n", err)
		}
	}

	return result, nil
}

// revertInvoiceStatusIfNotPaid reverts invoice status to Pending if not fully paid
func (s *paymentsvcs) revertInvoiceStatusIfNotPaid(ctx context.Context, invoiceID primitive.ObjectID) error {
	// Get invoice
	inv, err := s.invoiceRepo.GetByFilter(ctx, bson.M{"_id": invoiceID, "trash": false})
	if err != nil {
		return fmt.Errorf("invoice not found: %v", err)
	}

	// Don't revert if already canceled
	if inv.Status == enums.InvoiceStatusCanceled {
		return nil
	}

	// Calculate total paid amount
	var agg []struct {
		Sum float64 `bson:"sum"`
	}
	err = s.paymentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"invoiceId": invoiceID, "status": enums.PaymentStatusPaid, "trash": false}},
		{"$group": bson.M{"_id": nil, "sum": bson.M{"$sum": "$amount"}}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &agg)
	})
	if err != nil {
		return fmt.Errorf("error calculating paid amount: %v", err)
	}

	totalPaid := 0.0
	if len(agg) > 0 {
		totalPaid = agg[0].Sum
	}

	// Calculate total due
	totalDue := inv.TotalAmount

	// Revert to Pending if not fully paid
	if totalPaid < totalDue-0.01 && inv.Status == enums.InvoiceStatusPaid {
		invoiceFilter := bson.M{"_id": invoiceID, "trash": false}
		invoiceUpdate := bson.M{
			"$set": bson.M{
				"status":    enums.InvoiceStatusPending,
				"updatedAt": time.Now(),
			},
		}
		_, err = s.invoiceRepo.Patch(ctx, invoiceFilter, invoiceUpdate)
		if err != nil {
			return fmt.Errorf("error reverting invoice status: %v", err)
		}
	}

	return nil
}

// updateInvoiceStatusIfPaid checks if invoice is fully paid and updates status accordingly
func (s *paymentsvcs) updateInvoiceStatusIfPaid(ctx context.Context, invoiceID primitive.ObjectID) error {
	// Get invoice
	inv, err := s.invoiceRepo.GetByFilter(ctx, bson.M{"_id": invoiceID, "trash": false})
	if err != nil {
		return fmt.Errorf("invoice not found: %v", err)
	}

	// Don't update if already paid or canceled
	if inv.Status == enums.InvoiceStatusPaid || inv.Status == enums.InvoiceStatusCanceled {
		return nil
	}

	// Calculate total paid amount
	var agg []struct {
		Sum float64 `bson:"sum"`
	}
	err = s.paymentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"invoiceId": invoiceID, "status": enums.PaymentStatusPaid, "trash": false}},
		{"$group": bson.M{"_id": nil, "sum": bson.M{"$sum": "$amount"}}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &agg)
	})
	if err != nil {
		return fmt.Errorf("error calculating paid amount: %v", err)
	}

	totalPaid := 0.0
	if len(agg) > 0 {
		totalPaid = agg[0].Sum
	}

	// Calculate total due
	totalDue := inv.TotalAmount

	// Update invoice status to Paid if fully paid (with small tolerance for floating point)
	if totalPaid >= totalDue-0.01 {
		invoiceFilter := bson.M{"_id": invoiceID, "trash": false}
		invoiceUpdate := bson.M{
			"$set": bson.M{
				"status":    enums.InvoiceStatusPaid,
				"updatedAt": time.Now(),
			},
		}
		_, err = s.invoiceRepo.Patch(ctx, invoiceFilter, invoiceUpdate)
		if err != nil {
			return fmt.Errorf("error updating invoice status: %v", err)
		}

		// If fully paid and linked to an order, notify order service
		if !inv.OrderID.IsZero() {
			go s.notifyOrderService(context.Background(), inv.OrderID.Hex(), "full_payment")
		}
	}

	return nil
}

func (s *paymentsvcs) notifyOrderService(ctx context.Context, orderId string, paymentId string) {
	notifier, err := do.Invoke[shared.PaymentNotifier](s.injector)
	if err != nil {
		fmt.Printf("Warning: failed to get PaymentNotifier: %v\n", err)
		return
	}

	if _, err := notifier.HandlePaymentSuccess(ctx, orderId, paymentId); err != nil {
		fmt.Printf("Error notifying payment success to order service: %v\n", err)
	}
}

func (s *paymentsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid payment id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.paymentRepo.Patch(ctx, filter, update)
	return err
}
