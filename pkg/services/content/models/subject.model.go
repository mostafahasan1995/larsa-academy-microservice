package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type SubjectPagination struct {
	Subjects   []Subject        `bson:"subjects" json:"subjects"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

type Subject struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	SubjectDto `bson:",inline"`
	CreatedBy  primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy  primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt  time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt  time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash      bool               `bson:"trash" json:"trash"`
}

func (s *Subject) CollName() string {
	return "academySubjects"
}

type SubjectDto struct {
	Name transl.Localizable[string] `bson:"name" json:"name"`
	Code string                     `bson:"code" json:"code"`
	// Country    string                     `bson:"country" json:"country"`
	// SchoolType string                     `bson:"schoolType" json:"schoolType"`
	// School     string                     `bson:"school" json:"school"`
	// GradeLevel string                     `bson:"gradeLevel" json:"gradeLevel"`
	// Grade      string                     `bson:"grade" json:"grade"`
	Version string                     `bson:"version" json:"version"`
	Note    transl.Localizable[string] `bson:"note" json:"note"`
}
