package types

import (
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Pagination struct {
	TotalPages float64 `json:"totalPages"`
	PerPage    int64   `json:"perPage"`
	TotalCount int64   `json:"totalCount"`
}

type User struct {
	//for now we only need the id we can add other fields as need
	Id       primitive.ObjectID
	UserData common.User
}

type FileField struct {
	Id           primitive.ObjectID `bson:"_id" json:"_id"`
	OriginalName string             `bson:"originalName" json:"originalName"`
	Path         string             `bson:"path" json:"path"`
	Service      string             `bson:"service" json:"service"`
	Expire       string             `bson:"expire" json:"expire"`
	Variants     []string           `bson:"variants" json:"variants"`
}

// type PhoneField struct {
// 	Pre     string `bson:"pre" json:"pre"`
// 	Content string `bson:"content" json:"content"`
// }

type ServiceToken struct {
	Token string `bson:"token" json:"token"`
}

type UserList struct {
	Users []common.User `bson:"users" json:"users"`
}

type CapabilityCheck struct {
	Capability string
	IsAllowed  bool
}

// for sending notification using api
type Notification struct {
	Recipient primitive.ObjectID   `json:"recipient,omitempty"`
	Topic     string               `json:"topic,omitempty"`
	Header    string               `json:"header,omitempty"`
	Content   string               `json:"content,omitempty"`
	Status    string               `json:"status,omitempty"` //read - unread
	Touched   bool                 `json:"touched,omitempty"`
	Type      string               `json:"type,omitempty"` //fcm
	Service   string               `json:"service,omitempty"`
	Data      map[string]string    `json:"data,omitempty"`
	Hidden    bool                 `json:"hidden,omitempty"`
	IsAction  bool                 `json:"isAction,omitempty"`
	UserIds   []primitive.ObjectID `json:"userIds,omitempty"`
}

type NotificationMsg struct {
	XClient         string
	XService        string
	Header          string
	Content         string
	Data            map[string]string
	Status          string
	Touched         bool
	UserIds         []primitive.ObjectID
	Topic           string
	Type            string
	CreatedBy       primitive.ObjectID
	Image           string
	Icon            string
	Badge           *int
	AndroidColor    string
	AndroidSound    string
	AndroidPriority string
	IOSSound        string
	ClickActionURL  string
	WebLink         string
	DeepLink        string
}
