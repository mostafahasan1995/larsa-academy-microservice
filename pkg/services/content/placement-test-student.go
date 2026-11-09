package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/constant"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (p *placementtestsvcs) SubmitPlacementTestAnswers(ctx context.Context, data *models.StudentPlacementTestAnswers) (*models.StudentPlacementTest, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	placementTest, err := p.GetById(ctx, data.PlacementTestId.Hex())
	if err != nil {
		return nil, constant.ErrPlacementTestNotFound
	}

	if placementTest.Type == "online" {
		return nil, constant.ErrOnlineTestNoAutoSubmit
	}

	count, err := p.studentPlacementTestRepo.Count(ctx, bson.M{
		"studentId":       cfg.User.Id,
		"placementTestId": placementTest.Id,
		"trash":           false,
	})
	if err != nil {
		return nil, errors.New("error count student placement test")
	}

	submitionNum := count + 1

	// Build map of correct answer indices per question
	correctAnswersMap := map[int][]int{}
	for _, q := range placementTest.Questions {
		var correctIndices []int
		for _, ans := range q.Answers {
			if ans.IsCorrect {
				correctIndices = append(correctIndices, ans.Num)
			}
		}
		if len(correctIndices) > 0 {
			correctAnswersMap[q.Num] = correctIndices
		}
	}

	answers := data.Answers

	var numOfCorrectAnswers, NumOfWrongAnswers int
	for _, answer := range answers {
		correctIndices, ok := correctAnswersMap[answer.QuestionNum]
		if !ok {
			continue
		}

		// Normalize chosen answers: prefer the new chosenAnswers array, fall back to single ChosenAnswer
		chosen := answer.ChosenAnswers
		if len(chosen) == 0 && answer.ChosenAnswer != 0 {
			chosen = []int{answer.ChosenAnswer}
		}

		isCorrect := false
		if len(correctIndices) == 1 {
			// Single-answer question: exactly one choice must match the correct index
			if len(chosen) == 1 && chosen[0] == correctIndices[0] {
				isCorrect = true
			}
		} else {
			// Multi-answer question: all and only correct indices must be selected
			if len(chosen) == len(correctIndices) {
				matchCount := 0
				for _, chosenIdx := range chosen {
					found := false
					for _, correctIdx := range correctIndices {
						if chosenIdx == correctIdx {
							found = true
							break
						}
					}
					if !found {
						matchCount = -1
						break
					}
					matchCount++
				}
				if matchCount == len(correctIndices) && matchCount != -1 {
					isCorrect = true
				}
			}
		}

		if isCorrect {
			numOfCorrectAnswers++
		} else {
			NumOfWrongAnswers++
		}
	}

	if len(placementTest.Questions) == 0 {
		return nil, constant.ErrPlacementTestNoQuestions
	}

	score := float64(numOfCorrectAnswers) / float64(len(placementTest.Questions)) * 100
	result := "fail"
	if score >= float64(placementTest.Settings.PassScore) {
		result = "pass"
	}

	// Update isCorrect field for each answer based on the correct answers
	for i, answer := range data.Answers {
		correctIndices, exists := correctAnswersMap[answer.QuestionNum]
		if !exists {
			continue
		}

		chosen := answer.ChosenAnswers
		if len(chosen) == 0 && answer.ChosenAnswer != 0 {
			chosen = []int{answer.ChosenAnswer}
		}

		isCorrect := false
		if len(correctIndices) == 1 {
			if len(chosen) == 1 && chosen[0] == correctIndices[0] {
				isCorrect = true
			}
		} else {
			if len(chosen) == len(correctIndices) {
				matchCount := 0
				for _, chosenIdx := range chosen {
					found := false
					for _, correctIdx := range correctIndices {
						if chosenIdx == correctIdx {
							found = true
							break
						}
					}
					if !found {
						matchCount = -1
						break
					}
					matchCount++
				}
				if matchCount == len(correctIndices) && matchCount != -1 {
					isCorrect = true
				}
			}
		}

		data.Answers[i].IsCorrect = isCorrect
	}

	teacherNullable := types.Nullable[primitive.ObjectID]{}
	teacherNullable.Value = &data.TeacherId

	studentPlacementTest := &models.StudentPlacementTest{
		Id:              primitive.NewObjectID(),
		RefType:         placementTest.RefType,
		RefId:           placementTest.RefId,
		StudentId:       cfg.User.Id,
		TeacherId:       teacherNullable,
		PlacementTestId: placementTest.Id,
		Type:            placementTest.Type,
		PlacementTest:   *placementTest,
		SubmitAt:        time.Now(),
		TotalTime:       data.TotalTime,
		Status:          enums.StudentPlacementTestStatusCompleted,
		StudentAnswers:  data.Answers,
		Score:           score,
		Result:          result,
		SubmissionNum:   submitionNum,
		UpdatedAt:       time.Now(),
		Trash:           false,
	}

	if err := p.studentPlacementTestRepo.Add(ctx, studentPlacementTest); err != nil {
		return nil, err
	}

	return studentPlacementTest, nil
}

