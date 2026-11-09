package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type Counter struct {
	Id   primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	Name string             `bson:"name" json:"name"`
	Seq  int64              `bson:"seq" json:"seq"`
}
