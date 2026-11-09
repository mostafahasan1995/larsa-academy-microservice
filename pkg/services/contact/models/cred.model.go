package models

import "go.mongodb.org/mongo-driver/bson/primitive"

type UserData struct {
	Id primitive.ObjectID `json:"_id"`
}
type AddUserResult struct {
	UserData    UserData
	DeleteOnErr func() error
}

type RegisterUserResult struct {
	UserData    UserData
	UserResponse []byte
	Headers      map[string][]string
	DeleteOnErr  func() error
}
