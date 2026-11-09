package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/gateway"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/messaging"
	messagingmodels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/services/sales"
	salesEnums "larsa-academy-microservice/pkg/services/sales/enums"
	salesModels "larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/shared"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"time"

	"math"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type OrderDetails struct {
	models.Order `bson:",inline"`
	Coachings    []models.Coaching        `bson:"coachings"`
	Courses      []models.Course          `bson:"courses"`
	Bundles      []models.Bundle          `bson:"bundles"`
	Materials    []models.PrintedMaterial `bson:"materials"`
}

type OrderSvcs interface {
	GetMyOrders(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.OrderPagination, error)
	GetById(ctx context.Context, id string) (*models.Order, error)
	GetByStudentId(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*models.OrderPagination, error)
	PayOrder(ctx context.Context, orderId string, provider enums.PaymentGateway) (any, error)
	EnrollStudent(ctx context.Context, orderId string) (any, error)
	HandlePaymentSuccess(ctx context.Context, orderId string, paymentId string) (any, error)
	ApplyCoupon(ctx context.Context, orderId string, code string) (*models.Order, error)
	CancelOrder(ctx context.Context, orderId string) (*models.Order, error)
}

type ordersvcs struct {
	orderRepo                  db.MainRepo[models.Order]
	coachingRepo               db.MainRepo[models.Coaching]
	courseRepo                 db.MainRepo[models.Course]
	studentSessionRepo         db.MainRepo[models.StudentSession]
	studentCourseRepo          db.MainRepo[models.StudentCourse]
	studentCoachingRepo        db.MainRepo[models.CoachingStudent]
	studentBundleRepo          db.MainRepo[models.StudentBundle]
	bundleRepo                 db.MainRepo[models.Bundle]
	renewalRepo                db.MainRepo[models.Renewal]
	printedMaterialStudentRepo db.MainRepo[models.PrintedMaterialStudent]
	withtxn                    *db.WithTxn
	paymentGateway             gateway.PaymentGw
	invoiceSvcs                sales.InvoiceSvcs
	pricingSvcs                sales.PricingSvcs
	parentSvcs                 shared.ParentChecker
	messageSvcs                messaging.MessageSvcs
}

// Ensure ordersvcs implements shared.PaymentNotifier
var _ shared.PaymentNotifier = (*ordersvcs)(nil)

func NewOrderSvcs(i *do.Injector) (OrderSvcs, error) {
	invoiceSvcs, err := do.Invoke[sales.InvoiceSvcs](i)
	if err != nil {
		return nil, err
	}
	pricingSvcs, err := do.Invoke[sales.PricingSvcs](i)
	if err != nil {
		return nil, err
	}
	s := &ordersvcs{
		orderRepo:                  db.NewRepo[models.Order](i),
		coachingRepo:               db.NewRepo[models.Coaching](i),
		courseRepo:                 db.NewRepo[models.Course](i),
		studentSessionRepo:         db.NewRepo[models.StudentSession](i),
		studentCourseRepo:          db.NewRepo[models.StudentCourse](i),
		studentCoachingRepo:        db.NewRepo[models.CoachingStudent](i),
		studentBundleRepo:          db.NewRepo[models.StudentBundle](i),
		bundleRepo:                 db.NewRepo[models.Bundle](i),
		printedMaterialStudentRepo: db.NewRepo[models.PrintedMaterialStudent](i),
		renewalRepo:                db.NewRepo[models.Renewal](i),
		//
		withtxn:        do.MustInvoke[*db.WithTxn](i),
		paymentGateway: do.MustInvoke[gateway.PaymentGw](i),
		invoiceSvcs:    invoiceSvcs,
		pricingSvcs:    pricingSvcs,
		parentSvcs:     do.MustInvoke[shared.ParentChecker](i),
		messageSvcs:    do.MustInvoke[messaging.MessageSvcs](i),
	}

	// Provide as PaymentNotifier
	do.ProvideValue[shared.PaymentNotifier](i, s)

	return s, nil
}

func (o *ordersvcs) GetMyOrders(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.OrderPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"studentId": cfg.User.Id, "trash": false}
	if query != nil {
		qFilter, err := query.ConvertToMongo()
		if err == nil {
			for k, v := range qFilter {
				filter[k] = v
			}
		}
	}

	total, err := o.orderRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"createdAt": -1}},
		{"$skip": skip},
		{"$limit": limit},
	}

	var orders []models.Order
	if err := o.orderRepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &orders)
	}); err != nil {
		return nil, err
	}

	o.enrichOrdersWithInvoice(ctx, orders)

	return &models.OrderPagination{
		Items: orders,
		Pagination: &types.Pagination{
			TotalCount: total,
			PerPage:    limit,
			TotalPages: math.Ceil(float64(total) / float64(limit)),
		},
	}, nil
}

func (o *ordersvcs) GetById(ctx context.Context, id string) (*models.Order, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, helpers.InvalidObjectId()
	}

	order, err := o.orderRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, helpers.NotFoundError("order not found")
	}

	cfg, _ := util.GetReqAppCfg(ctx)
	if order.StudentId != cfg.User.Id {
		// Check if user is parent of the student
		isParent, err := o.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, order.StudentId)
		if err != nil || !isParent {
			return nil, helpers.Forbidden("unauthorized to view this order")
		}
	}

	o.enrichOrder(ctx, order)
	return order, nil
}

func (o *ordersvcs) GetByStudentId(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*models.OrderPagination, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, helpers.InvalidObjectId()
	}

	cfg, _ := util.GetReqAppCfg(ctx)
	if _studentId != cfg.User.Id {
		// Check if user is parent of the student
		isParent, err := o.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, _studentId)
		if err != nil || !isParent {
			return nil, helpers.Forbidden("unauthorized to view orders for this student")
		}
	}

	filter := bson.M{"studentId": _studentId, "trash": false}
	if query != nil {
		qFilter, err := query.ConvertToMongo()
		if err == nil {
			for k, v := range qFilter {
				filter[k] = v
			}
		}
	}

	total, err := o.orderRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"createdAt": -1}},
		{"$skip": skip},
		{"$limit": limit},
	}

	var orders []models.Order
	if err := o.orderRepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &orders)
	}); err != nil {
		return nil, err
	}

	o.enrichOrdersWithInvoice(ctx, orders)

	return &models.OrderPagination{
		Items: orders,
		Pagination: &types.Pagination{
			TotalCount: total,
			PerPage:    limit,
			TotalPages: math.Ceil(float64(total) / float64(limit)),
		},
	}, nil
}

