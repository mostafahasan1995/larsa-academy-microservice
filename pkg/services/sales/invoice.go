package sales

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	contentEnums "larsa-academy-microservice/pkg/services/content/enums"
	contentModels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/sales/enums"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"strconv"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type InvoiceSvcs interface {
	GetById(ctx context.Context, id string) (*models.Invoice, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) ([]models.Invoice, *types.Pagination, error)
	GetAll(ctx context.Context) ([]models.Invoice, error)
	Add(ctx context.Context, data *models.Invoice) (*models.Invoice, error)
	GetDueDate(ctx context.Context) (time.Time, error)
	CreateFromOrder(ctx context.Context, order *contentModels.Order) (*models.Invoice, error)
	CreatePayment(ctx context.Context, invoiceID string, payment *models.Payment) (*models.Payment, error)
	Update(ctx context.Context, id string, data *models.Invoice) (*models.Invoice, error)
	RefreshFromOrder(ctx context.Context, id string, order *contentModels.Order) (*models.Invoice, error)
	Delete(ctx context.Context, id string) error
	CalculateProducts(ctx context.Context, req *models.ProductCalculationRequest) (*models.ProductCalculationResponse, error)
	CalculateInvoiceTotal(ctx context.Context, req *models.InvoiceTotalCalculationRequest) (*models.InvoiceTotalCalculationResponse, error)
}

type invoicesvcs struct {
	invoiceRepo  db.MainRepo[models.Invoice]
	couponRepo   db.MainRepo[models.Coupon]
	courseRepo   db.MainRepo[contentModels.Course]
	coachingRepo db.MainRepo[contentModels.Coaching]
	bundleRepo   db.MainRepo[contentModels.Bundle]
	materialRepo db.MainRepo[contentModels.PrintedMaterial]
	studentRepo  db.MainRepo[contactModels.Student]
	paymentSvcs  PaymentSvcs
	pricingSvcs  PricingSvcs
}

func NewInvoiceSvcs(i *do.Injector) (InvoiceSvcs, error) {
	paymentSvcs, err := do.Invoke[PaymentSvcs](i)
	if err != nil {
		return nil, err
	}
	pricingSvcs, err := do.Invoke[PricingSvcs](i)
	if err != nil {
		return nil, err
	}
	return &invoicesvcs{
		invoiceRepo:  db.NewRepo[models.Invoice](i),
		couponRepo:   db.NewRepo[models.Coupon](i),
		courseRepo:   db.NewRepo[contentModels.Course](i),
		coachingRepo: db.NewRepo[contentModels.Coaching](i),
		bundleRepo:   db.NewRepo[contentModels.Bundle](i),
		materialRepo: db.NewRepo[contentModels.PrintedMaterial](i),
		studentRepo:  db.NewRepo[contactModels.Student](i),
		paymentSvcs:  paymentSvcs,
		pricingSvcs:  pricingSvcs,
	}, nil
}

func (s *invoicesvcs) GetById(ctx context.Context, id string) (*models.Invoice, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid invoice id")
	}
	invoice, err := s.invoiceRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	// Populate product refs
	if err := s.populateProductRefs(ctx, invoice); err != nil {
		// Log error but don't fail the request
		fmt.Printf("Warning: failed to populate product refs: %v\n", err)
	}

	// Populate customer name if empty
	if len(invoice.CustomerName) == 0 && !invoice.CustomerID.IsZero() {
		student, err := s.studentRepo.GetByFilter(ctx, bson.M{"_id": invoice.CustomerID, "trash": false})
		if err == nil && student != nil {
			invoice.CustomerName = transl.MergeLocalizable(student.GeneralInfo.FirstName, student.GeneralInfo.LastName, " ")
		}
	}

	return invoice, nil
}

// populateProductRefs populates RefDetails for each product in the invoice
func (s *invoicesvcs) populateProductRefs(ctx context.Context, invoice *models.Invoice) error {
	for i := range invoice.Products {
		product := &invoice.Products[i]
		if product.Ref.IsZero() {
			continue
		}

		refDetails, err := s.productLookup(ctx, product.ProductType, product.Ref)
		if err != nil {
			// Continue with other products even if one fails
			continue
		}
		product.ProductObj = refDetails
	}
	return nil
}

