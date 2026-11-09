package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseStudentTest struct {
	Id                  primitive.ObjectID         `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId            primitive.ObjectID         `bson:"courseId" json:"courseId"`
	TestId              primitive.ObjectID         `bson:"testId" json:"testId"`
	StudentId           primitive.ObjectID         `bson:"studentId" json:"studentId"`
	Title               transl.Localizable[string] `bson:"title" json:"title"`
	Mark                float64                    `bson:"mark" json:"mark"`
	Questions           []Question                 `bson:"questions" json:"questions"`
	Settings            TestSettings               `bson:"settings" json:"settings"`
	Answers             []CourseStudentTestAnswer  `bson:"answers" json:"answers"`
	Grade               float64                    `bson:"grade" json:"grade"`
	NumOfCorrectAnswers int                        `bson:"numOfCorrectAnswers" json:"numOfCorrectAnswers"`
	Status              string                     `bson:"status" json:"status"` // pending - submitted - timed_out
	StartTime           time.Time                  `bson:"startTime" json:"startTime"`
	SubmissionDate      time.Time                  `bson:"submissionDate" json:"submissionDate"`
	CreatedAt           time.Time                  `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt           time.Time                  `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy           primitive.ObjectID         `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy           primitive.ObjectID         `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash               bool                       `bson:"trash" json:"trash"`
}

type CourseStudentTestAnswer struct {
	QuestionNum   int   `bson:"questionNum" json:"questionNum"`
	ChosenAnswers []int `bson:"chosenAnswers" json:"chosenAnswers"`
}

type CourseTestWithAttempt struct {
	Id       primitive.ObjectID         `json:"_id"`
	Title    transl.Localizable[string] `json:"title"`
	Mark     float64                    `json:"mark"`
	DueDate  time.Time                  `json:"dueDate"`
	Settings TestSettings               `json:"settings"`
	Attempt  *CourseStudentTest         `json:"attempt,omitempty"`
}

type StudentCourseTestAnswersDto struct {
	Answers []CourseStudentTestAnswer `json:"answers" validate:"required"`
}

type CourseStudentTestRes struct {
	CourseStudentTest `bson:",inline"`
	FirstName         transl.Localizable[string] `json:"firstName"`
	LastName          transl.Localizable[string] `json:"lastName"`
	Email             string                     `json:"email"`
	Picture           string                     `json:"picture"`
	Class             string                     `json:"class"`
}

func (c *CourseStudentTest) CollName() string {
	return "academyCourseStudentTests"
}