func (o *ordersvcs) enrichOrdersWithInvoice(ctx context.Context, orders []models.Order) {
	for i := range orders {
		o.enrichOrder(ctx, &orders[i])
	}
}

func (o *ordersvcs) enrichOrder(ctx context.Context, ord *models.Order) {
	if ord == nil || ord.InvoiceId.IsZero() {
		return
	}
	inv, err := o.invoiceSvcs.GetById(ctx, ord.InvoiceId.Hex())
	if err != nil || inv == nil {
		return
	}
	ord.TotalPrice = inv.TotalAmount
	ord.Currency = inv.Currency
}

func (o *ordersvcs) validateOrderEnrollments(ctx context.Context, order *models.Order) (bool, error) {
	hasConflict := false
	for _, item := range order.Items {
		switch item.Type {
		case enums.CartTypeCourse:
			isRenewable, _ := o.renewalRepo.Count(ctx, bson.M{
				"studentId":   order.StudentId,
				"refId":       item.RefId,
				"contentType": enums.RenewalContentTypeCourse,
				"trash":       false,
			})
			if isRenewable == 0 {
				count, _ := o.studentCourseRepo.Count(ctx, bson.M{"studentId": order.StudentId, "courseId": item.RefId, "trash": false})
				if count > 0 && !order.IsRenewal {
					inBundle := false
					for _, bundleItem := range order.Items {
						if bundleItem.Type == enums.CartTypeBundle {
							inBundle = true
							break
						}
					}
					if !inBundle {
						hasConflict = true
					}
				}
			}
		case enums.CartTypeCoaching:
			continue
		case enums.CartTypeBundle:
			count, _ := o.studentBundleRepo.Count(ctx, bson.M{"studentId": order.StudentId, "bundleId": item.RefId, "trash": false})
			if count > 0 && !order.IsRenewal {
				hasConflict = true
			}
		}
	}
	return hasConflict, nil
}

func (o *ordersvcs) checkAndAutoCancelOrder(ctx context.Context, orderId string, invoice *salesModels.Invoice) error {

	oid, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return helpers.NotFoundError("order not found")
	}
	order, err := o.orderRepo.GetByFilter(ctx, bson.M{"_id": oid, "trash": false})
	if err != nil {
		return helpers.NotFoundError("order not found")
	}

	if order.Status != enums.OrderStatusPending {
		return nil
	}

	overdue := !invoice.DueDate.IsZero() && time.Now().After(invoice.DueDate)

	enrollmentConflict, _ := o.validateOrderEnrollments(ctx, order)

	if overdue || enrollmentConflict {
		_, err := o.orderRepo.Patch(ctx, bson.M{"_id": order.Id}, bson.M{"$set": bson.M{
			"status":    enums.OrderStatusCancelled,
			"updatedAt": time.Now(),
		}})
		if err != nil {
			return err
		}

		if overdue {
			return errors.New("order has been cancelled due to overdue payment")
		}
		return errors.New("order has been cancelled because you are already enrolled in some items")
	}

	return nil
}

