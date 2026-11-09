package models

import (
	"larsa-academy-microservice/pkg/services/points/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Reward represents an item in the rewards catalog
type Reward struct {
	ID                primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Title             string             `bson:"title" json:"title"`
	Description       string             `bson:"description" json:"description"`
	Type              enums.RewardType   `bson:"type" json:"type"`
	PointsRequired    int                `bson:"pointsRequired" json:"pointsRequired"`
	QuantityAvailable int                `bson:"quantityAvailable,omitempty" json:"quantityAvailable,omitempty"` // -1 for unlimited
	Status            enums.RewardStatus `bson:"status" json:"status"`
	StartDate         *time.Time         `bson:"startDate,omitempty" json:"startDate,omitempty"`
	EndDate           *time.Time         `bson:"endDate,omitempty" json:"endDate,omitempty"`
	Image             types.FileField    `bson:"image,omitempty" json:"image,omitempty"`

	// For Coupon type reward
	CouponID primitive.ObjectID `bson:"couponId,omitempty" json:"couponId,omitempty"` // Link to sales coupon

	// For Course/Coaching Program type reward
	ProductID   primitive.ObjectID `bson:"productId,omitempty" json:"productId,omitempty"`     // Course or Coaching ID
	ProductType string             `bson:"productType,omitempty" json:"productType,omitempty"` // Course, Coaching, PrintedMaterials

	// Class restricts the reward to one or more student grades/classes (matches academyStudents.academicInfo.class)
	Class []string `bson:"class,omitempty" json:"class,omitempty"`

	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash     bool               `bson:"trash" json:"trash"`
}

func (r *Reward) CollName() string {
	return "academyRewards"
}

type RewardPagination struct {
	Rewards    []Reward         `bson:"rewards" json:"rewards"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
