package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentAssignment struct {
	Id             primitive.ObjectID `bson:"id" json:"id"`
	StudentId      primitive.ObjectID `bson:"studentId" json:"studentId"`
	CourseId       primitive.ObjectID `bson:"courseId" json:"courseId"`
	AssignmentId   primitive.ObjectID `bson:"assignmentId" json:"assignmentId"`
	SubmissionDate time.Time          `bson:"submissionDate" json:"submissionDate"`
	Answers        []AssignmentAnswer `bson:"answers" json:"answers"`
	TotalScore     float64            `bson:"totelScore" json:"totalScore"`
}

func (s *StudentAssignment) CollName() string {
	return "academyStudentAssignments"
}

type AssignmentAnswer struct {
	Question any     `bson:"question" json:"question"` // what ever question in assignment
	Answer   string  `bson:"answer" json:"answer"`     //student answer
	Score    float32 `bson:"score" json:"score"`
}
