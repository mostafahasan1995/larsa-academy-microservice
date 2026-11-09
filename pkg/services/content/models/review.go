package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ReviewDto struct {
	Type    string             `bson:"type" json:"type" validate:"required,oneof=coaching course bundle"` //coaching - course - bundle
	RefId   primitive.ObjectID `bson:"refId" json:"refId" validate:"required"`                            //course or coaching id
	Rating  int                `bson:"rating" json:"rating" validate:"required,min=1,max=5"`
	Comment string             `bson:"comment" json:"comment" `
}

type Review struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	UserId       primitive.ObjectID `bson:"userId" json:"userId"`
	Date         time.Time          `bson:"date" json:"date"`
	ReviewStatus string             `bson:"reviewStatus" json:"reviewStatus"`
	ReviewDto    `bson:",inline"`
	CreatedAt    time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy    primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy    primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash        bool               `bson:"trash" json:"trash"`
}

func (r *Review) CollName() string {
	return "academyContentReviews"
}

type ReviewPagination struct {
	Reviews    []ReviewRes      `bson:"reviews" json:"reviews"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

type ReviewRes struct {
	Review    `bson:",inline"`
	FirstName string `bson:"firstName" json:"firstName"`
	LastName  string `bson:"lastName" json:"lastName"`
	Email     string `bson:"email" json:"email"`
	Picture   string `bson:"picture" json:"picture"`
}

type ReviewApprovalDto struct {
	ReviewStatus string `json:"reviewStatus" validate:"omitempty,oneof=approved rejected pending"` // "approved" "rejected" "pending"
}