func (p *placementtestsvcs) OnlinePlacementTestSchedule(ctx context.Context, data *models.OnlinePlacementTestScheduleDto) (*models.OnlinePlacementTestSchedule, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	placementTest, err := p.GetById(ctx, data.PlacementTestId.Hex())
	if err != nil {
		return nil, constant.ErrPlacementTestNotFound
	}
	if placementTest.Type != "online" {
		return nil, constant.ErrPlacementTestNotOnline
	}
	if placementTest.RefType != "coaching" || placementTest.RefId != data.CoachingProgramId {
		return nil, constant.ErrCoachingProgramMismatch
	}
	// attempt limits and duplicate pending/scheduled checks
	// Only count attempts that actually progressed to a session, not mere requests
	attemptCount, err := p.studentPlacementTestRepo.Count(ctx, bson.M{
		"studentId":       cfg.User.Id,
		"placementTestId": placementTest.Id,
		"status": bson.M{"$in": []enums.StudentPlacementTestStatus{
			enums.StudentPlacementTestStatusInProgress,
			enums.StudentPlacementTestStatusCompleted,
			enums.StudentPlacementTestStatusEvaluated,
			enums.StudentPlacementTestStatusMissed,
		}},
		"trash": false,
	})
	if err != nil {
		return nil, err
	}
	if !placementTest.Settings.AllowRetry && attemptCount > 0 {
		return nil, constant.ErrRetryNotAllowed
	}
	if placementTest.Settings.NumOfAttempts > 0 && attemptCount >= int64(placementTest.Settings.NumOfAttempts) {
		return nil, constant.ErrMaxAttemptsReached
	}
	existingPending, _ := p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{
		"studentId":       cfg.User.Id,
		"placementTestId": placementTest.Id,
		"status": bson.M{"$in": []enums.StudentPlacementTestStatus{
			enums.StudentPlacementTestStatusPending,
			enums.StudentPlacementTestStatusScheduled,
			enums.StudentPlacementTestStatusInProgress,
		}},
		"trash": false,
	})
	if existingPending != nil {
		return nil, constant.ErrPendingTestExists
	}

	// basic time validation
	if err := validateTimeFormat(data.From); err != nil {
		return nil, fmt.Errorf("invalid 'from' time: %v", err)
	}
	if err := validateTimeFormat(data.To); err != nil {
		return nil, fmt.Errorf("invalid 'to' time: %v", err)
	}
	fromParts := strings.Split(data.From, ":")
	toParts := strings.Split(data.To, ":")
	fromHour, _ := strconv.Atoi(fromParts[0])
	fromMin, _ := strconv.Atoi(fromParts[1])
	toHour, _ := strconv.Atoi(toParts[0])
	toMin, _ := strconv.Atoi(toParts[1])
	if fromHour*60+fromMin >= toHour*60+toMin {
		return nil, constant.ErrFromTimeAfterToTime
	}

	onlinePlacementTestSchedule := &models.OnlinePlacementTestSchedule{
		Id:                             primitive.NewObjectID(),
		StudentId:                      cfg.User.Id,
		PlacementTestId:                placementTest.Id,
		Status:                         enums.StudentPlacementTestStatusPending,
		RequestedAt:                    time.Now(),
		OnlinePlacementTestScheduleDto: *data,
	}

	// Validate coaching program exists
	if _, err := p.coachingSvcs.GetById(ctx, data.CoachingProgramId.Hex(), false); err != nil {
		return nil, err
	}

	// If teacher provided, validate the teacher belongs to the coaching program.
	// Keep status as pending; approval must be done via the admin/supervisor flow.
	if data.TeacherId.Value != nil {
		coachingProgramId, err := primitive.ObjectIDFromHex(data.CoachingProgramId.Hex())
		if err != nil {
			return nil, err
		}
		teacherIds, err := p.teacherAssignmentSvc.GetCoachingTeachers(ctx, coachingProgramId)
		if err != nil {
			return nil, err
		}
		_, found := util.SliceFind(teacherIds, func(c primitive.ObjectID) bool {
			return c == *data.TeacherId.Value
		})
		if !found {
			return nil, constant.ErrTeacherNotPartOfCoaching
		}
	}

	if err := p.onlinePlacementTestScheduleRepo.Add(ctx, onlinePlacementTestSchedule); err != nil {
		return nil, err
	}

	// create linked student placement test (pending, no teacher)
	spt := &models.StudentPlacementTest{
		Id:              primitive.NewObjectID(),
		RefType:         placementTest.RefType,
		RefId:           placementTest.RefId,
		StudentId:       cfg.User.Id,
		ScheduleId:      onlinePlacementTestSchedule.Id,
		TeacherId:       types.Nullable[primitive.ObjectID]{}, // nil value
		PlacementTestId: placementTest.Id,
		Type:            placementTest.Type,
		PlacementTest:   *placementTest,
		SubmitAt:        time.Now(),
		TotalTime:       0,
		Status:          enums.StudentPlacementTestStatusPending,
		Date:            func() *time.Time { d := normalizeDate(data.Date); return &d }(),
		From:            data.From,
		To:              data.To,
		RequestedAt:     time.Now(),
		UpdatedAt:       time.Now(),
		Trash:           false,
	}

	if err := p.studentPlacementTestRepo.Add(ctx, spt); err != nil {
		return nil, err
	}

	// back link
	if _, err := p.onlinePlacementTestScheduleRepo.Patch(ctx, bson.M{"_id": onlinePlacementTestSchedule.Id}, bson.M{"$set": bson.M{
		"studentPlacementTestId": spt.Id,
	}}); err != nil {
		return nil, err
	}
	onlinePlacementTestSchedule.StudentPlacementTestId = spt.Id

	// If the schedule is already marked as Scheduled (unlikely for a new request but possible if auto-approved in future),
	// we would handle Zoom creation here. For now, it's always Pending.

	return onlinePlacementTestSchedule, nil
}

