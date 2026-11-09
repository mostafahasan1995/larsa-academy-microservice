package points

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/messaging"
	messagingmodels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/types"
	"math"
	"time"

	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type PointTransactionSvcs interface {
	GetById(ctx context.Context, id string) (*models.PointTransaction, error)
	GetByStudentId(ctx context.Context, studentId string, skip, limit int64) (*models.PointTransactionPagination, error)
	Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.PointTransactionPagination, error)
	GetAll(ctx context.Context, conditions *query.Conditions) ([]models.PointTransaction, error)
	Add(ctx context.Context, data *models.PointTransaction) (*models.PointTransaction, error)
	BulkAssign(ctx context.Context, req *models.BulkPointAssignmentRequest) ([]models.PointTransaction, error)
	Update(ctx context.Context, id string, data *models.PointTransaction) (*models.PointTransaction, error)
	Delete(ctx context.Context, id string) error
}

type pointTransactionsvcs struct {
	transactionRepo  db.MainRepo[models.PointTransaction]
	studentPointRepo db.MainRepo[models.StudentPoint]
	withTxn          *db.WithTxn
	messageSvcs      messaging.MessageSvcs
}

func NewPointTransactionSvcs(i *do.Injector) (PointTransactionSvcs, error) {
	return &pointTransactionsvcs{
		transactionRepo:  db.NewRepo[models.PointTransaction](i),
		studentPointRepo: db.NewRepo[models.StudentPoint](i),
		withTxn:          do.MustInvoke[*db.WithTxn](i),
		messageSvcs:      do.MustInvoke[messaging.MessageSvcs](i),
	}, nil
}

func (s *pointTransactionsvcs) GetById(ctx context.Context, id string) (*models.PointTransaction, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid transaction id")
	}

	// Use aggregation to populate StudentObj
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "studentObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$studentObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
	}

	var result []models.PointTransaction
	err = s.transactionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("transaction not found")
	}

	return &result[0], nil
}

func (s *pointTransactionsvcs) GetByStudentId(ctx context.Context, studentId string, skip, limit int64) (*models.PointTransactionPagination, error) {
	_id, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}
	// Create conditions for studentId filter - use ObjectID directly
	conditions := &query.Conditions{
		Columns: []query.Column{
			{
				Name:  "studentId",
				Exp:   query.Eq,
				Value: _id,
			},
		},
	}
	return s.Get(ctx, skip, limit, conditions)
}

func (s *pointTransactionsvcs) Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.PointTransactionPagination, error) {
	filter := bson.M{"trash": false}

	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}
		queryFilter, err := conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		// Merge query filter with trash filter
		for k, v := range queryFilter {
			filter[k] = v
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "studentObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$studentObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"createdAt": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.PointTransaction `bson:"data"`
	}

	err := s.transactionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	transactions := []models.PointTransaction{}
	if len(result) > 0 {
		transactions = result[0].Data
	}
	pages := 0.0
	if total > 0 && limit > 0 {
		pages = math.Ceil(float64(total) / float64(limit))
	}
	return &models.PointTransactionPagination{
		Transactions: transactions,
		Pagination: types.Pagination{
			TotalPages: pages,
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *pointTransactionsvcs) GetAll(ctx context.Context, conditions *query.Conditions) ([]models.PointTransaction, error) {
	filter := bson.M{"trash": false}

	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}
		queryFilter, err := conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		// Merge query filter with trash filter
		for k, v := range queryFilter {
			filter[k] = v
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "studentObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$studentObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{"$sort": bson.M{"createdAt": -1}},
	}

	var result []models.PointTransaction
	err := s.transactionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *pointTransactionsvcs) Add(ctx context.Context, data *models.PointTransaction) (*models.PointTransaction, error) {
	data.ID = primitive.NewObjectID()
	data.CreatedAt = time.Now()
	data.CreatedBy = ctx.Value(util.ReqUser).(*types.User).Id
	data.UpdatedBy = ctx.Value(util.ReqUser).(*types.User).Id

	// Add transaction record
	if err := s.transactionRepo.Add(ctx, data); err != nil {
		return nil, err
	}

	// Update student point balance
	studentPointSvcs := &studentPointsvcs{studentPointRepo: s.studentPointRepo}
	if _, err := studentPointSvcs.GetByStudentId(ctx, data.StudentID.Hex()); err != nil {
		_, err := studentPointSvcs.CreateOrUpdate(ctx, &models.StudentPoint{
			ID:          primitive.NewObjectID(),
			StudentID:   data.StudentID,
			StudentName: data.StudentName,

			TotalPoints:     data.Points,
			RedeemedPoints:  0,
			RemainingPoints: data.Points,
			ThisMonthPoints: data.Points,
			CreatedAt:       time.Now(),
		})
		if err != nil {
			return nil, err
		}
		return data, nil
	}

	_, err := studentPointSvcs.UpdateBalance(ctx, data.StudentID.Hex(), int(data.Points))
	if err != nil {
		// Optional: Rollback transaction if point update fails
		// This is a potential improvement for transaction consistency
		return nil, fmt.Errorf("failed to update student points: %v", err)
	}

	if data.Points > 0 {
		_ = s.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{data.StudentID}, &messagingmodels.Notification{
			ServiceName: "academy",
			Title:       "Points earned",
			Content:     "You have earned points.",
			Recipient:   data.StudentID.Hex(),
		})
	}

	return data, nil
}

