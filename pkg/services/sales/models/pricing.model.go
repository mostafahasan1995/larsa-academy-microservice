package models

import (
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Result struct for PricingSvcs
type PriceResult struct {
	RefId    primitive.ObjectID `json:"refId"`
	Price    float64            `json:"price"`
	Currency string             `json:"currency"`
	Error    string             `json:"error,omitempty"`
}
