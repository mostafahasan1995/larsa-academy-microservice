package models

import (
	"larsa-academy-microservice/pkg/services/support/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ComplaintDto struct {
	Summary           string                  `bson:"summary" json:"summary"`
	RelatedStudent    primitive.ObjectID      `bson:"relatedStudent" json:"relatedStudent"`
	Description       string                  `bson:"description" json:"description"`
	ComplaintUserRole enums.ComplaintUserRole `bson:"complaintUserRole" json:"complaintUserRole" validate:"required,oneof=student parent teacher"`
	ComplaintType     primitive.ObjectID      `bson:"complaintType" json:"complaintType"`
}
type LogHistory struct {
	ChangeActionType enums.ChangeActionType `bson:"changeActionType" json:"changeActionType"`
	From             any                    `bson:"from" json:"from"`
	To               any                    `bson:"to" json:"to"`
}
type ComplaintStatusDto struct {
	Status enums.ComplaintStatus `bson:"status" json:"status" validate:"required,oneof=open inProgress resolved"`
	Reason string                `bson:"reason" json:"reason"`
}

type AssignToDto struct {
	Note string `bson:"note" json:"note"`
}

type ReplyDto struct {
	Message string `bson:"message" json:"message" validate:"required"`
}

type Reply struct {
	Id               primitive.ObjectID `bson:"_id" json:"_id"`
	Message          string             `bson:"message" json:"message"`
	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	CreatedBy        primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	CreatedByName    string             `bson:"createdByName" json:"createdByName"`
	CreatedByPicture string             `bson:"createdByPicture,omitempty" json:"createdByPicture,omitempty"`
}

type Complaint struct {
	Id                 primitive.ObjectID `bson:"_id" json:"_id"`
	AssignTo           primitive.ObjectID `bson:"assignTo" json:"assignTo"`
	LogHistory         []LogHistory       `bson:"logHistory" json:"logHistory"`
	Replies            []Reply            `bson:"replies" json:"replies"`
	ComplaintDto       `bson:",inline"`
	ComplaintStatusDto `bson:",inline"`
	SupervisorNote     string `bson:"supervisorNote" json:"supervisorNote"`

	CreatedAt     time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt     time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy     primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy     primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash         bool               `bson:"trash" json:"trash"`
	CreatedByName string             `bson:"-" json:"createdByName"`
}

type UserCompact struct {
	Id        primitive.ObjectID `bson:"_id" json:"_id"`
	FirstName string             `bson:"firstName" json:"firstName"`
	LastName  string             `bson:"lastName" json:"lastName"`
}

type ComplaintWithDetails struct {
	Id                 primitive.ObjectID `bson:"_id" json:"_id"`
	AssignTo           primitive.ObjectID `bson:"assignTo" json:"assignTo"`
	LogHistory         []LogHistory       `bson:"logHistory" json:"logHistory"`
	Replies            []Reply            `bson:"replies" json:"replies"`
	ComplaintDto       `bson:",inline"`
	ComplaintStatusDto `bson:",inline"`
	SupervisorNote     string                 `bson:"supervisorNote" json:"supervisorNote"`
	AssignToObj        UserCompact            `bson:"assignToObj" json:"assignToObj"`
	StudentObj         UserCompact            `bson:"studentObj" json:"studentObj"`
	ComplaintTypeObj   ComplaintTypeWithOwner `bson:"complaintTypeObj" json:"complaintTypeObj"`

	CreatedAt        time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt        time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy        primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy        primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash            bool               `bson:"trash" json:"trash"`
	CreatedByName    string             `bson:"createdByName" json:"createdByName"`
	CreatedByPicture string             `bson:"createdByPicture,omitempty" json:"createdByPicture,omitempty"`
}

type ComplaintPagination struct {
	Complaints []ComplaintWithDetails `bson:"complaints" json:"complaints"`
	Pagination types.Pagination       `bson:"pagination" json:"pagination"`
}
