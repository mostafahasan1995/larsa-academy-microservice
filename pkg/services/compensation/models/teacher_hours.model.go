package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	HoursStatusRecorded = "recorded"
	HoursStatusRejected = "rejected"
)

type TeacherHours struct {
	Id              primitive.ObjectID  `bson:"_id,omitempty" json:"_id,omitempty"`
	TeacherId       primitive.ObjectID  `bson:"teacherId" json:"teacherId" validate:"required"`
	Ref             string              `bson:"ref" json:"ref" validate:"required,oneof=course coaching"`
	RefId           primitive.ObjectID  `bson:"refId" json:"refId" validate:"required"`
	SessionId       *primitive.ObjectID `bson:"sessionId,omitempty" json:"sessionId,omitempty"`
	OccurredAt      time.Time           `bson:"occurredAt" json:"occurredAt" validate:"required"`
	DurationMinutes float64             `bson:"durationMinutes" json:"durationMinutes" validate:"required,gt=0"`
	HourlyRate      float64             `bson:"hourlyRate" json:"hourlyRate" validate:"required,gt=0"`
	Currency        string              `bson:"currency" json:"currency" validate:"required"`
	PricingId       primitive.ObjectID  `bson:"pricingId" json:"pricingId" validate:"required"`
	Amount          float64             `bson:"amount" json:"amount"`
	Status          string              `bson:"status" json:"status" validate:"required,oneof=recorded rejected"`
	RecordedBy      primitive.ObjectID  `bson:"recordedBy" json:"recordedBy"`
	Notes           string              `bson:"notes,omitempty" json:"notes,omitempty"`
	AuditLog        []AuditEntry        `bson:"auditLog,omitempty" json:"auditLog,omitempty"`
	CreatedAt       time.Time           `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt       time.Time           `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash           bool                `bson:"trash" json:"trash"`
}

func (th *TeacherHours) CollName() string {
	return "academyTeacherHours"
}

type TeacherHoursCreateDto struct {
	Ref             string    `json:"ref" validate:"required,oneof=course coaching"`
	RefId           string    `json:"refId" validate:"required,len=24"`
	SessionId       string    `json:"sessionId"`
	OccurredAt      time.Time `json:"occurredAt" validate:"required"`
	DurationMinutes float64   `json:"durationMinutes" validate:"required,gt=0"`
	Notes           string    `json:"notes"`
}

type TeacherHoursUpdateDto struct {
	OccurredAt      *time.Time `json:"occurredAt,omitempty"`
	DurationMinutes *float64   `json:"durationMinutes,omitempty" validate:"omitempty,gt=0"`
	Notes           *string    `json:"notes,omitempty"`
}

type TeacherHoursPagination struct {
	TeacherHours []TeacherHours   `json:"teacherHours"`
	Pagination   types.Pagination `json:"pagination"`
}

type PricingSummary struct {
	PricingId      primitive.ObjectID `json:"pricingId" bson:"pricingId"`
	Criteria       PricingCriteria    `json:"criteria" bson:"criteria"`
	Subject        SubjectInfo        `json:"subject,omitempty" bson:"subject,omitempty"`
	HourlyRate     float64            `json:"hourlyRate" bson:"hourlyRate"`
	Currency       string             `json:"currency" bson:"currency"`
	IsActive       bool               `json:"isActive" bson:"isActive"`
	HoursRecorded  float64            `json:"hoursRecorded" bson:"hoursRecorded"`
	TotalAmount    float64            `json:"totalAmount" bson:"totalAmount"`
	RecordCount    int                `json:"recordCount" bson:"recordCount"`
	RejectedHours  float64            `json:"rejectedHours,omitempty" bson:"rejectedHours,omitempty"`
	RejectedAmount float64            `json:"rejectedAmount,omitempty" bson:"rejectedAmount,omitempty"`
	RejectedCount  int                `json:"rejectedCount,omitempty" bson:"rejectedCount,omitempty"`
}

// SubjectInfo represents minimal subject details for pricing enrichment.
type SubjectInfo struct {
	Id   primitive.ObjectID         `json:"_id" bson:"_id"`
	Name transl.Localizable[string] `json:"name" bson:"name"`
	Code string                     `json:"code" bson:"code"`
}

type TeacherCompensationSummary struct {
	TeacherId             primitive.ObjectID `json:"teacherId" bson:"teacherId"`
	Teacher               TeacherBasicInfo   `json:"teacher,omitempty" bson:"teacher,omitempty"`
	TotalHours            float64            `json:"totalHours" bson:"totalHours"`
	TotalAmountByCurrency map[string]float64 `json:"totalAmountByCurrency" bson:"totalAmountByCurrency"`
	PricingSummaries      []PricingSummary   `json:"pricingSummaries" bson:"pricingSummaries"`
	StatusBreakdown       map[string]int     `json:"statusBreakdown,omitempty" bson:"statusBreakdown,omitempty"`
}

// CompensationSummary represents aggregate statistics across all teachers
type CompensationSummary struct {
	TotalTeachers         int                `json:"totalTeachers"`         // Number of teachers with recorded hours
	TotalHours            float64            `json:"totalHours"`            // Total hours recorded across all teachers
	TotalAmountByCurrency map[string]float64 `json:"totalAmountByCurrency"` // Total amount owed, grouped by currency
	AverageRatePerHour    map[string]float64 `json:"averageRatePerHour"`    // Average rate per hour, grouped by currency
}
