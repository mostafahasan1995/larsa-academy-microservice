package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EnrolledCoachingResponse represents a coaching with enriched enrollment data
type EnrolledCoachingResponse struct {
	Coaching          CoachingEnriched   `bson:"coaching" json:"coaching"`
	Progress          int                `bson:"progress" json:"progress"`
	NumOfSessions     int                `bson:"numOfSessions" json:"numOfSessions"`
	CompletedSessions int                `bson:"completedSessions" json:"completedSessions"`
	StartDate         time.Time          `bson:"startDate" json:"startDate"`
	LastSessionDate   time.Time          `bson:"lastSessionDate" json:"lastSessionDate"`
	NextSession       *StudentSession    `bson:"nextSession,omitempty" json:"nextSession,omitempty"`
	UpcomingSessions  []StudentSession   `bson:"upcomingSessions" json:"upcomingSessions"`
	RecentSessions    []StudentSession   `bson:"recentSessions" json:"recentSessions"`
	Teachers          []TeacherBasicInfo `bson:"teachers" json:"teachers"`
}

// CoachingEnriched contains full coaching details
type CoachingEnriched struct {
	Id              primitive.ObjectID         `bson:"_id" json:"_id"`
	Title           transl.Localizable[string] `bson:"title" json:"title"`
	Description     transl.Localizable[string] `bson:"description" json:"description"`
	Thumbnail       types.FileField            `bson:"thumbnail" json:"thumbnail"`
	ProgramType     string                     `bson:"programType" json:"programType"`
	Level           string                     `bson:"level" json:"level"`
	Class           string                     `bson:"class" json:"class"`
	SchoolType      string                     `bson:"schoolType" json:"schoolType"`
	SchoolId        primitive.ObjectID         `bson:"schoolId" json:"schoolId"`
	SchoolObj       *School                    `bson:"schoolObj,omitempty" json:"schoolObj,omitempty"`
	Country         string                     `bson:"country" json:"country"`
	MinSessionNum   int                        `bson:"minSessionNum" json:"minSessionNum"`
	MaxSessionNum   int                        `bson:"maxSessionNum" json:"maxSessionNum"`
	SessionDuration int                        `bson:"sessionDuration" json:"sessionDuration"`
	CreatedAt       time.Time                  `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time                  `bson:"updatedAt" json:"updatedAt"`
}

// EnrolledCoachingPagination wraps paginated enrolled coachings
type EnrolledCoachingPagination struct {
	Coachings  []EnrolledCoachingResponse `bson:"coachings" json:"coachings"`
	Pagination types.Pagination           `bson:"pagination" json:"pagination"`
}
