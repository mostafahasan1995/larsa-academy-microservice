package content

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/points"
	"larsa-academy-microservice/pkg/services/points/enums"
	pointsModels "larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)


type ChallengeSubmissionSvcs interface {
	SubmitChallenge(ctx context.Context, studentId string, data *models.SubmitChallengeDto) (*models.ChallengeSubmission, error)
	GetStudentSubmissions(ctx context.Context, studentId string, skip, limit int64) (*models.ChallengeSubmissionPagination, error)
	GetSubmissionById(ctx context.Context, id string) (*models.ChallengeSubmission, error)
	GetChallengeSubmissionsWithStats(ctx context.Context, challengeId string, skip, limit int64) (*models.ChallengeSubmissionStats, error)
}

type challengeSubmissionSvcs struct {
	submissionRepo       db.MainRepo[models.ChallengeSubmission]
	challengeRepo        db.MainRepo[models.Challenge]
	pointTransactionSvcs points.PointTransactionSvcs
	pointsRuleSvcs       points.PointsRuleSvcs
}

func NewChallengeSubmissionSvcs(i *do.Injector) (ChallengeSubmissionSvcs, error) {
	return &challengeSubmissionSvcs{
		submissionRepo:       db.NewRepo[models.ChallengeSubmission](i),
		challengeRepo:        db.NewRepo[models.Challenge](i),
		pointTransactionSvcs: do.MustInvoke[points.PointTransactionSvcs](i),
		pointsRuleSvcs:       do.MustInvoke[points.PointsRuleSvcs](i),
	}, nil
}

