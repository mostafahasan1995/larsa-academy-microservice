package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TeacherAssignment struct {
	Id                   primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TeacherAssignmentDto `bson:",inline"`
	CreatedAt            time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt            time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy            primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy            primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash                bool               `bson:"trash" json:"trash"`
}

func (t *TeacherAssignment) CollName() string {
	return "academyTeacherAssignments"
}

type TeacherAssignmentDto struct {
	Type       string               `bson:"type" json:"type"`             // "course" or "coaching"
	TargetId   primitive.ObjectID   `bson:"targetId" json:"targetId"`     // courseId or coachingId
	TeacherIds []primitive.ObjectID `bson:"teacherIds" json:"teacherIds"` // Array of teacher IDs
	SubjectIds []primitive.ObjectID `bson:"subjectIds" json:"subjectIds"` // Extracted from course/coaching in service layer
	SchoolType string               `bson:"schoolType" json:"schoolType"`
	Level      string               `bson:"level" json:"level"`
	Country    string               `bson:"country" json:"country"`
	Status     string               `bson:"status" json:"status"` // "assigned", "pending"
}

// For bulk operations
type BulkTeacherAssignmentDto struct {
	Type       string               `json:"type"` // "course" or "coaching"
	TargetId   primitive.ObjectID   `json:"targetId"`
	TeacherIds []primitive.ObjectID `json:"teacherIds"`
	Status     string               `json:"status,omitempty"`
}

// Response with populated data
type TeacherAssignmentWithDetails struct {
	TeacherAssignment `bson:",inline"`
	TeacherObjs       []interface{} `bson:"teacherObjs,omitempty" json:"teacherObjs,omitempty"` // Populated teachers
	SubjectObjs       []interface{} `bson:"subjectObjs,omitempty" json:"subjectObjs,omitempty"` // Populated subjects
	TargetObj         interface{}   `bson:"targetObj,omitempty" json:"targetObj,omitempty"`     // Course or Coaching object
}

type TeacherAssignmentPagination struct {
	Assignments []TeacherAssignmentWithDetails `json:"assignments"`
	Pagination  types.Pagination               `json:"pagination"`
}

// Statistics model
type TeacherAssignmentStats struct {
	TotalCourses              int `json:"totalCourses"`
	AssignedCourses           int `json:"assignedCourses"`
	WaitingForAssignment      int `json:"waitingForAssignment"`
	TotalCoaching             int `json:"totalCoaching"`
	AssignedCoaching          int `json:"assignedCoaching"`
	WaitingCoachingAssignment int `json:"waitingCoachingAssignment"`
}
