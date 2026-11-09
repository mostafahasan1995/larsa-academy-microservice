package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoachingAssignmentDto struct {
	Title   transl.Localizable[string] `bson:"title" json:"title"`
	Status  string                     `bson:"status" json:"status"`
	Session string                     `bson:"session" json:"session"`
	Mark    int                        `bson:"mark" json:"mark"`
	DueDate time.Time                  `bson:"dueDate" json:"dueDate"`
	Content []Question                 `bson:"content" json:"content"`
}

type CoachingAssignment struct {
	Id                    primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingAssignmentDto `bson:",inline"`
	CoachingId            primitive.ObjectID `bson:"coachingId" json:"coachingId" validate:"required"`
	CreatedAt             time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt             time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy             primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy             primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash                 bool               `bson:"trash" json:"trash"`
}

func (c *CoachingAssignment) CollName() string {
	return "academyCoachingAssignments"
}

type CoachingAssignmentPagination struct {
	CoachingAssignments []CoachingAssignment `json:"coachingAssignments"`
	Pagination          types.Pagination     `json:"pagination"`
}

//student assignment

type StudentAssignmentAnswers struct {
	AssignmentId primitive.ObjectID `bson:"assignmentId" json:"assignmentId" validate:"required"`
}

type CoachingAssignmentWithAttempt struct {
	CoachingAssignment `bson:",inline"`
	Attempt            *CoachingStudentAssignment `json:"attempt"`
}