func (s *challengeSubmissionSvcs) SubmitChallenge(ctx context.Context, studentId string, data *models.SubmitChallengeDto) (*models.ChallengeSubmission, error) {
	// Parse IDs
	studentObjId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	challengeObjId, err := primitive.ObjectIDFromHex(data.ChallengeId)
	if err != nil {
		return nil, errors.New("invalid challenge id")
	}

	// Fetch challenge
	challenge, err := s.challengeRepo.GetByFilter(ctx, bson.M{"_id": challengeObjId, "trash": false})
	if err != nil {
		return nil, errors.New("challenge not found")
	}

	// Validate challenge is active
	if challenge.Settings.Status != "active" {
		return nil, errors.New("challenge is not active")
	}

	// Validate submission timing (time availability and AllowLateSubmission)
	now := time.Now()
	timeAvail := challenge.TimeAvailability
	hasTimeRange := !timeAvail.StartTime.IsZero() || !timeAvail.EndTime.IsZero()

	if hasTimeRange {
		if !timeAvail.StartTime.IsZero() && now.Before(timeAvail.StartTime) {
			return nil, errors.New("challenge has not started yet")
		}
		if !timeAvail.EndTime.IsZero() && now.After(timeAvail.EndTime) {
			if !challenge.Settings.AllowLateSubmission {
				return nil, errors.New("challenge submission period has ended")
			}
		}
	} else {
		if !challenge.Date.IsZero() {
			dateUTC := challenge.Date.UTC()
			startOfDay := time.Date(dateUTC.Year(), dateUTC.Month(), dateUTC.Day(), 0, 0, 0, 0, time.UTC)
			endOfDay := time.Date(dateUTC.Year(), dateUTC.Month(), dateUTC.Day(), 23, 59, 59, 999999999, time.UTC)
			nowUTC := now.UTC()
			if nowUTC.Before(startOfDay) {
				return nil, errors.New("challenge has not started yet")
			}
			if nowUTC.After(endOfDay) && !challenge.Settings.AllowLateSubmission {
				return nil, errors.New("challenge submission period has ended")
			}
		}
	}

	// Get max attemptNumber from existing submissions for this student
	attemptCount := int64(0)
	maxAttemptPipeline := []bson.M{
		{"$match": bson.M{
			"studentId":   studentObjId,
			"challengeId": challengeObjId,
			"trash":       false,
		}},
		{"$group": bson.M{
			"_id":         nil,
			"maxAttempt":  bson.M{"$max": "$attemptNumber"},
		}},
	}
	var maxResult []struct {
		MaxAttempt int `bson:"maxAttempt"`
	}
	if err := s.submissionRepo.Aggregate(ctx, maxAttemptPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &maxResult)
	}); err != nil {
		return nil, err
	}
	if len(maxResult) > 0 && maxResult[0].MaxAttempt > 0 {
		attemptCount = int64(maxResult[0].MaxAttempt)
	}

	// Validate attempt limits (AllowRetries first, then MaxAttempts)
	if !challenge.Settings.AllowRetries {
		if attemptCount >= 1 {
			return nil, errors.New("retries not allowed")
		}
	} else {
		if attemptCount >= int64(challenge.Settings.MaxAttempts) {
			return nil, fmt.Errorf("maximum attempts (%d) exceeded", challenge.Settings.MaxAttempts)
		}
	}

	// Calculate score
	score, _ := s.calculateScore(challenge, data.StudentAnswers)

	// Determine performance level
	studentLevel, err := s.determinePerformanceLevel(ctx, score)
	if err != nil {
		// If we can't determine level, default to empty string
		studentLevel = ""
	}

	// Calculate reward points
	rewardPoints := s.calculateRewardPoints(challenge, score)

	// Get user config for audit fields
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Populate question and answer snapshots for historical record
	enrichedAnswers := s.populateAnswerSnapshots(challenge, data.StudentAnswers)

	// Create submission record
	submission := &models.ChallengeSubmission{
		Id:             primitive.NewObjectID(),
		StudentId:      studentObjId,
		ChallengeId:    challengeObjId,
		StudentAnswers: enrichedAnswers,
		AvgScore:       score,
		RewardPoints:   rewardPoints,
		StudentLevel:   studentLevel,
		AttemptNumber:  int(attemptCount) + 1,
		Status:         "submitted",
		CreatedAt:      now,
		UpdatedAt:      now,
		CreatedBy:      cfg.User.Id,
		UpdatedBy:      cfg.User.Id,
		Trash:          false,
	}

	// Save submission
	if err := s.submissionRepo.Add(ctx, submission); err != nil {
		return nil, err
	}

	// Create point transaction if student earned points
	if rewardPoints > 0 && challenge.Settings.AutoAwardPoints {
		if err := s.createPointTransaction(ctx, submission, studentObjId); err != nil {
			// Log error but don't fail the submission
			fmt.Printf("Failed to create point transaction: %v\n", err)
		}
	}

	return submission, nil
}

func (s *challengeSubmissionSvcs) calculateScore(challenge *models.Challenge, studentAnswers []models.ChallengeStudentAnswer) (float64, int) {
	totalQuestions := len(challenge.Quiz.Questions)
	if totalQuestions == 0 {
		return 0, 0
	}

	correctCount := 0

	// Create a map of question answers for quick lookup
	questionMap := make(map[string]*models.ChallengeQuizQuestion)
	for i := range challenge.Quiz.Questions {
		questionMap[challenge.Quiz.Questions[i].Id.Hex()] = &challenge.Quiz.Questions[i]
	}

	// Check each student answer
	for _, studentAnswer := range studentAnswers {
		question, exists := questionMap[studentAnswer.QuestionId.Hex()]
		if !exists {
			continue
		}

		// Get correct answer indices
		correctIndices := []int{}
		for i, ans := range question.Answers {
			if ans.IsCorrect {
				correctIndices = append(correctIndices, i)
			}
		}

		// Check if student's answers match correct answers
		if s.arraysEqual(studentAnswer.SelectedAnswers, correctIndices) {
			correctCount++
		}
	}

	// Calculate percentage score
	score := (float64(correctCount) / float64(totalQuestions)) * 100
	return score, correctCount
}