// productLookup fetches product details based on type and ref ID
func (s *invoicesvcs) productLookup(ctx context.Context, productType enums.ProductType, refID primitive.ObjectID) (*models.ProductObj, error) {
	switch productType {
	case enums.ProductTypeCourse:
		course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": refID, "trash": false})
		if err != nil {
			return nil, fmt.Errorf("course not found: %v", err)
		}
		return &models.ProductObj{Course: course}, nil

	case enums.ProductTypeCoaching:
		coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": refID, "trash": false})
		if err != nil {
			return nil, fmt.Errorf("coaching not found: %v", err)
		}
		return &models.ProductObj{Coaching: coaching}, nil

	case enums.ProductTypeBundle:
		bundle, err := s.bundleRepo.GetByFilter(ctx, bson.M{"_id": refID, "trash": false})
		if err != nil {
			return nil, fmt.Errorf("bundle not found: %v", err)
		}
		return &models.ProductObj{Bundle: bundle}, nil

	case enums.ProductTypeMaterial:
		material, err := s.materialRepo.GetByFilter(ctx, bson.M{"_id": refID, "trash": false})
		if err != nil {
			return nil, fmt.Errorf("material not found: %v", err)
		}
		return &models.ProductObj{PrintedMaterial: material}, nil

	default:
		return nil, fmt.Errorf("unsupported product type: %s", productType)
	}
}

func (s *invoicesvcs) mapProductTypeToCartType(pType enums.ProductType) contentEnums.CartType {
	switch pType {
	case enums.ProductTypeCourse:
		return contentEnums.CartTypeCourse
	case enums.ProductTypeCoaching:
		return contentEnums.CartTypeCoaching
	case enums.ProductTypeBundle:
		return contentEnums.CartTypeBundle
	case enums.ProductTypeMaterial:
		return contentEnums.CartTypeMaterial
	default:
		return ""
	}
}

func (s *invoicesvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) ([]models.Invoice, *types.Pagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, nil, err
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
		Data []models.Invoice `bson:"data"`
	}

	err = s.invoiceRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	pages := 0.0
	if total > 0 && limit > 0 {
		pages = math.Ceil(float64(total) / float64(limit))
	}
	pagination := &types.Pagination{
		TotalPages: pages,
		PerPage:    limit,
		TotalCount: total,
	}
	invoices := []models.Invoice{}
	if len(result) > 0 {
		invoices = result[0].Data
	}

	// Populate customer names if empty
	for i := range invoices {
		if len(invoices[i].CustomerName) == 0 && !invoices[i].CustomerID.IsZero() {
			student, err := s.studentRepo.GetByFilter(ctx, bson.M{"_id": invoices[i].CustomerID, "trash": false})
			if err == nil && student != nil {
				invoices[i].CustomerName = transl.MergeLocalizable(student.GeneralInfo.FirstName, student.GeneralInfo.LastName, " ")
			}
		}
	}

	return invoices, pagination, nil
}

func (s *invoicesvcs) GetAll(ctx context.Context) ([]models.Invoice, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}
	var result []models.Invoice
	err := s.invoiceRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	// Populate customer names if empty
	for i := range result {
		if len(result[i].CustomerName) == 0 && !result[i].CustomerID.IsZero() {
			student, err := s.studentRepo.GetByFilter(ctx, bson.M{"_id": result[i].CustomerID, "trash": false})
			if err == nil && student != nil {
				result[i].CustomerName = transl.MergeLocalizable(student.GeneralInfo.FirstName, student.GeneralInfo.LastName, " ")
			}
		}
	}

	return result, nil
}

func (s *invoicesvcs) GetDueDate(ctx context.Context) (time.Time, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return time.Now().Add(24 * time.Hour), nil // Default to 24 hours if config fails
	}

	// Default to 24 hours (1 day) if not configured
	dueDateHours := 24.0
	val, err := common.GetOptionValue("INVOICE_CREATION_DUE_DATE_HOURS", cfg.Hp)
	if err == nil && val != nil {
		switch v := val.(type) {
		case float64:
			dueDateHours = v
		case int64:
			dueDateHours = float64(v)
		case int:
			dueDateHours = float64(v)
		case string:
			if h, err := strconv.ParseFloat(v, 64); err == nil {
				dueDateHours = h
			}
		}
	}

	return time.Now().Add(time.Duration(dueDateHours) * time.Hour), nil
}

