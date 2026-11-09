package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/util"
	"strconv"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (c *coursesvcs) StartCourseStudentTest(ctx context.Context, testId string) (*models.CourseStudentTest, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	test, err := c.courseTestRepo.GetByFilter(ctx, bson.M{"_id": tId, "trash": false})
	if err != nil {
		return nil, errors.New("test not found")
	}

	// Verify enrollment
	enrollment, err := c.studentCourseRepo.GetByFilter(ctx, bson.M{
		"courseId":  test.CourseId,
		"studentId": cfg.User.Id,
		"trash":     false,
	})
	if err != nil || enrollment == nil {
		return nil, errors.New("student not enrolled in this course")
	}

	// Check if there is an existing pending test
	existing, _ := c.courseStudentTestRepo.GetByFilter(ctx, bson.M{
		"testId":    tId,
		"studentId": cfg.User.Id,
		"status":    "pending",
		"trash":     false,
	})
	if existing != nil {
		return existing, nil
	}

	studentTest := &models.CourseStudentTest{
		Id:        primitive.NewObjectID(),
		CourseId:  test.CourseId,
		TestId:    test.Id,
		StudentId: cfg.User.Id,
		Title:     test.Title,
		Mark:      test.Mark,
		Questions: test.Questions,
		Settings:  test.Settings,
		Status:    "pending",
		StartTime: time.Now(),
		CreatedAt: time.Now(),
		CreatedBy: cfg.User.Id,
		UpdatedAt: time.Now(),
		UpdatedBy: cfg.User.Id,
	}

	if err := c.courseStudentTestRepo.Add(ctx, studentTest); err != nil {
		return nil, err
	}

	return studentTest, nil
}

func (c *coursesvcs) SubmitCourseStudentTest(ctx context.Context, studentTestId string, data *models.StudentCourseTestAnswersDto) (*models.CourseStudentTest, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	stId, err := primitive.ObjectIDFromHex(studentTestId)
	if err != nil {
		return nil, errors.New("invalid student test id")
	}

	studentTest, err := c.courseStudentTestRepo.GetByFilter(ctx, bson.M{"_id": stId, "trash": false})
	if err != nil {
		return nil, errors.New("student test not found")
	}

	if studentTest.StudentId != cfg.User.Id {
		return nil, errors.New("unauthorized")
	}

	if studentTest.Status != "pending" {
		return nil, errors.New("test already " + studentTest.Status)
	}

	// Check time limit
	if studentTest.Settings.EnableTimeLimit {
		limitMin, _ := strconv.Atoi(studentTest.Settings.TimeLimit)
		if limitMin > 0 {
			deadline := studentTest.StartTime.Add(time.Duration(limitMin) * time.Minute)
			// Add a small grace period (e.g. 1 minute)
			if time.Now().After(deadline.Add(time.Minute)) {
				studentTest.Status = "timed_out"
			}
		}
	}

	if studentTest.Status == "pending" {
		studentTest.Status = "submitted"
	}

	studentTest.Answers = data.Answers
	studentTest.SubmissionDate = time.Now()
	studentTest.UpdatedAt = time.Now()
	studentTest.UpdatedBy = cfg.User.Id

	// Grade the test
	grade := 0.0
	correctCount := 0

	// Create a map for quick access to questions
	questionMap := make(map[int]models.Question)
	for _, q := range studentTest.Questions {
		questionMap[q.Num] = q
	}

	// Weight of each question
	questionWeight := 0.0
	if len(studentTest.Questions) > 0 {
		questionWeight = studentTest.Mark / float64(len(studentTest.Questions))
	}

	for _, answerDto := range data.Answers {
		question, ok := questionMap[answerDto.QuestionNum]
		if !ok {
			continue
		}

		isCorrect := false
		if question.AnswerType == "single" {
			// Treat chosenAnswers as indices into the Answers slice
			if len(answerDto.ChosenAnswers) == 1 {
				chosenIdx := answerDto.ChosenAnswers[0]
				if chosenIdx >= 0 && chosenIdx < len(question.Answers) {
					if question.Answers[chosenIdx].IsCorrect {
						isCorrect = true
					}
				}
			}
		} else if question.AnswerType == "multi" {
			// Must select ALL correct answers and NO incorrect ones.
			// chosenAnswers are treated as indices into the Answers slice.
			correctIndices := []int{}
			for idx, ans := range question.Answers {
				if ans.IsCorrect {
					correctIndices = append(correctIndices, idx)
				}
			}

			if len(answerDto.ChosenAnswers) == len(correctIndices) {
				matchCount := 0
				for _, chosenIdx := range answerDto.ChosenAnswers {
					if chosenIdx < 0 || chosenIdx >= len(question.Answers) {
						matchCount = -1
						break
					}
					for _, correctIdx := range correctIndices {
						if chosenIdx == correctIdx {
							matchCount++
							break
						}
					}
					if matchCount == -1 {
						break
					}
				}
				if matchCount == len(correctIndices) && matchCount != -1 {
					isCorrect = true
				}
			}
		}

		if isCorrect {
			correctCount++
			grade += questionWeight
		}
	}

	studentTest.Grade = grade
	studentTest.NumOfCorrectAnswers = correctCount

	filter := bson.M{"_id": stId}
	update := bson.M{"$set": studentTest}
	_, err = c.courseStudentTestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{studentTest.StudentId}, &messageModels.Notification{
		ServiceName: "academy",
		Title:       "Test graded",
		Content:     "Your teacher has graded your test.",
		Recipient:   studentTest.StudentId.Hex(),
	})

	return studentTest, nil
}

