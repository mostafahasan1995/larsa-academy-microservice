package points

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
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

type PointsRuleSvcs interface {
	GetById(ctx context.Context, id string) (*models.PointsRule, error)
	Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.PointsRulePagination, error)
	GetAll(ctx context.Context, conditions *query.Conditions) ([]models.PointsRule, error)
	Add(ctx context.Context, data *models.PointsRule) (*models.PointsRule, error)
	BulkAdd(ctx context.Context, data []*models.PointsRule) ([]*models.PointsRule, error)
	Update(ctx context.Context, id string, data *models.PointsRule) (*models.PointsRule, error)
	BulkPatch(ctx context.Context, updates []models.BulkPointsRuleUpdate) error
	Delete(ctx context.Context, id string) error
}

type pointsRulesvcs struct {
	ruleRepo db.MainRepo[models.PointsRule]
	withTxn  *db.WithTxn
}

func NewPointsRuleSvcs(i *do.Injector) (PointsRuleSvcs, error) {
	return &pointsRulesvcs{
		ruleRepo: db.NewRepo[models.PointsRule](i),
		withTxn:  do.MustInvoke[*db.WithTxn](i),
	}, nil
}

func (s *pointsRulesvcs) GetById(ctx context.Context, id string) (*models.PointsRule, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid points rule id")
	}
	return s.ruleRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
}

func (s *pointsRulesvcs) Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.PointsRulePagination, error) {
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
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"order": 1, "_id": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.PointsRule `bson:"data"`
	}

	err := s.ruleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	rules := []models.PointsRule{}
	if len(result) > 0 {
		rules = result[0].Data
	}
	pages := 0.0
	if total > 0 && limit > 0 {
		pages = math.Ceil(float64(total) / float64(limit))
	}
	return &models.PointsRulePagination{
		Rules: rules,
		Pagination: types.Pagination{
			TotalPages: pages,
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *pointsRulesvcs) GetAll(ctx context.Context, conditions *query.Conditions) ([]models.PointsRule, error) {
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
		{"$sort": bson.M{"order": 1, "_id": -1}},
	}

	var result []models.PointsRule
	err := s.ruleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *pointsRulesvcs) Add(ctx context.Context, data *models.PointsRule) (*models.PointsRule, error) {
	data.ID = primitive.NewObjectID()
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	user := ctx.Value(util.ReqUser).(*types.User)
	data.CreatedBy = user.Id
	data.UpdatedBy = user.Id
	if err := s.ruleRepo.Add(ctx, data); err != nil {
		return nil, err
	}
	return data, nil
}

// BulkAdd adds multiple points rules at once using transaction
func (s *pointsRulesvcs) BulkAdd(ctx context.Context, data []*models.PointsRule) ([]*models.PointsRule, error) {
	if len(data) == 0 {
		return nil, errors.New("no points rules provided")
	}

	user := ctx.Value(util.ReqUser).(*types.User)
	now := time.Now()

	// Prepare data for bulk insert
	insertData := make([]any, len(data))
	for i, rule := range data {
		if rule.ID.IsZero() {
			rule.ID = primitive.NewObjectID()
		}
		rule.CreatedAt = now
		rule.UpdatedAt = now
		rule.CreatedBy = user.Id
		rule.UpdatedBy = user.Id
		insertData[i] = rule
	}

	// Execute in transaction
	_, err := s.withTxn.Exec(ctx, func(sessionCtx mongo.SessionContext) (any, error) {
		return nil, s.ruleRepo.AddMany(sessionCtx, insertData)
	})

	if err != nil {
		return nil, fmt.Errorf("error in bulk add transaction: %v", err)
	}

	return data, nil
}

func (s *pointsRulesvcs) Update(ctx context.Context, id string, data *models.PointsRule) (*models.PointsRule, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid points rule id")
	}
	data.UpdatedAt = time.Now()
	user := ctx.Value(util.ReqUser).(*types.User)
	data.UpdatedBy = user.Id
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": data}
	return s.ruleRepo.Patch(ctx, filter, update)
}

// BulkPatch updates multiple points rules at once using transaction
func (s *pointsRulesvcs) BulkPatch(ctx context.Context, updates []models.BulkPointsRuleUpdate) error {
	if len(updates) == 0 {
		return errors.New("no updates provided")
	}

	user := ctx.Value(util.ReqUser).(*types.User)
	now := time.Now()

	// Prepare bulk write operations
	writeOps := make([]mongo.WriteModel, 0, len(updates))

	for _, update := range updates {
		_id, err := primitive.ObjectIDFromHex(update.ID)
		if err != nil {
			return fmt.Errorf("invalid points rule id: %s", update.ID)
		}

		filter := bson.M{"_id": _id, "trash": false}
		updateDoc := bson.M{
			"$set": bson.M{
				"updatedAt": now,
				"updatedBy": user.Id,
			},
		}

		// Add fields to update
		setDoc := updateDoc["$set"].(bson.M)
		if update.Data.Status != nil {
			setDoc["status"] = *update.Data.Status
		}
		if update.Data.Order != nil {
			setDoc["order"] = *update.Data.Order
		}
		if update.Data.ActivityName != "" {
			setDoc["activityName"] = update.Data.ActivityName
		}
		if update.Data.Description != "" {
			setDoc["description"] = update.Data.Description
		}
		if update.Data.RichText != "" {
			setDoc["richText"] = update.Data.RichText
		}
		if update.Data.Icon != "" {
			setDoc["icon"] = update.Data.Icon
		}
		if update.Data.FixedPoints != nil {
			setDoc["fixedPoints"] = *update.Data.FixedPoints
		}
		if update.Data.PerformanceTiers != nil {
			setDoc["performanceTiers"] = update.Data.PerformanceTiers
		}
		if update.Data.MonthlyPoints != nil {
			setDoc["monthlyPoints"] = update.Data.MonthlyPoints
		}
		if update.Data.QuarterlyPoints != nil {
			setDoc["quarterlyPoints"] = update.Data.QuarterlyPoints
		}
		if update.Data.ConsecutiveBonus != nil {
			setDoc["consecutiveBonus"] = *update.Data.ConsecutiveBonus
		}
		if update.Data.IsCommitmentRule != nil {
			setDoc["isCommitmentRule"] = *update.Data.IsCommitmentRule
		}

		// Create update model
		updateModel := mongo.NewUpdateOneModel().
			SetFilter(filter).
			SetUpdate(updateDoc)
		writeOps = append(writeOps, updateModel)
	}

	// Execute in transaction
	_, err := s.withTxn.Exec(ctx, func(sessionCtx mongo.SessionContext) (any, error) {
		return s.ruleRepo.BulkWrite(sessionCtx, writeOps)
	})

	if err != nil {
		return fmt.Errorf("error in bulk patch transaction: %v", err)
	}

	return nil
}

func (s *pointsRulesvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid points rule id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.ruleRepo.Patch(ctx, filter, update)
	return err
}
