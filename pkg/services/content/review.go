package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ReviewSvcs interface {
	GetReviewsByRef(ctx context.Context, typ string, refId string, skip, limit int64) (*models.ReviewPagination, error)
	GetReviewsByUserId(ctx context.Context, userId string, skip, limit int64) (*models.ReviewPagination, error)
	GetReviewsByUserIdWithQuery(ctx context.Context, userId string, skip, limit int64, q *query.Conditions) (*models.ReviewPagination, error)
	GetReviewsByUserIdAndCoachings(ctx context.Context, userId string, coachingIds []primitive.ObjectID, skip, limit int64) (*models.ReviewPagination, error)
	GetAverageRatingByRef(ctx context.Context, typ string, refId string) (*float64, error)
	Add(ctx context.Context, data *models.ReviewDto) (*models.Review, error)
	UpdateReview(ctx context.Context, reviewId string, data *models.ReviewDto) (*models.Review, error)
	UpdateApproval(ctx context.Context, reviewId string, data *models.ReviewApprovalDto) (*models.Review, error)
	DeleteReview(ctx context.Context, reviewId string) error
}

type reviewsvcs struct {
	repo db.MainRepo[models.Review]
}

func NewReviewSvcs(i *do.Injector) (ReviewSvcs, error) {
	return &reviewsvcs{
		repo: db.NewRepo[models.Review](i),
	}, nil
}

func (r *reviewsvcs) GetReviewsByRef(ctx context.Context, typ string, refId string, skip, limit int64) (*models.ReviewPagination, error) {
	_id, err := primitive.ObjectIDFromHex(refId)
	if err != nil {
		return nil, errors.New("invalid ref id")
	}

	filter := bson.M{"refId": _id, "type": typ, "trash": false}

	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateUserField...)

	var result []models.ReviewRes
	if err := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
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

	return &models.ReviewPagination{
		Reviews:    result,
		Pagination: pagination,
	}, nil
}

func (r *reviewsvcs) GetReviewsByUserId(ctx context.Context, userId string, skip, limit int64) (*models.ReviewPagination, error) {
	_userId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	filter := bson.M{"userId": _userId, "trash": false}

	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateUserField...)

	var result []models.ReviewRes
	if err := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
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

	return &models.ReviewPagination{
		Reviews:    result,
		Pagination: pagination,
	}, nil
}

func (r *reviewsvcs) GetReviewsByUserIdWithQuery(ctx context.Context, userId string, skip, limit int64, q *query.Conditions) (*models.ReviewPagination, error) {
	_userId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	filter := bson.M{"userId": _userId, "trash": false}

	pipeline := []bson.M{
		{"$match": filter},
	}

	// Apply query filters if provided
	if q != nil && len(q.Columns) > 0 {
		if err := q.CheckValid(); err == nil {
			queryFilter, err := q.ConvertToMongo()
			if err == nil {
				// Merge query filter with existing match conditions
				pipeline = append(pipeline, bson.M{"$match": queryFilter})
			}
		}
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateUserField...)

	result := []models.ReviewRes{}
	if err := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Ensure result is never nil (empty array instead)
	if result == nil {
		result = []models.ReviewRes{}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ReviewPagination{
		Reviews:    result,
		Pagination: pagination,
	}, nil
}

func (r *reviewsvcs) GetReviewsByUserIdAndCoachings(ctx context.Context, userId string, coachingIds []primitive.ObjectID, skip, limit int64) (*models.ReviewPagination, error) {
	_userId, err := primitive.ObjectIDFromHex(userId)
	if err != nil {
		return nil, errors.New("invalid user id")
	}

	filter := bson.M{
		"userId": _userId,
		"type":   "coaching",
		"refId":  bson.M{"$in": coachingIds},
		"trash":  false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateUserField...)

	var result []models.ReviewRes
	if err := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
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

	return &models.ReviewPagination{
		Reviews:    result,
		Pagination: pagination,
	}, nil
}

func (r *reviewsvcs) Add(ctx context.Context, data *models.ReviewDto) (*models.Review, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	review := &models.Review{
		Id:           primitive.NewObjectID(),
		UserId:       cfg.User.Id,
		Date:         time.Now(),
		ReviewDto:    *data,
		ReviewStatus: "pending",
		CreatedAt:    time.Now(),
		CreatedBy:    cfg.User.Id,
		UpdatedAt:    time.Now(),
		UpdatedBy:    cfg.User.Id,
	}

	if err := r.repo.Add(ctx, review); err != nil {
		return nil, err
	}

	return review, nil
}

func (r *reviewsvcs) UpdateReview(ctx context.Context, reviewId string, data *models.ReviewDto) (*models.Review, error) {
	_id, err := primitive.ObjectIDFromHex(reviewId)
	if err != nil {
		return nil, errors.New("invalid review id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{
		"rating":    data.Rating,
		"comment":   data.Comment,
		"updatedAt": time.Now(),
		"updatedBy": cfg.User.Id,
	}}

	updatedReview, err := r.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, errors.New("failed to update review")
	}

	return updatedReview, nil
}

func (r *reviewsvcs) UpdateApproval(ctx context.Context, reviewId string, data *models.ReviewApprovalDto) (*models.Review, error) {
	_id, err := primitive.ObjectIDFromHex(reviewId)
	if err != nil {
		return nil, errors.New("invalid review id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	filter := bson.M{"_id": _id, "trash": false}

	update := bson.M{
		"$set": bson.M{
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	// Handle approved field as plain string: "approved", "rejected", or "pending"
	if data.ReviewStatus != "" {
		// Set to the provided value
		update["$set"].(bson.M)["reviewStatus"] = data.ReviewStatus
	}

	updatedReview, err := r.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, errors.New("failed to update review approval")
	}

	return updatedReview, nil
}

func (r *reviewsvcs) DeleteReview(ctx context.Context, reviewId string) error {
	_id, err := primitive.ObjectIDFromHex(reviewId)
	if err != nil {
		return errors.New("invalid review id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	if _, err := r.repo.Patch(ctx, filter, update); err != nil {
		return errors.New("failed to delete review")
	}

	return nil
}

// GetAverageRatingByRef calculates the average rating for a reference (course/coaching/bundle)
func (r *reviewsvcs) GetAverageRatingByRef(ctx context.Context, typ string, refId string) (*float64, error) {
	_id, err := primitive.ObjectIDFromHex(refId)
	if err != nil {
		return nil, errors.New("invalid ref id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"refId": _id, "type": typ, "trash": false}},
		{"$group": bson.M{
			"_id":   nil,
			"avg":   bson.M{"$avg": "$rating"},
			"count": bson.M{"$sum": 1},
		}},
	}

	var result struct {
		Avg   float64 `bson:"avg"`
		Count int     `bson:"count"`
	}

	if err := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.Decode(&result)
	}); err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	if result.Count == 0 {
		return nil, nil // No reviews found
	}

	return &result.Avg, nil
}
