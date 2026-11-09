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

// GetAllCourseCategories retrieves a paginated list of course categories
func (c *coursesvcs) GetAllCourseCategories(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.CourseCategoryPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	// Default filter
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}

	// Add query filters
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Count total using same pipeline before pagination
	countPipeline := append([]bson.M{}, pipeline...)
	countPipeline = append(countPipeline, bson.M{"$count": "total"})

	var countResults []struct {
		Total int64 `bson:"total"`
	}
	// Use aggregate to count
	if err := c.courseCategoryRepo.Aggregate(ctx, countPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &countResults)
	}); err != nil {
		// If count fails, assume 0
		countResults = []struct {
			Total int64 `bson:"total"`
		}{}
	}

	totalCount := int64(0)
	if len(countResults) > 0 {
		totalCount = countResults[0].Total
	}

	// Pagination
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.CourseCategory
	if err := c.courseCategoryRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(totalCount) / float64(limit))
	}

	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &models.CourseCategoryPagination{
		Categories: result,
		Pagination: pagination,
	}, nil
}

// GetCourseCategory retrieves a single course category
func (c *coursesvcs) GetCourseCategory(ctx context.Context, id string) (*models.CourseCategory, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid category id")
	}

	category, err := c.courseCategoryRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("category not found")
	}

	return category, nil
}

// AddCourseCategory adds a new course category
func (c *coursesvcs) AddCourseCategory(ctx context.Context, data *models.CourseCategoryDto) (*models.CourseCategory, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	category := &models.CourseCategory{
		Id:                primitive.NewObjectID(),
		CourseCategoryDto: *data,
		CreatedAt:         time.Now(),
		CreatedBy:         cfg.User.Id,
		UpdatedAt:         time.Now(),
		UpdatedBy:         cfg.User.Id,
	}

	if err := c.courseCategoryRepo.Add(ctx, category); err != nil {
		return nil, err
	}

	return category, nil
}

// UpdateCourseCategory updates an existing course category
func (c *coursesvcs) UpdateCourseCategory(ctx context.Context, id string, data *models.CourseCategoryDto) (*models.CourseCategory, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid category id")
	}

	// Verify existence
	_, err = c.GetCourseCategory(ctx, id)
	if err != nil {
		return nil, err
	}

	category := &models.CourseCategory{
		Id:                _id,
		CourseCategoryDto: *data,
		UpdatedAt:         time.Now(),
		UpdatedBy:         cfg.User.Id,
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": category}

	updatedCategory, err := c.courseCategoryRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedCategory, nil
}

// DeleteCourseCategory soft deletes a course category
func (c *coursesvcs) DeleteCourseCategory(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid category id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseCategoryRepo.Patch(ctx, filter, update)
	return err
}
