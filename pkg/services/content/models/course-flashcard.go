package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseFlashcardDto struct {
	Name     transl.Localizable[string] `bson:"name" json:"name"`
	Category primitive.ObjectID         `bson:"category" json:"category"`
	Question transl.Localizable[string] `bson:"question" json:"question"`
	Answer   transl.Localizable[string] `bson:"answer" json:"answer"`
}

type CourseFlashcard struct {
	Id                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId           primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseFlashcardDto `bson:",inline"`
	CreatedBy          primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy          primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt          time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt          time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash              bool               `bson:"trash" json:"trash"`
}

func (c *CourseFlashcard) CollName() string {
	return "academyCourseFlashcards"
}

// category
type CourseFlashcardCategory struct {
	Id                         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CouserId                   primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseFlashcardCategoryDto `bson:",inline"`
	CreatedBy                  primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy                  primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt                  time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt                  time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash                      bool               `bson:"trash" json:"trash"`
}

type CourseFlashcardCategoryDto struct {
	Name transl.Localizable[string] `bson:"name" json:"name"`
}

func (c *CourseFlashcardCategory) CollName() string {
	return "academyCourseFlashcardCategories"
}
