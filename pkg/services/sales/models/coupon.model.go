package models

import (
	contentModels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/sales/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Coupon struct {
	ID                  primitive.ObjectID   `bson:"_id,omitempty" json:"_id,omitempty"`
	Code                string               `bson:"code" json:"code"`
	Status              enums.CouponStatus   `bson:"status" json:"status"`
	Type                enums.CouponType     `bson:"type" json:"type"` // general or custom
	InfiniteRedemption  bool                 `bson:"infiniteRedemption" json:"infiniteRedemption"`
	TotalRedemptions    int                  `bson:"totalRedemptions" json:"totalRedemptions"`
	RedeemedCount       int                  `bson:"redeemedCount" json:"redeemedCount"`
	AutoApplyType       enums.AutoApplyType  `bson:"autoApplyType" json:"autoApplyType"`
	Products            []primitive.ObjectID `bson:"products" json:"products"`
	CoursesIds          []primitive.ObjectID `bson:"coursesIds" json:"coursesIds"`
	CoachingProgramsIds []primitive.ObjectID `bson:"coachingProgramsIds" json:"coachingProgramsIds"`
	BundlesIds          []primitive.ObjectID `bson:"bundlesIds" json:"bundlesIds"`
	DiscountType        enums.DiscountType   `bson:"discountType" json:"discountType"`
	DiscountValue       float64              `bson:"discountValue" json:"discountValue"`
	Currency            string               `bson:"currency" json:"currency"`
	ExpirationDate      *time.Time           `bson:"expirationDate,omitempty" json:"expirationDate,omitempty"`
	Forever             bool                 `bson:"forever" json:"forever"`
	CreatedAt           time.Time            `bson:"createdAt" json:"createdAt"`
	UpdatedAt           time.Time            `bson:"updatedAt" json:"updatedAt"`
	CreatedBy           primitive.ObjectID   `bson:"createdBy" json:"createdBy"`
	UpdatedBy           primitive.ObjectID   `bson:"updatedBy" json:"updatedBy"`
	Trash               bool                 `bson:"trash" json:"trash"`

	// Populated fields
	CoursesData          []contentModels.Course   `bson:"-" json:"coursesData,omitempty"`
	CoachingProgramsData []contentModels.Coaching `bson:"-" json:"coachingProgramsData,omitempty"`
	BundlesData          []contentModels.Bundle   `bson:"-" json:"bundlesData,omitempty"`
}

func (c *Coupon) CollName() string {
	return "academySalesCoupons"
}

type CouponPagination struct {
	Coupons    []Coupon         `bson:"coupons" json:"coupons"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
