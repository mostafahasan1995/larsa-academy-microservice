package models

import (
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type GlobalSearchResult struct {
	Id        primitive.ObjectID         `json:"id" bson:"id"`
	Name      transl.Localizable[string] `json:"name" bson:"name"`
	Image     types.FileField            `json:"image" bson:"image"`
	Type      string                     `json:"type" bson:"type"`
	Trash     bool                       `json:"-" bson:"trash"`
	SchoolObj *School                    `json:"schoolObj,omitempty" bson:"schoolObj,omitempty"`
}

type GlobalSearchRequest struct {
	Entities []string          `json:"entities"`
	Query    *query.Conditions `json:"query"`
	Page     int               `json:"page"`
	Limit    int               `json:"limit"`
}

type GlobalSearchResponse struct {
	Results    []GlobalSearchResult `json:"results"`
	Pagination types.Pagination     `json:"pagination"`
}
