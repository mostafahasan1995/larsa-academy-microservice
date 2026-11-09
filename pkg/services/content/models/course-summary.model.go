package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseSummary struct {
	Id               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId         primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseSummaryDto `bson:",inline"`
	Trash            bool               `bson:"trash" json:"trash"`
	CreatedBy        primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy        primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt        time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt        time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
}

func (c *CourseSummary) CollName() string {
	return "academyCourseSummaries"
}

type CourseSummaryDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Status      string                     `bson:"status" json:"status"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Content     []Content                  `bson:"content" json:"content"`
}
