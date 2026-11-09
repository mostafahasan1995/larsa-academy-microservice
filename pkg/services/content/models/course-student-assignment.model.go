package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CourseStudentAssignment struct {
	Id                         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId                  primitive.ObjectID `bson:"studentId" json:"studentId"`
	CourseId                   primitive.ObjectID `bson:"courseId" json:"courseId"`
	AssignmentId               primitive.ObjectID `bson:"assignmentId" json:"assignmentId"`
	CourseStudentAssignmentDto `bson:",inline"`
	SubmissionDate             time.Time          `bson:"submissionDate" json:"submissionDate"`
	UpdatedAt                  time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	UpdatedBy                  primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt                  time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	CreatedBy                  primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	Trash                      bool               `bson:"trash" json:"trash"`
}

type CourseStudentAssignmentDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Answer      string                     `bson:"answer" json:"answer"`
	Image       []types.FileField          `bson:"image" json:"image"`
	Status      string                     `bson:"status" json:"status"`                         // draft - submitted
	Grade       *int                       `bson:"grade,omitempty" json:"grade,omitempty"`       // Grade from 0-100
	Feedback    string                     `bson:"feedback,omitempty" json:"feedback,omitempty"` // Teacher's feedback/comment
	GradedAt    *time.Time                 `bson:"gradedAt,omitempty" json:"gradedAt,omitempty"`
	GradedBy    primitive.ObjectID         `bson:"gradedBy,omitempty" json:"gradedBy,omitempty"`
}

type CourseAssignmentWithAttempt struct {
	Id          primitive.ObjectID         `json:"_id"`
	Title       transl.Localizable[string] `json:"title"`
	Description transl.Localizable[string] `json:"description"`
	Deadline    time.Time                  `json:"deadline"`
	Subject     string                     `json:"subject"`
	Chapter     string                     `json:"chapter"`
	Status      string                     `json:"status"`
	Attempt     *CourseStudentAssignment   `json:"attempt,omitempty"`
}

func (c *CourseStudentAssignment) CollName() string {
	return "academyCourseStudentAssignments"
}

type GradeCourseStudentAssignmentDto struct {
	Grade    int    `json:"grade" validate:"required,min=0,max=100"`
	Feedback string `json:"feedback"`
}

type CourseStudentAssignmentRes struct {
	CourseStudentAssignment `bson:",inline"`
	FirstName               transl.Localizable[string] `json:"firstName"`
	LastName                transl.Localizable[string] `json:"lastName"`
	Email                   string                     `json:"email"`
	Picture                 string                     `json:"picture"`
	Class                   string                     `json:"class"`
}
