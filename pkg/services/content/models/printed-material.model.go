package models

import (
	"errors"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PrintedMaterialPagination struct {
	PrintedMaterials []EnrichedPrintedMaterial `bson:"printedMaterials" json:"printedMaterials"`
	Pagination       types.Pagination          `bson:"pagination" json:"pagination"`
}

type PrintedMaterial struct {
	Id                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PrintedMaterialDto `bson:",inline"`
	CreatedBy          primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy          primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt          time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt          time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash              bool               `bson:"trash" json:"trash"`
}

type EnrichedPrintedMaterial struct {
	PrintedMaterial `bson:",inline"`
	SchoolObj       *School `bson:"schoolObj,omitempty" json:"schoolObj,omitempty"`
}

func (pm *PrintedMaterial) CollName() string {
	return "academyPrintedMaterials"
}

func (pm *PrintedMaterial) GetName() transl.Localizable[string] {
	return pm.Title
}

type PrintedMaterialDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title" validate:"required"`
	Subject     primitive.ObjectID         `bson:"subject" json:"subject" validate:"required"`
	SubjectObj  *Subject                   `bson:"subjectObj,omitempty" json:"subjectObj,omitempty"`
	Country     string                     `bson:"country" json:"country"`
	SchoolType  string                     `bson:"schoolType" json:"schoolType" validate:"required,oneof=private public"`
	SchoolId    primitive.ObjectID         `bson:"schoolId,omitempty" json:"schoolId,omitempty"`
	GradeLevel  string                     `bson:"gradeLevel" json:"gradeLevel"`
	Grade       string                     `bson:"grade" json:"grade"`
	Version     string                     `bson:"version" json:"version"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Note        transl.Localizable[string] `bson:"note" json:"note"`
	Image       types.FileField            `bson:"image" json:"image"`
	// Media is an optional marketing gallery (images/videos) shown before purchase
	Media       []MediaItem                `bson:"media,omitempty" json:"media,omitempty"`

	// Pricing Information
	IsFree                 bool                       `bson:"isFree" json:"isFree"`
	Price                  float64                    `bson:"price" json:"price"`
	Currency               string                     `bson:"currency" json:"currency"`
	PricingPlanName        transl.Localizable[string] `bson:"pricingPlanName" json:"pricingPlanName"`
	PricingPlanSubtitle    transl.Localizable[string] `bson:"pricingPlanSubtitle" json:"pricingPlanSubtitle"`
	PricingPlanDescription transl.Localizable[string] `bson:"pricingPlanDescription" json:"pricingPlanDescription"`
	SetAsDefaultPrice      bool                       `bson:"setAsDefaultPrice" json:"setAsDefaultPrice"`
}

func (dto *PrintedMaterialDto) Validate(v *validator.Validate) error {
	if err := helpers.GenericValidation(v, dto); err != nil {
		return err
	}

	if dto.SchoolType == "private" {
		if dto.SchoolId.IsZero() {
			return errors.New("schoolId is required for private school type")
		}
	} else if dto.SchoolType == "public" {
		if !dto.SchoolId.IsZero() {
			return errors.New("schoolId should not be defined for public school type")
		}
	}

	return nil
}

// cart
type MaterialSubscribe struct {
	//any data required for material subscription
}
