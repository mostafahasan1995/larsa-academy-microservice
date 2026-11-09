package models

import (
	// "larsa-academy-microservice/pkg/services/support/enums"
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type FeedbackDto struct {
	TeacherId   primitive.ObjectID `bson:"teacherId" json:"teacherId"`
	StudentId   primitive.ObjectID `bson:"studentId" json:"studentId"`
	Status      string             `bson:"status" json:"status"`
	Content     string             `bson:"content" json:"content"`
	IsShowInApp bool               `bson:"isShowInApp" json:"isShowInApp"`
	Rate        int                `bson:"rate" json:"rate" validate:"required,min=1,max=5"`
}

type Feedback struct {
	Id          primitive.ObjectID `bson:"_id" json:"_id"`
	FeedbackDto `bson:",inline"`
	CreatedAt   time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy   primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy   primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash       bool               `bson:"trash" json:"trash"`

	// Populated fields (not stored in database)
	StudentObj *contactModels.Student `bson:"-" json:"studentObj,omitempty"`
	TeacherObj *contactModels.Teacher `bson:"-" json:"teacherObj,omitempty"`
}

func (f *Feedback) CollName() string {
	return "academyFeedbacks"
}

type FeedbackPagination struct {
	Feedbacks  []Feedback       `bson:"feedbacks" json:"feedbacks"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

type FeedbackStats struct {
	Total                       int64 `json:"total"`
	TotalApproved               int64 `json:"totalApproved"`
	TotalRejected               int64 `json:"totalRejected"`
	TotalPending                int64 `json:"totalPending"`
	TotalSynchronizedWithGoogle int64 `json:"totalSynchronizedWithGoogle"`
}
