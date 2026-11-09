package models

import (
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/points/enums"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// PointTransaction records every point change (earned/redeemed/adjusted)
type PointTransaction struct {
	ID              primitive.ObjectID      `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentID       primitive.ObjectID      `bson:"studentId" json:"studentId"`
	StudentName     string                  `bson:"studentName" json:"studentName"`
	Class           string                  `bson:"class" json:"class"`
	Points          int                     `bson:"points" json:"points"` // Positive for earned, negative for redeemed
	TransactionType enums.TransactionType   `bson:"transactionType" json:"transactionType"`
	Source          enums.TransactionSource `bson:"source" json:"source"`                                 // System, Auto-Generated, or Manual
	IssuedBy        primitive.ObjectID      `bson:"issuedBy,omitempty" json:"issuedBy,omitempty"`         // Teacher/Admin ID (for manual)
	IssuedByName    string                  `bson:"issuedByName,omitempty" json:"issuedByName,omitempty"` // Teacher/Admin name (for manual)
	Comment         string                  `bson:"comment,omitempty" json:"comment,omitempty"`
	ReferenceID     primitive.ObjectID      `bson:"referenceId,omitempty" json:"referenceId,omitempty"`     // Reference to assignment, exam, redeem request, etc.
	ReferenceType   string                  `bson:"referenceType,omitempty" json:"referenceType,omitempty"` // Assignment, Exam, RedeemRequest, etc.
	CreatedAt       time.Time               `bson:"createdAt" json:"createdAt"`
	Trash           bool                    `bson:"trash" json:"trash"`
	CreatedBy       primitive.ObjectID      `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy       primitive.ObjectID      `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`

	// Populated field (not stored in database, but populated via aggregation)
	StudentObj *contactModels.Student `bson:"studentObj,omitempty" json:"studentObj,omitempty"`
}

func (p *PointTransaction) CollName() string {
	return "academyPointTransactions"
}

type PointTransactionPagination struct {
	Transactions []PointTransaction `bson:"transactions" json:"transactions"`
	Pagination   types.Pagination   `bson:"pagination" json:"pagination"`
}

// BulkPointAssignmentRequest represents a request to assign points to multiple students
type BulkPointAssignmentRequest struct {
	StudentIds      []string                `json:"studentIds" validate:"required,min=1,dive,required"` // Array of student IDs
	Points          int                     `json:"points" validate:"required"`
	Class           string                  `json:"class,omitempty"`
	TransactionType enums.TransactionType   `json:"transactionType" validate:"required"`
	Source          enums.TransactionSource `json:"source"`
	Comment         string                  `json:"comment,omitempty"`
	ReferenceID     primitive.ObjectID      `json:"referenceId,omitempty"`
	ReferenceType   string                  `json:"referenceType,omitempty"`
	IssuedBy        primitive.ObjectID      `json:"issuedBy,omitempty"`
	IssuedByName    string                  `json:"issuedByName,omitempty"`
}
