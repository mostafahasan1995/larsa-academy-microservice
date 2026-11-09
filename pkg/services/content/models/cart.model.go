package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CartItemDto struct {
	Type  enums.CartType     `bson:"type" json:"type" validate:"required,oneof=coaching course bundle material"` // coaching - course - bundle - material
	RefId primitive.ObjectID `bson:"refId" json:"refId" validate:"required"`                                     //coachingProgramId - courseId - bundleId - materialId
	Data  any                `bson:"data" json:"data"`
}

type CartItem struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CartItemDto `bson:",inline"`
	Price       *float64       `bson:"price,omitempty" json:"price"`
	Currency    string         `bson:"currency,omitempty" json:"currency"`
	Meta        map[string]any `bson:"meta" json:"meta"`
	CreatedAt   time.Time      `bson:"createdAt" json:"createdAt"`
}

type Cart struct {
	Id          primitive.ObjectID   `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId   primitive.ObjectID   `bson:"studentId" json:"studentId"`
	Items       []CartItem           `bson:"items" json:"items"`
	CoachingIds []primitive.ObjectID `bson:"coachingIds" json:"-"`
	CourseIds   []primitive.ObjectID `bson:"courseIds" json:"-"`
	BundleIds   []primitive.ObjectID `bson:"bundleIds" json:"-"`
	MaterialIds []primitive.ObjectID `bson:"materialIds" json:"-"`
	TotalPrice  float64              `bson:"-" json:"totalPrice"`
	Discount    float64              `bson:"-" json:"discount"`
	CouponCode  string               `bson:"couponCode,omitempty" json:"couponCode"`
	Currency    string               `bson:"-" json:"currency"`
}

func (c *Cart) GetItems() []CartItem {
	return c.Items
}

func (c *Cart) SetIds(coachingIds, courseIds, bundleIds, materialIds []primitive.ObjectID) {
	c.CoachingIds = coachingIds
	c.CourseIds = courseIds
	c.BundleIds = bundleIds
	c.MaterialIds = materialIds
}

func (c *Cart) SortIds() {
	SortItemsIds(c)
}

func (c *Cart) CollName() string {
	return "academyCart"
}
