package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (c *coachingsvcs) StartCoachingStudentTest(ctx context.Context, testId string) (*models.CoachingStudentTest, error) {
	_id, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	test, err := c.coachingTestRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get test")
	}

	coachingStudentTest := &models.CoachingStudentTest{
		Id:         primitive.NewObjectID(),
		CoachingId: test.CoachingId,
		TestId:     test.Id,
		StudentId:  cfg.User.Id,
		Title:      test.Title,
		Session:    test.Session,
		Mark:       test.Mark,
		Content:    test.Content,
		DueDate:    test.DueDate,
		Status:     "pending",
		// Grade:          "",
		// SubmissionDate: time.Time{},
	}

	if err := c.coachingStudentTestRepo.Add(ctx, coachingStudentTest); err != nil {
		return nil, err
	}

	return coachingStudentTest, nil
}

func (c *coachingsvcs) SubmitCoachingStudentTest(ctx context.Context, studentTestId string, data *models.StudentCoachingTestAnswers) (*models.CoachingStudentTest, error) {
	_id, err := primitive.ObjectIDFromHex(studentTestId)
	if err != nil {
		return nil, errors.New("invalid student test id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	coachingStudentTest, err := c.coachingStudentTestRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student test")
	}

	if coachingStudentTest.StudentId != cfg.User.Id {
		return nil, errors.New("you are not allowed to submit this test")
	}

	if coachingStudentTest.Status == "submitted" {
		return nil, errors.New("test already submitted")
	}

	questions := coachingStudentTest.Content

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

	grade := coachingStudentTest.Mark / len(questions) * numOfCorrectAnswers

	coachingStudentTest.Answers = data.Answers
	coachingStudentTest.Status = "submitted"
	coachingStudentTest.SubmissionDate = time.Now()
	coachingStudentTest.Grade = grade
	coachingStudentTest.NumOfCorrectAnswers = numOfCorrectAnswers

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": coachingStudentTest}

	updatedCoachingStudentTest, err := c.coachingStudentTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{updatedCoachingStudentTest.StudentId}, &messageModels.Notification{
		ServiceName: "academy",
		Title:       "Test graded",
		Content:     "Your teacher has graded your test.",
		Recipient:   updatedCoachingStudentTest.StudentId.Hex(),
	})

	return updatedCoachingStudentTest, nil

}

func (c *coachingsvcs) GetCoachingTestStudentSubmissions(ctx context.Context, coachingId, studentTestId string, query *query.Conditions) ([]models.CoachingStudentTestRes, error) {
	cId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	tId, err := primitive.ObjectIDFromHex(studentTestId)
	if err != nil {
		return nil, errors.New("invalid student test id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"coachingId": cId, "testId": tId, "trash": false}},
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
	}
	pipeline = append(pipeline, populateStudentData...)

	var result []models.CoachingStudentTestRes
	if err := c.coachingStudentTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *coachingsvcs) GetCoachingStudentTest(ctx context.Context, studentTestId string) (*models.CoachingStudentTestRes, error) {
	_id, err := primitive.ObjectIDFromHex(studentTestId)
	if err != nil {
		return nil, errors.New("invalid student test id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}
	pipeline = append(pipeline, populateStudentData...)

	var result []models.CoachingStudentTestRes
	if err := c.coachingStudentTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("student test not found")
	}

	return &result[0], nil
}

func (c *coachingsvcs) UpdateCoachingStudentTestReviewStatus(ctx context.Context, studentTestId string, reviewStatus string) (*models.CoachingStudentTest, error) {
	_id, err := primitive.ObjectIDFromHex(studentTestId)
	if err != nil {
		return nil, errors.New("invalid student test id")
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

	updatedTest, err := c.coachingStudentTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedTest, nil
}