func (s *invoicesvcs) Add(ctx context.Context, data *models.Invoice) (*models.Invoice, error) {
	data.ID = primitive.NewObjectID()
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	data.InvoiceNum = util.GenerateInvoiceNumber()
	if data.DueDate.IsZero() {
		dueDate, _ := s.GetDueDate(ctx)
		data.DueDate = dueDate
	}
	if err := s.invoiceRepo.Add(ctx, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *invoicesvcs) CreatePayment(ctx context.Context, invoiceID string, payment *models.Payment) (*models.Payment, error) {
	invID, err := primitive.ObjectIDFromHex(invoiceID)
	if err != nil {
		return nil, errors.New("invalid invoice id")
	}
	payment.InvoiceID = invID
	return s.paymentSvcs.Add(ctx, payment)
}

func (s *invoicesvcs) CreateFromOrder(ctx context.Context, order *contentModels.Order) (*models.Invoice, error) {
	if order == nil {
		return nil, errors.New("order cannot be nil")
	}

	invoice := &models.Invoice{
		OrderID:    order.Id,
		CustomerID: order.StudentId,
		Currency:   "USD", // Default currency
		Status:     enums.InvoiceStatusPending,
		Products:   make([]models.InvoiceProduct, 0, len(order.Items)),
	}

	// Populate customer name from student record
	if !invoice.CustomerID.IsZero() {
		student, err := s.studentRepo.GetByFilter(ctx, bson.M{"_id": invoice.CustomerID, "trash": false})
		if err == nil && student != nil {
			invoice.CustomerName = transl.MergeLocalizable(student.GeneralInfo.FirstName, student.GeneralInfo.LastName, " ")
		}
	}

	for _, item := range order.Items {
		product := models.InvoiceProduct{
			Price:         *item.Price,
			OriginalPrice: *item.Price,
			Ref:           item.RefId,
			TotalAmount:   *item.Price,
			FinalPrice:    *item.Price,
			Currency:      "USD",
			Data:          item.Data,
		}

		// Map content cart types to sales product types
		switch item.Type {
		case contentEnums.CartTypeCourse:
			product.ProductType = enums.ProductTypeCourse
		case contentEnums.CartTypeCoaching:
			product.ProductType = enums.ProductTypeCoaching
		case contentEnums.CartTypeBundle:
			product.ProductType = enums.ProductTypeBundle
		case contentEnums.CartTypeMaterial:
			product.ProductType = enums.ProductTypeMaterial
		}

		// Try to get name from meta or lookup
		if item.Meta != nil {
			if name, ok := item.Meta["name"].(transl.Localizable[string]); ok {
				product.Name = name
			} else if name, ok := item.Meta["name"].(string); ok {
				product.Name = transl.Localizable[string]{"en": name}
			}
		}

		invoice.Products = append(invoice.Products, product)
	}

	// If names are missing, they will be populated via productLookup when GetById is called
	// but we can also trigger a lookup here if needed.
	if err := s.populateProductRefs(ctx, invoice); err != nil {
		fmt.Printf("Warning: failed to populate product refs during invoice creation: %v\n", err)
	}

	// Update names from populated objects if still empty
	for i := range invoice.Products {
		p := &invoice.Products[i]
		if len(p.Name) == 0 && p.ProductObj != nil {
			if p.ProductObj.Course != nil {
				p.Name = p.ProductObj.Course.GetName()
			} else if p.ProductObj.Coaching != nil {
				p.Name = p.ProductObj.Coaching.GetName()
			} else if p.ProductObj.Bundle != nil {
				p.Name = p.ProductObj.Bundle.GetName()
			} else if p.ProductObj.PrintedMaterial != nil {
				p.Name = p.ProductObj.PrintedMaterial.GetName()
			} else {
				p.Name = transl.Localizable[string]{"en": fmt.Sprintf("%s %s", string(p.ProductType), p.Ref.Hex())}
			}
		}
	}

	// Use CalculateTotalAmount to handle coupons or fees if they were added to the order
	calcProducts := make([]*models.InvoiceProduct, 0, len(invoice.Products))
	for i := range invoice.Products {
		calcProducts = append(calcProducts, &invoice.Products[i])
	}

	// Use new calculation methods
	prodCalcRes, err := s.CalculateProducts(ctx, &models.ProductCalculationRequest{
		Products: calcProducts,
	})
	if err == nil && prodCalcRes != nil {
		// If coupon code is present, we assume the order.Discount is derived from it,
		// so we don't pass it as an additional fixed discount to avoid double counting.
		discountToUse := order.Discount
		if order.CouponCode != "" {
			discountToUse = 0
		}

		invCalcRes, err := s.CalculateInvoiceTotal(ctx, &models.InvoiceTotalCalculationRequest{
			SubTotal:     prodCalcRes.SubTotal,
			Discount:     discountToUse,
			DiscountType: "fixed", // Order.Discount is usually fixed amount
			CouponCode:   order.CouponCode,
			Fees:         0, // Fees can be added later if needed
		})

		if err == nil && invCalcRes != nil {
			invoice.OriginalTotalAmount = invCalcRes.OriginalTotalAmount
			invoice.Discount = invCalcRes.Discount
			invoice.CouponCode = invCalcRes.CouponCode
			invoice.AmountAfterDiscount = invCalcRes.AmountAfterDiscount
			invoice.Fees = invCalcRes.Fees
			invoice.TotalAmount = invCalcRes.TotalAmount
			invoice.SubTotal = invCalcRes.SubTotal

			// Update product final prices from calculation
			for i := range invoice.Products {
				for _, pCalc := range prodCalcRes.Products {
					if invoice.Products[i].Ref == pCalc.ProductID {
						invoice.Products[i].FinalPrice = pCalc.FinalPrice
						invoice.Products[i].TotalAmount = pCalc.FinalPrice
						invoice.Products[i].CouponCode = pCalc.CouponCode
						invoice.Products[i].CouponCodeDiscount = pCalc.CouponCodeDiscount
						break
					}
				}
			}
		}
	} else {
		// Fallback to basic totals if calculation fails
		fmt.Printf("Warning: invoice calculation failed, falling back to basic totals: %v\n", err)
		invoice.OriginalTotalAmount = order.TotalPrice
		invoice.SubTotal = order.TotalPrice
		invoice.TotalAmount = order.TotalPrice
	}

	return s.Add(ctx, invoice)
}

func (s *invoicesvcs) RefreshFromOrder(ctx context.Context, id string, order *contentModels.Order) (*models.Invoice, error) {
	if order == nil {
		return nil, errors.New("order cannot be nil")
	}

	existingInvoice, err := s.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	if existingInvoice.Status == enums.InvoiceStatusPaid {
		return existingInvoice, nil // Don't refresh paid invoices
	}

	// Re-calculate invoice data from order
	newInvoice := &models.Invoice{
		OrderID:    order.Id,
		CustomerID: order.StudentId,
		Currency:   existingInvoice.Currency,
		Status:     existingInvoice.Status,
		Products:   make([]models.InvoiceProduct, 0, len(order.Items)),
	}

	for _, item := range order.Items {
		product := models.InvoiceProduct{
			Price:         *item.Price,
			OriginalPrice: *item.Price,
			Ref:           item.RefId,
			TotalAmount:   *item.Price,
			FinalPrice:    *item.Price,
			Currency:      "USD",
			Data:          item.Data,
		}

		switch item.Type {
		case contentEnums.CartTypeCourse:
			product.ProductType = enums.ProductTypeCourse
		case contentEnums.CartTypeCoaching:
			product.ProductType = enums.ProductTypeCoaching
		case contentEnums.CartTypeBundle:
			product.ProductType = enums.ProductTypeBundle
		case contentEnums.CartTypeMaterial:
			product.ProductType = enums.ProductTypeMaterial
		}

		if item.Meta != nil {
			if name, ok := item.Meta["name"].(transl.Localizable[string]); ok {
				product.Name = name
			} else if name, ok := item.Meta["name"].(string); ok {
				product.Name = transl.Localizable[string]{"en": name}
			}
		}

		newInvoice.Products = append(newInvoice.Products, product)
	}

	if err := s.populateProductRefs(ctx, newInvoice); err != nil {
		fmt.Printf("Warning: failed to populate product refs during invoice refresh: %v\n", err)
	}

	for i := range newInvoice.Products {
		p := &newInvoice.Products[i]
		if len(p.Name) == 0 && p.ProductObj != nil {
			if p.ProductObj.Course != nil {
				p.Name = p.ProductObj.Course.GetName()
			} else if p.ProductObj.Coaching != nil {
				p.Name = p.ProductObj.Coaching.GetName()
			} else if p.ProductObj.Bundle != nil {
				p.Name = p.ProductObj.Bundle.GetName()
			} else if p.ProductObj.PrintedMaterial != nil {
				p.Name = p.ProductObj.PrintedMaterial.GetName()
			} else {
				p.Name = transl.Localizable[string]{"en": fmt.Sprintf("%s %s", string(p.ProductType), p.Ref.Hex())}
			}
		}
	}

	calcProducts := make([]*models.InvoiceProduct, 0, len(newInvoice.Products))
	for i := range newInvoice.Products {
		calcProducts = append(calcProducts, &newInvoice.Products[i])
	}

	// Use new calculation methods
	prodCalcRes, err := s.CalculateProducts(ctx, &models.ProductCalculationRequest{
		Products: calcProducts,
	})

	if err == nil && prodCalcRes != nil {
		// If coupon code is present, we assume the order.Discount is derived from it,
		// so we don't pass it as an additional fixed discount to avoid double counting.
		discountToUse := order.Discount
		if order.CouponCode != "" {
			discountToUse = 0
		}

		invCalcRes, err := s.CalculateInvoiceTotal(ctx, &models.InvoiceTotalCalculationRequest{
			SubTotal:     prodCalcRes.SubTotal,
			Discount:     discountToUse,
			DiscountType: "fixed",
			CouponCode:   order.CouponCode,
			Fees:         0,
		})

		if err == nil && invCalcRes != nil {
			newInvoice.OriginalTotalAmount = invCalcRes.OriginalTotalAmount
			newInvoice.Discount = invCalcRes.Discount
			newInvoice.CouponCode = invCalcRes.CouponCode
			newInvoice.AmountAfterDiscount = invCalcRes.AmountAfterDiscount
			newInvoice.Fees = invCalcRes.Fees
			newInvoice.TotalAmount = invCalcRes.TotalAmount
			newInvoice.SubTotal = invCalcRes.SubTotal

			for i := range newInvoice.Products {
				for _, pCalc := range prodCalcRes.Products {
					if newInvoice.Products[i].Ref == pCalc.ProductID {
						newInvoice.Products[i].FinalPrice = pCalc.FinalPrice
						newInvoice.Products[i].TotalAmount = pCalc.FinalPrice
						newInvoice.Products[i].CouponCode = pCalc.CouponCode
						newInvoice.Products[i].CouponCodeDiscount = pCalc.CouponCodeDiscount
						break
					}
				}
			}
		}
	}

	// Update the existing invoice with new data
	existingInvoice.Products = newInvoice.Products
	existingInvoice.OriginalTotalAmount = newInvoice.OriginalTotalAmount
	existingInvoice.Discount = newInvoice.Discount
	existingInvoice.CouponCode = newInvoice.CouponCode
	existingInvoice.AmountAfterDiscount = newInvoice.AmountAfterDiscount
	existingInvoice.Fees = newInvoice.Fees
	existingInvoice.TotalAmount = newInvoice.TotalAmount
	existingInvoice.SubTotal = newInvoice.SubTotal

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid invoice id")
	}

	existingInvoice.UpdatedAt = time.Now()
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"products":            existingInvoice.Products,
			"originalTotalAmount": existingInvoice.OriginalTotalAmount,
			"discount":            existingInvoice.Discount,
			"couponCode":          existingInvoice.CouponCode,
			"amountAfterDiscount": existingInvoice.AmountAfterDiscount,
			"fees":                existingInvoice.Fees,
			"totalAmount":         existingInvoice.TotalAmount,
			"subTotal":            existingInvoice.SubTotal,
			"updatedAt":           existingInvoice.UpdatedAt,
		},
	}
	return s.invoiceRepo.Patch(ctx, filter, update)
}

