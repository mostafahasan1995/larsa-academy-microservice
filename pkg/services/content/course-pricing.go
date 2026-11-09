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

func (c *coursesvcs) AddCoursePricing(ctx context.Context, courseId string, data *models.CoursePricingDto) (*models.CoursePricing, error) {
	// Add authorization check - admin only
	canManage, err := c.authSvc.CanManageCoursePricing(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, errors.New("unauthorized to manage course pricing")
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	coursePricing := &models.CoursePricing{
		Id:               primitive.NewObjectID(),
		CourseId:         _id,
		CoursePricingDto: *data,
		CreatedAt:        time.Now(),
		CreatedBy:        cfg.User.Id,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	if err := c.coursePricingRepo.Add(ctx, coursePricing); err != nil {
		return nil, err
	}

	return coursePricing, nil
}

func (c *coursesvcs) UpdateCoursePricing(ctx context.Context, courseId string, pricingId string, data *models.CoursePricingDto) (*models.CoursePricing, error) {
	// Add authorization check - admin only
	canManage, err := c.authSvc.CanManageCoursePricing(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, errors.New("unauthorized to manage course pricing")
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	pId, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return nil, errors.New("invalid pricing id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	coursePricing := &models.CoursePricing{
		Id:               pId,
		CourseId:         cId,
		CoursePricingDto: *data,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	filter := bson.M{"_id": pId, "courseId": cId, "trash": false}
	update := bson.M{"$set": coursePricing}

	updatedCoursePricing, err := c.coursePricingRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedCoursePricing, nil
}

func (c *coursesvcs) DeleteCoursePricing(ctx context.Context, courseId string, pricingId string) error {
	// Add authorization check - admin only
	canManage, err := c.authSvc.CanManageCoursePricing(ctx, courseId)
	if err != nil {
		return err
	}
	if !canManage {
		return errors.New("unauthorized to manage course pricing")
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}

	pId, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return errors.New("invalid pricing id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": pId, "courseId": cId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.coursePricingRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (c *coursesvcs) GetCoursePricing(ctx context.Context, courseId string, skip, limit int64) (*models.CoursePricingPagination, error) {
	// Add authorization check - users who can view the course can view its pricing
	canView, err := c.authSvc.CanViewCourse(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view this course")
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	filter := bson.M{"courseId": cId, "trash": false}
	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := c.coursePricingRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.CoursePricing
	if err := c.coursePricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
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

	return &models.CoursePricingPagination{
		CoursePricings: result,
		Pagination:     pagination,
	}, nil
}
