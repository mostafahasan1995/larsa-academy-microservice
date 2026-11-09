package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoachingStudentTest struct {
	Id                  primitive.ObjectID         `bson:"_id" json:"_id"`
	CoachingId          primitive.ObjectID         `bson:"coachingId" json:"coachingId"`
	TestId              primitive.ObjectID         `bson:"testId" json:"testId"`
	StudentId           primitive.ObjectID         `bson:"studentId" json:"studentId"`
	Title               transl.Localizable[string] `bson:"title" json:"title"`
	Session             string                     `bson:"session" json:"session"`
	Mark                int                        `bson:"mark" json:"mark"`
	Content             []Question                 `bson:"content" json:"content"`
	Answers             []CoachingTestAnswer       `bson:"answers" json:"answers"`
	DueDate             time.Time                  `bson:"dueDate" json:"dueDate"`
	Grade               int                        `bson:"grade" json:"grade"`
	NumOfCorrectAnswers int                        `bson:"numOfCorrectAnswers" json:"numOfCorrectAnswers"`
	Status              string                     `bson:"status" json:"status"` //pending - submitted
	ReviewStatus        *string                    `bson:"reviewStatus,omitempty" json:"reviewStatus,omitempty"`
	SubmissionDate      time.Time                  `bson:"submissionDate" json:"submissionDate"`
	Trash               bool                       `bson:"trash" json:"trash"`
}

func (c *CoachingStudentTest) CollName() string {
	return "academyCoachingStudentTests"
}

type CoachingTestAnswer struct {
	QuestionNum  int `bson:"questionNum" json:"questionNum"`
	ChosenAnswer int `bson:"chosenAnswer" json:"chosenAnswer"`
}

type StudentCoachingTestAnswers struct {
	Answers []CoachingTestAnswer `bson:"answers" json:"answers" validate:"required"`
}

type CoachingStudentTestRes struct {
	CoachingStudentTest `bson:",inline"`
	FirstName           transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName            transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email               string                     `bson:"email" json:"email"`
}
