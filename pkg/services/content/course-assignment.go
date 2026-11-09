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

// GetAllCourseAssignments retrieves all assignments for a course
func (c *coursesvcs) GetAllCourseAssignments(ctx context.Context, courseId string, query *query.Conditions) ([]models.CourseAssignment, error) {
	// Add authorization check - users who can view the course can see assignments
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

	var result []models.CourseAssignment
	errAg := c.courseAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}
	return result, nil
}

// GetCourseAssignment retrieves a single assignment by ID
func (c *coursesvcs) GetCourseAssignment(ctx context.Context, courseId, assignmentId string) (*models.CourseAssignment, error) {
	// Add authorization check - users who can view the course can see assignments
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

	aId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	assignment, err := c.courseAssignmentRepo.GetByFilter(ctx, bson.M{"courseId": cId, "_id": aId, "trash": false})
	if err != nil {
		return nil, err
	}
	return assignment, nil
}

// AddCourseAssignment adds a new assignment to a course
func (c *coursesvcs) AddCourseAssignment(ctx context.Context, courseId string, data *models.CourseAssignmentDto) (*models.CourseAssignment, error) {
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

	courseAssignment := &models.CourseAssignment{
		Id:                  primitive.NewObjectID(),
		CourseId:            _id,
		CourseAssignmentDto: *data,
		CreatedAt:           time.Now(),
		CreatedBy:           cfg.User.Id,
		UpdatedAt:           time.Now(),
		UpdatedBy:           cfg.User.Id,
	}

	if err := c.courseAssignmentRepo.Add(ctx, courseAssignment); err != nil {
		return nil, err
	}

	return courseAssignment, nil
}

// UpdateCourseAssignment updates an existing course assignment
func (c *coursesvcs) UpdateCourseAssignment(ctx context.Context, courseId, assignmentId string, data *models.CourseAssignmentDto) (*models.CourseAssignment, error) {
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

	aId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	courseAssignment := &models.CourseAssignment{
		Id:                  aId,
		CourseId:            cId,
		CourseAssignmentDto: *data,
		UpdatedAt:           time.Now(),
		UpdatedBy:           cfg.User.Id,
	}

	filter := bson.M{"courseId": cId, "_id": aId, "trash": false}
	update := bson.M{"$set": courseAssignment}

	updatedCourseAssignment, err := c.courseAssignmentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return updatedCourseAssignment, nil
}

// DeleteCourseAssignment soft deletes a course assignment
func (c *coursesvcs) DeleteCourseAssignment(ctx context.Context, courseId, assignmentId string) error {
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

	aId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return errors.New("invalid assignment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"courseId": cId, "_id": aId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.courseAssignmentRepo.Patch(ctx, filter, update)
	return err
}
