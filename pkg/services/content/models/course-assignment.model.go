package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseAssignment struct {
	Id                  primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId            primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseAssignmentDto `bson:",inline"`
	CreatedBy           primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy           primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt           time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt           time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash               bool               `bson:"trash" json:"trash"`
}

func (c *CourseAssignment) CollName() string {
	return "academyCourseAssignments"
}

type CourseAssignmentDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Deadline    time.Time                  `bson:"deadline" json:"deadline"`
	Subject     string                     `bson:"subject" json:"subject"`
	Chapter     string                     `bson:"chapter" json:"chapter"`
	Status      string                     `bson:"status" json:"status"`
	FileUpload  []types.FileField          `bson:"file_upload" json:"file_upload"`
}