// UpdatePlacementTestSchedule updates the time of a placement test schedule (for calendar events)
func (p *placementtestsvcs) UpdatePlacementTestSchedule(ctx context.Context, scheduleId string, data *models.UpdatePlacementTestScheduleDto) (*models.OnlinePlacementTestSchedule, error) {
	_id, err := primitive.ObjectIDFromHex(scheduleId)
	if err != nil {
		return nil, constant.ErrInvalidScheduleId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	schedule, err := p.onlinePlacementTestScheduleRepo.GetByFilter(ctx, bson.M{"_id": _id})
	if err != nil {
		return nil, constant.ErrScheduleNotFound
	}

	// Authorization
	isAdminAll, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)
	isSupervisorCap, _ := util.HasCapability(ctx, CapabilityManageSupervisedPlacementTests)
	isAdmin := isAdminAll || isSupervisorCap

	isTeacher := false
	if schedule.TeacherId.Value != nil && *schedule.TeacherId.Value == cfg.User.Id {
		isTeacher = true
	}

	if !isAdmin && !isTeacher {
		return nil, constant.ErrUnauthorizedUpdate
	}

	// supervisor scoping: ensure the teacher is supervised by current user
	if isSupervisorCap && !isAdminAll && schedule.TeacherId.Value != nil {
		ok, err := p.supervisorSvc.IsTeacherUnderSupervisor(ctx, *schedule.TeacherId.Value, cfg.User.Id)
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, constant.ErrUnauthorizedUpdate
		}
	}

	// load linked student placement test
	var studentTest *models.StudentPlacementTest
	if schedule.StudentPlacementTestId != primitive.NilObjectID {
		studentTest, _ = p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{"_id": schedule.StudentPlacementTestId, "trash": false})
	}
	if studentTest == nil {
		studentTest, _ = p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{"scheduleId": schedule.Id, "trash": false})
	}

	// validate time if provided
	if data.From != nil {
		if err := validateTimeFormat(*data.From); err != nil {
			return nil, fmt.Errorf("invalid 'from' time: %v", err)
		}
	}
	if data.To != nil {
		if err := validateTimeFormat(*data.To); err != nil {
			return nil, fmt.Errorf("invalid 'to' time: %v", err)
		}
	}
	if data.From != nil && data.To != nil {
		fromParts := strings.Split(*data.From, ":")
		toParts := strings.Split(*data.To, ":")
		fromHour, _ := strconv.Atoi(fromParts[0])
		fromMin, _ := strconv.Atoi(fromParts[1])
		toHour, _ := strconv.Atoi(toParts[0])
		toMin, _ := strconv.Atoi(toParts[1])
		if fromHour*60+fromMin >= toHour*60+toMin {
			return nil, constant.ErrFromTimeAfterToTime
		}
	}

	// status/state checks
	if studentTest != nil {
		if studentTest.Status == enums.StudentPlacementTestStatusCompleted || studentTest.Status == enums.StudentPlacementTestStatusEvaluated {
			if !isAdmin {
				return nil, constant.ErrCompletedTestUpdate
			}
		}
	}

	setFields := bson.M{}
	setTestFields := bson.M{}

	// status change handling
	if data.Status != "" {
		if !isAdmin {
			return nil, constant.ErrStatusChangeAuth
		}
		switch data.Status {
		case enums.StudentPlacementTestStatusScheduled:
			// approval requires teacher id
			isTeacherProvided := data.TeacherId != nil && data.TeacherId.IsProvided() && data.TeacherId.Value != nil
			hasExistingTeacher := schedule.TeacherId.Value != nil

			if !isTeacherProvided && !hasExistingTeacher {
				return nil, constant.ErrTeacherRequiredForApproval
			}
			setFields["status"] = enums.StudentPlacementTestStatusScheduled
			setFields["approvedBy"] = cfg.User.Id
			now := time.Now()
			setFields["approvedAt"] = now
			if isTeacherProvided {
				setFields["teacherId"] = data.TeacherId.Value
			}
			if studentTest != nil {
				setTestFields["status"] = enums.StudentPlacementTestStatusScheduled
				setTestFields["approvedBy"] = cfg.User.Id
				setTestFields["approvedAt"] = now
				if isTeacherProvided {
					setTestFields["teacherId"] = data.TeacherId.Value
				}
			}
		case enums.StudentPlacementTestStatusRejected:
			setFields["status"] = enums.StudentPlacementTestStatusRejected
			setFields["rejectedBy"] = cfg.User.Id
			now := time.Now()
			setFields["rejectedAt"] = now
			if data.RejectionReason != nil {
				setFields["rejectionReason"] = *data.RejectionReason
			}
			if studentTest != nil {
				setTestFields["status"] = enums.StudentPlacementTestStatusRejected
				setTestFields["rejectedBy"] = cfg.User.Id
				setTestFields["rejectedAt"] = now
				if data.RejectionReason != nil {
					setTestFields["rejectionReason"] = *data.RejectionReason
				}
			}
		default:
			// allow other transitions only if admin
			setFields["status"] = data.Status
			if studentTest != nil {
				setTestFields["status"] = data.Status
			}
		}
	}

	// teacher update (reassign) - admin/supervisor only
	if data.TeacherId != nil && data.TeacherId.IsProvided() {
		if !isAdmin {
			return nil, constant.ErrTeacherChangeAuth
		}
		if data.TeacherId.Value == nil {
			return nil, constant.ErrTeacherNull
		}
		// validate teacher belongs to coaching program
		coachingProgramId, err := primitive.ObjectIDFromHex(schedule.CoachingProgramId.Hex())
		if err != nil {
			return nil, err
		}
		teacherIds, err := p.teacherAssignmentSvc.GetCoachingTeachers(ctx, coachingProgramId)
		if err != nil {
			return nil, err
		}
		_, found := util.SliceFind(teacherIds, func(c primitive.ObjectID) bool {
			return c == *data.TeacherId.Value
		})
		if !found {
			return nil, constant.ErrTeacherNotPartOfCoaching
		}
		if isSupervisorCap && !isAdminAll {
			ok, err := p.supervisorSvc.IsTeacherUnderSupervisor(ctx, *data.TeacherId.Value, cfg.User.Id)
			if err != nil {
				return nil, err
			}
			if !ok {
				return nil, constant.ErrTeacherReassignAuth
			}
		}
		setFields["teacherId"] = data.TeacherId.Value
		if studentTest != nil {
			setTestFields["teacherId"] = data.TeacherId.Value
		}
	}

	// coaching program update (admin only)
	if data.CoachingProgramId != primitive.NilObjectID {
		if !isAdmin {
			return nil, constant.ErrCoachingChangeAuth
		}
		setFields["coachingProgramId"] = data.CoachingProgramId
		if studentTest != nil {
			setTestFields["refId"] = data.CoachingProgramId
		}
	}

	// time updates
	if data.Date != nil {
		normalizedDate := normalizeDate(*data.Date)
		setFields["date"] = normalizedDate
		if studentTest != nil {
			setTestFields["date"] = normalizedDate
		}
	}
	if data.From != nil {
		setFields["from"] = *data.From
		if studentTest != nil {
			setTestFields["from"] = *data.From
		}
	}
	if data.To != nil {
		setFields["to"] = *data.To
		if studentTest != nil {
			setTestFields["to"] = *data.To
		}
	}

	if len(setFields) == 0 && len(setTestFields) == 0 {
		return schedule, nil
	}

	setFields["updatedAt"] = time.Now()

	if len(setTestFields) > 0 {
		setTestFields["updatedAt"] = time.Now()
		if _, err := p.studentPlacementTestRepo.Patch(ctx, bson.M{"scheduleId": schedule.Id, "trash": false}, bson.M{"$set": setTestFields}); err != nil {
			return nil, err
		}
	}

	updatedSchedule, err := p.onlinePlacementTestScheduleRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": setFields})
	if err != nil {
		return nil, err
	}

	// Handle Zoom Meeting creation/cleanup
	if (data.Status != "" && data.Status != schedule.Status) || data.Date != nil || data.From != nil || data.To != nil {
		go func() {
			bgCtx := context.WithValue(context.Background(), util.ReqAppCfg, cfg)
			// If status is no longer scheduled, delete meeting
			if updatedSchedule.Status != enums.StudentPlacementTestStatusScheduled && updatedSchedule.ZoomMeetingId != nil {
				_ = p.zoomSvc.DeleteMeeting(bgCtx, *updatedSchedule.ZoomMeetingId)
				_, _ = p.onlinePlacementTestScheduleRepo.Patch(bgCtx, bson.M{"_id": updatedSchedule.Id}, bson.M{"$unset": bson.M{"zoomMeetingId": ""}})
				if studentTest != nil {
					_, _ = p.studentPlacementTestRepo.Patch(bgCtx, bson.M{"_id": studentTest.Id}, bson.M{"$unset": bson.M{"zoomMeetingId": ""}})
				}
				updatedSchedule.ZoomMeetingId = nil
			}

			// If status is scheduled and time changed or no meeting exists
			if updatedSchedule.Status == enums.StudentPlacementTestStatusScheduled {
				// If time changed and meeting exists, delete old one first
				if updatedSchedule.ZoomMeetingId != nil && (data.Date != nil || data.From != nil || data.To != nil) {
					_ = p.zoomSvc.DeleteMeeting(bgCtx, *updatedSchedule.ZoomMeetingId)
					updatedSchedule.ZoomMeetingId = nil
				}

				if updatedSchedule.ZoomMeetingId == nil {
					// Topic: Placement Test - [Student Name]
					topic := "Placement Test"
					// We need student name. For now let's use a generic topic if student lookup is not easily available,
					// or we can try to find it.
					// Coaching used userLookupSvc which is not in placementtestsvcs.
					// Let's check if we can get it from supervisorSvc or similar, or just use a good default.
					topic = fmt.Sprintf("Placement Test - %s", updatedSchedule.Id.Hex())

					// Calculate start time
					startTime := updatedSchedule.Date
					fromParts := strings.Split(updatedSchedule.From, ":")
					toParts := strings.Split(updatedSchedule.To, ":")

					if len(fromParts) == 2 {
						h, _ := strconv.Atoi(fromParts[0])
						m, _ := strconv.Atoi(fromParts[1])
						startTime = time.Date(updatedSchedule.Date.Year(), updatedSchedule.Date.Month(), updatedSchedule.Date.Day(), h, m, 0, 0, time.UTC)
					}

					duration := 60 // Default 60 mins for placement test
					if len(fromParts) == 2 && len(toParts) == 2 {
						h1, _ := strconv.Atoi(fromParts[0])
						m1, _ := strconv.Atoi(fromParts[1])
						h2, _ := strconv.Atoi(toParts[0])
						m2, _ := strconv.Atoi(toParts[1])

						diff := (h2*60 + m2) - (h1*60 + m1)
						if diff > 0 {
							duration = diff
						}
					}

					zm, err := p.zoomSvc.CreateMeeting(bgCtx, "placement-test", updatedSchedule.Id, startTime, duration, topic)
					if err == nil {
						_, _ = p.onlinePlacementTestScheduleRepo.Patch(bgCtx, bson.M{"_id": updatedSchedule.Id}, bson.M{"$set": bson.M{"zoomMeetingId": zm.Id}})
						if studentTest != nil {
							_, _ = p.studentPlacementTestRepo.Patch(bgCtx, bson.M{"_id": studentTest.Id}, bson.M{"$set": bson.M{"zoomMeetingId": zm.Id}})
						}
					}
				}
			}
		}()
	}

	if data.Status == enums.StudentPlacementTestStatusScheduled {
		_ = p.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{schedule.StudentId}, &messageModels.Notification{
			ServiceName: "academy",
			Title:       "Placement test approved",
			Content:     "Your placement test request has been approved.",
			Recipient:   schedule.StudentId.Hex(),
		})
	} else if data.Status == enums.StudentPlacementTestStatusRejected {
		_ = p.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{schedule.StudentId}, &messageModels.Notification{
			ServiceName: "academy",
			Title:       "Placement test request rejected",
			Content:     "Your placement test request has been rejected.",
			Recipient:   schedule.StudentId.Hex(),
		})
	}

	return updatedSchedule, nil
}

