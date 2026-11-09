package models

import (
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/support/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type TeacherBenefitDto struct {
	TeacherId  primitive.ObjectID      `bson:"teacherId" json:"teacherId" validate:"required"`
	BenefitId  primitive.ObjectID      `bson:"benefitId" json:"benefitId" validate:"required"`
	FromDate   time.Time               `bson:"fromDate" json:"fromDate" validate:"required"`
	ToDate     time.Time               `bson:"toDate" json:"toDate" validate:"required"`
	Visibility enums.BenefitVisibility `bson:"visibility" json:"visibility" validate:"required"`
}

type TeacherBenefit struct {
	Id                primitive.ObjectID `bson:"_id" json:"_id"`
	TeacherBenefitDto `bson:",inline"`
	CreatedAt         time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt         time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy         primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy         primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash             bool               `bson:"trash" json:"trash"`

	// Populated fields (not stored in database, but populated via aggregation)
	BenefitObj *Benefit               `bson:"benefitObj,omitempty" json:"benefitObj,omitempty"`
	TeacherObj *contactModels.Teacher `bson:"teacherObj,omitempty" json:"teacherObj,omitempty"`
}

func (tb *TeacherBenefit) CollName() string {
	return "academyTeacherBenefits"
}

type TeacherBenefitPagination struct {
	TeacherBenefits []TeacherBenefit `bson:"teacherBenefits" json:"teacherBenefits"`
	Pagination      types.Pagination `bson:"pagination" json:"pagination"`
}

// BulkTeacherBenefitRequest for bulk add
type BulkTeacherBenefitRequest struct {
	TeacherBenefits []TeacherBenefitDto `json:"teacherBenefits" validate:"required,min=1,dive"`
}

// BulkTeacherBenefitUpdateItem represents a single update item
type BulkTeacherBenefitUpdateItem struct {
	Id         string                  `json:"id" validate:"required"`
	TeacherId  primitive.ObjectID      `json:"teacherId" validate:"required"`
	BenefitId  primitive.ObjectID      `json:"benefitId" validate:"required"`
	FromDate   time.Time               `json:"fromDate" validate:"required"`
	ToDate     time.Time               `json:"toDate" validate:"required"`
	Visibility enums.BenefitVisibility `json:"visibility" validate:"required"`
}

// BulkTeacherBenefitUpdateRequest for bulk edit
type BulkTeacherBenefitUpdateRequest struct {
	TeacherBenefits []BulkTeacherBenefitUpdateItem `json:"teacherBenefits" validate:"required,min=1,dive"`
}

// BulkTeacherBenefitResponse for both operations
type BulkTeacherBenefitResponse struct {
	SuccessCount int              `json:"successCount"`
	FailureCount int              `json:"failureCount"`
	Successes    []TeacherBenefit `json:"successes"`
	Failures     []BulkErrorItem  `json:"failures"`
}

// BulkErrorItem represents a failure in bulk operation
type BulkErrorItem struct {
	Index int    `json:"index"`
	Id    string `json:"id,omitempty"`
	Error string `json:"error"`
}