func (c *coursesvcs) GetCourseStudentTest(ctx context.Context, studentTestId string) (*models.CourseStudentTest, error) {
	stId, err := primitive.ObjectIDFromHex(studentTestId)
	if err != nil {
		return nil, errors.New("invalid student test id")
	}

	studentTest, err := c.courseStudentTestRepo.GetByFilter(ctx, bson.M{"_id": stId, "trash": false})
	if err != nil {
		return nil, errors.New("student test not found")
	}

	// Authorization check
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Student can view their own test
	if studentTest.StudentId == cfg.User.Id {
		return studentTest, nil
	}

	// Teacher/Admin check
	canManage, err := c.authSvc.CanManageCourse(ctx, studentTest.CourseId.Hex())
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, errors.New("unauthorized")
	}

	return studentTest, nil
}

func (c *coursesvcs) GetAllCourseStudentTests(ctx context.Context, testId string, query *query.Conditions) ([]models.CourseStudentTest, error) {
	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	test, err := c.courseTestRepo.GetByFilter(ctx, bson.M{"_id": tId, "trash": false})
	if err != nil {
		return nil, errors.New("test not found")
	}

	// Authorization check
	canManage, err := c.authSvc.CanManageCourse(ctx, test.CourseId.Hex())
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, errors.New("unauthorized")
	}

	filter := bson.M{"testId": tId, "trash": false}
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
		mongoFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		for k, v := range mongoFilter {
			filter[k] = v
		}
	}

	var results []models.CourseStudentTest
	err = c.courseStudentTestRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (c *coursesvcs) GetCourseTestsForStudent(ctx context.Context, courseId string) ([]models.CourseTestWithAttempt, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	enrollment, err := c.studentCourseRepo.GetByFilter(ctx, bson.M{
		"courseId":  cId,
		"studentId": cfg.User.Id,
		"trash":     false,
	})
	if err != nil || enrollment == nil {
		return nil, errors.New("student not enrolled in this course")
	}

	var tests []models.CourseTest
	err = c.courseTestRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"courseId": cId,
			"status":   "active",
			"trash":    false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &tests)
	})
	if err != nil {
		return nil, err
	}

	testIds := make([]primitive.ObjectID, len(tests))
	for i, t := range tests {
		testIds[i] = t.Id
	}

	var attempts []models.CourseStudentTest
	if len(testIds) > 0 {
		err = c.courseStudentTestRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"testId":    bson.M{"$in": testIds},
				"studentId": cfg.User.Id,
				"trash":     false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &attempts)
		})
		if err != nil {
			return nil, err
		}
	}

	// Build a map of the latest attempt per test (by SubmissionDate)
	attemptMap := make(map[primitive.ObjectID]models.CourseStudentTest)
	for _, a := range attempts {
		if existing, ok := attemptMap[a.TestId]; !ok || a.SubmissionDate.After(existing.SubmissionDate) {
			attemptMap[a.TestId] = a
		}
	}

	results := make([]models.CourseTestWithAttempt, len(tests))
	for i, t := range tests {
		res := models.CourseTestWithAttempt{
			Id:       t.Id,
			Title:    t.Title,
			Mark:     t.Mark,
			DueDate:  t.DueDate,
			Settings: t.Settings,
		}
		if attempt, ok := attemptMap[t.Id]; ok {
			res.Attempt = &attempt
		}
		results[i] = res
	}

	return results, nil
}

func (c *coursesvcs) GetCourseTestStudentSubmissions(ctx context.Context, courseId, testId string, query *query.Conditions) ([]models.CourseStudentTestRes, error) {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	tId, err := primitive.ObjectIDFromHex(testId)
	if err != nil {
		return nil, errors.New("invalid test id")
	}

	test, err := c.courseTestRepo.GetByFilter(ctx, bson.M{"_id": tId, "trash": false})
	if err != nil {
		return nil, errors.New("test not found")
	}

	if test.CourseId != cId {
		return nil, errors.New("test does not belong to this course")
	}

	canManage, err := c.authSvc.CanManageCourse(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, errors.New("unauthorized")
	}

	filter := bson.M{"testId": tId, "trash": false}
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
		mongoFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		for k, v := range mongoFilter {
			filter[k] = v
		}
	}

	populateTestStudentData := []bson.M{
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
			"picture":   "$student.generalInfo.picture.path",
			"class":     "$student.academicInfo.class",
		}},
	}

	pipeline := []bson.M{
		{"$match": filter},
	}
	pipeline = append(pipeline, populateTestStudentData...)

	var results []models.CourseStudentTestRes
	err = c.courseStudentTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	})
	if err != nil {
		return nil, err
	}

	return results, nil
}

func (c *coursesvcs) DeleteCourseStudentTest(ctx context.Context, submissionId, courseId string) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	subId, err := primitive.ObjectIDFromHex(submissionId)
	if err != nil {
		return errors.New("invalid submission id")
	}

	canDeleteAny, _ := util.HasCapability(ctx, CapabilityDeleteAnySubmission)
	canDeleteAssigned, _ := util.HasCapability(ctx, CapabilityDeleteAssignedCourseSubmissions)

	if !canDeleteAny && !canDeleteAssigned {
		return errors.New("unauthorized to delete submissions")
	}

	if !canDeleteAny {
		canManage, err := c.authSvc.CanManageCourse(ctx, courseId)
		if err != nil || !canManage {
			return errors.New("unauthorized to delete submissions for this course")
		}
	}

	submission, err := c.courseStudentTestRepo.GetByFilter(ctx, bson.M{"_id": subId, "trash": false})
	if err != nil {
		return errors.New("submission not found")
	}

	if submission.CourseId.Hex() != courseId {
		return errors.New("submission does not belong to this course")
	}

	now := time.Now()
	filter := bson.M{"_id": subId}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": now, "updatedBy": cfg.User.Id}}

	_, err = c.courseStudentTestRepo.Patch(ctx, filter, update)
	return err
}
