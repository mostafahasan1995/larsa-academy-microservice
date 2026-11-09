package models

import (
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type BenefitDto struct {
	Name          string             `bson:"name" json:"name" validate:"required"`
	BenefitTypeId primitive.ObjectID `bson:"benefitTypeId" json:"benefitTypeId" validate:"required"`
	BenefitValue  string             `bson:"benefitValue" json:"benefitValue" `
	Description   string             `bson:"description" json:"description"`
}

type Benefit struct {
	Id         primitive.ObjectID `bson:"_id" json:"_id"`
	BenefitDto `bson:",inline"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy  primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy  primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash      bool               `bson:"trash" json:"trash"`

	// Populated field (not stored in database, but populated via aggregation)
	BenefitTypeObj *BenefitType `bson:"benefitTypeObj,omitempty" json:"benefitTypeObj,omitempty"`
}

func (b *Benefit) CollName() string {
	return "academyBenefits"
}

type BenefitPagination struct {
	Benefits   []Benefit        `bson:"benefits" json:"benefits"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}
