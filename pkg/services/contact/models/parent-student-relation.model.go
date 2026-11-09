package models

import (
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ParentStudentRelationDto struct {
	FirstId      primitive.ObjectID `bson:"firstId" json:"firstId"`
	FirstName    string             `bson:"firstName" json:"firstName"`
	FirstType    enums.PersonType   `bson:"firstType" json:"firstType" validate:"required,oneof=student parent"`
	FirstObj     any                `bson:"firstObj" json:"firstObj"`
	SecondId     primitive.ObjectID `bson:"secondId" json:"secondId"`
	SecondName   string             `bson:"secondName" json:"secondName"`
	SecondType   enums.PersonType   `bson:"secondType" json:"secondType" validate:"required,oneof=student parent"`
	SecondObj    any                `bson:"secondObj" json:"secondObj"`
	RelationType string             `bson:"relationType" json:"relationType"`
}

type ParentStudentRelation struct {
	Id                       primitive.ObjectID `bson:"_id" json:"_id"`
	ParentStudentRelationDto `bson:",inline"`
	CreatedAt                time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt                time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy                primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy                primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash                    bool               `bson:"trash" json:"trash"`
}

func (p *ParentStudentRelation) CollName() string {
	return "academyParentsStudentRelations"
}


type ParentStudentRelationsPagination struct {
	ParentStudentRelations    []ParentStudentRelation         `bson:"parentStudentRelations" json:"parentStudentRelations"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}