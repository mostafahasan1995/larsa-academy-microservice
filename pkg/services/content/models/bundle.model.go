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

type Bundle struct {
	Id        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	BundleDto `bson:",inline"`
	CreatedBy primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash     bool               `bson:"trash" json:"trash"`
}

type EnrichedBundle struct {
	Bundle    `bson:",inline"`
	Pricing   *BundlePricing `bson:"pricing,omitempty" json:"pricing,omitempty"`
	SchoolObj *School        `bson:"schoolObj,omitempty" json:"schoolObj,omitempty"`
}

type StudentBundleResponse struct {
	Bundle     `bson:",inline"`
	Pricing    *BundlePricing `bson:"pricing,omitempty" json:"pricing,omitempty"`
	IsFavorite bool           `json:"isFavorite"`
	IsEnrolled bool           `json:"isEnrolled"`
}

type StudentBundlePagination struct {
	Bundles    []StudentBundleResponse `json:"bundles"`
	Pagination types.Pagination        `json:"pagination"`
}

func (b *Bundle) CollName() string {
	return "academyBundles"
}

func (b *Bundle) GetName() transl.Localizable[string] {
	return b.Name
}

type BundleDto struct {
	Name        transl.Localizable[string] `bson:"name" json:"name" validate:"required"`
	Status      string                     `bson:"status" json:"status" validate:"required"`
	Country     string                     `bson:"country" json:"country"`
	SchoolType  string                     `bson:"schoolType" json:"schoolType" `
	SchoolId    primitive.ObjectID         `bson:"schoolId,omitempty" json:"schoolId,omitempty" `
	Grade       []string                   `bson:"grade" json:"grade" `
	Level       string                     `bson:"level" json:"level" `
	Subject     []string                   `bson:"subject" json:"subject" `
	Thumbnail   types.FileField            `bson:"thumbnail" json:"thumbnail" `
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Content     []BundleContent            `bson:"content" json:"content" `
	//Pricing     any                        `bson:"pricing" json:"pricing" `
	Settings any `bson:"settings" json:"settings"`
}

func (dto *BundleDto) Validate(v *validator.Validate) error {
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

	if err := ValidateBundleContentTypes(dto.Content); err != nil {
		return err
	}

	return nil
}

func ValidateBundleContentTypes(content []BundleContent) error {
	if len(content) == 0 {
		return nil
	}

	firstType := content[0].Type
	for _, item := range content {
		if item.Type != firstType {
			return errors.New("bundle content must be of only one type: either all courses or all coaching")
		}
	}

	return nil
}

type BundleContent struct {
	Type  string             `bson:"type" json:"type"` // course - coaching
	RefId primitive.ObjectID `bson:"refId" json:"refId"`
	Meta  map[string]any     `bson:"meta" json:"meta"`
	Data  any                `bson:"-" json:"data,omitempty"` //refId data
}

type BundlePagination struct {
	Bundles    []EnrichedBundle `json:"bundles"`
	Pagination types.Pagination `json:"pagination"`
}

type BundleSubscribe struct {
}
