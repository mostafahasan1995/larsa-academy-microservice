package models

import (
	contentModels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/sales/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Invoice represents a sales invoice
// Add more fields as needed per UI requirements

type Invoice struct {
	ID                  primitive.ObjectID         `bson:"_id,omitempty" json:"_id,omitempty"`
	InvoiceNum          string                     `bson:"invoiceNum" json:"invoiceNum"`
	OrderID             primitive.ObjectID         `bson:"orderId,omitempty" json:"orderId,omitempty"`
	CustomerID          primitive.ObjectID         `bson:"customerId" json:"customerId"`
	CustomerName        transl.Localizable[string] `bson:"-" json:"customerName"`
	OriginalTotalAmount float64                    `bson:"originalTotalAmount" json:"originalTotalAmount"`
	SubTotal            float64                    `bson:"subTotal" json:"subTotal"`
	TotalAmount         float64                    `bson:"totalAmount" json:"totalAmount"`
	Discount            float64                    `bson:"discount" json:"discount"`
	DiscountType        string                     `bson:"discountType" json:"discountType"`
	CouponCode          string                     `bson:"couponCode" json:"couponCode"`
	CouponCodeDiscount  float64                    `bson:"couponCodeDiscount" json:"couponCodeDiscount"`
	AmountAfterDiscount float64                    `bson:"amountAfterDiscount" json:"amountAfterDiscount"`
	Fees                float64                    `bson:"fees" json:"fees"`
	Currency            string                     `bson:"currency" json:"currency"`
	Notes               string                     `bson:"notes" json:"notes"`
	DueDate             time.Time                  `bson:"dueDate" json:"dueDate"`
	Status              enums.InvoiceStatus        `bson:"status" json:"status" oneof:"Paid,Pending,Canceled"`
	CreatedAt           time.Time                  `bson:"createdAt" json:"createdAt"`
	UpdatedAt           time.Time                  `bson:"updatedAt" json:"updatedAt"`
	CreatedBy           primitive.ObjectID         `bson:"createdBy" json:"createdBy"`
	UpdatedBy           primitive.ObjectID         `bson:"updatedBy" json:"updatedBy"`

	Products []InvoiceProduct `bson:"products" json:"products"`
	Trash    bool             `bson:"trash" json:"trash"`
}

func (i *Invoice) CollName() string {
	return "academySalesInvoices"
}

type InvoiceProduct struct {
	Name               transl.Localizable[string] `bson:"name" json:"name"`
	Price              float64                    `bson:"price" json:"price"`
	OriginalPrice      float64                    `bson:"originalPrice" json:"originalPrice"`
	CouponCode         string                     `bson:"couponCode" json:"couponCode"`
	CouponCodeDiscount float64                    `bson:"couponCodeDiscount" json:"couponCodeDiscount"`
	DiscountAmount     *float64                   `bson:"discountAmount" json:"discountAmount,omitempty"`
	DiscountType       string                     `bson:"discountType" json:"discountType"`
	DiscountValue      float64                    `bson:"discountValue" json:"discountValue,omitempty"`
	TotalAmount        float64                    `bson:"totalAmount" json:"totalAmount"`
	FinalPrice         float64                    `bson:"finalPrice" json:"finalPrice"`
	Currency           string                     `bson:"currency" json:"currency"`

	ProductType enums.ProductType  `bson:"productType" json:"productType"`
	Ref         primitive.ObjectID `bson:"productId" json:"productId"` // Reference to course, coaching, or bundle
	Data        any                `bson:"data" json:"data"`
	ProductObj  *ProductObj        `bson:"-" json:"productObj,omitempty"`
}

type ProductObj struct {
	Course          *contentModels.Course          `json:"course,omitempty"`
	Coaching        *contentModels.Coaching        `json:"coaching,omitempty"`
	Bundle          *contentModels.Bundle          `json:"bundle,omitempty"`
	PrintedMaterial *contentModels.PrintedMaterial `json:"printedMaterial,omitempty"`
}

// InvoicePagination for paged response

type InvoicePagination struct {
	Invoices   []Invoice        `bson:"invoices" json:"invoices"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

// ProductCalculationRequest for API 1: calculate products only
type ProductCalculationRequest struct {
	Products []*InvoiceProduct `json:"products"`
}

// ProductCalculationResponse for API 1: returns product calculations and subtotal
type ProductCalculationResponse struct {
	Products            []ProductCalculation `json:"products"`
	OriginalTotalAmount float64              `json:"originalTotalAmount"`
	ProductDiscount     float64              `json:"productDiscount"`
	SubTotal            float64              `json:"subTotal"`
}

// InvoiceTotalCalculationRequest for API 2: calculate invoice-level discount, coupon, fees
type InvoiceTotalCalculationRequest struct {
	SubTotal     float64 `json:"subTotal"`
	Discount     float64 `json:"discount"`
	DiscountType string  `json:"discountType"`
	CouponCode   string  `json:"couponCode"`
	Fees         float64 `json:"fees"`
}

// InvoiceTotalCalculationResponse for API 2: returns final invoice structure
type InvoiceTotalCalculationResponse struct {
	OriginalTotalAmount float64 `json:"originalTotalAmount"`
	SubTotal            float64 `json:"subTotal"`
	Discount            float64 `json:"discount"`
	DiscountType        string  `json:"discountType"`
	DiscountValue       float64 `json:"discountValue,omitempty"`
	CouponCode          string  `json:"couponCode,omitempty"`
	CouponCodeDiscount  float64 `json:"couponCodeDiscount"`
	AmountAfterDiscount float64 `json:"amountAfterDiscount"`
	Fees                float64 `json:"fees"`
	TotalAmount         float64 `json:"totalAmount"`
}

// Deprecated: Use ProductCalculationRequest and InvoiceTotalCalculationRequest instead
// type InvoiceCalculationRequest struct {
// 	Products   []*InvoiceProduct `json:"products"`
// 	Fees       float64           `json:"fees"`
// 	CouponCode string            `json:"couponCode"`
// 	Discount   float64           `json:"discount"`
// }

// ProductCalculation represents a detailed product calculation with coupon support
type ProductCalculation struct {
	ProductID          primitive.ObjectID         `json:"productId"`
	Name               transl.Localizable[string] `json:"name"`
	OriginalPrice      float64                    `json:"originalPrice"`
	DiscountType       string                     `json:"discountType,omitempty"`
	DiscountValue      float64                    `json:"discountValue,omitempty"`
	CouponCode         string                     `json:"couponCode,omitempty"`
	CouponCodeDiscount float64                    `json:"couponCodeDiscount"`
	CouponDetails      *CouponDetails             `json:"couponDetails,omitempty"`
	DiscountAmount     float64                    `json:"discountAmount"`
	FinalPrice         float64                    `json:"finalPrice"`
	ProductObj         *ProductObj                `json:"productObj,omitempty"`
}

// CouponDetails provides detailed information about applied coupon
type CouponDetails struct {
	Code           string  `json:"code"`
	DiscountType   string  `json:"discountType"`
	DiscountValue  float64 `json:"discountValue"`
	ApplicableType string  `json:"applicableType"`
}

// Deprecated: Use ProductCalculationResponse and InvoiceTotalCalculationResponse instead
type InvoiceTotalCalculation struct {
	Products            []ProductCalculation `json:"products"`
	OriginalTotalAmount float64              `json:"originalTotalAmount"`
	Discount            float64              `json:"discount"`
	CouponCode          string               `json:"couponCode,omitempty"`
	AmountAfterDiscount float64              `json:"amountAfterDiscount"`
	Fees                float64              `json:"fees"`
	TotalAmount         float64              `json:"totalAmount"`
}