func (o *ordersvcs) PayOrder(ctx context.Context, orderId string, provider enums.PaymentGateway) (any, error) {
	cfg, _ := util.GetReqAppCfg(ctx)
	_id, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return nil, errors.New("invalid order id")
	}

	type aux struct {
		models.Order        `bson:",inline"`
		models.PricingPlans `bson:",inline"`
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		coachingPricingLookup,
		coursePricingLookup,
		bundlePricingLookup,
		printedMaterialLookup,
	}

	var result []aux
	if err := o.orderRepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	}); err != nil {
		return nil, errors.New("failed to get order")
	}

	var orderData aux
	if len(result) == 0 {
		// Fallback to simple lookup if pricing lookup failed (e.g. some items don't have pricing yet)
		order, err := o.orderRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
		if err != nil {
			return nil, errors.New("order not found")
		}
		orderData = aux{Order: *order}
	} else {
		orderData = result[0]
	}

	if orderData.Order.Status != enums.OrderStatusPending {
		return nil, errors.New("you can only pay pending orders")
	}

	hasConflict, _ := o.validateOrderEnrollments(ctx, &orderData.Order)
	if hasConflict {
		return nil, errors.New("you are already enrolled in some items in this order")
	}

	// Create/Update Invoice - This is the source of truth for pricing
	var invoice *salesModels.Invoice
	if orderData.Order.InvoiceId.IsZero() {
		// Use transaction to ensure invoice creation and order update are atomic
		invResult, err := o.withtxn.Exec(ctx, func(sessCtx mongo.SessionContext) (any, error) {
			// Reload order inside transaction
			freshOrder, err := o.orderRepo.GetByFilter(sessCtx, bson.M{"_id": _id, "trash": false})
			if err != nil {
				return nil, err
			}
			if !freshOrder.InvoiceId.IsZero() {
				return o.invoiceSvcs.GetById(sessCtx, freshOrder.InvoiceId.Hex())
			}

			// CalcPrices to ensure order items have prices for invoice creation
			priceResults, err := o.pricingSvcs.GetPricesBatch(sessCtx, orderData.Order.Items)
			if err != nil {
				return nil, err
			}
			for i := range orderData.Order.Items {
				for _, pr := range priceResults {
					if orderData.Order.Items[i].RefId == pr.RefId && pr.Error == "" {
						orderData.Order.Items[i].Price = &pr.Price
						break
					}
				}
			}

			inv, err := o.invoiceSvcs.CreateFromOrder(sessCtx, &orderData.Order)
			if err != nil {
				return nil, fmt.Errorf("failed to create invoice: %w", err)
			}

			// Update order with invoice ID and sync total price from invoice
			if _, err := o.orderRepo.Patch(sessCtx, bson.M{"_id": orderData.Order.Id}, bson.M{"$set": bson.M{"invoiceId": inv.ID, "totalPrice": inv.TotalAmount}}); err != nil {
				return nil, fmt.Errorf("failed to update order with invoice ID and total: %w", err)
			}
			return inv, nil
		})

		if err != nil {
			return nil, err
		}
		invoice = invResult.(*salesModels.Invoice)
	} else {
		// Existing invoice - refresh it from the current order state
		// CalcPrices to ensure order items have latest prices for invoice refresh
		priceResults, err := o.pricingSvcs.GetPricesBatch(ctx, orderData.Order.Items)
		if err != nil {
			return nil, err
		}
		for i := range orderData.Order.Items {
			for _, pr := range priceResults {
				if orderData.Order.Items[i].RefId == pr.RefId && pr.Error == "" {
					orderData.Order.Items[i].Price = &pr.Price
					break
				}
			}
		}

		inv, err := o.invoiceSvcs.RefreshFromOrder(ctx, orderData.Order.InvoiceId.Hex(), &orderData.Order)
		if err != nil {
			// Fallback to getting existing if refresh fails
			fmt.Printf("Warning: failed to refresh invoice from order: %v\n", err)
			inv, err = o.invoiceSvcs.GetById(ctx, orderData.Order.InvoiceId.Hex())
			if err != nil {
				return nil, fmt.Errorf("failed to get existing invoice: %w", err)
			}
		}
		invoice = inv

		// Sync order total with invoice (Source of Truth)
		if orderData.Order.TotalPrice != invoice.TotalAmount {
			if _, err := o.orderRepo.Patch(ctx, bson.M{"_id": orderData.Order.Id}, bson.M{"$set": bson.M{"totalPrice": invoice.TotalAmount}}); err != nil {
				fmt.Printf("Warning: failed to sync order total with invoice: %v\n", err)
			}
		}
	}

	// Check for overdue invoice or enrollment conflicts and auto-cancel if needed
	if err := o.checkAndAutoCancelOrder(ctx, orderId, invoice); err != nil {
		return nil, err
	}

	if invoice.TotalAmount == 0 {
		return o.HandlePaymentSuccess(ctx, orderId, "FREE_ORDER")
	}

	// Start payment process
	// Source of truth for pricing is the Invoice
	username := cfg.User.UserData.FirstName + " " + cfg.User.UserData.LastName
	email := cfg.User.UserData.Email

	orderRequest := gateway.Order{
		ProductId: orderId,
		Amount:    invoice.TotalAmount,
		Currency:  invoice.Currency,
		Username:  username,
		Email:     &email,
	}

	paymentResponse, err := o.paymentGateway.PayOrder(ctx, &gateway.OrderRequest{
		Provider: provider,
		Order:    orderRequest,
	})

	if err != nil {
		fmt.Printf("Failed to initiate payment: %v", err)
		return nil, errors.New("Failed to initiate payment")
	}

	return paymentResponse, nil
}

