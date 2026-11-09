package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoachingStudent struct {
	Id                 primitive.ObjectID     `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId          primitive.ObjectID     `bson:"studentId" json:"studentId"`
	CoachingId         primitive.ObjectID     `bson:"coachingId" json:"coachingId"`
	Status             enums.EnrollmentStatus `bson:"status" json:"status"`
	RenewMonthly       bool                   `bson:"renewMonthly" json:"renewMonthly"`
	CommitmentProgress float64                `bson:"commitmentProgress" json:"commitmentProgress"` // Percentage of commitment combining sessions and assignments
	CreatedAt          time.Time              `bson:"createdAt" json:"createdAt"`
	UpdatedAt          time.Time              `bson:"updatedAt" json:"updatedAt"`
	Trash              bool                   `bson:"trash" json:"trash"`
}

func (c *CoachingStudent) CollName() string {
	return "academyCoachingStudents"
}
