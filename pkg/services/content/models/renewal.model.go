package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Renewal struct {
	Id         primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	RenewalDto `bson:",inline"`
	//
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash     bool               `bson:"trash" json:"trash"`
}

func (r *Renewal) CollName() string {
	return "academyRenewals"
}

type RenewalDto struct {
	StudentId       primitive.ObjectID       `bson:"studentId" json:"studentId"`
	RenewalType     enums.RenewalType        `bson:"renewalType" json:"renewalType"` //monthly - yearly
	ContentType     enums.RenewalContentType `bson:"contentType" json:"contentType"` //course - coaching - bundle
	RefId           primitive.ObjectID       `bson:"refId" json:"refId"`
	StartDate       time.Time                `bson:"startDate" json:"startDate"`
	Data            any                      `bson:"data" json:"data"`
	NextRenewalDate time.Time                `bson:"nextRenewalDate" json:"nextRenewalDate"`
	Status          enums.RenewalStatus      `bson:"status" json:"status"` //active  - cancelled
}