func (s *invoicesvcs) Update(ctx context.Context, id string, data *models.Invoice) (*models.Invoice, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid invoice id")
	}

	// Get existing invoice to check status change
	existingInvoice, err := s.invoiceRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("invoice not found")
	}

	// Check if status is changing to Paid
	statusChangedToPaid := existingInvoice.Status != enums.InvoiceStatusPaid && data.Status == enums.InvoiceStatusPaid

	data.UpdatedAt = time.Now()
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": data}
	result, err := s.invoiceRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// If status changed to Paid, decrement redeemCount for all coupons used
	if statusChangedToPaid {
		if err := s.decrementCouponRedeemCounts(ctx, data); err != nil {
			// Log error but don't fail the update
			// You might want to handle this differently based on your requirements
			fmt.Printf("Warning: failed to decrement coupon redeem counts: %v\n", err)
		}
	}

	return result, nil
}

// decrementCouponRedeemCounts decrements redeemCount for all coupons used in the invoice
func (s *invoicesvcs) decrementCouponRedeemCounts(ctx context.Context, invoice *models.Invoice) error {
	couponCodes := make(map[string]bool)

	// Collect coupon codes from products
	for _, product := range invoice.Products {
		if product.CouponCode != "" {
			couponCodes[product.CouponCode] = true
		}
	}

	// Add invoice-level coupon code if present
	if invoice.CouponCode != "" {
		couponCodes[invoice.CouponCode] = true
	}

	// Decrement redeemCount for each unique coupon
	for couponCode := range couponCodes {
		coupon, err := s.lookupCoupon(ctx, couponCode)
		if err != nil {
			return fmt.Errorf("error looking up coupon %s: %v", couponCode, err)
		}
		if coupon == nil {
			continue // Coupon not found, skip
		}

		// Skip if infinite redemption
		if coupon.InfiniteRedemption {
			continue
		}

		// Decrement redeemCount (ensure it doesn't go below 0)
		newRedeemedCount := coupon.RedeemedCount + 1
		if newRedeemedCount > coupon.TotalRedemptions {
			// This shouldn't happen if validation is correct, but handle it gracefully
			continue
		}

		// Update coupon redeemCount
		couponFilter := bson.M{"_id": coupon.ID, "trash": false}
		couponUpdate := bson.M{
			"$set": bson.M{
				"redeemedCount": newRedeemedCount,
				"updatedAt":     time.Now(),
			},
		}
		_, err = s.couponRepo.Patch(ctx, couponFilter, couponUpdate)
		if err != nil {
			return fmt.Errorf("error updating coupon %s redeem count: %v", couponCode, err)
		}
	}

	return nil
}

