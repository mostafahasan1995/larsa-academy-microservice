package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BenefitTypeDto struct {
	Name        string `bson:"name" json:"name" validate:"required"`
	Icon        string `bson:"icon" json:"icon"`
	Description string `bson:"description" json:"description"`
}

type BenefitType struct {
	Id             primitive.ObjectID `bson:"_id" json:"_id"`
	BenefitTypeDto `bson:",inline"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy      primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy      primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash          bool               `bson:"trash" json:"trash"`
}

func (b *BenefitType) CollName() string {
	return "academyBenefitTypes"
}

type BenefitTypePagination struct {
	BenefitTypes []BenefitType    `bson:"benefitTypes" json:"benefitTypes"`
	Pagination   types.Pagination `bson:"pagination" json:"pagination"`
}
