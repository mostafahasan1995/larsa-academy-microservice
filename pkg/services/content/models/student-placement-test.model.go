package models

import (
	"time"

	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// student placement tests
type StudentPlacementTest struct {
	Id              primitive.ObjectID                 `bson:"_id,omitempty" json:"_id,omitempty"`
	RefType         string                             `bson:"refType" json:"refType"` //course - coaching
	RefId           primitive.ObjectID                 `bson:"refId" json:"refId"`     // course - coaching id
	StudentId       primitive.ObjectID                 `bson:"studentId" json:"studentId"`
	ScheduleId      primitive.ObjectID                 `bson:"scheduleId,omitempty" json:"scheduleId,omitempty"`
	TeacherId       types.Nullable[primitive.ObjectID] `bson:"teacherId,omitempty" json:"teacherId,omitempty"`
	PlacementTestId primitive.ObjectID                 `bson:"placementTestId" json:"placementTestId"`
	Type            string                             `bson:"type" json:"type"` // electronic - online
	PlacementTest   PlacementTest                      `bson:"placementTest" json:"placementTest"`
	SubmitAt        time.Time                          `bson:"submitAt" json:"submitAt"`
	TotalTime       int                                `bson:"totalTime" json:"totalTime"`
	Status          enums.StudentPlacementTestStatus   `bson:"status" json:"status"` // pending - scheduled - rejected - completed - evaluated - cancelled
	Date            *time.Time                         `bson:"date,omitempty" json:"date,omitempty"`
	From            string                             `bson:"from,omitempty" json:"from,omitempty"` // 24 hour format
	To              string                             `bson:"to,omitempty" json:"to,omitempty"`     // 24 hour format
	SessionDate     *time.Time                         `bson:"sessionDate,omitempty" json:"sessionDate,omitempty"`
	StudentAnswers  []StudentAnswer                    `bson:"studentAnswers" json:"studentAnswers"`
	Score           float64                            `bson:"score" json:"score"`
	Result          string                             `bson:"result" json:"result"` // pass - fail
	SubmissionNum   int64                              `bson:"submissionNum" json:"submissionNum"`
	Notes           string                             `bson:"notes" json:"notes"`
	QuickIndecators []string                           `bson:"quickIndecators" json:"quickIndecators"`
	RequestedAt     time.Time                          `bson:"requestedAt,omitempty" json:"requestedAt,omitempty"`
	ApprovedBy      *primitive.ObjectID                `bson:"approvedBy,omitempty" json:"approvedBy,omitempty"`
	ApprovedAt      *time.Time                         `bson:"approvedAt,omitempty" json:"approvedAt,omitempty"`
	RejectedBy      *primitive.ObjectID                `bson:"rejectedBy,omitempty" json:"rejectedBy,omitempty"`
	RejectedAt      *time.Time                         `bson:"rejectedAt,omitempty" json:"rejectedAt,omitempty"`
	RejectionReason string                             `bson:"rejectionReason,omitempty" json:"rejectionReason,omitempty"`
	ZoomMeetingId   *primitive.ObjectID                `bson:"zoomMeetingId,omitempty" json:"zoomMeetingId,omitempty"`
	UpdatedAt       time.Time                          `bson:"updatedAt" json:"updatedAt"`
	Trash           bool                               `bson:"trash" json:"trash"`
}

func (s *StudentPlacementTest) CollName() string {
	return "academyStudentPlacementTests"
}

// StudentPlacementTestList is returned by GET /api/placement-tests/student/{placementTestId}
// to group the base placement test with the student's submissions/schedules.
type StudentPlacementTestList struct {
	PlacementTest PlacementTest          `json:"placementTest"`
	Submissions   []StudentPlacementTest `json:"submissions"`
}

type OnlinePlacementTestSchedule struct {
	Id                     primitive.ObjectID               `bson:"_id" json:"_id"`
	StudentId              primitive.ObjectID               `bson:"studentId" json:"-"`
	PlacementTestId        primitive.ObjectID               `bson:"placementTestId,omitempty" json:"placementTestId,omitempty"`
	StudentPlacementTestId primitive.ObjectID               `bson:"studentPlacementTestId,omitempty" json:"studentPlacementTestId,omitempty"`
	Status                 enums.StudentPlacementTestStatus `bson:"status" json:"status"`
	RequestedAt            time.Time                        `bson:"requestedAt,omitempty" json:"requestedAt,omitempty"`
	ApprovedBy             *primitive.ObjectID              `bson:"approvedBy,omitempty" json:"approvedBy,omitempty"`
	ApprovedAt             *time.Time                       `bson:"approvedAt,omitempty" json:"approvedAt,omitempty"`
	RejectedBy             *primitive.ObjectID              `bson:"rejectedBy,omitempty" json:"rejectedBy,omitempty"`
	RejectedAt             *time.Time                       `bson:"rejectedAt,omitempty" json:"rejectedAt,omitempty"`
	RejectionReason        string                           `bson:"rejectionReason,omitempty" json:"rejectionReason,omitempty"`

	ZoomMeetingId *primitive.ObjectID `bson:"zoomMeetingId,omitempty" json:"zoomMeetingId,omitempty"`

	OnlinePlacementTestScheduleDto `bson:",inline"`
}

type OnlinePlacementTestScheduleDto struct {
	PlacementTestId   primitive.ObjectID                 `bson:"placementTestId" json:"placementTestId" validate:"required"`
	TeacherId         types.Nullable[primitive.ObjectID] `bson:"teacherId,omitempty" json:"teacherId,omitempty"`
	CoachingProgramId primitive.ObjectID                 `bson:"coachingProgramId" json:"coachingProgramId"`
	Date              time.Time                          `bson:"date" json:"date"`
	From              string                             `bson:"from" json:"from"` // 24 hour format
	To                string                             `bson:"to" json:"to"`     // 24 hour format
}

// UpdatePlacementTestScheduleDto is used by the unified update endpoint.
// All fields are optional to support partial updates.
type UpdatePlacementTestScheduleDto struct {
	Status            enums.StudentPlacementTestStatus    `bson:"status,omitempty" json:"status,omitempty"`
	TeacherId         *types.Nullable[primitive.ObjectID] `bson:"teacherId,omitempty" json:"teacherId,omitempty"`
	CoachingProgramId primitive.ObjectID                  `bson:"coachingProgramId,omitempty" json:"coachingProgramId,omitempty"`
	Date              *time.Time                          `bson:"date,omitempty" json:"date,omitempty"`
	From              *string                             `bson:"from,omitempty" json:"from,omitempty"`
	To                *string                             `bson:"to,omitempty" json:"to,omitempty"`
	RejectionReason   *string                             `bson:"rejectionReason,omitempty" json:"rejectionReason,omitempty"`
}

func (o *OnlinePlacementTestSchedule) CollName() string {
	return "academyOnlinePlacementTestSchedules"
}

// student
type StudentPlacementTestAnswers struct {
	TeacherId       primitive.ObjectID `bson:"teacherId" json:"teacherId"`
	PlacementTestId primitive.ObjectID `bson:"placementTestId" json:"placementTestId" validate:"required"`
	Answers         []StudentAnswer    `bson:"answers" json:"answers" validate:"required"`
	TotalTime       int                `bson:"totalTime" json:"totalTime" ` //in minutes
}

type StudentAnswer struct {
	QuestionNum   int   `bson:"questionNum" json:"questionNum"`
	ChosenAnswers []int `bson:"chosenAnswers,omitempty" json:"chosenAnswers,omitempty"`
	ChosenAnswer  int   `bson:"ChosenAnswer,omitempty" json:"ChosenAnswer,omitempty"`
	IsCorrect     bool  `bson:"isCorrect" json:"isCorrect"`
}

// teacher
type TeacherEvaluation struct {
	Score           float64  `bson:"score" json:"score"`
	Notes           string   `bson:"notes" json:"notes"`
	QuickIndecators []string `bson:"quickIndecators" json:"quickIndecators"`
}

//schedule online test

// OnlinePlacementTestSchedulePagination is returned by the list schedules endpoint
type OnlinePlacementTestSchedulePagination struct {
	Schedules  []OnlinePlacementTestSchedule `json:"schedules"`
	Pagination types.Pagination              `json:"pagination"`
}

// StudentPlacementTestPagination is returned by the list all submissions endpoint
type StudentPlacementTestPagination struct {
	Submissions []StudentPlacementTest `json:"submissions"`
	Pagination  types.Pagination       `json:"pagination"`
}
