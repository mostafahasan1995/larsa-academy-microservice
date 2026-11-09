package models

import (
	"context"
	"larsa-academy-microservice/pkg/types"
	"log"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// StudentPoint tracks a student's point balance and summary
type StudentPoint struct {
	ID              primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentID       primitive.ObjectID `bson:"studentId" json:"studentId" `
	StudentObj      interface{}        `bson:"studentObj,omitempty" json:"studentObj,omitempty"` // Populated student details
	StudentName     string             `bson:"studentName" json:"studentName"`
	Grade           string             `bson:"grade" json:"grade"`
	Class           string             `bson:"class" json:"class"`
	TotalPoints     int                `bson:"totalPoints" json:"totalPoints"`         // Total points earned (cumulative)
	RedeemedPoints  int                `bson:"redeemedPoints" json:"redeemedPoints"`   // Points spent on rewards
	RemainingPoints int                `bson:"remainingPoints" json:"remainingPoints"` // Available points (Total - Redeemed)
	ThisMonthPoints int                `bson:"thisMonthPoints" json:"thisMonthPoints"` // Points earned this month
	CreatedAt       time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt       time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy       primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy       primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash           bool               `bson:"trash" json:"trash"`
}

func (s *StudentPoint) CollName() string {
	return "academyStudentPoints"
}
func EnsureStudentPointIndexes(db *mongo.Database) {
	coll := db.Collection("academyStudentPoints")

	indexModel := mongo.IndexModel{
		Keys: bson.D{{Key: "studentId", Value: 1}}, // index on studentId
		Options: options.Index().
			SetUnique(true).                    // enforce uniqueness
			SetName("unique_student_id_index"), // optional name
	}

	_, err := coll.Indexes().CreateOne(context.Background(), indexModel)
	if err != nil {
		log.Fatalf("failed to create index: %v", err)
	}
}

type StudentPointPagination struct {
	StudentPoints []StudentPointWithStats `bson:"studentPoints" json:"studentPoints"`
	Pagination    types.Pagination        `bson:"pagination" json:"pagination"`
}

// StudentPointWithStats extends StudentPoint with additional statistics
type StudentPointWithStats struct {
	StudentPoint      `bson:",inline"`
	EarnedLastMonth   int `bson:"earnedLastMonth" json:"earnedLastMonth"` // Points earned in the previous month
}

// PointsStatistics represents statistics for the points system
type PointsStatistics struct {
	TotalPointsDistributed int          `json:"totalPointsDistributed"`
	ActiveRewards          int          `json:"activeRewards"`
	TopStudents            []TopStudent `json:"topStudents"`
}

// TopStudent represents a top student in the statistics
type TopStudent struct {
	Rank        int    `json:"rank"`
	StudentName string `json:"studentName"`
	Grade       string `json:"grade"`
	Points      int    `json:"points"`
}
