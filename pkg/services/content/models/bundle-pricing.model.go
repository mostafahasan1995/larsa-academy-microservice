package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BundlePricing struct {
	Id               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	BundleId         primitive.ObjectID `bson:"bundleId" json:"bundleId"`
	BundlePricingDto `bson:",inline"`
	CreatedBy        primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy        primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt        time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt        time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash            bool               `bson:"trash" json:"trash"`
}

func (bp *BundlePricing) CollName() string {
	return "academyBundlePricings"
}

type BundlePricingDto struct {
	PlanType    string                     `bson:"planType" json:"planType"` // free - one-time payment - recurring payment - subscription - installments
	Name        transl.Localizable[string] `bson:"name" json:"name"`
	Price       float64                    `bson:"price" json:"price"`
	Currency    string                     `bson:"currency" json:"currency"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Active      bool                       `bson:"active" json:"active"`
}

type BundlePricingPagination struct {
	BundlePricings []BundlePricing  `json:"bundlePricings"`
	Pagination     types.Pagination `json:"pagination"`
}
