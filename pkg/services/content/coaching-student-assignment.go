package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var populateStudentData = []bson.M{
	{"$lookup": bson.M{
		"from":         "academyStudents",
		"localField":   "studentId",
		"foreignField": "_id",
		"as":           "student",
	}},
	{"$unwind": "$student"},
	{"$set": bson.M{
		"firstName": "$student.generalInfo.firstName",
		"lastName":  "$student.generalInfo.lastName",
		"email":     "$student.accountStudentSetting.email",
	}},
}

// student assignment
func (c *coachingsvcs) StartCoachingAssignment(ctx context.Context, assignmentId string) (*models.CoachingStudentAssignment, error) {
	_id, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	assignment, err := c.coachingAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		fmt.Println("error get assignment", err)
		return nil, errors.New("error get assignment")
	}

	coachingStudentAssignment := &models.CoachingStudentAssignment{
		Id:           primitive.NewObjectID(),
		CoachingId:   assignment.CoachingId,
		AssignmentId: assignment.Id,
		StudentId:    cfg.User.Id,
		Title:        assignment.Title,
		Session:      assignment.Session,
		Mark:         assignment.Mark,
		Content:      assignment.Content,
		DueDate:      assignment.DueDate,
		Status:       "pending",
	}

	if err := c.coachingStudentAssignmentRepo.Add(ctx, coachingStudentAssignment); err != nil {
		return nil, err
	}

	// Update commitment progress after creating new assignment
	_ = c.CalculateCommitmentProgress(ctx, cfg.User.Id.Hex(), coachingStudentAssignment.CoachingId.Hex())

	return coachingStudentAssignment, nil
}

func (c *coachingsvcs) SubmitCoachingStudentAssignment(ctx context.Context, studentAssignmentId string, data *models.StudentCoachingAssignmentAnswers) (*models.CoachingStudentAssignment, error) {
	_id, err := primitive.ObjectIDFromHex(studentAssignmentId)
	if err != nil {
		return nil, errors.New("invalid student assignment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	coachingStudentAssignment, err := c.coachingStudentAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student assignment")
	}

	if coachingStudentAssignment.StudentId != cfg.User.Id {
		return nil, errors.New("you are not allowed to submit this assignment")
	}

	if coachingStudentAssignment.Status == "submitted" {
		return nil, errors.New("assignment already submitted")
	}

	questions := coachingStudentAssignment.Content

	answers := data.Answers

	var numOfCorrectAnswers int
	for _, answer := range answers {
		q := questions[answer.QuestionNum]
		answers := q.Answers
		answer := answers[answer.ChosenAnswer]
		if answer.IsCorrect {
			numOfCorrectAnswers++
		}
	}

	grade := coachingStudentAssignment.Mark / len(questions) * numOfCorrectAnswers

	coachingStudentAssignment.Grade = grade
	coachingStudentAssignment.NumOfCorrectAnswers = numOfCorrectAnswers
	coachingStudentAssignment.Answers = data.Answers
	coachingStudentAssignment.Status = "submitted"
	coachingStudentAssignment.SubmissionDate = time.Now()

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": coachingStudentAssignment}

	updatedCoachingStudentAssignment, err := c.coachingStudentAssignmentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Update commitment progress after assignment submission
	_ = c.CalculateCommitmentProgress(ctx, cfg.User.Id.Hex(), coachingStudentAssignment.CoachingId.Hex())

	_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{updatedCoachingStudentAssignment.StudentId}, &messageModels.Notification{
		ServiceName: "academy",
		Title:       "Assignment graded",
		Content:     "Your teacher has graded your assignment.",
		Recipient:   updatedCoachingStudentAssignment.StudentId.Hex(),
	})

	return updatedCoachingStudentAssignment, nil

}

func (c *coachingsvcs) GetCoachingAssignmentStudentSubmissions(ctx context.Context, coachingId, assignmentId string, query *query.Conditions) ([]models.CoachingStudentAssignmentRes, error) {
	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	asId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"coachingId": cId, "assignmentId": asId, "trash": false}},
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
	}
	pipeline = append(pipeline, populateStudentData...)

	var result []models.CoachingStudentAssignmentRes
	if err := c.coachingStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil

}

func (c *coachingsvcs) GetCoachingStudentAssignment(ctx context.Context, studentAssignmentId string) (*models.CoachingStudentAssignmentRes, error) {
	_id, err := primitive.ObjectIDFromHex(studentAssignmentId)
	if err != nil {
		return nil, errors.New("invalid student assignment id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}
	pipeline = append(pipeline, populateStudentData...)

	var result []models.CoachingStudentAssignmentRes
	if err := c.coachingStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("student assignment not found")
	}

	return &result[0], nil
}

func (c *coachingsvcs) UpdateCoachingStudentAssignmentReviewStatus(ctx context.Context, studentAssignmentId string, reviewStatus string) (*models.CoachingStudentAssignment, error) {
	_id, err := primitive.ObjectIDFromHex(studentAssignmentId)
	if err != nil {
		return nil, errors.New("invalid student assignment id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate review status - allow "reviewed" or empty string (to clear)
	if reviewStatus != "" && reviewStatus != "reviewed" {
		return nil, errors.New("invalid review status. Allowed values: 'reviewed' or empty string")
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"reviewStatus": reviewStatus,
			"updatedAt":    time.Now(),
			"updatedBy":    cfg.User.Id,
		},
	}

	updatedAssignment, err := c.coachingStudentAssignmentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedAssignment, nil
}
