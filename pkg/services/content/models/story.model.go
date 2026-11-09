package models

import (
	"errors"
	"time"

	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Story struct {
	Id            primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StoryDto      `bson:",inline"`
	CreatedBy     primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	CreatedByName string             `bson:"createdByName,omitempty" json:"createdByName,omitempty"` // Creator name from context
	UpdatedBy     primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt     time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt     time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash         bool               `bson:"trash" json:"trash"`

	// Populated fields (not stored in DB)
	CourseObj *Course `bson:"courseObj,omitempty" json:"courseObj,omitempty"`
}

func (s *Story) CollName() string {
	return "academyStories"
}

type StoryDto struct {
	Title            transl.Localizable[string] `bson:"title" json:"title" validate:"required"`
	Status           string                     `bson:"status" json:"status" `
	ScheduleDate     time.Time                  `bson:"scheduleDate" json:"scheduleDate"`
	Visibility       string                     `bson:"visibility" json:"visibility"`
	// Scope defines the visibility of the story: "global" or "course"
	Scope           string             `bson:"scope,omitempty" json:"scope,omitempty"`
	Course           primitive.ObjectID         `bson:"course" json:"course"`
	TimeAvailability TimeAvailability           `bson:"timeAvailability" json:"timeAvailability"`
	Content          []StoryContent             `bson:"content" json:"content"`
	Settings         StorySettings              `bson:"settings" json:"settings"`
}

func (s *StoryDto) Validate(v *validator.Validate) error {
	if err := helpers.GenericValidation(v, s); err != nil {
		return err
	}

	// Normalize and validate scope
	if s.Scope == "" {
		s.Scope = "global"
	}

	if s.Scope != "global" && s.Scope != "course" {
		return errors.New("scope must be either 'global' or 'course'")
	}

	if s.Scope == "course" && s.Course.IsZero() {
		return errors.New("course is required when scope is 'course'")
	}

	return nil
}

type StoryContent struct {
	Type string         `bson:"type" json:"type"` // file - video - audio - text
	Data any            `bson:"data" json:"data"`
	Meta map[string]any `bson:"meta" json:"meta"`
}

type StorySettings struct {
	RequireApproval bool `bson:"requireApproval" json:"requireApproval"`
	PinToTop        bool `bson:"pinToTop" json:"pinToTop"`
	ShowInDailyFeed bool `bson:"showInDailyFeed" json:"showInDailyFeed"`
}

type StoryPagination struct {
	Stories    []Story          `json:"stories"`
	Pagination types.Pagination `json:"pagination"`
}
