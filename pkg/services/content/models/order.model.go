package models

import (
	"fmt"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Order struct {
	Id          primitive.ObjectID   `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId   primitive.ObjectID   `bson:"studentId" json:"studentId"`
	Items       []CartItem           `bson:"items" json:"items"`
	CoachingIds []primitive.ObjectID `bson:"coachingIds" json:"-"`
	CourseIds   []primitive.ObjectID `bson:"courseIds" json:"-"`
	BundleIds   []primitive.ObjectID `bson:"bundleIds" json:"-"`
	MaterialIds []primitive.ObjectID `bson:"materialIds" json:"-"`
	TotalPrice  float64              `bson:"totalPrice" json:"totalPrice"`
	CouponCode  string               `bson:"couponCode,omitempty" json:"couponCode,omitempty"`
	Discount    float64              `bson:"discount,omitempty" json:"discount,omitempty"`
	InvoiceId   primitive.ObjectID   `bson:"invoiceId,omitempty" json:"invoiceId,omitempty"`
	Status      enums.OrderStatus    `bson:"status" json:"status"`       //pending - paid - failed - canceled - refunded
	IsRenewal   bool                 `bson:"isRenewal" json:"isRenewal"` //true - false
	CreatedBy   primitive.ObjectID   `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID   `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time            `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time            `bson:"updatedAt" json:"updatedAt"`
	Trash       bool                 `bson:"trash" json:"trash"`
	// Currency is filled from invoice when present (not persisted)
	Currency string `bson:"-" json:"currency,omitempty"`
}

func (o *Order) GetItems() []CartItem {
	return o.Items
}

func (o *Order) SetIds(coachingIds, courseIds, bundleIds, materialIds []primitive.ObjectID) {
	o.CoachingIds = coachingIds
	o.CourseIds = courseIds
	o.BundleIds = bundleIds
	o.MaterialIds = materialIds
}

func (o *Order) SortIds() {
	SortItemsIds(o)
}

func (o *Order) CollName() string {
	return "academyOrders"
}

type OrderPagination struct {
	Items      []Order           `json:"items"`
	Pagination *types.Pagination `json:"pagination"`
}

type PricingPlans struct {
	CoachingsPricings []CoachingPricing `bson:"coachingsPricings" json:"coachingsPricings"`
	CoursesPricings   []CoursePricing   `bson:"coursesPricings" json:"coursesPricings"`
	BundlesPricings   []BundlePricing   `bson:"bundlesPricings" json:"bundlesPricings"`
	MaterialsPricings []PrintedMaterial `bson:"materialsPricings" json:"materialsPricings"`
}

func (pp *PricingPlans) CalcPrices(items []CartItem) ([]CartItem, error) {
	coachingPricesMap := make(map[primitive.ObjectID]CoachingPricing)
	for _, p := range pp.CoachingsPricings {
		coachingPricesMap[p.CoachingId] = p
	}
	coursePricesMap := make(map[primitive.ObjectID]CoursePricing)
	for _, p := range pp.CoursesPricings {
		coursePricesMap[p.CourseId] = p
	}
	bundlePricesMap := make(map[primitive.ObjectID]BundlePricing)
	for _, p := range pp.BundlesPricings {
		bundlePricesMap[p.BundleId] = p
	}

	materialsPricesMap := make(map[primitive.ObjectID]PrintedMaterial)
	for _, p := range pp.MaterialsPricings {
		materialsPricesMap[p.Id] = p
	}

	var itemsWithPrice []CartItem
	for _, item := range items {
		switch item.Type {
		case enums.CartTypeCoaching:
			cs, err := util.Converter[*CoachingSubscribe](item.Data)
			if err != nil {
				return nil, fmt.Errorf("failed to get coaching subscribe: %w", err)
			}
			pricePaln, ok := coachingPricesMap[item.RefId]
			if !ok {
				item.Price = nil
				itemsWithPrice = append(itemsWithPrice, item)
				continue
			}
			price := pricePaln.PricePerSession * float64(cs.NumOfSessions)
			item.Price = &price
			itemsWithPrice = append(itemsWithPrice, item)
		case enums.CartTypeCourse:
			pricePlan, ok := coursePricesMap[item.RefId]
			if !ok {
				item.Price = nil
				itemsWithPrice = append(itemsWithPrice, item)
				continue
			}
			price := pricePlan.Price
			item.Price = &price
			itemsWithPrice = append(itemsWithPrice, item)
		case enums.CartTypeBundle:
			pricePlan, ok := bundlePricesMap[item.RefId]
			if !ok {
				item.Price = nil
				itemsWithPrice = append(itemsWithPrice, item)
				continue
			}
			price := pricePlan.Price
			item.Price = &price
			itemsWithPrice = append(itemsWithPrice, item)
		case enums.CartTypeMaterial:
			material, ok := materialsPricesMap[item.RefId]
			if !ok {
				item.Price = nil
				itemsWithPrice = append(itemsWithPrice, item)
				continue
			}
			price := material.Price
			item.Price = &price
			itemsWithPrice = append(itemsWithPrice, item)
		default:
			return nil, fmt.Errorf("invalid cart type")
		}
	}

	return itemsWithPrice, nil
}
