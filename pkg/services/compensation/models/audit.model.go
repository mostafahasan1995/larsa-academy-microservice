package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// AuditEntry is stored inline on pricing and hours documents (append-only).
type AuditEntry struct {
	Action      string             `bson:"action" json:"action"`
	PerformedBy primitive.ObjectID `bson:"performedBy" json:"performedBy"`
	Reason      string             `bson:"reason,omitempty" json:"reason,omitempty"`
	At          time.Time          `bson:"at" json:"at"`
	OldValues   map[string]any     `bson:"oldValues,omitempty" json:"oldValues,omitempty"`
	NewValues   map[string]any     `bson:"newValues,omitempty" json:"newValues,omitempty"`
	RequestId   string             `bson:"requestId,omitempty" json:"requestId,omitempty"`
	TraceId     string             `bson:"traceId,omitempty" json:"traceId,omitempty"`
}