func (o *ordersvcs) EnrollStudent(ctx context.Context, orderId string) (any, error) {
	_id, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return nil, errors.New("invalid order id")
	}

	result, err := o.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		filter := bson.M{"_id": _id, "trash": false}

		// Reload order inside transaction for idempotency
		order, err := o.orderRepo.GetByFilter(ctx, filter)
		if err != nil {
			return nil, fmt.Errorf("failed to get order: %w", err)
		}

		if order.Status == enums.OrderStatusPaid {
			// Already enrolled, return success
			return order, nil
		}

		// Update order status to paid inside the transaction
		updates := bson.M{
			"status":    enums.OrderStatusPaid,
			"updatedAt": time.Now(),
		}

		if _, err := o.orderRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": updates}); err != nil {
			return nil, fmt.Errorf("failed to update order status: %w", err)
		}

		// Use the pipeline to get fully populated order details for enrollment
		pipeline := []bson.M{
			{"$match": filter},
			{
				"$lookup": bson.M{
					"from": "academyCoachings",
					"let": bson.M{
						"coachingIds": "$coachingIds",
					},
					"pipeline": []bson.M{
						{"$match": bson.M{"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$in": bson.A{"$_id", "$$coachingIds"}},
								bson.M{"$eq": bson.A{"$trash", false}},
							},
						}}},
					},
					"as": "coachings",
				},
			},
			{
				"$lookup": bson.M{
					"from": "academyCourses",
					"let": bson.M{
						"courseIds": "$courseIds",
					},
					"pipeline": []bson.M{
						{"$match": bson.M{"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$in": bson.A{"$_id", "$$courseIds"}},
								bson.M{"$eq": bson.A{"$trash", false}},
							},
						}}},
					},
					"as": "courses",
				},
			},
			{
				"$lookup": bson.M{
					"from": "academyBundles",
					"let": bson.M{
						"bundleIds": "$bundleIds",
					},
					"pipeline": []bson.M{
						{"$match": bson.M{"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$in": bson.A{"$_id", "$$bundleIds"}},
								bson.M{"$eq": bson.A{"$trash", false}},
							},
						}}},
					},
					"as": "bundles",
				},
			},
			{
				"$lookup": bson.M{
					"from": "academyPrintedMaterials",
					"let": bson.M{
						"materialIds": "$materialIds",
					},
					"pipeline": []bson.M{
						{"$match": bson.M{"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$in": bson.A{"$_id", "$$materialIds"}},
								bson.M{"$eq": bson.A{"$trash", false}},
							},
						}}},
					},
					"as": "materials",
				},
			},
		}

		var result []OrderDetails
		if err := o.orderRepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
			return cursor.All(ctx, &result)
		}); err != nil {
			return nil, err
		}

		var orderDetails OrderDetails
		if len(result) == 0 {
			return nil, errors.New("order not found")
		}

		orderDetails = result[0]

		var coachingWriteOps, courseWriteOps, bundleWriteOps, materialWriteOps, renewalWriteOps, enrollmentUpdateOps []mongo.WriteModel

		if err := o.addCoachings(ctx, orderDetails, func(coachingOps, renewalOps, enrollmentOps []mongo.WriteModel) {
			coachingWriteOps = append(coachingWriteOps, coachingOps...)
			renewalWriteOps = append(renewalWriteOps, renewalOps...)
			// If it's a renewal, we might have enrollment status updates
			if orderDetails.Order.IsRenewal {
				coachingWriteOps = append(coachingWriteOps, enrollmentOps...)
			}
		}); err != nil {
			return nil, err
		}

		if err := o.addCourses(ctx, orderDetails, func(courseOps, renewalOps, enrollmentOps []mongo.WriteModel) {
			courseWriteOps = append(courseWriteOps, courseOps...)
			renewalWriteOps = append(renewalWriteOps, renewalOps...)
			// If it's a renewal, we might have enrollment status updates
			if orderDetails.Order.IsRenewal {
				courseWriteOps = append(courseWriteOps, enrollmentOps...)
			}
		}); err != nil {
			return nil, err
		}

		if err := o.addBundles(ctx, orderDetails, func(coachingOps, courseOps, bundleOps, renewalOps, enrollmentOps []mongo.WriteModel) {
			coachingWriteOps = append(coachingWriteOps, coachingOps...)
			courseWriteOps = append(courseWriteOps, courseOps...)
			bundleWriteOps = append(bundleWriteOps, bundleOps...)
			renewalWriteOps = append(renewalWriteOps, renewalOps...)
			// For bundles, enrollmentOps might contain course/coaching activation/upserts
			enrollmentUpdateOps = append(enrollmentUpdateOps, enrollmentOps...)
			// If it's a renewal, we might have enrollment status updates for the bundle itself
			if orderDetails.Order.IsRenewal {
				bundleWriteOps = append(bundleWriteOps, enrollmentOps...)
			}
		}); err != nil {
			return nil, err
		}

		if err := o.addMaterials(ctx, orderDetails, func(writeOps []mongo.WriteModel) {
			materialWriteOps = append(materialWriteOps, writeOps...)
		}); err != nil {
			return nil, err
		}

		if len(coachingWriteOps) > 0 {
			if _, err := o.studentSessionRepo.BulkWrite(ctx, coachingWriteOps); err != nil {
				return nil, fmt.Errorf("failed to add student sessions: %w", err)
			}
		}

		if len(enrollmentUpdateOps) > 0 {
			if _, err := o.studentCoachingRepo.BulkWrite(ctx, enrollmentUpdateOps); err != nil {
				return nil, fmt.Errorf("failed to update coaching enrollment status: %w", err)
			}
		}

		if len(courseWriteOps) > 0 {

			if _, err := o.studentCourseRepo.BulkWrite(ctx, courseWriteOps); err != nil {
				return nil, fmt.Errorf("failed to add student courses: %w", err)
			}
		}

		if len(bundleWriteOps) > 0 {
			if _, err := o.studentBundleRepo.BulkWrite(ctx, bundleWriteOps); err != nil {
				return nil, fmt.Errorf("failed to add student bundles: %w", err)
			}
		}

		if !orderDetails.Order.IsRenewal {
			if len(renewalWriteOps) > 0 {
				if _, err := o.renewalRepo.BulkWrite(ctx, renewalWriteOps); err != nil {
					return nil, fmt.Errorf("failed to add renewals: %w", err)
				}
			}
		}

		if len(materialWriteOps) > 0 {
			if _, err := o.printedMaterialStudentRepo.BulkWrite(ctx, materialWriteOps); err != nil {
				return nil, fmt.Errorf("failed to add printed material students: %w", err)
			}
		}

		return orderDetails.Order, nil
	})
	if err != nil {
		return nil, err
	}

	if result != nil {
		order := result.(*models.Order)
		_ = o.messageSvcs.SendNotification(ctx, &messagingmodels.Notification{
			ServiceName: "academy",
			Title:       "Purchase complete",
			Content:     "Your purchase has been completed successfully.",
			Recipient:   order.StudentId.Hex(),
		})
		// Notify parents (child bought)
		parentIds, _ := o.parentSvcs.GetParentUserIdsForStudent(ctx, order.StudentId)
		for _, pid := range parentIds {
			_ = o.messageSvcs.SendNotification(ctx, &messagingmodels.Notification{
				ServiceName: "academy",
				Title:       "Child purchase",
				Content:     "Your child has made a new purchase.",
				Recipient:   pid.Hex(),
			})
		}
	}

	return result, nil
}

func (o *ordersvcs) HandlePaymentSuccess(ctx context.Context, orderId string, paymentId string) (any, error) {
	_id, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return nil, errors.New("invalid order id")
	}

	// Get order to find invoice ID
	order, err := o.orderRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, fmt.Errorf("failed to get order: %w", err)
	}

	// Update invoice status and create payment record
	if !order.InvoiceId.IsZero() {
		invoice, err := o.invoiceSvcs.GetById(ctx, order.InvoiceId.Hex())
		if err == nil && invoice != nil && invoice.Status != salesEnums.InvoiceStatusPaid {
			// Create payment record first (Add will update invoice status to Paid)
			payment := &salesModels.Payment{
				ID:            primitive.NewObjectID(),
				InvoiceID:     order.InvoiceId,
				InvoiceNum:    invoice.InvoiceNum,
				CustomerId:    order.StudentId,
				Amount:        invoice.TotalAmount,
				Status:        salesEnums.PaymentStatusPaid,
				PaymentType:   salesEnums.PaymentTypeElectronic,
				PaymentMethod: string(enums.MYFATOORAH),
				DateTime:      time.Now(),
				CreatedAt:     time.Now(),
				UpdatedAt:     time.Now(),
				CreatedBy:     order.StudentId,
				UpdatedBy:     order.StudentId,
				Trash:         false,
			}
			if _, err := o.invoiceSvcs.CreatePayment(ctx, order.InvoiceId.Hex(), payment); err != nil {
				fmt.Printf("Warning: failed to create payment record: %v\n", err)
			}
		}
	}

	// Enroll student (deliver items and update order status inside transaction)
	return o.EnrollStudent(ctx, orderId)
}

