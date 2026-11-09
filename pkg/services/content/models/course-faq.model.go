package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseFaq struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId     primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseFaqDto `bson:",inline"`
	CreatedBy    primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy    primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash        bool               `bson:"trash" json:"trash"`
}

func (c *CourseFaq) CollName() string {
	return "academyCourseFaqs"
}

type CourseFaqDto struct {
	Question transl.Localizable[string] `bson:"question" json:"question"`
	Answer   transl.Localizable[string] `bson:"answer" json:"answer"`
}
