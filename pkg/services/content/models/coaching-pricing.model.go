package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoachingPricing struct {
	Id                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingId         primitive.ObjectID `bson:"coachingId" json:"coachingId" validate:"required"`
	CoachingPricingDto `bson:",inline"`
	CreatedAt          time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt          time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy          primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy          primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash              bool               `bson:"trash" json:"trash"`
}

type CoachingPricingDto struct {
	Name             transl.Localizable[string] `bson:"name" json:"name" validate:"required"`
	SubscriptionType string                     `bson:"subscriptionType" json:"subscriptionType" validate:"required"` // free - paid - subscription
	PricePerSession  float64                    `bson:"pricePerSession" json:"pricePerSession" validate:"required"`
	Currency         string                     `bson:"currency" json:"currency" validate:"required"`
	Frequency        string                     `bson:"frequency" json:"frequency" `
	Active           bool                       `bson:"active" json:"active"`
}

func (cp *CoachingPricing) CollName() string {
	return "academyCoachingPricings"
}

type CoachingPricingPagination struct {
	CoachingPricings []CoachingPricing `json:"coachingPricings"`
	Pagination       types.Pagination  `json:"pagination"`
}
