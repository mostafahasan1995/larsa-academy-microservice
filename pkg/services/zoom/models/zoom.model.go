package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ZoomAccount struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Email       string             `bson:"email" json:"email"`
	AccountName string             `bson:"accountName" json:"accountName"`

	// Status: "active", "maintenance", "suspended"
	Status string `bson:"status" json:"status"`

	// Server-to-Server OAuth Credentials
	Credentials struct {
		AccountId     string `bson:"accountId" json:"-"`
		ClientId      string `bson:"clientId" json:"-"`
		ClientSecret  string `bson:"clientSecret" json:"-"`
		WebhookSecret string `bson:"webhookSecret" json:"-"`
	} `bson:"credentials" json:"-"`

	CreatedAt time.Time `bson:"createdAt" json:"createdAt"`
}

func (z *ZoomAccount) CollName() string {
	return "academyZoomAccounts"
}

type ZoomUser struct {
	Id            primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	ZoomAccountId primitive.ObjectID `bson:"zoomAccountId" json:"zoomAccountId"`
	Email         string             `bson:"email" json:"email"`
	ZoomUserId    string             `bson:"zoomUserId" json:"zoomUserId"`
	FirstName     string             `bson:"firstName" json:"firstName"`
	LastName      string             `bson:"lastName" json:"lastName"`

	// Status: "active", "inactive"
	Status string `bson:"status" json:"status"`

	LastUsedAt time.Time `bson:"lastUsedAt" json:"lastUsedAt"` // For Round-Robin
	CreatedAt  time.Time `bson:"createdAt" json:"createdAt"`
}

func (z *ZoomUser) CollName() string {
	return "academyZoomUsers"
}

type ZoomMeeting struct {
	Id primitive.ObjectID `bson:"_id,omitempty" json:"id"`

	// Resource Allocation
	ZoomAccountId primitive.ObjectID `bson:"zoomAccountId" json:"zoomAccountId"`
	ZoomUserId    primitive.ObjectID `bson:"zoomUserId" json:"zoomUserId"`

	// Polymorphic Reference (Who owns this meeting?)
	RefType string             `bson:"refType" json:"refType"` // e.g., "session", "placement_test"
	RefId   primitive.ObjectID `bson:"refId" json:"refId"`

	// Zoom Metadata
	ZoomMeetingId string `bson:"zoomMeetingId" json:"zoomMeetingId"` // 123456789
	JoinUrl       string `bson:"joinUrl" json:"joinUrl"`
	StartUrl      string `bson:"startUrl" json:"startUrl"` // Sensitive: Contains host token
	Password      string `bson:"password" json:"password"`

	// Scheduling (UTC)
	StartTime       time.Time `bson:"startTime" json:"startTime"`
	DurationMinutes int       `bson:"durationMinutes" json:"durationMinutes"`
	// OccupiedUntil = StartTime + Duration + 15m Buffer
	OccupiedUntil time.Time `bson:"occupiedUntil" json:"occupiedUntil"`

	// State
	Status string `bson:"status" json:"status"` // "scheduled", "started", "completed", "cancelled"

	// Telemetry (From Webhooks)
	ActualStartTime *time.Time `bson:"actualStartTime" json:"actualStartTime"`
	ActualEndTime   *time.Time `bson:"actualEndTime" json:"actualEndTime"`
	TeacherJoined   bool       `bson:"teacherJoined" json:"teacherJoined"`
	RecordingUrl    string     `bson:"recordingUrl,omitempty" json:"recordingUrl,omitempty"`
}

func (z *ZoomMeeting) CollName() string {
	return "academyZoomMeetings"
}
