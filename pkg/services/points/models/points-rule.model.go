package models

import (
	"larsa-academy-microservice/pkg/services/points/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PointsRule defines automated rules for earning points
type PointsRule struct {
	ID           primitive.ObjectID     `bson:"_id,omitempty" json:"_id,omitempty"`
	ActivityName string                 `bson:"activityName" json:"activityName"` // e.g., "Daily Challenge", "Assignments"
	ActivityType enums.TransactionType  `bson:"activityType" json:"activityType"`
	Category     enums.ActivityCategory `bson:"category" json:"category"` // MainActivities or AdditionalActivities
	Description  string                 `bson:"description" json:"description"`
	RichText     string                 `bson:"richText,omitempty" json:"richText,omitempty"` // Long detailed description
	Icon         string                 `bson:"icon,omitempty" json:"icon,omitempty"`         // Icon string
	Order        int                    `bson:"order" json:"order"`                           // Order for sorting activities
	Status       bool                   `bson:"status" json:"status"`                         // Active/Inactive (boolean)

	// Performance-based point tiers
	PerformanceTiers []PerformanceTier `bson:"performanceTiers,omitempty" json:"performanceTiers,omitempty"`

	// Fixed point value (if not performance-based)
	FixedPoints int `bson:"fixedPoints,omitempty" json:"fixedPoints,omitempty"`

	// Commitment-specific fields (for monthly/quarterly commitment)
	IsCommitmentRule bool              `bson:"isCommitmentRule" json:"isCommitmentRule"`
	MonthlyPoints    *CommitmentPoints `bson:"monthlyPoints,omitempty" json:"monthlyPoints,omitempty"`
	QuarterlyPoints  *CommitmentPoints `bson:"quarterlyPoints,omitempty" json:"quarterlyPoints,omitempty"`
	ConsecutiveBonus int               `bson:"consecutiveBonus,omitempty" json:"consecutiveBonus,omitempty"` // Bonus for 3 consecutive excellent months

	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash     bool               `bson:"trash" json:"trash"`
}

// PerformanceTier defines points awarded based on performance percentage
type PerformanceTier struct {
	Level         enums.PerformanceLevel `bson:"level" json:"level"` // Good, Average, Excellent
	MinPercentage float64                `bson:"minPercentage" json:"minPercentage"`
	MaxPercentage float64                `bson:"maxPercentage" json:"maxPercentage"`
	PointsAwarded int                    `bson:"pointsAwarded" json:"pointsAwarded"`
	Desc          string                 `bson:"desc,omitempty" json:"desc,omitempty"` // Description for this tier
}

// CommitmentPoints defines points for commitment evaluation
type CommitmentPoints struct {
	Weak      int `bson:"weak" json:"weak"`           // < 60%
	Average   int `bson:"average" json:"average"`     // 60% - 80%
	Excellent int `bson:"excellent" json:"excellent"` // > 80%
}

func (p *PointsRule) CollName() string {
	return "academyPointsRules"
}

type PointsRulePagination struct {
	Rules      []PointsRule     `bson:"rules" json:"rules"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

// BulkPointsRuleUpdate represents a single update in a bulk patch operation
type BulkPointsRuleUpdate struct {
	ID   string              `json:"id"`   // Points rule ID
	Data PointsRulePatchData `json:"data"` // Fields to update
}

// PointsRulePatchData contains optional fields for partial updates
type PointsRulePatchData struct {
	Status           *bool             `json:"status,omitempty"`
	Order            *int              `json:"order,omitempty"`
	ActivityName     string            `json:"activityName,omitempty"`
	Description      string            `json:"description,omitempty"`
	RichText         string            `json:"richText,omitempty"`
	Icon             string            `json:"icon,omitempty"`
	FixedPoints      *int              `json:"fixedPoints,omitempty"`
	PerformanceTiers []PerformanceTier `json:"performanceTiers,omitempty"`
	MonthlyPoints    *CommitmentPoints `json:"monthlyPoints,omitempty"`
	QuarterlyPoints  *CommitmentPoints `json:"quarterlyPoints,omitempty"`
	ConsecutiveBonus *int              `json:"consecutiveBonus,omitempty"`
	IsCommitmentRule *bool             `json:"isCommitmentRule,omitempty"`
}
