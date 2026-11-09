package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoursePricingDto struct {
	Plan                 string                     `bson:"plan" json:"plan"` // free - one-time payment - recurring payment - subscription - installments
	Price                float64                    `bson:"price" json:"price"`
	Currency             string                     `bson:"currency" json:"currency"`
	Name                 transl.Localizable[string] `bson:"name" json:"name"`
	Subtitle             transl.Localizable[string] `bson:"subtitle" json:"subtitle"`
	Description          transl.Localizable[string] `bson:"description" json:"description"`
	Config               PricingConfig              `bson:"config" json:"config"`
	RecurringSettings    RecurringSettings          `bson:"recurringSettings" json:"recurringSettings"`
	SubscriptionSettings SubscriptionSettings       `bson:"subscriptionSettings" json:"subscriptionSettings"`
	InstallmentsSettings InstallmentsSettings       `bson:"installmentsSettings" json:"installmentsSettings"`
}

type PricingConfig struct {
	IsDefault             bool      `bson:"isDefault" json:"isDefault"`
	InstantsCancellation  bool      `bson:"instantsCancellation" json:"instantsCancellation"`
	LimitedAccessDuration bool      `bson:"limitedAccessDuration" json:"limitedAccessDuration"`
	LimitedDurationType   string    `bson:"limitedDurationType" json:"limitedDurationType"` // e.g date or number of days
	Date                  time.Time `bson:"date" json:"date"`
	Num                   int       `bson:"num" json:"num"`
	Unit                  string    `bson:"unit" json:"unit"` //e.g days
}

type RecurringSettings struct {
	NumOfMonthlyPayments int `bson:"numOfMonthlyPayments" json:"numOfMonthlyPayments"`
}

type SubscriptionSettings struct {
	Frequency string `bson:"frequency" json:"frequency"`
}

type InstallmentsSettings struct {
	Installments int    `bson:"installments" json:"installments"`
	Recurrence   string `bson:"recurrence" json:"recurrence"`
}

type CoursePricing struct {
	Id               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId         primitive.ObjectID `bson:"courseId" json:"courseId"`
	CoursePricingDto `bson:",inline"`
	CreatedBy        primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy        primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt        time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt        time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash            bool               `bson:"trash" json:"trash"`
}

func (cp *CoursePricing) CollName() string {
	return "academyCoursePricings"
}

type CoursePricingPagination struct {
	CoursePricings []CoursePricing  `json:"coursePricings"`
	Pagination     types.Pagination `json:"pagination"`
}
