package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"
)

type SupervisorProfileUpdateDto struct {
	FirstName      transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName       transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Gender         string                     `bson:"gender" json:"gender"`
	DateOfBirth    time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality    string                     `bson:"nationality" json:"nationality"`
	ProfilePicture types.FileField            `bson:"profilePicture" json:"profilePicture"`
	Address        Address                    `bson:"address" json:"address"`
}

type ParentProfileUpdateDto struct {
	FirstName   transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName    transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Gender      string                     `bson:"gender" json:"gender"`
	DateOfBirth time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality string                     `bson:"nationality" json:"nationality"`
	Picture     types.FileField            `bson:"picture" json:"picture"`
	Address     ParentAddressDetails       `bson:"address" json:"address"`
}
