package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PricingCriteria struct {
	SubjectId  primitive.ObjectID `bson:"subjectId" json:"subjectId" validate:"required"`
	SchoolType string             `bson:"schoolType" json:"schoolType" validate:"required"`
	Country    string             `bson:"country" json:"country" validate:"required"`
	Level      string             `bson:"level" json:"level" validate:"required"`
}

// TeacherPricing represents hourly rate per teacher per criteria (subject, school type, etc).
type TeacherPricing struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	TeacherId  primitive.ObjectID `bson:"teacherId" json:"teacherId" validate:"required"`
	Criteria   PricingCriteria    `bson:"criteria" json:"criteria" validate:"required,dive"`
	HourlyRate float64            `bson:"hourlyRate" json:"hourlyRate" validate:"required,gt=0"`
	Currency   string             `bson:"currency" json:"currency" validate:"required"`
	IsActive   bool               `bson:"isActive" json:"isActive"`
	Notes      string             `bson:"notes,omitempty" json:"notes,omitempty"`
	AuditLog   []AuditEntry       `bson:"auditLog,omitempty" json:"auditLog,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt  time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy  primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy  primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash      bool               `bson:"trash" json:"trash"`
}

func (tp *TeacherPricing) CollName() string {
	return "academyTeacherPricings"
}

type TeacherPricingDetails struct {
	TeacherPricing `bson:",inline"`
	Teacher        TeacherBasicInfo `bson:"teacher" json:"teacher"`
	Subject        SubjectInfo      `bson:"subject" json:"subject"`
}

type PricingCriteriaDto struct {
	SubjectId  string `json:"subjectId" validate:"required,len=24"`
	SchoolType string `json:"schoolType" validate:"required"`
	Country    string `json:"country" validate:"required"`
	Level      string `json:"level" validate:"required"`
}

type TeacherPricingCreateDto struct {
	Criteria   PricingCriteriaDto `json:"criteria" validate:"required"`
	HourlyRate float64            `json:"hourlyRate" validate:"required,gt=0"`
	Currency   string             `json:"currency" validate:"required"`
	IsActive   bool               `json:"isActive"`
	Notes      string             `json:"notes"`
}

type TeacherPricingUpdateDto struct {
	Criteria   *PricingCriteriaDto `json:"criteria" validate:"omitempty"`
	HourlyRate float64             `json:"hourlyRate" validate:"omitempty,gt=0"`
	Currency   string              `json:"currency" validate:"omitempty"`
	IsActive   *bool               `json:"isActive"`
	Notes      string              `json:"notes"`
}

type TeacherPricingPagination struct {
	TeacherPricings []TeacherPricingDetails `json:"teacherPricings"`
	Pagination      types.Pagination        `json:"pagination"`
}

// TeacherBasicInfo holds minimal teacher details for pricing responses.
type TeacherBasicInfo struct {
	Id        primitive.ObjectID         `bson:"_id" json:"_id"`
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email     string                     `bson:"email" json:"email"`
}
