package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ItemsHolder interface {
	GetItems() []CartItem
	SetIds(coachingIds, courseIds, bundleIds, materialIds []primitive.ObjectID)
}

func SortItemsIds(h ItemsHolder) {
	items := h.GetItems()
	coachingIds := []primitive.ObjectID{}
	courseIds := []primitive.ObjectID{}
	bundleIds := []primitive.ObjectID{}
	materialIds := []primitive.ObjectID{}
	for _, item := range items {
		switch item.Type {
		case enums.CartTypeCoaching:
			coachingIds = append(coachingIds, item.RefId)
		case enums.CartTypeCourse:
			courseIds = append(courseIds, item.RefId)
		case enums.CartTypeBundle:
			bundleIds = append(bundleIds, item.RefId)
		case enums.CartTypeMaterial:
			materialIds = append(materialIds, item.RefId)
		}
	}

	h.SetIds(coachingIds, courseIds, bundleIds, materialIds)
}

// MediaItem represents a single media element (image or video) for marketing galleries
type MediaItem struct {
	Type  string         `bson:"type" json:"type"`   // image, video
	File  types.FileField `bson:"file" json:"file"`  // underlying stored file
	Order int            `bson:"order" json:"order"` // optional ordering within gallery
}
