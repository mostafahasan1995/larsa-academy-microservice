package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (b *bundlesvcs) AddBundlePricing(ctx context.Context, bundleId string, data *models.BundlePricingDto) (*models.BundlePricing, error) {
	if canManage, err := b.authSvc.CanManageBundlePricing(ctx, bundleId); err != nil || !canManage {
		return nil, errors.New("unauthorized to manage bundle pricing")
	}

	_id, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return nil, errors.New("invalid bundle id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	bundlePricing := &models.BundlePricing{
		Id:               primitive.NewObjectID(),
		BundleId:         _id,
		BundlePricingDto: *data,
		CreatedAt:        time.Now(),
		CreatedBy:        cfg.User.Id,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	if err := b.bundlePricingRepo.Add(ctx, bundlePricing); err != nil {
		return nil, err
	}

	return bundlePricing, nil
}

func (b *bundlesvcs) UpdateBundlePricing(ctx context.Context, bundleId string, pricingId string, data *models.BundlePricingDto) (*models.BundlePricing, error) {
	if canManage, err := b.authSvc.CanManageBundlePricing(ctx, bundleId); err != nil || !canManage {
		return nil, errors.New("unauthorized to manage bundle pricing")
	}

	bId, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return nil, errors.New("invalid bundle id")
	}

	pId, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return nil, errors.New("invalid pricing id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	bundlePricing := &models.BundlePricing{
		Id:               pId,
		BundleId:         bId,
		BundlePricingDto: *data,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	filter := bson.M{"_id": pId, "bundleId": bId, "trash": false}
	update := bson.M{"$set": bundlePricing}

	updatedBundlePricing, err := b.bundlePricingRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedBundlePricing, nil
}

func (b *bundlesvcs) DeleteBundlePricing(ctx context.Context, bundleId string, pricingId string) error {
	if canManage, err := b.authSvc.CanManageBundlePricing(ctx, bundleId); err != nil || !canManage {
		return errors.New("unauthorized to manage bundle pricing")
	}

	bId, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return errors.New("invalid bundle id")
	}

	pId, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return errors.New("invalid pricing id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": pId, "bundleId": bId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = b.bundlePricingRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (b *bundlesvcs) GetBundlePricing(ctx context.Context, bundleId string, skip, limit int64) (*models.BundlePricingPagination, error) {
	if canView, err := b.authSvc.CanViewBundle(ctx, bundleId); err != nil || !canView {
		return nil, errors.New("unauthorized to view bundle pricing")
	}

	bId, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return nil, errors.New("invalid bundle id")
	}

	filter := bson.M{"bundleId": bId, "trash": false}
	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := b.bundlePricingRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.BundlePricing
	if err := b.bundlePricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
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

	return &models.BundlePricingPagination{
		BundlePricings: result,
		Pagination:     pagination,
	}, nil
}