func (o *ordersvcs) ApplyCoupon(ctx context.Context, orderId string, code string) (*models.Order, error) {
	_id, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return nil, helpers.InvalidObjectId()
	}

	order, err := o.orderRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, helpers.NotFoundError("order not found")
	}

	if order.Status != enums.OrderStatusPending {
		return nil, errors.New("cannot apply coupon to a non-pending order")
	}

	// Calculate subtotal from items
	var subTotal float64
	for _, item := range order.Items {
		if item.Price != nil {
			subTotal += *item.Price
		}
	}

	// Validate coupon
	calcRes, err := o.invoiceSvcs.CalculateInvoiceTotal(ctx, &salesModels.InvoiceTotalCalculationRequest{
		SubTotal:   subTotal,
		CouponCode: code,
	})
	if err != nil {
		return nil, fmt.Errorf("invalid coupon: %w", err)
	}

	// Update order
	order.CouponCode = code
	order.Discount = calcRes.Discount
	order.TotalPrice = calcRes.TotalAmount
	order.UpdatedAt = time.Now()

	_, err = o.orderRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": bson.M{
		"couponCode": order.CouponCode,
		"discount":   order.Discount,
		"totalPrice": order.TotalPrice,
		"updatedAt":  order.UpdatedAt,
	}})
	if err != nil {
		return nil, err
	}

	// Refresh invoice if exists
	if !order.InvoiceId.IsZero() {
		_, err = o.invoiceSvcs.RefreshFromOrder(ctx, order.InvoiceId.Hex(), order)
		if err != nil {
			fmt.Printf("Warning: failed to refresh invoice: %v\n", err)
		}
	}

	return order, nil
}

func (o *ordersvcs) CancelOrder(ctx context.Context, orderId string) (*models.Order, error) {
	_id, err := primitive.ObjectIDFromHex(orderId)
	if err != nil {
		return nil, helpers.InvalidObjectId()
	}

	order, err := o.orderRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, helpers.NotFoundError("order not found")
	}

	cfg, _ := util.GetReqAppCfg(ctx)
	if order.StudentId != cfg.User.Id {
		isParent, err := o.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, order.StudentId)
		if err != nil || !isParent {
			return nil, helpers.Forbidden("unauthorized to cancel this order")
		}
	}

	if order.Status != enums.OrderStatusPending {
		return nil, errors.New("only pending orders can be cancelled")
	}

	order.Status = enums.OrderStatusCancelled
	order.UpdatedAt = time.Now()

	_, err = o.orderRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": bson.M{
		"status":    enums.OrderStatusCancelled,
		"updatedAt": time.Now(),
	}})
	if err != nil {
		return nil, err
	}

	return order, nil
}

func (o *ordersvcs) getStudentLastCoachingSessionNum(ctx context.Context, studentId, coachingProgramId primitive.ObjectID) (int, error) {
	pipeline := []bson.M{
		{"$match": bson.M{
			"coachingProgramId": coachingProgramId,
			"studentId":         studentId,
			"trash":             false,
		}},
		{"$group": bson.M{
			"_id":        nil,
			"maxSession": bson.M{"$max": "$sessionNum"},
		}},
	}

	var result []struct {
		MaxSession int `bson:"maxSession"`
	}

	err := o.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return 0, err
	}

	if len(result) == 0 {
		return 0, nil
	}

	return result[0].MaxSession, nil
}

func (o *ordersvcs) addCoachingSessions(ctx context.Context, studentId primitive.ObjectID, coachingProgramId, bundleId primitive.ObjectID, numOfSessions int) ([]mongo.WriteModel, error) {
	var writeOps []mongo.WriteModel

	sessionNum, err := o.getStudentLastCoachingSessionNum(ctx, studentId, coachingProgramId)
	if err != nil {
		return nil, errors.New("failed to get student last coaching session num")
	}

	coaching, err := o.coachingRepo.GetByFilter(ctx, bson.M{"_id": coachingProgramId, "trash": false})
	if err != nil {
		return nil, errors.New("failed to get coaching program for session inheritance")
	}

	var templateSession *models.CoachingSession

	for i := 0; i < numOfSessions; i++ {
		currSessionNum := sessionNum + i + 1
		var sessionName, description transl.Localizable[string]
		var agendas []models.SessionAgenda
		var notes []models.SessionNote
		var resources []models.SessionResource
		var recordings []types.FileField

		for _, s := range coaching.Sessions {
			if s.SessionNum == currSessionNum {
				sessionName = s.SessionName
				description = s.Description
				templateSession = &s

				agendas = make([]models.SessionAgenda, len(s.AgendaItems))
				for j, item := range s.AgendaItems {
					agendas[j] = models.SessionAgenda{
						Item:    item.Item,
						Checked: false,
						Date:    time.Now(),
					}
				}

				notes = make([]models.SessionNote, len(s.Notes))
				for j, item := range s.Notes {
					notes[j] = models.SessionNote{
						Note: item.Note,
						Date: item.Date,
					}
				}

				resources = make([]models.SessionResource, len(s.Resources))
				for j, item := range s.Resources {
					resources[j] = models.SessionResource{
						Resource: item,
					}
				}

				recordings = make([]types.FileField, len(s.Recordings))
				copy(recordings, s.Recordings)

				break
			}
		}

		if templateSession == nil {
			agendas = []models.SessionAgenda{}
			notes = []models.SessionNote{}
			resources = []models.SessionResource{}
			recordings = []types.FileField{}
		}

		addOp := mongo.NewInsertOneModel().SetDocument(models.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         studentId,
			CoachingProgramId: coachingProgramId,
			BundleId:          bundleId,
			SessionNum:        currSessionNum,
			SessionName:       sessionName,
			Description:       description,
			DurationMinutes:   coaching.SessionDuration,
			Agendas:           agendas,
			Notes:             notes,
			Resources:         resources,
			Recordings:        recordings,
			Status:            enums.StudentSessionStatusPending,
			CreatedAt:         time.Now(),
			CreatedBy:         studentId,
			UpdatedAt:         time.Now(),
			UpdatedBy:         studentId,
			Trash:             false,
		})

		writeOps = append(writeOps, addOp)
	}

	return writeOps, nil

}

