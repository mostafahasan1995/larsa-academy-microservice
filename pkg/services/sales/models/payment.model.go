package models

import (
	"larsa-academy-microservice/pkg/services/sales/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Payment struct {
	ID            primitive.ObjectID  `bson:"_id,omitempty" json:"_id,omitempty"`
	InvoiceID     primitive.ObjectID  `bson:"invoiceId" json:"invoiceId"`
	InvoiceNum    string              `bson:"invoiceNum" json:"invoiceNum"`
	CustomerId    primitive.ObjectID  `bson:"customerId" json:"customerId"`
	Amount        float64             `bson:"amount" json:"amount"`
	Status        enums.PaymentStatus `bson:"status" json:"status"`
	PaymentType   enums.PaymentType   `bson:"paymentType" json:"paymentType"`
	PaymentMethod string              `bson:"paymentMethod" json:"paymentMethod"`
	DateTime      time.Time           `bson:"dateTime" json:"dateTime"`
	CreatedAt     time.Time           `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time           `bson:"updatedAt" json:"updatedAt"`
	CreatedBy     primitive.ObjectID  `bson:"createdBy" json:"createdBy"`
	UpdatedBy     primitive.ObjectID  `bson:"updatedBy" json:"updatedBy"`
	Trash         bool                `bson:"trash" json:"trash"`
}

func (p *Payment) CollName() string {
	return "academySalesPayments"
}

type PaymentPagination struct {
	Payments   []Payment        `bson:"payments" json:"payments"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
