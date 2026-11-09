package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentBundle struct {
	Id        primitive.ObjectID     `bson:"_id,omitempty" json:"_id,omitempty"`
	BundleId  primitive.ObjectID     `bson:"bundleId" json:"bundleId"`
	StudentId primitive.ObjectID     `bson:"studentId" json:"studentId"`
	Status    enums.EnrollmentStatus `bson:"status" json:"status"`
	CreatedAt time.Time              `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time              `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy primitive.ObjectID     `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy primitive.ObjectID     `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash     bool                   `bson:"trash" json:"trash"`
}

func (b *StudentBundle) CollName() string {
	return "academyBundleStudents"
}

type MemberBundlePagination struct {
	StudentBundles []StudentBundleRes `bson:"studentBundles" json:"studentBundles"`
	Pagination     types.Pagination   `bson:"pagination" json:"pagination"`
}

type StudentBundleRes struct {
	StudentBundle `bson:",inline"`
	FirstName     transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName      transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email         string                     `bson:"email" json:"email"`
}