// DeletePlacementTestSchedule deletes a placement test schedule (for calendar events)
func (p *placementtestsvcs) DeletePlacementTestSchedule(ctx context.Context, scheduleId string) error {
	_id, err := primitive.ObjectIDFromHex(scheduleId)
	if err != nil {
		return constant.ErrInvalidScheduleId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	schedule, err := p.onlinePlacementTestScheduleRepo.GetByFilter(ctx, bson.M{"_id": _id})
	if err != nil {
		return constant.ErrScheduleNotFound
	}

	isAdminAll, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)
	isSupervisorCap, _ := util.HasCapability(ctx, CapabilityManageSupervisedPlacementTests)
	isAdmin := isAdminAll || isSupervisorCap

	if !isAdmin {
		// For non-admins, must be the teacher assigned to this schedule
		if schedule.TeacherId.Value == nil || *schedule.TeacherId.Value != cfg.User.Id {
			return constant.ErrUnauthorizedDelete
		}
	} else if isSupervisorCap && !isAdminAll {
		// supervisor: ensure teacher is under this supervisor
		if schedule.TeacherId.Value != nil {
			ok, err := p.supervisorSvc.IsTeacherUnderSupervisor(ctx, *schedule.TeacherId.Value, cfg.User.Id)
			if err != nil {
				return err
			}
			if !ok {
				return constant.ErrUnauthorizedDelete
			}
		}
	}

	// Cascade: cancel and soft-delete linked student placement test if exists
	var studentTest *models.StudentPlacementTest
	if schedule.StudentPlacementTestId != primitive.NilObjectID {
		studentTest, _ = p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{"_id": schedule.StudentPlacementTestId, "trash": false})
	}
	if studentTest == nil {
		studentTest, _ = p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{"scheduleId": schedule.Id, "trash": false})
	}
	if studentTest != nil {
		_, _ = p.studentPlacementTestRepo.Patch(ctx, bson.M{"_id": studentTest.Id}, bson.M{"$set": bson.M{
			"status":    enums.StudentPlacementTestStatusCancelled,
			"updatedAt": time.Now(),
			"trash":     true,
		}})
	}

	// Delete Zoom meeting if exists
	if schedule.ZoomMeetingId != nil {
		go func() {
			bgCtx := context.Background()
			_ = p.zoomSvc.DeleteMeeting(bgCtx, *schedule.ZoomMeetingId)
		}()
	}

	// Delete the schedule (hard delete since schedules don't have trash field)
	filter := bson.M{"_id": _id}
	err = p.onlinePlacementTestScheduleRepo.DeleteMain(ctx, filter)
	if err != nil {
		return err
	}

	return nil
}

