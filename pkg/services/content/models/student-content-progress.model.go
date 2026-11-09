package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentContentProgress struct {
	Id                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId          primitive.ObjectID `bson:"studentId" json:"studentId"`
	CourseId           primitive.ObjectID `bson:"courseId" json:"courseId"`
	LessonId           primitive.ObjectID `bson:"lessonId" json:"lessonId"`
	ContentId          primitive.ObjectID `bson:"contentId" json:"contentId"`
	CurrentTimeSeconds int                `bson:"currentTimeSeconds" json:"currentTimeSeconds"`
	UpdatedAt          time.Time          `bson:"updatedAt" json:"updatedAt"`
	Trash              bool               `bson:"trash" json:"trash"`
}

func (s *StudentContentProgress) CollName() string {
	return "academyStudentContentProgress"
}

type StudentContentProgressPagination struct {
	Progress   []StudentContentProgress `json:"progress"`
	Pagination types.Pagination         `json:"pagination"`
}
