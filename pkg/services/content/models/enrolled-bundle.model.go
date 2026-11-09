package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EnrolledBundleResponse represents a bundle with enriched enrollment data
type EnrolledBundleResponse struct {
	Bundle      BundleEnriched `bson:"bundle" json:"bundle"`
	Enrollment  StudentBundle  `bson:"enrollment" json:"enrollment"`
	Progress    int            `bson:"progress" json:"progress"` // Total progress of bundle content (0-100)
	ContentInfo []ContentInfo  `bson:"contentInfo" json:"contentInfo"` // Progress info for each content item
}

// BundleEnriched contains full bundle details
type BundleEnriched struct {
	Id          primitive.ObjectID         `bson:"_id" json:"_id"`
	Name        transl.Localizable[string] `bson:"name" json:"name"`
	Status      string                     `bson:"status" json:"status"`
	Thumbnail   types.FileField            `bson:"thumbnail" json:"thumbnail"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Content     []BundleContent            `bson:"content" json:"content"`
	CreatedAt   time.Time                  `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time                  `bson:"updatedAt" json:"updatedAt"`
}

// ContentInfo contains progress information for a bundle content item
type ContentInfo struct {
	Type     string                     `bson:"type" json:"type"` // course or coaching
	RefId    primitive.ObjectID         `bson:"refId" json:"refId"`
	Progress int                        `bson:"progress" json:"progress"` // Progress percentage (0-100)
	Name     transl.Localizable[string] `bson:"name,omitempty" json:"name,omitempty"`
}

// EnrolledBundlePagination wraps paginated enrolled bundles
type EnrolledBundlePagination struct {
	Bundles    []EnrolledBundleResponse `bson:"bundles" json:"bundles"`
	Pagination types.Pagination        `bson:"pagination" json:"pagination"`
}

