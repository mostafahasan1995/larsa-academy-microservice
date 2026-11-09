package models

import (
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/points/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// RedeemRequest represents a student's request to redeem points for a reward
type RedeemRequest struct {
	ID             primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentID      primitive.ObjectID `bson:"studentId" json:"studentId"`
	StudentName    string             `bson:"studentName" json:"studentName"`
	Grade          string             `bson:"grade" json:"grade"`
	Class          string             `bson:"class" json:"class"`
	RewardID       primitive.ObjectID `bson:"rewardId" json:"rewardId"`
	RewardTitle    string             `bson:"rewardTitle" json:"rewardTitle"`
	PointsRequired int                `bson:"pointsRequired" json:"pointsRequired"`
	Status         enums.RedeemStatus `bson:"status" json:"status"`
	RequestDate    time.Time          `bson:"requestDate" json:"requestDate"`
	ApprovedBy     primitive.ObjectID `bson:"approvedBy,omitempty" json:"approvedBy,omitempty"`
	ApprovedByName string             `bson:"approvedByName,omitempty" json:"approvedByName,omitempty"`
	ApprovedAt     *time.Time         `bson:"approvedAt,omitempty" json:"approvedAt,omitempty"`
	RejectedReason string             `bson:"rejectedReason,omitempty" json:"rejectedReason,omitempty"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy      primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy      primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash          bool               `bson:"trash" json:"trash"`

	// Populated fields (not stored in database, but populated via aggregation)
	StudentObj *contactModels.Student `bson:"studentObj,omitempty" json:"studentObj,omitempty"`
	RewardObj  *Reward                `bson:"rewardObj,omitempty" json:"rewardObj,omitempty"`
}

func (r *RedeemRequest) CollName() string {
	return "academyRedeemRequests"
}

type RedeemRequestPagination struct {
	Requests   []RedeemRequest  `bson:"requests" json:"requests"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
