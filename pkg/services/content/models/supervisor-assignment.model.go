package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SupervisorAssignment struct {
	Id                      primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	SupervisorAssignmentDto `bson:",inline"`
	CreatedAt               time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt               time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy               primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy               primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
}

func (s *SupervisorAssignment) CollName() string {
	return "academySupervisorAssignments"
}

type SupervisorAssignmentDto struct {
	SupervisorId    primitive.ObjectID   `bson:"supervisorId" json:"supervisorId"`
	AssignType      string               `bson:"assignType" json:"assignType" validate:"required,oneof=direct dynamic"` // "direct" or "dynamic"
	RefType         string               `bson:"refType" json:"refType" validate:"required,oneof=subject teacher"`      // "teacher" or "subject"
	RefIds          []primitive.ObjectID `bson:"refIds" json:"refIds"`                                                  // Teacher IDs or Subject IDs
	SchoolType      string               `bson:"schoolType" json:"schoolType"`
	Country         []string             `bson:"country" json:"country"`
	Level           []string             `bson:"level" json:"level"`
	MatchedTeachers []primitive.ObjectID `bson:"matchedTeachers,omitempty" json:"matchedTeachers,omitempty"`      // Teachers assigned to courses/coachings matching this assignment (for dynamic subject assignments)
	Status          string               `bson:"status" json:"status" validate:"required,oneof=pending assigned"` // "assigned", "pending"
	Trash           bool                 `bson:"trash" json:"trash"`
}

// Validation rules:
// - When assignType is "dynamic", refType MUST be "subject"
// - When assignType is "direct", refType MUST be "teacher"

type SupervisorAssignmentWithDetails struct {
	SupervisorAssignment `bson:",inline"`
	SupervisorObj        interface{}   `bson:"supervisorObj,omitempty" json:"supervisorObj,omitempty"`
	RefObjs              []interface{} `bson:"refObjs,omitempty" json:"refObjs,omitempty"`                   // Teachers or Subjects
	AffectedTeachers     int           `bson:"affectedTeachers,omitempty" json:"affectedTeachers,omitempty"` // For dynamic
}

type SupervisorAssignmentPagination struct {
	Assignments []SupervisorAssignmentWithDetails `json:"assignments"`
	Pagination  types.Pagination                  `json:"pagination"`
}