func (o *ordersvcs) addCoachings(ctx context.Context, orderDetails OrderDetails, callBack func(coachingOps, renewalOps, enrollmentOps []mongo.WriteModel)) error {
	coachingItems := util.SliceFilter(orderDetails.Items, func(item models.CartItem) bool {
		return item.Type == enums.CartTypeCoaching
	})

	if len(coachingItems) == 0 {
		return nil
	}

	coachingMap := make(map[primitive.ObjectID]models.Coaching)
	for _, coaching := range orderDetails.Coachings {
		coachingMap[coaching.Id] = coaching
	}

	writeOps := []mongo.WriteModel{}
	renewalWriteOps := []mongo.WriteModel{}
	enrollmentUpdateOps := []mongo.WriteModel{}
	//item represent a coaching program
	for _, item := range coachingItems {
		if coaching, ok := coachingMap[item.RefId]; ok {
			cs, err := util.Converter[*models.CoachingSubscribe](item.Data)
			if err != nil {
				return fmt.Errorf("failed to convert cart item data: %w", err)
			}

			if !orderDetails.IsRenewal {
				// Check for existing renewal to avoid duplicates during manual top-up
				existingRenewal, err := o.renewalRepo.GetByFilter(ctx, bson.M{
					"studentId":   orderDetails.StudentId,
					"refId":       item.RefId,
					"contentType": enums.RenewalContentTypeCoaching,
					"trash":       false,
				})

				if err == nil && existingRenewal != nil {
					// Update existing renewal data if needed
					renewalWriteOps = append(renewalWriteOps, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"_id": existingRenewal.Id}).
						SetUpdate(bson.M{"$set": bson.M{"data": cs, "updatedAt": time.Now(), "status": enums.RenewalStatusActive}}))
				} else if cs.RenewMonthly {
					renewalWriteOps = append(renewalWriteOps, mongo.NewInsertOneModel().SetDocument(models.Renewal{
						RenewalDto: models.RenewalDto{
							StudentId:       orderDetails.StudentId,
							RenewalType:     enums.RenewalTypeMonthly,
							ContentType:     enums.RenewalContentTypeCoaching,
							RefId:           item.RefId,
							StartDate:       time.Now(),
							Data:            cs,
							NextRenewalDate: time.Now().AddDate(0, 1, 0),
							Status:          enums.RenewalStatusActive,
						},
						CreatedAt: time.Now(),
						CreatedBy: orderDetails.StudentId,
						UpdatedAt: time.Now(),
						UpdatedBy: orderDetails.StudentId,
						Trash:     false,
					}))
				}
				// Create parent enrollment record
				enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"studentId": orderDetails.StudentId, "coachingId": item.RefId}).
					SetUpdate(bson.M{
						"$setOnInsert": bson.M{
							"_id":       primitive.NewObjectID(),
							"createdAt": time.Now(),
						},
						"$set": bson.M{
							"studentId":    orderDetails.StudentId,
							"coachingId":   item.RefId,
							"status":       enums.EnrollmentStatusActive,
							"renewMonthly": cs.RenewMonthly,
							"updatedAt":    time.Now(),
							"trash":        false,
						},
					}).SetUpsert(true))
			} else {
				// If it's a renewal, reactivate the coaching enrollment
				enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"studentId": orderDetails.StudentId, "coachingId": item.RefId, "trash": false}).
					SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))
			}

			coachingSessionsWriteOps, err := o.addCoachingSessions(ctx, orderDetails.StudentId, item.RefId, primitive.NilObjectID, cs.NumOfSessions)
			if err != nil {
				return fmt.Errorf("failed to add coaching sessions: %w", err)
			}
			writeOps = append(writeOps, coachingSessionsWriteOps...)

		} else {
			return fmt.Errorf("coaching program %s not found", coaching.Title["en"])
		}
	}

	callBack(writeOps, renewalWriteOps, enrollmentUpdateOps)

	if len(enrollmentUpdateOps) > 0 {
		if _, err := o.studentCoachingRepo.BulkWrite(ctx, enrollmentUpdateOps); err != nil {
			return fmt.Errorf("failed to update coaching enrollment status: %w", err)
		}
	}

	return nil
}

