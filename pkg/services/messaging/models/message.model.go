package models

import (
	"larsa-academy-microservice/pkg/services/messaging/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Message struct {
	Id          primitive.ObjectID     `bson:"_id,omitempty" json:"_id,omitempty"`
	Type        enums.MsgTyps          `bson:"type" json:"type"` // for pre defined templates
	Email       string                 `bson:"email" json:"email"`
	Mobile      string                 `bson:"mobile" json:"mobile"`
	Subject     string                 `bson:"subject" json:"subject"`
	Message     string                 `bson:"message" json:"message"`         //for use with whatsapp
	MessageHtml string                 `bson:"messageHtml" json:"messageHtml"` //for use with email
	Attachments []types.FileField      `bson:"attachments" json:"attachments"`
	SenderId    primitive.ObjectID     `bson:"senderId" json:"senderId"`
	CreatedBy   primitive.ObjectID     `bson:"createdBy" json:"createdBy"`
	CreatedAt   time.Time              `bson:"createdAt" json:"createdAt"`
	Status      string                 `bson:"status" json:"status"` //sent - not sent
	Target      string                 `bson:"target" json:"target"` // email-whatsapp
	Others      map[string]interface{} `bson:"others,omitempty" json:"others,omitempty"`
}