func (s *challengeSubmissionSvcs) arraysEqual(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}

	// Create maps to compare
	aMap := make(map[int]bool)
	for _, v := range a {
		aMap[v] = true
	}

	for _, v := range b {
		if !aMap[v] {
			return false
		}
	}

	return true
}

func (s *challengeSubmissionSvcs) populateAnswerSnapshots(challenge *models.Challenge, studentAnswers []models.ChallengeStudentAnswer) []models.ChallengeStudentAnswer {
	// Create a map of questions for quick lookup
	questionMap := make(map[string]*models.ChallengeQuizQuestion)
	for i := range challenge.Quiz.Questions {
		questionMap[challenge.Quiz.Questions[i].Id.Hex()] = &challenge.Quiz.Questions[i]
	}

	// Enrich each answer with snapshots
	enrichedAnswers := make([]models.ChallengeStudentAnswer, len(studentAnswers))
	for i, answer := range studentAnswers {
		enrichedAnswers[i] = answer

		// Find the question
		question, exists := questionMap[answer.QuestionId.Hex()]
		if !exists {
			continue
		}

		// Store question snapshot
		enrichedAnswers[i].QuestionSnapshot = question.Question

		// Store all answers snapshot
		enrichedAnswers[i].AnswersSnapshot = question.Answers

		// Store selected answers text based on indices
		selectedTexts := make([]transl.Localizable[string], 0, len(answer.SelectedAnswers))
		for _, index := range answer.SelectedAnswers {
			if index >= 0 && index < len(question.Answers) {
				selectedTexts = append(selectedTexts, question.Answers[index].Answer)
			}
		}
		enrichedAnswers[i].SelectedAnswersText = selectedTexts
	}

	return enrichedAnswers
}

func (s *challengeSubmissionSvcs) determinePerformanceLevel(ctx context.Context, score float64) (string, error) {
	// Fetch PointsRule for DailyChallenge
	rules, err := s.pointsRuleSvcs.GetAll(ctx, nil)
	if err != nil {
		return "", err
	}

	var dailyChallengeRule *pointsModels.PointsRule
	for i := range rules {
		if rules[i].ActivityType == enums.TransactionTypeDailyChallenge {
			dailyChallengeRule = &rules[i]
			break
		}
	}

	if dailyChallengeRule == nil || len(dailyChallengeRule.PerformanceTiers) == 0 {
		return "", errors.New("no performance tiers found for daily challenge")
	}

	// Find matching tier
	for _, tier := range dailyChallengeRule.PerformanceTiers {
		if score >= tier.MinPercentage && score <= tier.MaxPercentage {
			return string(tier.Level), nil
		}
	}

	// Default to weakest level if no match
	return string(enums.PerformanceLevelWeak), nil
}

func (s *challengeSubmissionSvcs) calculateRewardPoints(challenge *models.Challenge, score float64) int {
	if score >= float64(challenge.Settings.NumOfScoreToEarnPoints) {
		return challenge.Settings.NumOfRewardPoints
	}
	return 0
}

func (s *challengeSubmissionSvcs) createPointTransaction(ctx context.Context, submission *models.ChallengeSubmission, studentId primitive.ObjectID) error {
	transaction := &pointsModels.PointTransaction{
		StudentID:       studentId,
		StudentName:     "", // TODO: Fetch student name from student record
		Points:          submission.RewardPoints,
		TransactionType: enums.TransactionTypeDailyChallenge,
		Source:          enums.TransactionSourceSystem,
		ReferenceID:     submission.Id,
		ReferenceType:   "dailychallenge",
	}

	_, err := s.pointTransactionSvcs.Add(ctx, transaction)
	return err
}

