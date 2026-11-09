package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseCategoryPagination struct {
	Categories []CourseCategory `bson:"categories" json:"categories"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

type CourseCategory struct {
	Id                primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseCategoryDto `bson:",inline"`
	CreatedBy         primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy         primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt         time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt         time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash             bool               `bson:"trash" json:"trash"`
}

func (c *CourseCategory) CollName() string {
	return "academyCourseCategories"
}

type CourseCategoryDto struct {
	Label transl.Localizable[string] `bson:"label" json:"label" validate:"required"`
	Value string                     `bson:"value" json:"value" validate:"required"`
}
