package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Comment struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	UserId     primitive.ObjectID `bson:"userId" json:"userId"`
	CommentDto `bson:",inline"`
	CreatedAt  time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt  time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy  primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy  primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash      bool               `bson:"trash" json:"trash"`
}

func (c *Comment) CollName() string {
	return "academyContentComments"
}

type CommentDto struct {
	Type      enums.CommentType  `bson:"type" json:"type" validate:"required,oneof=course bundle coaching"` // course - bundle - coaching
	RefId     primitive.ObjectID `bson:"refId" json:"refId" validate:"required"`
	Content   string             `bson:"content" json:"content" validate:"required"`
	IsPublish bool               `bson:"isPublish" json:"isPublish"`
	ReplyTo   primitive.ObjectID `bson:"replyTo" json:"replyTo"`
}

type CommentRes struct {
	Comment   `bson:",inline"`
	Replies   []CommentRes `bson:"replies" json:"replies"`
	FirstName string       `bson:"firstName" json:"firstName"`
	LastName  string       `bson:"lastName" json:"lastName"`
	Email     string       `bson:"email" json:"email"`
	Picture   string       `bson:"picture" json:"picture"`
}

type CommentPagination struct {
	Comments   []CommentRes     `bson:"comments" json:"comments"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
