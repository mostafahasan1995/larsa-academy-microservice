package models

import (
	"fmt"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PlacementTest struct {
	Id               primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	PlacementTestDto `bson:",inline"`
	// Enriched fields (not stored in DB, populated from ref)
	Country     string             `bson:"-" json:"country,omitempty"`
	ProgramType string             `bson:"-" json:"programType,omitempty"`
	SchoolType  string             `bson:"-" json:"schoolType,omitempty"`
	SchoolId    primitive.ObjectID `bson:"-" json:"schoolId,omitempty"`
	SchoolObj   *School            `bson:"-" json:"schoolObj,omitempty"`
	Stage       string             `bson:"-" json:"stage,omitempty"`
	Class       []string           `bson:"-" json:"class,omitempty"`
	CreatedBy   primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash       bool               `bson:"trash" json:"trash"`
}

func (p *PlacementTest) CollName() string {
	return "academyPlacementTests"
}

type PlacementTestDto struct {
	Title     transl.Localizable[string] `bson:"title" json:"title" validate:"required"`
	Status    string                     `bson:"status" json:"status" validate:"required,oneof=draft active closed"` //draft - active - closed
	Type      string                     `bson:"type" json:"type" validate:"required,oneof=electronic online"`       // electronic - online
	RefType   string                     `bson:"refType" json:"refType" validate:"required,oneof=course coaching"`   //course - coaching
	RefId     primitive.ObjectID         `bson:"refId" json:"refId" validate:"required"`                             //course - coaching id
	TestMark  int                        `bson:"testMark" json:"testMark" validate:"required,gt=0"`
	Questions []Question                 `bson:"questions" json:"questions"`
	Settings  PlacementTestSettings      `bson:"settings" json:"settings" validate:"required"`
}

type PlacementTestSettings struct {
	AllowRetry        bool `bson:"allowRetry" json:"allowRetry"`
	NumOfAttempts     int  `bson:"numOfAttempts" json:"numOfAttempts" validate:"gte=1"`
	RetryNeedApproval bool `bson:"retryNeedApproval" json:"retryNeedApproval"`
	PassScore         int  `bson:"passscore" json:"passscore" validate:"min=0"`
}

func (p *PlacementTest) PreSave() {
	var questions []Question
	for index, q := range p.Questions {
		q.Num = index + 1
		answers := []Answer{}
		for answerIndex, answer := range q.Answers {
			answer.Num = answerIndex + 1
			answers = append(answers, answer)
		}
		q.Answers = answers
		questions = append(questions, q)
	}

	p.Questions = questions

}

func (p *PlacementTest) Populate() []bson.M {
	return []bson.M{}
}

func (dto *PlacementTestDto) Validate(v *validator.Validate) error {
	// First run generic validation
	if err := helpers.GenericValidation(v, dto); err != nil {
		return err
	}

	// Custom validation for questions based on test type
	if dto.Type == "online" && len(dto.Questions) > 0 {
		return fmt.Errorf("online placement tests cannot have questions")
	}
	if dto.Type == "electronic" && len(dto.Questions) == 0 {
		return fmt.Errorf("electronic placement tests must have questions")
	}

	// Validate that pass score is not greater than test mark
	if dto.Settings.PassScore > dto.TestMark {
		return fmt.Errorf("pass score (%d) cannot be greater than test mark (%d)", dto.Settings.PassScore, dto.TestMark)
	}

	return nil
}

type PlacementTestPagination struct {
	PlacementTests []PlacementTest  `bson:"placementTests" json:"placementTests"`
	Pagination     types.Pagination `bson:"pagination" json:"pagination"`
}

// schedule
