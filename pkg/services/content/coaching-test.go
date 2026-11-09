package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetCoachingAllTests retrieves all coaching tests
func (c *coachingsvcs) GetCoachingAllTests(ctx context.Context, coachingId string, query *query.Conditions) ([]models.CoachingTest, error) {
	if canView, err := c.authSvc.CanViewCoaching(ctx, coachingId); err != nil || !canView {
		return nil, errors.New("unauthorized to view coaching tests")
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"coachingId": _id, "trash": false}},
		{"$match": filter},
	}

	var result []models.CoachingTest
	if err := c.coachingTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// AddCoachingTest creates a new coaching test
func (c *coachingsvcs) AddCoachingTest(ctx context.Context, coachingId string, data *models.CoachingTestDto) (*models.CoachingTest, error) {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to add coaching test")
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	test := &models.CoachingTest{
		Id:              primitive.NewObjectID(),
		CoachingId:      _id,
		CoachingTestDto: *data,
		CreatedAt:       time.Now(),
		CreatedBy:       cfg.User.Id,
		UpdatedAt:       time.Now(),
		UpdatedBy:       cfg.User.Id,
	}

	if err := c.coachingTestRepo.Add(ctx, test); err != nil {
		return nil, err
	}

	return test, nil
}

// UpdateCoachingTest updates an existing coaching test
func (c *coachingsvcs) UpdateCoachingTest(ctx context.Context, coachingId, testId string, data *models.CoachingTestDto) (*models.CoachingTest, error) {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to update coaching test")
	}

	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	test := &models.CoachingTest{
		Id:              tId,
		CoachingId:      cId,
		CoachingTestDto: *data,
		UpdatedAt:       time.Now(),
		UpdatedBy:       cfg.User.Id,
	}

	filter := bson.M{"_id": tId, "coachingId": cId, "trash": false}
	update := bson.M{"$set": test}

	updatedTest, err := c.coachingTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedTest, nil
}

// DeleteCoachingTest soft deletes a coaching test
func (c *coachingsvcs) DeleteCoachingTest(ctx context.Context, testId string) error {
	_id, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return errors.New("invalid test id")
	}

	test, err := c.coachingTestRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return errors.New("coaching test not found")
	}

	if canManage, err := c.authSvc.CanManageCoaching(ctx, test.CoachingId.Hex()); err != nil || !canManage {
		return errors.New("unauthorized to delete coaching test")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.coachingTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}