func (o *ordersvcs) addCourses(ctx context.Context, orderDetails OrderDetails, callBack func(courseOps, renewalOps, enrollmentOps []mongo.WriteModel)) error {
	courseItems := util.SliceFilter(orderDetails.Items, func(item models.CartItem) bool {
		return item.Type == enums.CartTypeCourse
	})

	if len(courseItems) == 0 {
		return nil
	}

	courseMap := make(map[primitive.ObjectID]models.Course)
	for _, course := range orderDetails.Courses {
		courseMap[course.Id] = course
	}

	writeOps := []mongo.WriteModel{}
	renewalWriteOps := []mongo.WriteModel{}
	enrollmentUpdateOps := []mongo.WriteModel{}

	for _, item := range courseItems {
		if course, ok := courseMap[item.RefId]; ok {
			if !orderDetails.IsRenewal {
				// Check for manual extension of an existing renewable course
				existingRenewal, err := o.renewalRepo.GetByFilter(ctx, bson.M{
					"studentId":   orderDetails.StudentId,
					"refId":       item.RefId,
					"contentType": enums.RenewalContentTypeCourse,
					"trash":       false,
				})

				if err == nil && existingRenewal != nil {
					// Manual extension: Advance NextRenewalDate
					newDate := existingRenewal.NextRenewalDate
					if existingRenewal.RenewalType == enums.RenewalTypeMonthly {
						newDate = newDate.AddDate(0, 1, 0)
					} else if existingRenewal.RenewalType == enums.RenewalTypeYearly {
						newDate = newDate.AddDate(1, 0, 0)
					}

					renewalWriteOps = append(renewalWriteOps, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"_id": existingRenewal.Id}).
						SetUpdate(bson.M{"$set": bson.M{
							"nextRenewalDate": newDate,
							"status":          enums.RenewalStatusActive,
							"updatedAt":       time.Now(),
						}}))

					// Ensure enrollment is active
					enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
						SetFilter(bson.M{"studentId": orderDetails.StudentId, "courseId": item.RefId, "trash": false}).
						SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))
				} else {
					// First-time purchase
					addOp := mongo.NewInsertOneModel().SetDocument(models.StudentCourse{
						Id:                primitive.NewObjectID(),
						StudentId:         orderDetails.StudentId,
						CourseId:          item.RefId,
						StartDate:         time.Now(),
						Status:            enums.EnrollmentStatusActive,
						CertificateStatus: "pending",
						Progress:          0,
						Logins:            0,
						LastActivity:      time.Now(),
						CompletedLessons:  []primitive.ObjectID{},
						CreatedAt:         time.Now(),
						CreatedBy:         orderDetails.StudentId,
						UpdatedAt:         time.Now(),
						UpdatedBy:         orderDetails.StudentId,
						Trash:             false,
					})
					writeOps = append(writeOps, addOp)
				}
			} else {
				// If it's a renewal, reactivate the course enrollment
				enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"studentId": orderDetails.StudentId, "courseId": item.RefId, "trash": false}).
					SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))
			}
		} else {
			return fmt.Errorf("course %s not found", course.BasicInfo.Name["en"])
		}

	}

	callBack(writeOps, renewalWriteOps, enrollmentUpdateOps)

	if len(enrollmentUpdateOps) > 0 {
		if _, err := o.studentCourseRepo.BulkWrite(ctx, enrollmentUpdateOps); err != nil {
			return fmt.Errorf("failed to update course enrollment status: %w", err)
		}
	}

	return nil
}