func (s *invoicesvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid invoice id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.invoiceRepo.Patch(ctx, filter, update)
	return err
}

// normalizeDiscountType converts various discount type representations to standard enum
func (s *invoicesvcs) normalizeDiscountType(discountType string) (enums.DiscountType, error) {
	switch strings.ToLower(discountType) {
	case "percentage", "%", "percent":
		return enums.DiscountTypePercent, nil
	case "fixed", "amount":
		return enums.DiscountTypeFixed, nil
	default:
		return "", fmt.Errorf("unsupported discount type: %s", discountType)
	}
}

// CalculateProducts calculates product-level discounts and coupons only
// Returns subtotal after product discounts and coupons are applied
func (s *invoicesvcs) CalculateProducts(ctx context.Context, req *models.ProductCalculationRequest) (*models.ProductCalculationResponse, error) {
	if req == nil {
		return nil, errors.New("calculation request cannot be nil")
	}
	data := req.Products
	if len(data) == 0 {
		return nil, errors.New("no products provided for calculation")
	}

	var result models.ProductCalculationResponse
	var subTotal, productDiscount float64

	for _, product := range data {
		// Validate input
		if product.Price < 0 {
			return nil, errors.New("invalid product price")
		}

		// Initial product calculation
		productCalc := models.ProductCalculation{
			ProductID: product.Ref,
		}

		// Lookup product details
		var productObj *models.ProductObj
		if !product.Ref.IsZero() {
			var err error
			productObj, err = s.productLookup(ctx, product.ProductType, product.Ref)
			if err == nil {
				productCalc.ProductObj = productObj
			}
			// Continue even if lookup fails, but we prefer to have it
		}

		// Resolve Name if missing
		productName := product.Name
		if len(productName) == 0 && productObj != nil {
			if productObj.Course != nil {
				productName = productObj.Course.GetName()
			} else if productObj.Coaching != nil {
				productName = productObj.Coaching.GetName()
			} else if productObj.Bundle != nil {
				productName = productObj.Bundle.GetName()
			} else if productObj.PrintedMaterial != nil {
				productName = productObj.PrintedMaterial.GetName()
			}
		}
		productCalc.Name = productName

		// Resolve Price if missing (0)
		basePrice := product.OriginalPrice
		if basePrice == 0 && !product.Ref.IsZero() {
			cartType := s.mapProductTypeToCartType(product.ProductType)
			if cartType != "" {
				// For coaching, we might need extra data (NumOfSessions)
				priceRes, err := s.pricingSvcs.GetPrice(ctx, cartType, product.Ref, product.Data)
				if err == nil && priceRes != nil {
					basePrice = priceRes.Price
				} else {
					fmt.Printf("Warning: failed to fetch dynamic price for product %s: %v\n", product.Ref.Hex(), err)
				}
			}
		}
		productCalc.OriginalPrice = basePrice

		// Step 1: apply direct product discount (if any)
		directDiscount := 0.0
		if product.DiscountType != "" && product.DiscountAmount != nil {
			normalizedDiscountType, err := s.normalizeDiscountType(product.DiscountType)
			if err != nil {
				return nil, err
			}
			// Populate discount type and value in response
			productCalc.DiscountType = string(normalizedDiscountType)
			productCalc.DiscountValue = *product.DiscountAmount

			switch normalizedDiscountType {
			case enums.DiscountTypePercent:
				directDiscount = basePrice * (*product.DiscountAmount / 100)
			case enums.DiscountTypeFixed:
				directDiscount = *product.DiscountAmount
			}
			if directDiscount > basePrice {
				directDiscount = basePrice
			}
		}

		priceAfterDirectDiscount := basePrice - directDiscount

		// Step 2: Coupon processing (if coupon code is provided at product level)
		couponDiscount := 0.0
		if product.CouponCode != "" {
			coupon, err := s.lookupCoupon(ctx, product.CouponCode)
			if err != nil {
				return nil, fmt.Errorf("coupon lookup error: %v", err)
			}

			if coupon != nil {
				// Validate coupon applicability
				if !s.isCouponValid(coupon) {
					return nil, fmt.Errorf("coupon %s is not valid", product.CouponCode)
				}

				// If coupon is custom type, check if it's applicable to this specific product
				if coupon.Type == enums.CouponTypeCustom {
					isProductApplicable := s.isCouponApplicableToProduct(coupon, product.Ref)
					if !isProductApplicable {
						return nil, fmt.Errorf("coupon %s is not applicable to this product", product.CouponCode)
					}
				}
				// General coupons can be used on any product, no need to check product applicability

				// Normalize discount type
				normalizedDiscountType, err := s.normalizeDiscountType(string(coupon.DiscountType))
				if err != nil {
					return nil, err
				}

				// Calculate discount based on coupon type
				switch normalizedDiscountType {
				case enums.DiscountTypePercent:
					couponDiscount = priceAfterDirectDiscount * (coupon.DiscountValue / 100)
				case enums.DiscountTypeFixed:
					couponDiscount = coupon.DiscountValue
				default:
					return nil, fmt.Errorf("unsupported discount type: %v", coupon.DiscountType)
				}

				// Ensure discount doesn't exceed product price
				if couponDiscount > priceAfterDirectDiscount {
					couponDiscount = priceAfterDirectDiscount
				}

				// Populate coupon details
				productCalc.CouponCode = product.CouponCode
				productCalc.CouponCodeDiscount = couponDiscount
				productCalc.CouponDetails = &models.CouponDetails{
					Code:           coupon.Code,
					DiscountType:   string(normalizedDiscountType),
					DiscountValue:  coupon.DiscountValue,
					ApplicableType: string(coupon.AutoApplyType),
				}
			}
		}

		totalProductDiscount := directDiscount + couponDiscount
		if totalProductDiscount > basePrice {
			totalProductDiscount = basePrice
		}

		// Calculate final price
		finalPrice := basePrice - totalProductDiscount

		// Populate product calculation
		productCalc.DiscountAmount = totalProductDiscount
		productCalc.FinalPrice = finalPrice

		// Add to calculation result
		result.Products = append(result.Products, productCalc)

		// Update totals
		subTotal += basePrice
		productDiscount += totalProductDiscount
	}

	// Populate response
	result.OriginalTotalAmount = subTotal
	result.ProductDiscount = productDiscount
	result.SubTotal = subTotal - productDiscount
	if result.SubTotal < 0 {
		result.SubTotal = 0
	}

	return &result, nil
}