// GetPlacementTestSchedules lists all placement test schedules with pagination and filtering
// Only admins and supervisors can access this endpoint
func (p *placementtestsvcs) GetPlacementTestSchedules(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.OnlinePlacementTestSchedulePagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Authorization: only admins and supervisors can list schedules
	isAdminAll, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)
	isSupervisorCap, _ := util.HasCapability(ctx, CapabilityManageSupervisedPlacementTests)
	isAdmin := isAdminAll || isSupervisorCap

	if !isAdmin {
		return nil, constant.ErrUnauthorizedUpdate
	}

	// Build base filter
	filter := bson.M{}

	// Convert query conditions to MongoDB filter
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("invalid query")
		}

		queryFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		if len(queryFilter) > 0 {
			filter = queryFilter
		}
	}

	// Supervisor scoping: filter by supervised teachers
	if isSupervisorCap && !isAdminAll {
		// Get all teachers supervised by this supervisor
		supervisedTeachers, err := p.supervisorSvc.GetTeachersBySupervisorId(ctx, cfg.User.Id.Hex(), nil)
		if err != nil {
			return nil, err
		}

		if len(supervisedTeachers) == 0 {
			// No supervised teachers, return empty result
			return &models.OnlinePlacementTestSchedulePagination{
				Schedules: []models.OnlinePlacementTestSchedule{},
				Pagination: types.Pagination{
					TotalPages: 0,
					PerPage:    limit,
					TotalCount: 0,
				},
			}, nil
		}

		// Filter schedules by supervised teachers
		teacherIds := make([]primitive.ObjectID, len(supervisedTeachers))
		for i, teacher := range supervisedTeachers {
			teacherIds[i] = teacher.Id
		}
		// teacherId is stored as Nullable, which stores the value directly when not null
		filter["teacherId"] = bson.M{"$in": teacherIds}
	}

	// Build pipeline
	pipeline := []bson.M{
		{"$match": filter},
	}

	// Count total
	count, err := p.onlinePlacementTestScheduleRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Add sorting, skip, and limit
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var schedules []models.OnlinePlacementTestSchedule
	if err := p.onlinePlacementTestScheduleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &schedules)
	}); err != nil {
		return nil, err
	}

	return &models.OnlinePlacementTestSchedulePagination{
		Schedules: schedules,
		Pagination: types.Pagination{
			TotalPages: math.Ceil(float64(count) / float64(limit)),
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

// GetAllStudentPlacementTests lists all student placement test submissions with pagination and filtering
// Only admins and supervisors can access this endpoint
func (p *placementtestsvcs) GetAllStudentPlacementTests(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentPlacementTestPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Authorization: only admins and supervisors can list all submissions
	isAdminAll, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)
	isSupervisorCap, _ := util.HasCapability(ctx, CapabilityManageSupervisedPlacementTests)
	isAdmin := isAdminAll || isSupervisorCap

	if !isAdmin {
		return nil, constant.ErrUnauthorizedUpdate
	}

	// Build base filter
	filter := bson.M{
		"trash": false,
	}

	// Convert query conditions to MongoDB filter
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("invalid query")
		}

		queryFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		if len(queryFilter) > 0 {
			// Merge query filter with base filter
			for k, v := range queryFilter {
				filter[k] = v
			}
		}
	}

	// Supervisor scoping: filter by supervised teachers
	if isSupervisorCap && !isAdminAll {
		// Get all teachers supervised by this supervisor
		supervisedTeachers, err := p.supervisorSvc.GetTeachersBySupervisorId(ctx, cfg.User.Id.Hex(), nil)
		if err != nil {
			return nil, err
		}

		if len(supervisedTeachers) == 0 {
			// No supervised teachers, return empty result
			return &models.StudentPlacementTestPagination{
				Submissions: []models.StudentPlacementTest{},
				Pagination: types.Pagination{
					TotalPages: 0,
					PerPage:    limit,
					TotalCount: 0,
				},
			}, nil
		}

		// Filter submissions by supervised teachers
		teacherIds := make([]primitive.ObjectID, len(supervisedTeachers))
		for i, teacher := range supervisedTeachers {
			teacherIds[i] = teacher.Id
		}
		// teacherId is stored as Nullable, which stores the value directly when not null
		filter["teacherId"] = bson.M{"$in": teacherIds}
	}

	// Build pipeline
	pipeline := []bson.M{
		{"$match": filter},
	}

	// Count total
	count, err := p.studentPlacementTestRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Add sorting, skip, and limit
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var submissions []models.StudentPlacementTest
	if err := p.studentPlacementTestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &submissions)
	}); err != nil {
		return nil, err
	}

	// Enrich embedded PlacementTest in each submission
	for i := range submissions {
		if err := p.enrichPlacementTest(ctx, &submissions[i].PlacementTest); err != nil {
			// Log error but continue
		}
	}

	return &models.StudentPlacementTestPagination{
		Submissions: submissions,
		Pagination: types.Pagination{
			TotalPages: math.Ceil(float64(count) / float64(limit)),
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

// GetStudentPlacementTestById retrieves a single student placement test submission by ID
// Authorization:
// - Students can view their own submissions
// - Teachers can view submissions they're assigned to
// - Admins and supervisors can view all submissions (with supervisor scoping)
func (p *placementtestsvcs) GetStudentPlacementTestById(ctx context.Context, id string) (*models.StudentPlacementTest, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, constant.ErrInvalidPlacementTestId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the submission
	studentPlacementTest, err := p.studentPlacementTestRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, constant.ErrStudentTestNotFound
		}
		return nil, err
	}

	// Authorization checks
	isAdminAll, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)
	isSupervisorCap, _ := util.HasCapability(ctx, CapabilityManageSupervisedPlacementTests)

	// If no user in context, it might be an internal call (enrichment)
	if cfg.User == nil {
		return studentPlacementTest, nil
	}

	// Check if user is the student
	isStudent := studentPlacementTest.StudentId == cfg.User.Id

	// Check if user is the assigned teacher
	isTeacher := false
	if studentPlacementTest.TeacherId.Value != nil && *studentPlacementTest.TeacherId.Value == cfg.User.Id {
		isTeacher = true
	}

	// Enrich embedded PlacementTest before returning
	if err := p.enrichPlacementTest(ctx, &studentPlacementTest.PlacementTest); err != nil {
		// Log error but continue
	}

	// Full admin access - can view all submissions
	if isAdminAll {
		return studentPlacementTest, nil
	}

	// For pending tests, teacherId might be nil.
	// Allow students to view their own pending/scheduled tests.
	if isStudent {
		return studentPlacementTest, nil
	}

	// Teacher can view submissions they're assigned to
	if isTeacher {
		return studentPlacementTest, nil
	}

	// Supervisor scoping: can only view submissions for their supervised teachers
	if isSupervisorCap && studentPlacementTest.TeacherId.Value != nil {
		ok, err := p.supervisorSvc.IsTeacherUnderSupervisor(ctx, *studentPlacementTest.TeacherId.Value, cfg.User.Id)
		if err != nil {
			return nil, err
		}
		if ok {
			// Supervisor can view submissions for their supervised teachers
			return studentPlacementTest, nil
		}
	}

	// No authorization - deny access
	return nil, constant.ErrUnauthorizedUpdate
}

// DeleteStudentPlacementTest soft deletes a student placement test submission (admin only)
func (p *placementtestsvcs) DeleteStudentPlacementTest(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return constant.ErrInvalidPlacementTestId
	}

	// Check admin capability - only admins can delete submissions
	isAdminAll, _ := util.HasCapability(ctx, CapabilityManageAllPlacementTests)
	if !isAdminAll {
		return constant.ErrUnauthorizedUpdate
	}

	// Soft delete by setting trash to true
	if _, err := p.studentPlacementTestRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": bson.M{"trash": true}}); err != nil {
		return err
	}

	return nil
}