func (o *ordersvcs) addBundles(
	ctx context.Context,
	orderDetails OrderDetails,
	callBack func(coachingWriteOps []mongo.WriteModel, courseWriteOps []mongo.WriteModel, bundleWriteOps, renewalOps, enrollmentOps []mongo.WriteModel),
) error {
	bundleItems := util.SliceFilter(orderDetails.Items, func(item models.CartItem) bool {
		return item.Type == enums.CartTypeBundle
	})

	if len(bundleItems) == 0 {
		return nil
	}

	bundleMap := make(map[primitive.ObjectID]models.Bundle)
	for _, bundle := range orderDetails.Bundles {
		bundleMap[bundle.Id] = bundle
	}

	bundleWriteOps := []mongo.WriteModel{}
	coachingWriteOps := []mongo.WriteModel{}
	courseWriteOps := []mongo.WriteModel{}
	renewalWriteOps := []mongo.WriteModel{}
	enrollmentUpdateOps := []mongo.WriteModel{}

	for _, bundleItem := range bundleItems {
		if bundle, ok := bundleMap[bundleItem.RefId]; ok {
			if !orderDetails.IsRenewal {
				for _, content := range bundle.Content {
					switch content.Type {
					case "coaching":
						{
							var numOfSessions int
							switch v := content.Meta["numOfSessions"].(type) {
							case int:
								numOfSessions = v
							case int32:
								numOfSessions = int(v)
							case int64:
								numOfSessions = int(v)
							case float64:
								numOfSessions = int(v)
							default:
								return fmt.Errorf("invalid num of sessions type: %T", content.Meta["numOfSessions"])
							}

							coachingSessionsWriteOps, err := o.addCoachingSessions(ctx, orderDetails.StudentId, content.RefId, bundle.Id, numOfSessions)
							if err != nil {
								return fmt.Errorf("failed to add coaching sessions: %w", err)
							}
							coachingWriteOps = append(coachingWriteOps, coachingSessionsWriteOps...)

							// Check for existing coaching renewal within bundle
							existingRenewal, err := o.renewalRepo.GetByFilter(ctx, bson.M{
								"studentId":   orderDetails.StudentId,
								"refId":       content.RefId,
								"contentType": enums.RenewalContentTypeCoaching,
								"trash":       false,
							})

							if err == nil && existingRenewal != nil {
								// Advance NextRenewalDate if it's a renewable coaching
								newDate := existingRenewal.NextRenewalDate
								if existingRenewal.RenewalType == enums.RenewalTypeMonthly {
									newDate = newDate.AddDate(0, 1, 0)
								} else if existingRenewal.RenewalType == enums.RenewalTypeYearly {
									newDate = newDate.AddDate(1, 0, 0)
								}

								renewalWriteOps = append(renewalWriteOps, mongo.NewUpdateOneModel().
									SetFilter(bson.M{"_id": existingRenewal.Id}).
									SetUpdate(bson.M{"$set": bson.M{
										"nextRenewalDate": newDate,
										"status":          enums.RenewalStatusActive,
										"updatedAt":       time.Now(),
									}}))
							}

							// Upsert coaching enrollment
							enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
								SetFilter(bson.M{"studentId": orderDetails.StudentId, "coachingId": content.RefId}).
								SetUpdate(bson.M{
									"$setOnInsert": bson.M{
										"_id":       primitive.NewObjectID(),
										"createdAt": time.Now(),
									},
									"$set": bson.M{
										"studentId":  orderDetails.StudentId,
										"coachingId": content.RefId,
										"status":     enums.EnrollmentStatusActive,
										"updatedAt":  time.Now(),
										"trash":      false,
									},
								}).SetUpsert(true))

						}

					case "course":
						{
							// Check for existing course renewal within bundle
							existingRenewal, err := o.renewalRepo.GetByFilter(ctx, bson.M{
								"studentId":   orderDetails.StudentId,
								"refId":       content.RefId,
								"contentType": enums.RenewalContentTypeCourse,
								"trash":       false,
							})

							if err == nil && existingRenewal != nil {
								// Advance NextRenewalDate
								newDate := existingRenewal.NextRenewalDate
								if existingRenewal.RenewalType == enums.RenewalTypeMonthly {
									newDate = newDate.AddDate(0, 1, 0)
								} else if existingRenewal.RenewalType == enums.RenewalTypeYearly {
									newDate = newDate.AddDate(1, 0, 0)
								}

								renewalWriteOps = append(renewalWriteOps, mongo.NewUpdateOneModel().
									SetFilter(bson.M{"_id": existingRenewal.Id}).
									SetUpdate(bson.M{"$set": bson.M{
										"nextRenewalDate": newDate,
										"status":          enums.RenewalStatusActive,
										"updatedAt":       time.Now(),
									}}))

								// Ensure course enrollment is active
								enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
									SetFilter(bson.M{"studentId": orderDetails.StudentId, "courseId": content.RefId, "trash": false}).
									SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))
							} else {
								// Check if already enrolled (even without renewal)
								count, _ := o.studentCourseRepo.Count(ctx, bson.M{"studentId": orderDetails.StudentId, "courseId": content.RefId, "trash": false})
								if count == 0 {
									courseWriteOps = append(courseWriteOps, mongo.NewInsertOneModel().SetDocument(models.StudentCourse{
										Id:                primitive.NewObjectID(),
										StudentId:         orderDetails.StudentId,
										CourseId:          content.RefId,
										BundleId:          bundle.Id,
										StartDate:         time.Now(),
										Status:            enums.EnrollmentStatusActive,
										CertificateStatus: "pending",
										Progress:          0,
										Logins:            0,
										LastActivity:      time.Now(),
										CompletedLessons:  []primitive.ObjectID{},
										CreatedAt:         time.Now(),
										CreatedBy:         orderDetails.StudentId,
										UpdatedAt:         time.Now(),
										UpdatedBy:         orderDetails.StudentId,
										Trash:             false,
									}))
								} else {
									// Just ensure it's active
									enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
										SetFilter(bson.M{"studentId": orderDetails.StudentId, "courseId": content.RefId, "trash": false}).
										SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))
								}
							}
						}
					default:
						return fmt.Errorf("invalid content type: %s", content.Type)
					}
				}

				bundleWriteOps = append(bundleWriteOps, mongo.NewInsertOneModel().SetDocument(models.StudentBundle{
					Id:        primitive.NewObjectID(),
					StudentId: orderDetails.StudentId,
					BundleId:  bundle.Id,
					Status:    enums.EnrollmentStatusActive,
					CreatedAt: time.Now(),
					CreatedBy: orderDetails.StudentId,
					UpdatedAt: time.Now(),
					UpdatedBy: orderDetails.StudentId,
				}))
			} else {
				// If it's a renewal, reactivate the bundle enrollment and its contents
				enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
					SetFilter(bson.M{"studentId": orderDetails.StudentId, "bundleId": bundle.Id, "trash": false}).
					SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))

				for _, content := range bundle.Content {
					switch content.Type {
					case "coaching":
						// Handle coaching sessions reactivation or addition if needed
						// For now, reactivate the sessions if we have a way to identify them,
						// but typically sessions are just added.
						var numOfSessions int
						switch v := content.Meta["numOfSessions"].(type) {
						case int:
							numOfSessions = v
						case int32:
							numOfSessions = int(v)
						case int64:
							numOfSessions = int(v)
						case float64:
							numOfSessions = int(v)
						default:
							return fmt.Errorf("invalid num of sessions type: %T", content.Meta["numOfSessions"])
						}

						coachingSessionsWriteOps, err := o.addCoachingSessions(ctx, orderDetails.StudentId, content.RefId, bundle.Id, numOfSessions)
						if err != nil {
							return fmt.Errorf("failed to add coaching sessions: %w", err)
						}
						coachingWriteOps = append(coachingWriteOps, coachingSessionsWriteOps...)
					case "course":
						enrollmentUpdateOps = append(enrollmentUpdateOps, mongo.NewUpdateOneModel().
							SetFilter(bson.M{"studentId": orderDetails.StudentId, "courseId": content.RefId, "bundleId": bundle.Id, "trash": false}).
							SetUpdate(bson.M{"$set": bson.M{"status": enums.EnrollmentStatusActive, "updatedAt": time.Now()}}))
					}
				}
			}

		} else {
			return fmt.Errorf("bundle %s not found", bundle.Name["en"])
		}

	}

	callBack(coachingWriteOps, courseWriteOps, bundleWriteOps, renewalWriteOps, enrollmentUpdateOps)

	if len(enrollmentUpdateOps) > 0 {
		if _, err := o.studentBundleRepo.BulkWrite(ctx, enrollmentUpdateOps); err != nil {
			return fmt.Errorf("failed to update bundle enrollment status: %w", err)
		}
	}

	return nil
}

func (o *ordersvcs) addMaterials(ctx context.Context, orderDetails OrderDetails, callBack func(materialWriteOps []mongo.WriteModel)) error {
	materialItems := util.SliceFilter(orderDetails.Items, func(item models.CartItem) bool {
		return item.Type == enums.CartTypeMaterial
	})

	if len(materialItems) == 0 {
		return nil
	}

	materialMap := make(map[primitive.ObjectID]models.PrintedMaterial)
	for _, material := range orderDetails.Materials {
		materialMap[material.Id] = material
	}

	writeOps := []mongo.WriteModel{}
	for _, item := range materialItems {
		if _, ok := materialMap[item.RefId]; ok {
			writeOps = append(writeOps, mongo.NewInsertOneModel().SetDocument(models.PrintedMaterialStudent{
				Id:         primitive.NewObjectID(),
				StudentId:  orderDetails.StudentId,
				MaterialId: item.RefId,
				CreatedAt:  time.Now(),
				CreatedBy:  orderDetails.StudentId,
				UpdatedAt:  time.Now(),
				UpdatedBy:  orderDetails.StudentId,
				Trash:      false,
			}))
		} else {
			return fmt.Errorf("material %s not found", item.RefId.Hex())
		}
	}

	callBack(writeOps)

	return nil

}