func (s *challengeSubmissionSvcs) GetStudentSubmissions(ctx context.Context, studentId string, skip, limit int64) (*models.ChallengeSubmissionPagination, error) {
	studentObjId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	filter := bson.M{
		"studentId": studentObjId,
		"trash":     false,
	}

	// Get total count
	count, err := s.submissionRepo.Count(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Get submissions using aggregate
	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"createdAt": -1}},
		{"$skip": skip},
		{"$limit": limit},
	}

	var submissions []models.ChallengeSubmission
	err = s.submissionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &submissions)
	})
	if err != nil {
		return nil, err
	}

	// Calculate total pages
	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.ChallengeSubmissionPagination{
		Submissions: submissions,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *challengeSubmissionSvcs) GetSubmissionById(ctx context.Context, id string) (*models.ChallengeSubmission, error) {
	objId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid submission id")
	}

	submission, err := s.submissionRepo.GetByFilter(ctx, bson.M{"_id": objId, "trash": false})
	if err != nil {
		return nil, err
	}

	return submission, nil
}

func (s *challengeSubmissionSvcs) GetChallengeSubmissionsWithStats(ctx context.Context, challengeId string, skip, limit int64) (*models.ChallengeSubmissionStats, error) {
	challengeObjId, err := primitive.ObjectIDFromHex(challengeId)
	if err != nil {
		return nil, errors.New("invalid challenge id")
	}

	filter := bson.M{
		"challengeId": challengeObjId,
		"trash":       false,
	}

	// Get all submissions for statistics calculation
	statsPipeline := []bson.M{
		{"$match": filter},
	}

	var allSubmissions []models.ChallengeSubmission
	err = s.submissionRepo.Aggregate(ctx, statsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &allSubmissions)
	})
	if err != nil {
		return nil, err
	}

	// Calculate statistics from all submissions
	totalSubmissions := len(allSubmissions)
	uniqueStudentsMap := make(map[string]bool)
	var totalScore float64
	var highestScore float64
	var lowestScore float64 = 100

	for i, sub := range allSubmissions {
		totalScore += sub.AvgScore
		uniqueStudentsMap[sub.StudentId.Hex()] = true

		if sub.AvgScore > highestScore {
			highestScore = sub.AvgScore
		}
		if sub.AvgScore < lowestScore || i == 0 {
			lowestScore = sub.AvgScore
		}
	}

	averageScore := 0.0
	if totalSubmissions > 0 {
		averageScore = totalScore / float64(totalSubmissions)
	}

	// Get paginated submissions with student lookup
	paginatedPipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"createdAt": -1}},
		// Lookup student information
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "studentInfo",
		}},
		{"$unwind": bson.M{
			"path":                       "$studentInfo",
			"preserveNullAndEmptyArrays": true,
		}},
		// Add student name field
		{"$addFields": bson.M{
			"studentName": bson.M{
				"$concat": []interface{}{
					bson.M{"$ifNull": []interface{}{"$studentInfo.generalInfo.firstName.ar", ""}},
					" ",
					bson.M{"$ifNull": []interface{}{"$studentInfo.generalInfo.lastName.ar", ""}},
				},
			},
		}},
		// Remove the temporary studentInfo field
		{"$project": bson.M{
			"studentInfo": 0,
		}},
		{"$skip": skip},
		{"$limit": limit},
	}

	var paginatedSubmissions []models.ChallengeSubmission
	err = s.submissionRepo.Aggregate(ctx, paginatedPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &paginatedSubmissions)
	})
	if err != nil {
		return nil, err
	}

	// Calculate pagination metadata
	totalPages := math.Ceil(float64(totalSubmissions) / float64(limit))

	return &models.ChallengeSubmissionStats{
		Submissions:      paginatedSubmissions,
		TotalSubmissions: totalSubmissions,
		UniqueStudents:   len(uniqueStudentsMap),
		AverageScore:     averageScore,
		HighestScore:     highestScore,
		LowestScore:      lowestScore,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: int64(totalSubmissions),
		},
	}, nil
}