func (s *invoicesvcs) CalculateInvoiceTotal(ctx context.Context, req *models.InvoiceTotalCalculationRequest) (*models.InvoiceTotalCalculationResponse, error) {
	if req == nil {
		return nil, errors.New("calculation request cannot be nil")
	}

	var result models.InvoiceTotalCalculationResponse
	result.OriginalTotalAmount = req.SubTotal
	result.SubTotal = req.SubTotal
	result.Fees = req.Fees
	result.CouponCode = strings.TrimSpace(req.CouponCode)
	result.DiscountType = strings.TrimSpace(req.DiscountType)

	amountAfterProductDiscount := req.SubTotal
	totalDiscount := 0.0
	invoiceCouponDiscount := 0.0

	// Step 1: Apply general discount if provided
	if req.Discount > 0 && req.DiscountType != "" {
		normalizedDiscountType, err := s.normalizeDiscountType(req.DiscountType)
		if err != nil {
			return nil, err
		}

		var generalDiscount float64
		switch normalizedDiscountType {
		case enums.DiscountTypePercent:
			generalDiscount = amountAfterProductDiscount * (req.Discount / 100)
		case enums.DiscountTypeFixed:
			generalDiscount = req.Discount
		default:
			return nil, fmt.Errorf("unsupported discount type: %s", req.DiscountType)
		}

		if generalDiscount > amountAfterProductDiscount {
			generalDiscount = amountAfterProductDiscount
		}
		totalDiscount += generalDiscount
		amountAfterProductDiscount -= generalDiscount
		result.DiscountType = string(normalizedDiscountType)
		result.DiscountValue = req.Discount
	}

	// Step 2: Apply invoice-level coupon if provided
	if result.CouponCode != "" {
		coupon, err := s.lookupCoupon(ctx, result.CouponCode)
		if err != nil {
			return nil, fmt.Errorf("coupon lookup error: %v", err)
		}
		if coupon == nil {
			return nil, fmt.Errorf("coupon %s not found", result.CouponCode)
		}
		if !s.isCouponValid(coupon) {
			return nil, fmt.Errorf("coupon %s is not valid", result.CouponCode)
		}
		// Invoice-level coupon must be general type
		if coupon.Type != enums.CouponTypeGeneral {
			return nil, fmt.Errorf("coupon %s is not a general coupon and cannot be used at invoice level", result.CouponCode)
		}
		// Also check that it's not restricted to specific products (additional safety check)
		if len(coupon.Products) > 0 {
			return nil, fmt.Errorf("coupon %s is restricted to specific products and cannot be used as invoice-level coupon", result.CouponCode)
		}

		normalizedDiscountType, err := s.normalizeDiscountType(string(coupon.DiscountType))
		if err != nil {
			return nil, err
		}

		switch normalizedDiscountType {
		case enums.DiscountTypePercent:
			invoiceCouponDiscount = amountAfterProductDiscount * (coupon.DiscountValue / 100)
		case enums.DiscountTypeFixed:
			invoiceCouponDiscount = coupon.DiscountValue
		default:
			return nil, fmt.Errorf("unsupported invoice coupon discount type: %v", coupon.DiscountType)
		}
		if invoiceCouponDiscount > amountAfterProductDiscount {
			invoiceCouponDiscount = amountAfterProductDiscount
		}
		totalDiscount += invoiceCouponDiscount
		amountAfterProductDiscount -= invoiceCouponDiscount
	}

	// Ensure non-negative values
	if amountAfterProductDiscount < 0 {
		amountAfterProductDiscount = 0
	}

	// Calculate final total
	finalTotal := amountAfterProductDiscount + result.Fees

	// Populate response
	result.Discount = totalDiscount
	result.CouponCodeDiscount = invoiceCouponDiscount
	result.AmountAfterDiscount = amountAfterProductDiscount
	result.TotalAmount = finalTotal

	return &result, nil
}

