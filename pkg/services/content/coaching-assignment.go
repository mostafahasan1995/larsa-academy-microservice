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

// GetCoachingAllAssignments retrieves all coaching assignments
func (c *coachingsvcs) GetCoachingAllAssignments(ctx context.Context, coachingId string, query *query.Conditions) ([]models.CoachingAssignment, error) {
	if canView, err := c.authSvc.CanViewCoaching(ctx, coachingId); err != nil || !canView {
		return nil, errors.New("unauthorized to view coaching assignments")
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

	var result []models.CoachingAssignment
	if err := c.coachingAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// AddCoachingAssignment creates a new coaching assignment
func (c *coachingsvcs) AddCoachingAssignment(ctx context.Context, coachingId string, data *models.CoachingAssignmentDto) (*models.CoachingAssignment, error) {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to add coaching assignment")
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	assignment := &models.CoachingAssignment{
		Id:                    primitive.NewObjectID(),
		CoachingId:            _id,
		CoachingAssignmentDto: *data,
		CreatedAt:             time.Now(),
		CreatedBy:             cfg.User.Id,
		UpdatedAt:             time.Now(),
		UpdatedBy:             cfg.User.Id,
	}

	if err := c.coachingAssignmentRepo.Add(ctx, assignment); err != nil {
		return nil, err
	}

	return assignment, nil
}

// UpdateCoachingAssignment updates an existing coaching assignment
func (c *coachingsvcs) UpdateCoachingAssignment(ctx context.Context, coachingId, assignmentId string, data *models.CoachingAssignmentDto) (*models.CoachingAssignment, error) {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to update coaching assignment")
	}

	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	asId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	assignment := &models.CoachingAssignment{
		Id:                    asId,
		CoachingId:            cId,
		CoachingAssignmentDto: *data,
		UpdatedAt:             time.Now(),
		UpdatedBy:             cfg.User.Id,
	}

	filter := bson.M{"_id": asId, "coachingId": cId, "trash": false}
	update := bson.M{"$set": assignment}

	updatedAssignment, err := c.coachingAssignmentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedAssignment, nil
}

// DeleteCoachingAssignment soft deletes a coaching assignment
func (c *coachingsvcs) DeleteCoachingAssignment(ctx context.Context, assignmentId string) error {
	_id, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return errors.New("invalid assignment id")
	}

	assignment, err := c.coachingAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return errors.New("coaching assignment not found")
	}

	if canManage, err := c.authSvc.CanManageCoaching(ctx, assignment.CoachingId.Hex()); err != nil || !canManage {
		return errors.New("unauthorized to delete coaching assignment")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.coachingAssignmentRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (c *coachingsvcs) GetCoachingAssignmentsForStudent(ctx context.Context, coachingId string) ([]models.CoachingAssignmentWithAttempt, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	// 1. Verify enrollment
	enrollment, err := c.coachingStudentRepo.GetByFilter(ctx, bson.M{
		"coachingId": cId,
		"studentId":  cfg.User.Id,
		"trash":      false,
	})
	if err != nil || enrollment == nil {
		return nil, errors.New("student not enrolled in this coaching program")
	}

	// 2. Get all active assignments for the coaching
	var assignments []models.CoachingAssignment
	err = c.coachingAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"coachingId": cId,
			"trash":      false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil {
		return nil, err
	}

	// 3. Get all student attempts for these assignments
	assignmentIds := make([]primitive.ObjectID, len(assignments))
	for i, a := range assignments {
		assignmentIds[i] = a.Id
	}

	var attempts []models.CoachingStudentAssignment
	if len(assignmentIds) > 0 {
		err = c.coachingStudentAssignmentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"assignmentId": bson.M{"$in": assignmentIds},
				"studentId":    cfg.User.Id,
				"trash":        false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &attempts)
		})
		if err != nil {
			return nil, err
		}
	}

	// 4. Map attempts to assignments
	attemptMap := make(map[primitive.ObjectID]models.CoachingStudentAssignment)
	for _, a := range attempts {
		attemptMap[a.AssignmentId] = a
	}

	results := make([]models.CoachingAssignmentWithAttempt, len(assignments))
	for i, a := range assignments {
		res := models.CoachingAssignmentWithAttempt{
			CoachingAssignment: a,
		}
		if attempt, ok := attemptMap[a.Id]; ok {
			res.Attempt = &attempt
		}
		results[i] = res
	}

	return results, nil
}
