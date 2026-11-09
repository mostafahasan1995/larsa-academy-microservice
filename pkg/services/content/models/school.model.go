package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type School struct {
	Id        primitive.ObjectID         `bson:"_id,omitempty" json:"_id,omitempty"`
	Name      transl.Localizable[string] `bson:"name" json:"name" validate:"required"`
	Country   string                     `bson:"country" json:"country" validate:"required"`
	CreatedAt time.Time                  `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time                  `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID         `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID         `bson:"updatedBy" json:"updatedBy"`
	Trash     bool                       `bson:"trash" json:"trash"`
}

type SchoolDto struct {
	Name    transl.Localizable[string] `bson:"name" json:"name" validate:"required"`
	Country string                     `bson:"country" json:"country" validate:"required"`
}

func (s *School) CollName() string {
	return "academySchools"
}

type SchoolPagination struct {
	Schools    []School         `bson:"schools" json:"schools"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
