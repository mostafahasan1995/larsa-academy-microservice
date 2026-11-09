package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// deactivateExistingActivePricing deactivates any existing active pricing plans based on the subscription type rules:
// - If activating a free plan: deactivate ALL other active plans
// - If activating a paid/subscription plan: deactivate other active plans of the same type
func (c *coachingsvcs) deactivateExistingActivePricing(ctx context.Context, coachingId primitive.ObjectID, subscriptionType string, excludePricingId *primitive.ObjectID) error {
	var filter bson.M

	if subscriptionType == "free" {
		// When activating a free plan, deactivate ALL other active plans (paid and subscription)
		filter = bson.M{
			"coachingId":       coachingId,
			"subscriptionType": bson.M{"$ne": "free"}, // Deactivate non-free plans
			"active":           true,
			"trash":            false,
		}
	} else {
		// When activating a paid or subscription plan, only deactivate other active plans of the same type
		filter = bson.M{
			"coachingId":       coachingId,
			"subscriptionType": subscriptionType,
			"active":           true,
			"trash":            false,
		}
	}

	// If we're updating an existing plan, exclude it from deactivation
	if excludePricingId != nil {
		filter["_id"] = bson.M{"$ne": excludePricingId}
	}

	update := bson.M{
		"$set": bson.M{
			"active":    false,
			"updatedAt": time.Now(),
		},
	}

	updateModel := mongo.NewUpdateManyModel().SetFilter(filter).SetUpdate(update)
	_, err := c.coachingPricingRepo.BulkWrite(ctx, []mongo.WriteModel{updateModel})
	return err
}

// deactivateFreePlans deactivates all active free plans for the coaching
func (c *coachingsvcs) deactivateFreePlans(ctx context.Context, coachingId primitive.ObjectID) error {
	filter := bson.M{
		"coachingId":       coachingId,
		"subscriptionType": "free",
		"active":           true,
		"trash":            false,
	}

	update := bson.M{
		"$set": bson.M{
			"active":    false,
			"updatedAt": time.Now(),
		},
	}

	updateModel := mongo.NewUpdateManyModel().SetFilter(filter).SetUpdate(update)
	_, err := c.coachingPricingRepo.BulkWrite(ctx, []mongo.WriteModel{updateModel})
	return err
}

func (c *coachingsvcs) AddCoachingPricing(ctx context.Context, coachingId string, data *models.CoachingPricingDto) (*models.CoachingPricing, error) {
	if canManage, err := c.authSvc.CanManageCoachingPricing(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to manage coaching pricing")
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// If the new pricing plan is active, check constraints and deactivate existing active plans
	if data.Active {
		// If trying to activate a non-free plan, deactivate any active free plans first
		if data.SubscriptionType != "free" {
			if err := c.deactivateFreePlans(ctx, _id); err != nil {
				return nil, err
			}
		}

		// Deactivate existing active plans based on subscription type rules
		if err := c.deactivateExistingActivePricing(ctx, _id, data.SubscriptionType, nil); err != nil {
			return nil, err
		}
	}

	coachingPricing := &models.CoachingPricing{
		Id:                 primitive.NewObjectID(),
		CoachingId:         _id,
		CoachingPricingDto: *data,
		CreatedAt:          time.Now(),
		CreatedBy:          cfg.User.Id,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
	}

	if err := c.coachingPricingRepo.Add(ctx, coachingPricing); err != nil {
		return nil, err
	}

	return coachingPricing, nil
}

func (c *coachingsvcs) UpdateCoachingPricing(ctx context.Context, coachingId string, pricingId string, data *models.CoachingPricingDto) (*models.CoachingPricing, error) {
	if canManage, err := c.authSvc.CanManageCoachingPricing(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to manage coaching pricing")
	}

	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	pId, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return nil, errors.New("invalid pricing id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// If the pricing plan is being activated, check constraints and deactivate existing active plans
	if data.Active {
		// If trying to activate a non-free plan, deactivate any active free plans first
		if data.SubscriptionType != "free" {
			if err := c.deactivateFreePlans(ctx, cId); err != nil {
				return nil, err
			}
		}

		// Deactivate existing active plans based on subscription type rules
		if err := c.deactivateExistingActivePricing(ctx, cId, data.SubscriptionType, &pId); err != nil {
			return nil, err
		}
	}

	coachingPricing := &models.CoachingPricing{
		Id:                 pId,
		CoachingId:         cId,
		CoachingPricingDto: *data,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
	}

	filter := bson.M{"_id": pId, "coachingId": cId, "trash": false}
	update := bson.M{"$set": coachingPricing}

	updatedCoachingPricing, err := c.coachingPricingRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedCoachingPricing, nil
}

func (c *coachingsvcs) DeleteCoachingPricing(ctx context.Context, coachingId string, pricingId string) error {
	if canManage, err := c.authSvc.CanManageCoachingPricing(ctx, coachingId); err != nil || !canManage {
		return errors.New("unauthorized to manage coaching pricing")
	}

	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return errors.New("invalid coaching id")
	}

	pId, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return errors.New("invalid pricing id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": pId, "coachingId": cId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.coachingPricingRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (c *coachingsvcs) GetCoachingPricing(ctx context.Context, coachingId string, skip, limit int64, q *query.Conditions) (*models.CoachingPricingPagination, error) {
	if canView, err := c.authSvc.CanViewCoaching(ctx, coachingId); err != nil || !canView {
		return nil, errors.New("unauthorized to view coaching pricing")
	}

	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	filter := bson.M{"coachingId": cId, "trash": false}

	// Merge with user-provided filters
	if q != nil {
		if err := q.CheckValid(); err != nil {
			return nil, err
		}

		userFilter, err := q.ConvertToMongo()
		if err != nil {
			return nil, err
		}

		if len(userFilter) > 0 {
			filter = bson.M{
				"$and": []bson.M{
					filter,
					userFilter,
				},
			}
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := c.coachingPricingRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.CoachingPricing
	if err := c.coachingPricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.CoachingPricingPagination{
		CoachingPricings: result,
		Pagination:       pagination,
	}, nil

}
