package models

import (
	"larsa-academy-microservice/pkg/transl"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoachingTestDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Status      string                     `bson:"status" json:"status"`
	Session     string                     `bson:"session" json:"session"`
	Mark        int                        `bson:"mark" json:"mark"`
	DueDate     time.Time                  `bson:"dueDate" json:"dueDate"`
	Content     []Question                 `bson:"content" json:"content"`
	Settings    TestSettings               `bson:"settings" json:"settings"`
	AIAssistant QuizAIAssistant            `bson:"aiAssistant" json:"aiAssistant"`
}

type CoachingTest struct {
	Id              primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingTestDto `bson:",inline"`
	CoachingId      primitive.ObjectID `bson:"coachingId" json:"coachingId" validate:"required"`
	CreatedAt       time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt       time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy       primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy       primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash           bool               `bson:"trash" json:"trash"`
}

type CoachingTestSettings struct {
	InstantAnswerFeedback bool      `bson:"instantAnswerFeedback" json:"instantAnswerFeedback"`
	OpenAt                time.Time `bson:"openAt" json:"openAt"`
	CloseAt               time.Time `bson:"closeAt" json:"closeAt"`
	EnableTimeLimit       bool      `bson:"enableTimeLimit" json:"enableTimeLimit"`
	TimeLimit             string    `bson:"timeLimit" json:"timeLimit"`
	NumOfAttempts         int       `bson:"numOfAttempts" json:"numOfAttempts"`
	Calc                  string    `bson:"calc" json:"calc"` // avg - max - min - sum
}

type CoachingTestAISettings struct {
	Sections []primitive.ObjectID `bson:"sections" json:"sections"`
	Lessons  []primitive.ObjectID `bson:"lessons" json:"lessons"`
}

func (c *CoachingTest) CollName() string {
	return "academyCoachingTests"
}
