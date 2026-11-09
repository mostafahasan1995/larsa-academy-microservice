package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseStudyNote struct {
	Id                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId           primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseStudyNoteDto `bson:",inline"`
	CreatedBy          primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy          primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt          time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt          time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash              bool               `bson:"trash" json:"trash"`
}

type CourseStudyNoteDto struct {
	Name     transl.Localizable[string] `bson:"name" json:"name"`
	Category primitive.ObjectID         `bson:"category" json:"category"`
	Content  transl.Localizable[string] `bson:"content" json:"content"`
	Tags     []string                   `bson:"tags,omitempty" json:"tags,omitempty"`
}

func (c *CourseStudyNote) CollName() string {
	return "academyCourseStudyNotes"
}

type CourseStudyNoteCategory struct {
	Id                         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CouserId                   primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseStudyNoteCategoryDto `bson:",inline"`
	CreatedBy                  primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy                  primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt                  time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt                  time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash                      bool               `bson:"trash" json:"trash"`
}

type CourseStudyNoteCategoryDto struct {
	Name transl.Localizable[string] `bson:"name" json:"name"`
}

func (c *CourseStudyNoteCategory) CollName() string {
	return "academyCourseStudyNoteCategories"
}