// BulkAssign assigns points to multiple students at once using a transaction
func (s *pointTransactionsvcs) BulkAssign(ctx context.Context, req *models.BulkPointAssignmentRequest) ([]models.PointTransaction, error) {
	if len(req.StudentIds) == 0 {
		return nil, errors.New("at least one student is required")
	}

	user := ctx.Value(util.ReqUser).(*types.User)
	now := time.Now()
	createdBy := user.Id

	// Prepare transactions and student point updates
	transactions := make([]any, 0, len(req.StudentIds))
	studentPointUpdates := make(map[primitive.ObjectID]*models.StudentPoint)

	// Create transaction for each student
	for _, studentIdStr := range req.StudentIds {
		// Convert string ID to ObjectID
		studentId, err := primitive.ObjectIDFromHex(studentIdStr)
		if err != nil {
			return nil, fmt.Errorf("invalid student ID %s: %v", studentIdStr, err)
		}

		transaction := &models.PointTransaction{
			ID:              primitive.NewObjectID(),
			StudentID:       studentId,
			StudentName:     "", // Will be populated if needed
			Class:           req.Class,
			Points:          req.Points,
			TransactionType: req.TransactionType,
			Source:          req.Source,
			Comment:         req.Comment,
			ReferenceID:     req.ReferenceID,
			ReferenceType:   req.ReferenceType,
			IssuedBy:        req.IssuedBy,
			IssuedByName:    req.IssuedByName,
			CreatedAt:       now,
			CreatedBy:       createdBy,
			UpdatedBy:       createdBy,
		}
		transactions = append(transactions, transaction)

		// Track student point updates
		if _, exists := studentPointUpdates[studentId]; !exists {
			studentPointUpdates[studentId] = &models.StudentPoint{
				StudentID: studentId,
			}
		}
	}

	// Execute in transaction
	result, err := s.withTxn.Exec(ctx, func(sessionCtx mongo.SessionContext) (any, error) {
		// 1. Insert all transactions
		if err := s.transactionRepo.AddMany(sessionCtx, transactions); err != nil {
			return nil, fmt.Errorf("failed to create transactions: %v", err)
		}

		// 2. Update student point balances
		for studentID := range studentPointUpdates {
			studentPointSvcs := &studentPointsvcs{studentPointRepo: s.studentPointRepo}

			// Check if student point record exists
			_, err := studentPointSvcs.GetByStudentId(sessionCtx, studentID.Hex())
			if err != nil {
				// Create new record if doesn't exist
				newSP := &models.StudentPoint{
					ID:              primitive.NewObjectID(),
					StudentID:       studentID,
					TotalPoints:     req.Points,
					RedeemedPoints:  0,
					RemainingPoints: req.Points,
					ThisMonthPoints: req.Points,
					CreatedAt:       now,
					UpdatedAt:       now,
					CreatedBy:       createdBy,
					UpdatedBy:       createdBy,
				}
				if err := s.studentPointRepo.Add(sessionCtx, newSP); err != nil {
					return nil, fmt.Errorf("failed to create student point record: %v", err)
				}
			} else {
				// Update existing record
				_, err = studentPointSvcs.UpdateBalance(sessionCtx, studentID.Hex(), req.Points)
				if err != nil {
					return nil, fmt.Errorf("failed to update student point balance: %v", err)
				}
			}
		}

		return transactions, nil
	})

	if err != nil {
		return nil, err
	}

	// Convert result to []models.PointTransaction
	transactionsResult := result.([]any)
	finalTransactions := make([]models.PointTransaction, len(transactionsResult))
	for i, t := range transactionsResult {
		finalTransactions[i] = *t.(*models.PointTransaction)
	}

	return finalTransactions, nil
}

func (s *pointTransactionsvcs) Update(ctx context.Context, id string, data *models.PointTransaction) (*models.PointTransaction, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid transaction id")
	}

	// Get existing transaction to ensure it exists and is not trashed
	existing, err := s.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	// Update only allowed fields (don't allow changing studentId or points that would affect balance)
	data.ID = existing.ID
	data.StudentID = existing.StudentID // Prevent changing student
	data.Points = existing.Points       // Prevent changing points (would affect student balance)
	data.CreatedAt = existing.CreatedAt
	data.CreatedBy = existing.CreatedBy
	data.UpdatedBy = ctx.Value(util.ReqUser).(*types.User).Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": data}
	return s.transactionRepo.Patch(ctx, filter, update)
}

func (s *pointTransactionsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid transaction id")
	}
	// Only mark as trash - do NOT affect student points
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.transactionRepo.Patch(ctx, filter, update)
	return err
}
