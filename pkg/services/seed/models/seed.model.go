package models

import (
	// "larsa-academy-microservice/pkg/services/seed/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SeedDto struct {
	IsCreateRoles        bool               `bson:"issCreateRoles" json:"isCreateRoles"`
	IsCreateCapabilities bool `bson:"seedId" json:"seedId"`
}

type Seed struct {
	Id        primitive.ObjectID `bson:"_id" json:"_id"`
	SeedDto   `bson:",inline"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash     bool               `bson:"trash" json:"trash"`
}

type SeedPagination struct {
	Seeds      []Seed           `bson:"seeds" json:"seeds"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
