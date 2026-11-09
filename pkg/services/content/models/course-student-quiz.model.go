package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseStudentQuiz struct {
	Id                  primitive.ObjectID         `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId           primitive.ObjectID         `bson:"studentId" json:"studentId"`
	CourseId            primitive.ObjectID         `bson:"courseId" json:"courseId"`
	Title               transl.Localizable[string] `bson:"title" json:"title"`
	Questions           []Question                 `bson:"questions" json:"questions"`
	Answers             []any                      `bson:"answers" json:"answers"`
	Grade               int                        `bson:"grade" json:"grade"`
	Result              string                     `bson:"result" json:"result"` // pass - fail
	Status              string                     `bson:"status" json:"status"` // pending - completed - failed
	NumOfCorrectAnswers int                        `bson:"numOfCorrectAnswers" json:"numOfCorrectAnswers"`
	SubmissionDate      time.Time                  `bson:"submissionDate" json:"submissionDate"`
	CreatedAt           time.Time                  `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt           time.Time                  `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy           primitive.ObjectID         `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy           primitive.ObjectID         `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash               bool                       `bson:"trash" json:"trash"`
}

func (c *CourseStudentQuiz) CollName() string {
	return "academyCourseStudentQuizzes"
}
