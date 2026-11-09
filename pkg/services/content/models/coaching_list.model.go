package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Lightweight placement test summary for coaching list responses.
type CoachingPlacementTest struct {
	Id             primitive.ObjectID         `bson:"_id" json:"_id"`
	Title          transl.Localizable[string] `bson:"title" json:"title"`
	Status         string                     `bson:"status" json:"status"`
	Type           string                     `bson:"type" json:"type"`
	TestMark       int                        `bson:"testMark" json:"testMark"`
	Settings       PlacementTestSettings      `bson:"settings" json:"settings"`
	QuestionsCount int                        `bson:"questionsCount" json:"questionsCount"`
	UpdatedAt      time.Time                  `bson:"updatedAt" json:"updatedAt"`
	StudentState   PlacementTestStudentState  `bson:"studentState,omitempty" json:"studentState,omitempty"`
}

// Per-student placement test state overlay.
type PlacementTestStudentState struct {
	HasSubmission bool                `bson:"hasSubmission" json:"hasSubmission"`
	SubmissionId  interface{}         `bson:"submissionId,omitempty" json:"submissionId,omitempty"`
	Score         interface{}         `bson:"score,omitempty" json:"score,omitempty"`
	Result        interface{}         `bson:"result,omitempty" json:"result,omitempty"`
	SubmissionNum interface{}         `bson:"submissionNum,omitempty" json:"submissionNum,omitempty"`
	Status        interface{}         `bson:"status,omitempty" json:"status,omitempty"`
	Scheduled     bool                `bson:"scheduled" json:"scheduled"`
	ScheduleDate  interface{}         `bson:"scheduleDate,omitempty" json:"scheduleDate,omitempty"`
	From          interface{}         `bson:"from,omitempty" json:"from,omitempty"`
	To            interface{}         `bson:"to,omitempty" json:"to,omitempty"`
	ZoomMeeting   *ZoomMeetingDetails `bson:"zoomMeeting,omitempty" json:"zoomMeeting,omitempty"`
}

// Coaching item for list responses with placement tests included.
type CoachingListItem struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingDto `bson:",inline"`
	CreatedBy   primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash       bool               `bson:"trash" json:"trash"`

	PlacementTests []CoachingPlacementTest `bson:"placementTests,omitempty" json:"placementTests,omitempty"`
}

type CoachingListPagination struct {
	Coachings  []CoachingListItem `json:"coachings"`
	Pagination types.Pagination   `json:"pagination"`
}

// Enriched coaching item for list responses with coach details
type EnrichedCoachingListItem struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingDto `bson:",inline"`
	CreatedBy   primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash       bool               `bson:"trash" json:"trash"`

	CoachsObjs     []EnrichedCoach         `bson:"coachsObjs,omitempty" json:"coachsObjs,omitempty"` // Enriched coach objects
	PlacementTests []CoachingPlacementTest `bson:"placementTests,omitempty" json:"placementTests,omitempty"`
	Pricing        *CoachingPricing        `bson:"pricing,omitempty" json:"pricing,omitempty"`
}

type StudentCoachingResponse struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingDto `bson:",inline"`
	CreatedBy   primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash       bool               `bson:"trash" json:"trash"`

	CoachsObjs     []EnrichedCoach         `bson:"coachsObjs,omitempty" json:"coachsObjs,omitempty"` // Enriched coach objects
	PlacementTests []CoachingPlacementTest `bson:"placementTests,omitempty" json:"placementTests,omitempty"`
	Pricing        *CoachingPricing        `bson:"pricing,omitempty" json:"pricing,omitempty"`
	IsFavorite     bool                    `json:"isFavorite"`
	IsEnrolled     bool                    `json:"isEnrolled"`
}

type StudentCoachingPagination struct {
	Coachings  []StudentCoachingResponse `json:"coachings"`
	Pagination types.Pagination          `json:"pagination"`
}

type EnrichedCoachingListPagination struct {
	Coachings  []EnrichedCoachingListItem `json:"coachings"`
	Pagination types.Pagination           `json:"pagination"`
}
