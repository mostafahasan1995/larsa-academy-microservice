package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TestSettings struct {
	RecordGrads     bool      `bson:"recordGrads" json:"recordGrads"`
	InstantFeedback bool      `bson:"instantFeedback" json:"instantFeedback"`
	StudentComments bool      `bson:"studentComments" json:"studentComments"`
	PublicReview    bool      `bson:"publicReview" json:"publicReview"`
	OpenAt          time.Time `bson:"openAt" json:"openAt"`
	CloseAt         time.Time `bson:"closeAt" json:"closeAt"`
	EnableTimeLimit bool      `bson:"enableTimeLimit" json:"enableTimeLimit"`
	TimeLimit       string    `bson:"timeLimit" json:"timeLimit"`
}

type CourseTestDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Status      string                     `bson:"status" json:"status" validate:"required,oneof=draft active"`
	Mark        float64                    `bson:"mark" json:"mark"`
	DueDate     time.Time                  `bson:"dueDate" json:"dueDate"`
	Questions   []Question                 `bson:"content" json:"content"`
	AiAssistant QuizAIAssistant            `bson:"aiAssistant" json:"aiAssistant"`
	Settings    TestSettings               `bson:"settings" json:"settings"`
}

type CourseTest struct {
	Id            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseId      primitive.ObjectID `bson:"courseId" json:"courseId"`
	CourseTestDto `bson:",inline"`
	CreatedBy     primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy     primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt     time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash         bool               `bson:"trash" json:"trash"`
}

func (c *CourseTest) CollName() string {
	return "academyCourseTests"
}
