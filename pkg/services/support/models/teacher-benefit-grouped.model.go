package models

import (
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TeacherBenefitGrouped struct {
	TeacherId  primitive.ObjectID     `json:"teacherId" bson:"teacherId"`
	TeacherObj *contactModels.Teacher `json:"teacherObj,omitempty" bson:"-"`
	Benefits   []TeacherBenefit       `json:"benefits" bson:"benefits"`
	TotalCount int                    `json:"totalCount" bson:"totalCount"`
}

type TeacherBenefitGroupedPagination struct {
	Grouped    []TeacherBenefitGrouped `json:"grouped" bson:"grouped"`
	Pagination types.Pagination        `json:"pagination" bson:"pagination"`
}
