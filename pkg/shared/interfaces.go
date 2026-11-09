package shared

import (
	"context"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type PaymentNotifier interface {
	HandlePaymentSuccess(ctx context.Context, orderId string, paymentId string) (any, error)
}

type ParentChecker interface {
	IsUserParentOfStudent(ctx context.Context, userId, studentId primitive.ObjectID) (bool, error)
	GetChildrenIds(ctx context.Context, parentId primitive.ObjectID) ([]primitive.ObjectID, error)
	GetParentUserIdsForStudent(ctx context.Context, studentId primitive.ObjectID) ([]primitive.ObjectID, error)
}