// lookupCoupon retrieves a coupon by its code from the database
func (s *invoicesvcs) lookupCoupon(ctx context.Context, couponCode string) (*models.Coupon, error) {
	// Use the coupon repository to find the coupon by code
	filter := bson.M{
		"code":  couponCode,
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
		{
			"$lookup": bson.M{
				"from":         "academyCourses",
				"localField":   "coursesIds",
				"foreignField": "_id",
				"as":           "coursesData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "coachingProgramsIds",
				"foreignField": "_id",
				"as":           "coachingProgramsData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyBundles",
				"localField":   "bundlesIds",
				"foreignField": "_id",
				"as":           "bundlesData",
			},
		},
	}

	var result []models.Coupon
	err := s.couponRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, fmt.Errorf("error looking up coupon: %v", err)
	}

	if len(result) == 0 {
		return nil, nil
	}

	return &result[0], nil
}

// isCouponValid checks if a coupon is valid for use
func (s *invoicesvcs) isCouponValid(coupon *models.Coupon) bool {
	// Check coupon status (case-insensitive)
	if strings.ToLower(string(coupon.Status)) != "active" {
		return false
	}

	// Check expiration if not set to forever
	if !coupon.Forever {
		// Check if expiration date is set and not expired
		if coupon.ExpirationDate != nil && time.Now().After(*coupon.ExpirationDate) {
			return false
		}
	}

	// Check redemption limits
	if !coupon.InfiniteRedemption {
		// Ensure redemptions haven't exceeded total allowed
		if coupon.RedeemedCount >= coupon.TotalRedemptions {
			return false
		}
	}

	return true
}

// isCouponApplicableToProduct checks if the coupon can be applied to a specific product
func (s *invoicesvcs) isCouponApplicableToProduct(coupon *models.Coupon, productID primitive.ObjectID) bool {
	// For custom coupons, product must be explicitly in the Products list
	if coupon.Type == enums.CouponTypeCustom {
		// Custom coupons must have products defined
		if len(coupon.Products) == 0 {
			return false
		}
		// Check if the product is in the coupon's product list
		for _, id := range coupon.Products {
			if id == productID {
				return true
			}
		}
		return false
	}

	// For general coupons, if no specific products are defined, coupon is applicable to all
	if len(coupon.Products) == 0 {
		return true
	}

	// Check if the product is in the coupon's product list
	for _, id := range coupon.Products {
		if id == productID {
			return true
		}
	}

	return false
}
