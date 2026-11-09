package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type TypeBreakdown struct {
	TypeId   primitive.ObjectID `json:"typeId" bson:"_id"`
	TypeName string             `json:"typeName" bson:"typeName"`
	Count    int64              `json:"count" bson:"count"`
}

type BenefitStats struct {
	TotalBenefitsGiven    int64           `json:"totalBenefitsGiven"`
	TeachersWithBenefits  int64           `json:"teachersWithBenefits"`
	AvgBenefitsPerTeacher float64         `json:"avgBenefitsPerTeacher"`
	TypeBreakdown         []TypeBreakdown `json:"typeBreakdown"`
}
