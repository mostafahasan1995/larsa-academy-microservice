package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ComplaintTypeDto struct {
	Code                  string               `bson:"code" json:"code" validate:"required"`
	Name                  string               `bson:"name" json:"name" validate:"required"`
	Owner                 primitive.ObjectID   `bson:"owner" json:"owner" validate:"required"`
	ExpectedTimeToSolve   string               `bson:"expectedTimeToSolve" json:"expectedTimeToSolve"`
	UpdateNotification    bool                 `bson:"updateNotification" json:"updateNotification"`
	InternalNotifications bool                 `bson:"internalNotifications" json:"internalNotifications"`
	TeamShouldBeNotified  []primitive.ObjectID `bson:"teamShouldBeNotified" json:"teamShouldBeNotified"`
}

type ComplaintType struct {
	Id               primitive.ObjectID `bson:"_id" json:"_id"`
	ComplaintTypeDto `bson:",inline"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy        primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy        primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash            bool               `bson:"trash" json:"trash"`
}

type ComplaintTypeWithOwner struct {
	Id               primitive.ObjectID `bson:"_id" json:"_id"`
	ComplaintTypeDto `bson:",inline"`
	OwnerObj         ComplaintOwnerInfo `bson:"ownerObj" json:"ownerObj"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy        primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy        primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash            bool               `bson:"trash" json:"trash"`
}

// ComplaintOwnerInfo is a compact owner representation used in complaintTypeObj.ownerObj
type ComplaintOwnerInfo struct {
	Id        primitive.ObjectID `bson:"_id" json:"_id"`
	FirstName string             `bson:"firstName" json:"firstName"`
	LastName  string             `bson:"lastName" json:"lastName"`
}

func (c *ComplaintType) CollName() string {
	return "academyComplaintTypes"
}

type ComplaintTypePagination struct {
	ComplaintTypes []ComplaintTypeWithOwner `bson:"complaintTypes" json:"complaintTypes"`
	Pagination     types.Pagination         `bson:"pagination" json:"pagination"`
}
