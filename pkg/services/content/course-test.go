package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetAllCourseTests retrieves all tests for a course
func (c *coursesvcs) GetAllCourseTests(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseTest, error) {
	// Add authorization check - users who can view the course can see tests
	canView, err := c.authSvc.CanViewCourse(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view this course")
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"courseId": _id, "trash": false}},
		{"$match": filter},
	}

	var result []models.CourseTest
	errAg := c.courseTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}
	return result, nil
}

// GetCourseTest retrieves a single test by ID
func (c *coursesvcs) GetCourseTest(ctx context.Context, courseId, testId string) (*models.CourseTest, error) {
	// Add authorization check - users who can view the course can see tests
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

	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	test, err := c.courseTestRepo.GetByFilter(ctx, bson.M{"courseId": cId, "_id": tId, "trash": false})
	if err != nil {
		return nil, err
	}
	return test, nil
}

// AddCourseTest adds a new test to a course
func (c *coursesvcs) AddCourseTest(ctx context.Context, courseId string, data *models.CourseTestDto) (*models.CourseTest, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to edit course resources"))
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseTest := &models.CourseTest{
		Id:            primitive.NewObjectID(),
		CourseId:      _id,
		CourseTestDto: *data,
		CreatedAt:     time.Now(),
		CreatedBy:     cfg.User.Id,
		UpdatedAt:     time.Now(),
		UpdatedBy:     cfg.User.Id,
	}

	if err := c.courseTestRepo.Add(ctx, courseTest); err != nil {
		return nil, err
	}

	return courseTest, nil
}

// UpdateCourseTest updates an existing course test
func (c *coursesvcs) UpdateCourseTest(ctx context.Context, courseId, testId string, data *models.CourseTestDto) (*models.CourseTest, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to edit course resources"))
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseTest := &models.CourseTest{
		Id:            tId,
		CourseId:      cId,
		CourseTestDto: *data,
		UpdatedAt:     time.Now(),
		UpdatedBy:     cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": tId, "trash": false}
	update := bson.M{"$set": courseTest}

	updatedCourseTest, err := c.courseTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return updatedCourseTest, nil
}

// DeleteCourseTest soft deletes a course test
func (c *coursesvcs) DeleteCourseTest(ctx context.Context, courseId, testId string) error {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return err
	}
	if !canEdit {
		return errors.New("unauthorized to edit course resources")
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}

	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return errors.New("invalid test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": tId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseTestRepo.Patch(ctx, filter, update)
	return err
}

