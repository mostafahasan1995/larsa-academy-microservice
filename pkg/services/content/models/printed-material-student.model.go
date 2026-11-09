package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PrintedMaterialStudent struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId  primitive.ObjectID `bson:"studentId" json:"studentId"`
	MaterialId primitive.ObjectID `bson:"materialId" json:"materialId"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	CreatedBy  primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
	UpdatedBy  primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash      bool               `bson:"trash" json:"trash"`
}

func (p *PrintedMaterialStudent) CollName() string {
	return "academyPrintedMaterialStudents"
}
