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

// GetAllCourseSummaries retrieves all summaries for a course
func (c *coursesvcs) GetAllCourseSummaries(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseSummary, error) {
	// Add authorization check - users who can view the course can see summaries
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

	var result []models.CourseSummary
	errAg := c.courseSummaryRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}
	return result, nil
}

// GetCourseSummary retrieves a single summary by ID
func (c *coursesvcs) GetCourseSummary(ctx context.Context, courseId, summaryId string) (*models.CourseSummary, error) {
	// Add authorization check - users who can view the course can see summaries
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

	sId, err := primitive.ObjectIDFromHex(summaryId)
	if err != nil {
		return nil, errors.New("invalid summary id")
	}

	summary, err := c.courseSummaryRepo.GetByFilter(ctx, bson.M{"courseId": cId, "_id": sId, "trash": false})
	if err != nil {
		return nil, err
	}
	return summary, nil
}

// AddCourseSummary adds a new summary to a course
func (c *coursesvcs) AddCourseSummary(ctx context.Context, courseId string, data *models.CourseSummaryDto) (*models.CourseSummary, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit course resources")
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseSummary := &models.CourseSummary{
		Id:               primitive.NewObjectID(),
		CourseId:         _id,
		CourseSummaryDto: *data,
		CreatedAt:        time.Now(),
		CreatedBy:        cfg.User.Id,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	if err := c.courseSummaryRepo.Add(ctx, courseSummary); err != nil {
		return nil, err
	}

	return courseSummary, nil
}

// UpdateCourseSummary updates an existing course summary
func (c *coursesvcs) UpdateCourseSummary(ctx context.Context, courseId, summaryId string, data *models.CourseSummaryDto) (*models.CourseSummary, error) {
	// Add authorization check - users who can edit course resources
	canEdit, err := c.authSvc.CanEditCourseResources(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canEdit {
		return nil, errors.New("unauthorized to edit course resources")
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	sId, err := primitive.ObjectIDFromHex(summaryId)
	if err != nil {
		return nil, errors.New("invalid summary id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseSummary := &models.CourseSummary{
		Id:               sId,
		CourseId:         cId,
		CourseSummaryDto: *data,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": sId, "trash": false}
	update := bson.M{"$set": courseSummary}

	updatedCourseSummary, err := c.courseSummaryRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return updatedCourseSummary, nil
}

// DeleteCourseSummary soft deletes a course summary
func (c *coursesvcs) DeleteCourseSummary(ctx context.Context, courseId, summaryId string) error {
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

	sId, err := primitive.ObjectIDFromHex(summaryId)
	if err != nil {
		return errors.New("invalid summary id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": sId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseSummaryRepo.Patch(ctx, filter, update)
	return err
}

