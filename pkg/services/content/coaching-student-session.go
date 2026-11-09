package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/query"
	contactsvcs "larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/content/constant"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	messageModels "larsa-academy-microservice/pkg/services/messaging/models"
	zoommodels "larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/pkg/services/zoom/pool"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"log/slog"
	"math"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// normalizeDate removes time components from a date, setting it to midnight UTC
func normalizeDate(date time.Time) time.Time {
	return time.Date(date.Year(), date.Month(), date.Day(), 0, 0, 0, 0, time.UTC)
}

func (c *coachingsvcs) GetStudentsPrograms(ctx context.Context, studentId string, requesterId string) ([]models.MyCoachingProgramResponse, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	_requesterId, err := primitive.ObjectIDFromHex(requesterId)
	if err != nil {
		return nil, errors.New("invalid requester id")
	}

	// Authorization check: requester must be the student or a parent of the student
	if studentId != requesterId {
		isParent, err := c.parentSvcs.IsUserParentOfStudent(ctx, _requesterId, _studentId)
		if err != nil {
			return nil, fmt.Errorf("error checking parent relationship: %w", err)
		}
		if !isParent {
			return nil, errors.New("unauthorized: requester is not the student or a parent")
		}
	}

	filter := bson.M{
		"studentId":         _studentId,
		"membershipRemoved": bson.M{"$ne": true},
	}
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": "$coachingProgramId",
			"sessions": bson.M{
				"$push": "$$ROOT",
			},
			"completedSessions": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{"$eq": []any{"$status", enums.StudentSessionStatusCompleted}},
						1,
						0,
					},
				},
			},
			"totalSessions": bson.M{"$sum": 1},
		}},
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "coachingProgram",
			},
		},
		{
			"$unwind": "$coachingProgram",
		},
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "coachingProgram.schoolId",
				"foreignField": "_id",
				"as":           "schoolObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$schoolObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$lookup": bson.M{
				"from": "academyCoachingStudents",
				"let":  bson.M{"coachingId": "$_id", "studentId": _studentId},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{
							"$and": []any{
								bson.M{"$eq": []any{"$coachingId", "$$coachingId"}},
								bson.M{"$eq": []any{"$studentId", "$$studentId"}},
								bson.M{"$eq": []any{"$trash", false}},
							},
						},
					}},
				},
				"as": "enrollment",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$enrollment",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academySupervisors",
				"localField":   "coachingProgram.supervisorIds",
				"foreignField": "_id",
				"as":           "supervisors",
			},
		},
		{
			"$project": bson.M{
				"_id":         "$coachingProgram._id",
				"title":       "$coachingProgram.title",
				"thumbnail":   "$coachingProgram.thumbnail",
				"description": "$coachingProgram.description",
				"sessions":    "$sessions",
				"coachs":      "$coachingProgram.coachs",
				"enrolledAt":  "$enrollment.createdAt",
				"schoolObj":   "$schoolObj",
				"progress": bson.M{
					"$toInt": bson.M{
						"$cond": []any{
							bson.M{"$gt": []any{"$totalSessions", 0}},
							bson.M{"$multiply": []any{
								bson.M{"$divide": []any{"$completedSessions", "$totalSessions"}},
								100,
							}},
							0,
						},
					},
				},
				"supervisors": bson.M{
					"$map": bson.M{
						"input": "$supervisors",
						"as":    "supervisor",
						"in": bson.M{
							"_id":       "$$supervisor._id",
							"firstName": "$$supervisor.firstName",
							"lastName":  "$$supervisor.lastName",
							"email":     "$$supervisor.accountSettings.email",
							"picture":   "$$supervisor.profilePicture",
						},
					},
				},
			},
		},
	}

	var result []models.MyCoachingProgramResponse
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// get student session resources - in coaching program scope
func (c *coachingsvcs) GetStudentSessionResources(ctx context.Context, coachingProgramId, resourceType string, sessionNum int) ([]models.StudentSessionResourceResponse, error) {
	cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}

	filter := bson.M{
		"coachingProgramId": cpId,
		"sessionNum":        sessionNum,
		"trash":             false,
	}

	projectStage := bson.M{
		"studentId": 1,
	}

	switch resourceType {
	case "notes":
		projectStage["notes"] = 1
	case "agendas":
		projectStage["agendas"] = 1
	case "resources":
		projectStage["resources"] = 1
	case "recordings":
		projectStage["recordings"] = 1
	default:
		return nil, errors.New("invalid resource type")
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$project": projectStage},
		{
			"$lookup": bson.M{
				"from": "academyStudents",
				"let":  bson.M{"studentId": "$studentId"},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{
							"$eq": []any{"$_id", "$$studentId"},
						},
					}},
					{"$project": bson.M{
						"generalInfo.firstName":       1,
						"generalInfo.lastName":        1,
						"accountStudentSetting.email": 1,
						"generalInfo.picture":         1,
					}},
				},
				"as": "student",
			},
		},
		{"$unwind": bson.M{
			"path":                       "$student",
			"preserveNullAndEmptyArrays": true,
		}},
	}

	var result []models.StudentSessionResourceResponse
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil

}

func (c *coachingsvcs) JustificationDecision(ctx context.Context, StudentSessionId string, data *models.JustificationDecision) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(StudentSessionId)
	if err != nil {
		return nil, errors.New("invalid session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("student session not found")
	}

	justification := studentSession.Justification

	if justification == nil {
		return nil, errors.New("no justification to make a decision")
	}
	if justification.Decision != enums.JustificationDecisionPending {
		return nil, errors.New("justification already has a decision")
	}

	justification.Decision = enums.JustificationDecision(data.Decision)
	justification.Note = data.Note
	justification.DecisionDate = time.Now()
	justification.DecisionBy = cfg.User.Id
	//
	oldStatus := studentSession.Status
	studentSession.Justification = justification
	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	if justification.Type == enums.JustificationTypeReschedule {
		if data.Decision == enums.JustificationDecisionApproved {
			rescheduleData, err := util.Converter[models.RescheduleSessionData](justification.Data)
			if err != nil {
				return nil, errors.New("error reschedule data")
			}
			studentSession.Status = enums.StudentSessionStatusScheduled
			studentSession.Date = normalizeDate(rescheduleData.Date)
			studentSession.From = rescheduleData.From
			studentSession.To = rescheduleData.To
		} else {
			studentSession.Status = enums.StudentSessionStatusScheduled
		}

	} else if justification.Type == enums.JustificationTypeAbsence {
		if data.Decision == enums.JustificationDecisionApproved {
			studentSession.CanReschedule = true

		}
	}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": studentSession})
	if err != nil {
		return nil, err
	}

	// Handle compensation hours recording/rejection based on status change
	c.handleSessionStatusChange(ctx, oldStatus, updatedSession.Status, updatedSession)

	// Update commitment progress if status changed to completed
	if oldStatus != enums.StudentSessionStatusCompleted && updatedSession.Status == enums.StudentSessionStatusCompleted {
		_ = c.CalculateCommitmentProgress(ctx, updatedSession.StudentId.Hex(), updatedSession.CoachingProgramId.Hex())
	}

	// Handle Zoom meeting for reschedule approval
	if justification.Type == enums.JustificationTypeReschedule && data.Decision == enums.JustificationDecisionApproved {
		// Delete old meeting if exists
		if updatedSession.ZoomMeetingId != nil {
			_ = c.zoomService.DeleteMeeting(ctx, *updatedSession.ZoomMeetingId)
		}

		// Create new meeting
		student, _ := c.userLookupSvc.GetUsersBasicInfo(ctx, []primitive.ObjectID{updatedSession.StudentId})
		topic := "Coaching Session"
		if info, ok := student[updatedSession.StudentId]; ok {
			topic = fmt.Sprintf("Session - %s", info.Name.GetContentByContext(ctx))
		}

		startTime := updatedSession.Date
		fromParts := strings.Split(updatedSession.From, ":")
		if len(fromParts) == 2 {
			h, _ := strconv.Atoi(fromParts[0])
			m, _ := strconv.Atoi(fromParts[1])
			startTime = time.Date(updatedSession.Date.Year(), updatedSession.Date.Month(), updatedSession.Date.Day(), h, m, 0, 0, time.UTC)
		}

		duration := updatedSession.DurationMinutes
		if duration <= 0 {
			duration = 60
		}

		zm, err := c.zoomService.CreateMeeting(ctx, "coaching", updatedSession.Id, startTime, duration, topic)
		if err != nil {
			slog.Error("Failed to create Zoom meeting for rescheduled coaching session", "sessionId", updatedSession.Id.Hex(), "error", err)
		} else {
			updatedSession, _ = c.studentSessionRepo.Patch(ctx, bson.M{"_id": updatedSession.Id}, bson.M{"$set": bson.M{"zoomMeetingId": zm.Id}})
		}
	}

	// Notify student about teacher's reschedule decision
	if justification.Type == enums.JustificationTypeReschedule {
		title := "Reschedule request rejected"
		content := "Your teacher has rejected your reschedule request."
		if data.Decision == enums.JustificationDecisionApproved {
			title = "Reschedule request approved"
			content = "Your teacher has approved your reschedule request."
		}
		_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{updatedSession.StudentId}, &messageModels.Notification{
			ServiceName: "academy",
			Title:       title,
			Content:     content,
			Recipient:   updatedSession.StudentId.Hex(),
		})
	}

	return updatedSession, nil
}

func (c *coachingsvcs) UpdateStudentSessionNotes(ctx context.Context, studentSessionId string, data *models.UpdateSessionNotesDto) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	// Authorization: Student, teacher assigned to session, or admin can update notes
	isAdmin, err := util.HasCapability(ctx, CapabilityManageSessionDetails)
	if err != nil {
		return nil, err
	}

	if !isAdmin {
		if studentSession.StudentId != cfg.User.Id && studentSession.TeacherId != cfg.User.Id {
			return nil, errors.New("you are not authorized to add note to this session")
		}
	}

	studentSession.Notes = data.Notes
	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": studentSession}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedSession, nil
}

func (c *coachingsvcs) UpdateStudentSessionAgendas(ctx context.Context, studentSessionId string, data *models.UpdateSessionAgendasDto) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	// Authorization: Student, teacher assigned to session, or admin can update agendas
	isAdmin, err := util.HasCapability(ctx, CapabilityManageSessionDetails)
	if err != nil {
		return nil, err
	}

	if !isAdmin {
		if studentSession.StudentId != cfg.User.Id && studentSession.TeacherId != cfg.User.Id {
			return nil, errors.New("you are not authorized to add note to this session")
		}
	}

	studentSession.Agendas = data.Agendas
	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": studentSession}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedSession, nil
}

func (c *coachingsvcs) UpdateStudentSessionResources(ctx context.Context, studentSessionId string, data *models.UpdateSessionResourcesDto) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	// Authorization: Student, teacher assigned to session, or admin can update resources
	isAdmin, err := util.HasCapability(ctx, CapabilityManageSessionDetails)
	if err != nil {
		return nil, err
	}

	if !isAdmin {
		if studentSession.StudentId != cfg.User.Id && studentSession.TeacherId != cfg.User.Id {
			return nil, errors.New("you are not authorized to add note to this session")
		}
	}

	studentSession.Resources = data.Resources
	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": studentSession}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedSession, nil
}

func (c *coachingsvcs) UpdateStudentSessionFeedback(ctx context.Context, studentSessionId string, data *models.UpdateSessionFeedbackDto) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	// Authorization: Only teacher or student assigned to session can update feedback
	isAdmin, err := util.HasCapability(ctx, CapabilityManageSessionDetails)
	if err != nil {
		return nil, err
	}

	isTeacher := studentSession.TeacherId == cfg.User.Id
	isStudent := studentSession.StudentId == cfg.User.Id

	if !isAdmin && !isTeacher && !isStudent {
		return nil, errors.New("you are not authorized to update feedback for this session")
	}

	// Validate field values
	if data.TeacherRating.IsProvided() && data.TeacherRating.Value != nil {
		rating := *data.TeacherRating.Value
		if rating < 0 || rating > 5 {
			return nil, errors.New("teacherRating must be between 0 and 5")
		}
	}
	if data.StudentRating.IsProvided() && data.StudentRating.Value != nil {
		rating := *data.StudentRating.Value
		if rating < 0 || rating > 5 {
			return nil, errors.New("studentRating must be between 0 and 5")
		}
	}
	if data.StudentEngagement.IsProvided() && data.StudentEngagement.Value != nil {
		engagement := *data.StudentEngagement.Value
		if engagement < 0 || engagement > 100 {
			return nil, errors.New("studentEngagement must be between 0 and 100")
		}
	}

	// Determine user role and validate field permissions
	if !isAdmin {
		if isTeacher {
			// Teacher can only update: teacherFeedback, teacherRating, studentEngagement
			if data.StudentFeedback.IsProvided() {
				return nil, errors.New("teachers cannot update studentFeedback")
			}
			if data.StudentRating.IsProvided() {
				return nil, errors.New("teachers cannot update studentRating")
			}
			if data.StudentFeedbackDate != nil {
				return nil, errors.New("teachers cannot update studentFeedbackDate")
			}
			// Allow teacherFeedback to be updated by teacher (as per requirement)
		} else if isStudent {
			// Student can only update: studentFeedback, studentRating, and teacherFeedback
			if data.TeacherRating.IsProvided() {
				return nil, errors.New("students cannot update teacherRating")
			}
			if data.StudentEngagement.IsProvided() {
				return nil, errors.New("students cannot update studentEngagement")
			}
			if data.TeacherFeedbackDate != nil {
				return nil, errors.New("students cannot update teacherFeedbackDate")
			}
		}
	}

	oldStatus := studentSession.Status

	// Initialize Feedback if it's nil
	if studentSession.Feedback == nil {
		studentSession.Feedback = &models.SessionFeedback{}
	}

	// Update fields based on what's provided
	updateFields := bson.M{}

	// Teacher fields
	if data.TeacherFeedback.IsProvided() {
		if data.TeacherFeedback.IsExplicitlyNull() {
			// Delete field by setting to null
			studentSession.Feedback.TeacherFeedback = types.Nullable[string]{Value: nil, IsExplicitNull: true}
			updateFields["feedback.teacherFeedback"] = nil
		} else if data.TeacherFeedback.Value != nil {
			studentSession.Feedback.TeacherFeedback = data.TeacherFeedback
			updateFields["feedback.teacherFeedback"] = *data.TeacherFeedback.Value
		}
	}
	if data.TeacherRating.IsProvided() {
		if data.TeacherRating.IsExplicitlyNull() {
			// Delete field by setting to null
			studentSession.Feedback.TeacherRating = types.Nullable[int]{Value: nil, IsExplicitNull: true}
			updateFields["feedback.teacherRating"] = nil
		} else if data.TeacherRating.Value != nil {
			studentSession.Feedback.TeacherRating = data.TeacherRating
			updateFields["feedback.teacherRating"] = *data.TeacherRating.Value
		}
	}
	if data.TeacherFeedbackDate != nil {
		studentSession.Feedback.TeacherFeedbackDate = data.TeacherFeedbackDate
		updateFields["feedback.teacherFeedbackDate"] = data.TeacherFeedbackDate
	}
	if data.StudentEngagement.IsProvided() {
		if data.StudentEngagement.IsExplicitlyNull() {
			// Delete field by setting to null
			studentSession.Feedback.StudentEngagement = types.Nullable[int]{Value: nil, IsExplicitNull: true}
			updateFields["feedback.studentEngagement"] = nil
		} else if data.StudentEngagement.Value != nil {
			studentSession.Feedback.StudentEngagement = data.StudentEngagement
			updateFields["feedback.studentEngagement"] = *data.StudentEngagement.Value
		}
	}

	// Student fields
	if data.StudentFeedback.IsProvided() {
		if data.StudentFeedback.IsExplicitlyNull() {
			// Delete field by setting to null
			studentSession.Feedback.StudentFeedback = types.Nullable[string]{Value: nil, IsExplicitNull: true}
			updateFields["feedback.studentFeedback"] = nil
		} else if data.StudentFeedback.Value != nil {
			studentSession.Feedback.StudentFeedback = data.StudentFeedback
			updateFields["feedback.studentFeedback"] = *data.StudentFeedback.Value
		}
	}
	if data.StudentRating.IsProvided() {
		if data.StudentRating.IsExplicitlyNull() {
			// Delete field by setting to null
			studentSession.Feedback.StudentRating = types.Nullable[int]{Value: nil, IsExplicitNull: true}
			updateFields["feedback.studentRating"] = nil
		} else if data.StudentRating.Value != nil {
			studentSession.Feedback.StudentRating = data.StudentRating
			updateFields["feedback.studentRating"] = *data.StudentRating.Value
		}
	}
	if data.StudentFeedbackDate != nil {
		studentSession.Feedback.StudentFeedbackDate = data.StudentFeedbackDate
		updateFields["feedback.studentFeedbackDate"] = data.StudentFeedbackDate
	}

	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id
	updateFields["updatedAt"] = studentSession.UpdatedAt
	updateFields["updatedBy"] = studentSession.UpdatedBy

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": updateFields}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Handle compensation hours recording/rejection based on status change
	c.handleSessionStatusChange(ctx, oldStatus, updatedSession.Status, updatedSession)

	return updatedSession, nil
}

//

func (c *coachingsvcs) BookingSessions(ctx context.Context, data *models.BookingSessions) (*models.BookingSessions, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	if len(data.Dates) != 2 {
		return nil, errors.New("invalid number of dates- must be 2")
	}

	filter := bson.M{
		"studentId":         cfg.User.Id,
		"coachingProgramId": data.CoachingProgramId,
		"status":            enums.StudentSessionStatusPending,
		"trash":             false,
	}
	pipeline := []bson.M{
		{"$match": filter},
	}

	var sessions []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	}); err != nil {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, errors.New("no sessions found")
	}

	dates := data.Dates
	d1 := dates[0].Date
	d2 := dates[1].Date
	firstDate := time.Date(d1.Year(), d1.Month(), d1.Day(), 0, 0, 0, 0, time.UTC)
	secondDate := time.Date(d2.Year(), d2.Month(), d2.Day(), 0, 0, 0, 0, time.UTC)

	writeOps := []mongo.WriteModel{}

	for i, session := range sessions {
		var update bson.M
		if i%2 == 0 {
			update = bson.M{
				"date":      firstDate,
				"from":      dates[0].From,
				"to":        dates[0].To,
				"teacherId": data.TeacherId,
				"status":    enums.StudentSessionStatusScheduled,
			}
			firstDate = firstDate.AddDate(0, 0, 7)
		} else {
			update = bson.M{
				"date":      secondDate,
				"from":      dates[1].From,
				"to":        dates[1].To,
				"teacherId": data.TeacherId,
				"status":    enums.StudentSessionStatusScheduled,
			}
			secondDate = secondDate.AddDate(0, 0, 7)
		}

		updateOp := mongo.NewUpdateOneModel()
		updateOp.SetFilter(bson.M{"_id": session.Id})
		updateOp.SetUpdate(bson.M{"$set": update})
		writeOps = append(writeOps, updateOp)
	}

	if len(writeOps) > 0 {
		// 1. Prepare and Validate Batch Capacity
		slots := make([]pool.Slot, len(sessions))
		for i := range sessions {
			var date time.Time
			var from string
			if i%2 == 0 {
				date = time.Date(data.Dates[0].Date.Year(), data.Dates[0].Date.Month(), data.Dates[0].Date.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, (i/2)*7)
				from = data.Dates[0].From
			} else {
				date = time.Date(data.Dates[1].Date.Year(), data.Dates[1].Date.Month(), data.Dates[1].Date.Day(), 0, 0, 0, 0, time.UTC).AddDate(0, 0, (i/2)*7)
				from = data.Dates[1].From
			}

			startTime := date
			fromParts := strings.Split(from, ":")
			if len(fromParts) == 2 {
				h, _ := strconv.Atoi(fromParts[0])
				m, _ := strconv.Atoi(fromParts[1])
				startTime = time.Date(date.Year(), date.Month(), date.Day(), h, m, 0, 0, time.UTC)
			}

			duration := sessions[i].DurationMinutes
			if duration <= 0 {
				duration = 60
			}

			slots[i] = pool.Slot{
				StartTime: startTime,
				Duration:  duration,
			}
		}

		// Pre-validate capacity for the entire batch
		// Note: We need to access the picker from the service.
		// Since zoomService doesn't expose the picker, and CreateMeeting already does picking,
		// we'll implement a custom flow here or modify zoomService.
		// For now, let's use a rollback mechanism since we want to be sure Zoom API also succeeds.

		createdZoomMeetingIds := []primitive.ObjectID{}
		rollback := func() {
			slog.Info("Rolling back Zoom meetings due to batch failure", "count", len(createdZoomMeetingIds))
			for _, zid := range createdZoomMeetingIds {
				_ = c.zoomService.DeleteMeeting(ctx, zid)
			}
		}

		// 2. Create Zoom meetings for scheduled sessions
		for i, session := range sessions {
			slot := slots[i]

			// Topic: [Session] - [Student Name]
			student, _ := c.userLookupSvc.GetUsersBasicInfo(ctx, []primitive.ObjectID{session.StudentId})
			topic := "Coaching Session"
			if info, ok := student[session.StudentId]; ok {
				topic = fmt.Sprintf("Session - %s", info.Name.GetContentByContext(ctx))
			}

			slog.Info("Creating Zoom meeting for coaching session", "sessionId", session.Id.Hex(), "startTime", slot.StartTime)
			zm, err := c.zoomService.CreateMeeting(ctx, "coaching", session.Id, slot.StartTime, slot.Duration, topic)
			if err != nil {
				slog.Error("Failed to create Zoom meeting for coaching session in batch", "sessionId", session.Id.Hex(), "error", err)
				rollback()
				return nil, fmt.Errorf("failed to create zoom meeting for session %d (capacity exceeded or API error): %w", i+1, err)
			}

			createdZoomMeetingIds = append(createdZoomMeetingIds, zm.Id)

			// Add ZoomMeetingId to the bulk update operation
			updateOp := writeOps[i].(*mongo.UpdateOneModel)
			update := updateOp.Update.(bson.M)["$set"].(bson.M)
			update["zoomMeetingId"] = zm.Id
		}

		// 3. Perform bulk update in DB
		_, err := c.studentSessionRepo.BulkWrite(ctx, writeOps)
		if err != nil {
			slog.Error("Failed to bulk update student sessions with zoom meeting info", "error", err)
			rollback()
			return nil, err
		}

		// Update commitment progress after booking sessions
		_ = c.CalculateCommitmentProgress(ctx, cfg.User.Id.Hex(), data.CoachingProgramId.Hex())

		// Notify student that sessions have been scheduled
		_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{cfg.User.Id}, &messageModels.Notification{
			ServiceName: "academy",
			Title:       "Sessions scheduled",
			Content:     "Your new sessions have been scheduled.",
			Recipient:   cfg.User.Id.Hex(),
		})
	}

	return data, nil

}

func (c *coachingsvcs) AbsenceJustificationRequest(ctx context.Context, studentSessionId string, data *models.AbsenceJustificationDto) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	// Check if user is the student or a parent of the student
	isStudent := studentSession.StudentId == cfg.User.Id
	isParent := false
	if !isStudent {
		var err error
		isParent, err = c.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, studentSession.StudentId)
		if err != nil {
			return nil, fmt.Errorf("error checking parent relationship: %w", err)
		}
	}

	if !isStudent && !isParent {
		return nil, errors.New("you are not authorized to add absence justification to this student session")
	}
	if studentSession.Status != enums.StudentSessionStatusMissed {
		return nil, fmt.Errorf("error session status %s must be missed", studentSession.Status)
	}

	prevJustifications := studentSession.PrevJustifications
	currentJustification := studentSession.Justification

	if currentJustification != nil {
		if currentJustification.Decision == enums.JustificationDecisionPending {
			return nil, errors.New("you already have a pending justification")
		} else {
			prevJustifications = append(prevJustifications, *currentJustification)
		}
	}

	currentJustification = &models.SessionJustification{
		Id:        primitive.NewObjectID(),
		Type:      enums.JustificationTypeAbsence,
		Reason:    data.Reason,
		Documents: data.Documents,
		Decision:  enums.JustificationDecisionPending,
		Data:      nil,
	}

	studentSession.Justification = currentJustification
	studentSession.PrevJustifications = prevJustifications
	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": studentSession}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedSession, nil
}

func (c *coachingsvcs) Reschedule(ctx context.Context, studentSessionId string, data *models.RescheduleSessionData) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	if studentSession.StudentId != cfg.User.Id {
		return nil, errors.New("you are not authorized to reschedule this session")
	}

	oldStatus := studentSession.Status
	if studentSession.CanReschedule {
		studentSession.Status = enums.StudentSessionStatusScheduled
		studentSession.Date = normalizeDate(data.Date)
		studentSession.From = data.From
		studentSession.To = data.To
		studentSession.UpdatedAt = time.Now()
		studentSession.UpdatedBy = cfg.User.Id
		studentSession.CanReschedule = false
	} else {
		return nil, errors.New("you can not reshedule this session")
	}

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": studentSession}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Handle compensation hours recording/rejection based on status change
	c.handleSessionStatusChange(ctx, oldStatus, updatedSession.Status, updatedSession)

	return updatedSession, nil
}

func (c *coachingsvcs) RescheduleRequest(ctx context.Context, studentSessionId string, data *models.RescheduleJustificationDto) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, errors.New("invalid session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("error get student session")
	}

	// Check if user is the student or a parent of the student
	isStudent := studentSession.StudentId == cfg.User.Id
	isParent := false
	if !isStudent {
		var err error
		isParent, err = c.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, studentSession.StudentId)
		if err != nil {
			return nil, fmt.Errorf("error checking parent relationship: %w", err)
		}
	}

	if !isStudent && !isParent {
		return nil, errors.New("you are not authorized to reschedule this session")
	}
	if studentSession.Status != "scheduled" {
		return nil, errors.New("session not scheduled yet")
	}

	prevJustifications := studentSession.PrevJustifications
	currentJustification := studentSession.Justification

	if currentJustification != nil {
		if currentJustification.Decision == enums.JustificationDecisionPending {
			return nil, errors.New("you already have a pending justification")
		} else {
			prevJustifications = append(prevJustifications, *currentJustification)
		}
	}

	//todo: check time before one hour
	currentJustification = &models.SessionJustification{
		Type:      enums.JustificationTypeReschedule,
		Reason:    data.Reason,
		Documents: data.Documents,
		Decision:  enums.JustificationDecisionPending,
		Data:      data.RescheduleSessionData,
		Date:      time.Now(),
	}

	studentSession.Justification = currentJustification
	studentSession.PrevJustifications = prevJustifications
	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	updatedSession, err := c.studentSessionRepo.Patch(ctx, bson.M{"_id": _id}, bson.M{"$set": studentSession})
	if err != nil {
		return nil, err
	}

	return updatedSession, nil

}

func (c *coachingsvcs) GetSessionsByStage(ctx context.Context, coachingProgramId string, stage string) ([]models.StudentSession, error) {
	cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

	var filter bson.M

	switch stage {
	case "upcoming":
		{
			filter = bson.M{
				"studentId":         cfg.User.Id,
				"coachingProgramId": cpId,
				"status":            enums.StudentSessionStatusScheduled,
				"trash":             false,
				"date":              bson.M{"$gte": startOfToday},
			}
		}
	case "completed":
		{
			filter = bson.M{
				"studentId":         cfg.User.Id,
				"coachingProgramId": cpId,
				"trash":             false,
				"$or": []bson.M{
					{
						"status": enums.StudentSessionStatusCompleted,
					},
					{
						"status": enums.StudentSessionStatusMissed,
					},
				},
			}

		}
	case "rescheduled":
		{
			filter = bson.M{
				"studentId":         cfg.User.Id,
				"coachingProgramId": cpId,
				"trash":             false,
				"$or": []bson.M{
					{
						"status": enums.StudentSessionStatusRescheduled,
					},
					{
						"status":                 enums.StudentSessionStatusScheduled,
						"justification.type":     enums.JustificationTypeReschedule,
						"justification.decision": enums.JustificationDecisionApproved,
					},
				},
			}

		}
	default:
		{
			return nil, errors.New("invalid stage")
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var sessions []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	}); err != nil {
		return nil, err
	}

	if err := c.enrichSessionsWithZoom(ctx, sessions); err != nil {
		return nil, err
	}

	return sessions, nil
}

// validateTimeFormat validates that a time string is in HH:MM format with valid values
func validateTimeFormat(timeStr string) error {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return errors.New("time must be in HH:MM format")
	}

	hour, err := strconv.Atoi(parts[0])
	if err != nil {
		return errors.New("hour must be a number")
	}
	if hour < 0 || hour > 23 {
		return errors.New("hour must be between 0 and 23")
	}

	minute, err := strconv.Atoi(parts[1])
	if err != nil {
		return errors.New("minute must be a number")
	}
	if minute < 0 || minute > 59 {
		return errors.New("minute must be between 0 and 59")
	}

	// Validate format has exactly 2 digits for hour and minute
	if len(parts[0]) != 2 || len(parts[1]) != 2 {
		return errors.New("time must be in HH:MM format (2 digits for hour and minute)")
	}

	return nil
}

// UpdateStudentSessionTime updates the time and/or status of a coaching session (for calendar events)
func (c *coachingsvcs) UpdateStudentSessionTime(ctx context.Context, studentSessionId string, data *models.UpdateSessionTimeData) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, constant.ErrInvalidStudentSessionId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, constant.ErrStudentSessionNotFound
	}

	// Authorization: Only teacher assigned to session or admin can update time
	// Check if user is admin
	isAdmin, err := util.HasCapability(ctx, CapabilityViewAllCalendarEvents)
	if err != nil {
		return nil, err
	}

	if !isAdmin {
		// For non-admins, must be the teacher assigned to this session
		if studentSession.TeacherId != cfg.User.Id {
			return nil, constant.ErrUnauthorizedUpdateSession
		}
	}

	oldStatus := studentSession.Status

	// Update time fields if provided
	if data.Date != nil {
		studentSession.Date = normalizeDate(*data.Date)
	}
	if data.From != nil {
		// Validate time format
		if err := validateTimeFormat(*data.From); err != nil {
			return nil, constant.ErrInvalidTimeFormat
		}
		studentSession.From = *data.From
	}
	if data.To != nil {
		// Validate time format
		if err := validateTimeFormat(*data.To); err != nil {
			return nil, constant.ErrInvalidTimeFormat
		}
		studentSession.To = *data.To
	}

	// Validate that From < To if both are provided or if we're updating one
	if (data.From != nil || data.To != nil) && studentSession.From != "" && studentSession.To != "" {
		fromParts := strings.Split(studentSession.From, ":")
		toParts := strings.Split(studentSession.To, ":")
		fromHour, _ := strconv.Atoi(fromParts[0])
		fromMin, _ := strconv.Atoi(fromParts[1])
		toHour, _ := strconv.Atoi(toParts[0])
		toMin, _ := strconv.Atoi(toParts[1])

		fromMinutes := fromHour*60 + fromMin
		toMinutes := toHour*60 + toMin
		if fromMinutes >= toMinutes {
			return nil, constant.ErrFromTimeAfterToTime
		}
	}

	// Handle status update if provided
	if data.Status != nil {
		newStatus := *data.Status

		// Validate status is one of the allowed values
		if newStatus != enums.StudentSessionStatusMissed &&
			newStatus != enums.StudentSessionStatusCancelled &&
			newStatus != enums.StudentSessionStatusCompleted &&
			newStatus != enums.StudentSessionStatusScheduled &&
			newStatus != enums.StudentSessionStatusPending &&
			newStatus != enums.StudentSessionStatusRescheduled {
			return nil, constant.ErrInvalidStatusValue
		}

		// For missed or completed, validate that session end date is in the past
		if newStatus == enums.StudentSessionStatusMissed || newStatus == enums.StudentSessionStatusCompleted {
			// Calculate session end time from Date + To time
			sessionEndTime := studentSession.Date
			if studentSession.To != "" {
				toParts := strings.Split(studentSession.To, ":")
				if len(toParts) == 2 {
					hour, _ := strconv.Atoi(toParts[0])
					min, _ := strconv.Atoi(toParts[1])
					sessionEndTime = time.Date(
						studentSession.Date.Year(),
						studentSession.Date.Month(),
						studentSession.Date.Day(),
						hour,
						min,
						0,
						0,
						studentSession.Date.Location(),
					)
				}
			}

			now := time.Now()
			if sessionEndTime.After(now) {
				return nil, constant.ErrStatusRequiresPastEndTime
			}
		}

		studentSession.Status = newStatus
	}

	// Update additional fields if provided
	if data.TeacherId != nil {
		studentSession.TeacherId = *data.TeacherId
	}
	if data.SessionNum != nil {
		studentSession.SessionNum = *data.SessionNum
	}
	if data.DurationMinutes != nil {
		studentSession.DurationMinutes = *data.DurationMinutes
	}
	if data.Notes != nil {
		studentSession.Notes = data.Notes
	}
	if data.Agendas != nil {
		studentSession.Agendas = data.Agendas
	}
	if data.Resources != nil {
		studentSession.Resources = data.Resources
	}
	if data.Recordings != nil {
		studentSession.Recordings = data.Recordings
	}
	if data.Feedback != nil {
		// Initialize Feedback if it's nil
		if studentSession.Feedback == nil {
			studentSession.Feedback = &models.SessionFeedback{}
		}
		f := data.Feedback
		if f.TeacherFeedback.IsProvided() {
			studentSession.Feedback.TeacherFeedback = f.TeacherFeedback
		}
		if f.TeacherRating.IsProvided() {
			studentSession.Feedback.TeacherRating = f.TeacherRating
		}
		if f.TeacherFeedbackDate != nil {
			studentSession.Feedback.TeacherFeedbackDate = f.TeacherFeedbackDate
		}
		if f.StudentEngagement.IsProvided() {
			studentSession.Feedback.StudentEngagement = f.StudentEngagement
		}
		if f.StudentFeedback.IsProvided() {
			studentSession.Feedback.StudentFeedback = f.StudentFeedback
		}
		if f.StudentRating.IsProvided() {
			studentSession.Feedback.StudentRating = f.StudentRating
		}
		if f.StudentFeedbackDate != nil {
			studentSession.Feedback.StudentFeedbackDate = f.StudentFeedbackDate
		}
	}

	if data.SessionName != nil {
		studentSession.SessionName = *data.SessionName
	}
	if data.Description != nil {
		studentSession.Description = *data.Description
	}

	studentSession.UpdatedAt = time.Now()
	studentSession.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": studentSession}

	updatedSession, err := c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Handle Zoom Meeting creation/cleanup (Synchronous)
	if oldStatus != updatedSession.Status || data.Date != nil || data.From != nil || data.To != nil {
		// If session is no longer scheduled, delete meeting
		if updatedSession.Status != enums.StudentSessionStatusScheduled && updatedSession.ZoomMeetingId != nil {
			_ = c.zoomService.DeleteMeeting(ctx, *updatedSession.ZoomMeetingId)
			updatedSession, _ = c.studentSessionRepo.Patch(ctx, bson.M{"_id": updatedSession.Id}, bson.M{"$unset": bson.M{"zoomMeetingId": ""}})
		}

		// If session is scheduled and time changed or no meeting exists
		if updatedSession.Status == enums.StudentSessionStatusScheduled {
			// If time changed and meeting exists, delete old one first
			if updatedSession.ZoomMeetingId != nil && (data.Date != nil || data.From != nil || data.To != nil) {
				_ = c.zoomService.DeleteMeeting(ctx, *updatedSession.ZoomMeetingId)
				updatedSession.ZoomMeetingId = nil
			}

			if updatedSession.ZoomMeetingId == nil {
				student, _ := c.userLookupSvc.GetUsersBasicInfo(ctx, []primitive.ObjectID{updatedSession.StudentId})
				topic := "Coaching Session"
				if info, ok := student[updatedSession.StudentId]; ok {
					topic = fmt.Sprintf("Session - %s", info.Name.GetContentByContext(ctx))
				}

				startTime := updatedSession.Date
				fromParts := strings.Split(updatedSession.From, ":")
				if len(fromParts) == 2 {
					h, _ := strconv.Atoi(fromParts[0])
					m, _ := strconv.Atoi(fromParts[1])
					startTime = time.Date(updatedSession.Date.Year(), updatedSession.Date.Month(), updatedSession.Date.Day(), h, m, 0, 0, time.UTC)
				}

				duration := updatedSession.DurationMinutes
				if duration <= 0 {
					duration = 60
				}

				zm, err := c.zoomService.CreateMeeting(ctx, "coaching", updatedSession.Id, startTime, duration, topic)
				if err != nil {
					slog.Error("Failed to create Zoom meeting for updated coaching session", "sessionId", updatedSession.Id.Hex(), "error", err)
				} else {
					updatedSession, _ = c.studentSessionRepo.Patch(ctx, bson.M{"_id": updatedSession.Id}, bson.M{"$set": bson.M{"zoomMeetingId": zm.Id}})
				}
			}
		}
	}

	// Handle compensation hours recording/rejection based on status change
	if data.Status != nil && oldStatus != updatedSession.Status {
		c.handleSessionStatusChange(ctx, oldStatus, updatedSession.Status, updatedSession)
	}

	// Notify parents when session is marked missed without justification
	if updatedSession.Status == enums.StudentSessionStatusMissed {
		noJustification := updatedSession.Justification == nil ||
			updatedSession.Justification.Decision != enums.JustificationDecisionApproved
		if noJustification {
			parentIds, _ := c.parentSvcs.GetParentUserIdsForStudent(ctx, updatedSession.StudentId)
			studentName := "Your son"
			if info, err := c.userLookupSvc.GetUsersBasicInfo(ctx, []primitive.ObjectID{updatedSession.StudentId}); err == nil {
				if u, ok := info[updatedSession.StudentId]; ok {
					studentName = u.Name.GetContentByContext(ctx)
				}
			}
			for _, pid := range parentIds {
				_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{pid}, &messageModels.Notification{
					ServiceName: "academy",
					Title:       "Session missed",
					Content:     studentName + " missed a session without providing any justification for his absence.",
					Recipient:   pid.Hex(),
				})
			}
		}
	}

	return updatedSession, nil
}

// DeleteStudentSession deletes/cancels a coaching session (for calendar events)
func (c *coachingsvcs) DeleteStudentSession(ctx context.Context, studentSessionId string) error {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return errors.New("invalid student session id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return errors.New("student session not found")
	}

	// Authorization: Only teacher assigned to session or admin can delete
	// Check if user is admin
	isAdmin, err := util.HasCapability(ctx, CapabilityViewAllCalendarEvents)
	if err != nil {
		return err
	}

	if !isAdmin {
		// For non-admins, must be the teacher assigned to this session
		if studentSession.TeacherId != cfg.User.Id {
			return errors.New("you are not authorized to delete this session")
		}
	}

	// Soft delete by setting trash flag
	oldStatus := studentSession.Status
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{
		"trash":     true,
		"status":    enums.StudentSessionStatusCancelled,
		"updatedAt": time.Now(),
		"updatedBy": cfg.User.Id,
	}}

	_, err = c.studentSessionRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	// Delete Zoom meeting if exists (Synchronous)
	if studentSession.ZoomMeetingId != nil {
		_ = c.zoomService.DeleteMeeting(ctx, *studentSession.ZoomMeetingId)
	}

	// Handle compensation hours recording/rejection based on status change
	// Create a temporary session object with cancelled status for the helper
	tempSession := *studentSession
	tempSession.Status = enums.StudentSessionStatusCancelled
	c.handleSessionStatusChange(ctx, oldStatus, enums.StudentSessionStatusCancelled, &tempSession)

	return nil
}

// BulkUpdateStudentSessions allows updating multiple sessions at once (agendas, resources, notes, recordings, duration, name, description)
func (c *coachingsvcs) BulkUpdateStudentSessions(ctx context.Context, data *models.BulkUpdateSessionsDto) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	_, err = c.withtxn.Exec(ctx, func(sc mongo.SessionContext) (interface{}, error) {
		for _, item := range data.Sessions {
			_id, err := primitive.ObjectIDFromHex(item.Id)
			if err != nil {
				return nil, errors.New("invalid session id: " + item.Id)
			}

			// Build update object
			updateSet := bson.M{
				"updatedAt": time.Now(),
				"updatedBy": cfg.User.Id,
			}

			if item.Agendas != nil {
				updateSet["agendas"] = item.Agendas
			}
			if item.Resources != nil {
				updateSet["resources"] = item.Resources
			}
			if item.Notes != nil {
				updateSet["notes"] = item.Notes
			}
			if item.Recordings != nil {
				updateSet["recordings"] = item.Recordings
			}
			if item.DurationMinutes != nil {
				updateSet["durationMinutes"] = *item.DurationMinutes
			}
			if item.SessionName != nil {
				updateSet["sessionName"] = *item.SessionName
			}
			if item.Description != nil {
				updateSet["description"] = *item.Description
			}

			filter := bson.M{"_id": _id, "trash": false}
			update := bson.M{"$set": updateSet}

			_, err = c.studentSessionRepo.Patch(sc, filter, update)
			if err != nil {
				return nil, err
			}
		}
		return nil, nil
	})
	return err
}

// GetStudentSessionById retrieves a student session by its ID
func (c *coachingsvcs) GetStudentSessionById(ctx context.Context, studentSessionId string) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(studentSessionId)
	if err != nil {
		return nil, constant.ErrInvalidStudentSessionId
	}

	studentSession, err := c.studentSessionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, constant.ErrStudentSessionNotFound
	}

	if err := c.enrichSessionsWithZoom(ctx, []models.StudentSession{*studentSession}); err != nil {
		return nil, err
	}

	return studentSession, nil
}

// GetSessionById retrieves a session by its ID with enriched student, teacher, and coaching objects
func (c *coachingsvcs) GetSessionById(ctx context.Context, sessionId string) (*models.StudentSession, error) {
	_id, err := primitive.ObjectIDFromHex(sessionId)
	if err != nil {
		return nil, constant.ErrInvalidStudentSessionId
	}

	// Use aggregation pipeline to enrich with student, teacher, and coaching info
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		// Lookup student
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "student",
		}},
		{"$unwind": bson.M{
			"path":                       "$student",
			"preserveNullAndEmptyArrays": true,
		}},
		// Lookup teacher
		{"$lookup": bson.M{
			"from":         "academyTeachers",
			"localField":   "teacherId",
			"foreignField": "_id",
			"as":           "teacher",
		}},
		{"$unwind": bson.M{
			"path":                       "$teacher",
			"preserveNullAndEmptyArrays": true,
		}},
		// Lookup coaching
		{"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "coachingProgramId",
			"foreignField": "_id",
			"as":           "coaching",
		}},
		{"$unwind": bson.M{
			"path":                       "$coaching",
			"preserveNullAndEmptyArrays": true,
		}},
		// Add enriched fields
		{"$addFields": bson.M{
			"studentInfo": bson.M{
				"$cond": []interface{}{
					bson.M{"$ne": []interface{}{"$student", nil}},
					bson.M{
						"id":        "$student._id",
						"firstName": "$student.generalInfo.firstName",
						"lastName":  "$student.generalInfo.lastName",
						"picture":   "$student.generalInfo.picture",
					},
					nil,
				},
			},
			"teacherInfo": bson.M{
				"$cond": []interface{}{
					bson.M{"$ne": []interface{}{"$teacher", nil}},
					bson.M{
						"id":        "$teacher._id",
						"firstName": "$teacher.firstName",
						"lastName":  "$teacher.lastName",
						"picture":   "$teacher.picture",
					},
					nil,
				},
			},
			"coachingInfo": bson.M{
				"$cond": []interface{}{
					bson.M{"$ne": []interface{}{"$coaching", nil}},
					bson.M{
						"id":        "$coaching._id",
						"title":     "$coaching.title",
						"thumbnail": "$coaching.thumbnail",
					},
					nil,
				},
			},
		}},
		// Remove temporary lookup fields
		{"$project": bson.M{
			"student":  0,
			"teacher":  0,
			"coaching": 0,
		}},
	}

	var sessions []models.StudentSession
	err = c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	if len(sessions) == 0 {
		return nil, constant.ErrStudentSessionNotFound
	}

	session := &sessions[0]

	// Enrich justification object with requestedByObj and decisionByObj
	c.enrichJustification(ctx, session)

	if err := c.enrichSessionsWithZoom(ctx, []models.StudentSession{*session}); err != nil {
		return nil, err
	}

	return session, nil
}

// enrichJustification enriches the justification object with requestedByObj and decisionByObj
func (c *coachingsvcs) enrichJustification(ctx context.Context, session *models.StudentSession) {
	if session.Justification == nil && len(session.PrevJustifications) == 0 {
		return
	}

	// Collect all user IDs for batch lookup
	var allUserIds []primitive.ObjectID

	// For current justification
	if session.Justification != nil {
		// Use session.UpdatedBy as requestedBy (the person who created/updated the justification)
		if !session.UpdatedBy.IsZero() {
			allUserIds = append(allUserIds, session.UpdatedBy)
			session.Justification.RequestedBy = session.UpdatedBy
		}
		if !session.Justification.DecisionBy.IsZero() {
			allUserIds = append(allUserIds, session.Justification.DecisionBy)
		}
	}

	// For prevJustifications, we don't have the requestedBy info stored, so we'll only enrich decisionBy
	for _, prevJust := range session.PrevJustifications {
		if !prevJust.DecisionBy.IsZero() {
			allUserIds = append(allUserIds, prevJust.DecisionBy)
		}
	}

	// Batch lookup all users
	userInfoMap := make(map[primitive.ObjectID]contactsvcs.UserBasicInfo)
	if len(allUserIds) > 0 {
		lookupResult, err := c.userLookupSvc.GetUsersBasicInfo(ctx, allUserIds)
		if err == nil {
			userInfoMap = lookupResult
		}
	}

	// Helper function to parse name into first and last name
	parseName := func(name transl.Localizable[string]) (firstName, lastName string) {
		// Extract localized name string using context
		nameStr := name.GetContentByContext(ctx)
		parts := strings.SplitN(strings.TrimSpace(nameStr), " ", 2)
		if len(parts) > 0 {
			firstName = parts[0]
		}
		if len(parts) > 1 {
			lastName = parts[1]
		}
		return
	}

	// Enrich current justification
	if session.Justification != nil {
		// Enrich RequestedByObj
		if !session.Justification.RequestedBy.IsZero() {
			if userInfo, ok := userInfoMap[session.Justification.RequestedBy]; ok {
				firstName, lastName := parseName(userInfo.Name)
				session.Justification.RequestedByObj = &models.AbsenceRequestReviewer{
					Id:        userInfo.Id,
					FirstName: firstName,
					LastName:  lastName,
					Email:     userInfo.Email,
					Picture:   "", // userLookup doesn't provide picture
				}
			}
		}

		// Enrich DecisionByObj
		if !session.Justification.DecisionBy.IsZero() {
			if userInfo, ok := userInfoMap[session.Justification.DecisionBy]; ok {
				firstName, lastName := parseName(userInfo.Name)
				session.Justification.DecisionByObj = &models.AbsenceRequestReviewer{
					Id:        userInfo.Id,
					FirstName: firstName,
					LastName:  lastName,
					Email:     userInfo.Email,
					Picture:   "", // userLookup doesn't provide picture
				}
			}
		}
	}

	// Enrich prevJustifications (only decisionByObj since we don't have requestedBy info)
	for idx := range session.PrevJustifications {
		if !session.PrevJustifications[idx].DecisionBy.IsZero() {
			if userInfo, ok := userInfoMap[session.PrevJustifications[idx].DecisionBy]; ok {
				firstName, lastName := parseName(userInfo.Name)
				session.PrevJustifications[idx].DecisionByObj = &models.AbsenceRequestReviewer{
					Id:        userInfo.Id,
					FirstName: firstName,
					LastName:  lastName,
					Email:     userInfo.Email,
					Picture:   "", // userLookup doesn't provide picture
				}
			}
		}
	}
}

func (c *coachingsvcs) GetCoachingProgramMembers(ctx context.Context, coachingProgramId string, skip, limit int64, query *query.Conditions) (*models.CoachingProgramMemberPagination, error) {
	if canViewMembers, err := c.authSvc.CanViewCoachingMembers(ctx, coachingProgramId); err != nil || !canViewMembers {
		return nil, errors.New("unauthorized to view coaching members")
	}

	_id, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}

	filter := bson.M{}
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("invalid convert to mongo filters")
		}
	}

	// 1. Base pipeline to find unique students in this program
	// We group by studentId first to distinct them.
	basePipeline := []bson.M{
		{"$match": bson.M{"coachingProgramId": _id, "trash": false}},
		{"$group": bson.M{
			"_id":           "$studentId",
			"numOfSessions": bson.M{"$sum": 1},
			"completedSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", enums.StudentSessionStatusCompleted}},
					1,
					0,
				},
			}},
			// count sessions that are NOT membershipRemoved (treat missing as not removed)
			"activeSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$ne": bson.A{"$membershipRemoved", true}},
					1,
					0,
				},
			}},
			"lastSessionDate": bson.M{"$max": "$date"},
			"startDate":       bson.M{"$min": "$date"},
		}},
		{"$addFields": bson.M{"progress": bson.M{"$toInt": bson.M{"$multiply": bson.A{bson.M{"$divide": bson.A{"$completedSessions", "$numOfSessions"}}, 100}}}}},
		// Exclude students that have no active sessions (all sessions marked membershipRemoved)
		{"$match": bson.M{"activeSessions": bson.M{"$gt": 0}}},
	}

	// 2. Lookup student details EARLY so we can filter by name/email
	studentLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "student",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$student",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$set": bson.M{
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"email":     "$student.accountStudentSetting.email",
				"picture":   "$student.generalInfo.picture",
			},
		},
	}

	// Combine base + lookup
	pipeline := append(basePipeline, studentLookup...)

	// 3. Apply Filters (now valid because student details exist)
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// 4. Count Total Students (Efficiently using the pipeline built so far)
	count, err := c.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// 5. Pagination & Session Fetching
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
		bson.M{"$lookup": bson.M{
			"from": "academyCoachingStudentSessions",
			"let":  bson.M{"sid": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$eq": []string{"$studentId", "$$sid"}}, "coachingProgramId": _id, "trash": false}},
				{"$sort": bson.M{"date": -1}},
			},
			"as": "sessions",
		}},
	)

	var result []models.CoachingProgramMember
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{TotalPages: totalPages, PerPage: limit, TotalCount: count}

	return &models.CoachingProgramMemberPagination{Members: result, Pagination: pagination}, nil
}

// GetSessionDetailsForSupervisor returns enriched session details for supervisor dashboard
func (c *coachingsvcs) GetSessionDetailsForSupervisor(ctx context.Context, sessionId string, supervisorId string) (*models.SessionDetailsResponse, error) {
	_sessionId, err := primitive.ObjectIDFromHex(sessionId)
	if err != nil {
		return nil, errors.New("invalid session id")
	}

	_supervisorId, err := primitive.ObjectIDFromHex(supervisorId)
	if err != nil {
		return nil, errors.New("invalid supervisor id")
	}

	// Aggregate session with student, teacher, and coaching lookups
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _sessionId, "trash": false}},
		// Lookup student
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "studentInfo",
		}},
		{"$unwind": bson.M{
			"path":                       "$studentInfo",
			"preserveNullAndEmptyArrays": false,
		}},
		// Lookup teacher
		{"$lookup": bson.M{
			"from":         "academyTeachers",
			"localField":   "teacherId",
			"foreignField": "_id",
			"as":           "teacherInfo",
		}},
		{"$unwind": bson.M{
			"path":                       "$teacherInfo",
			"preserveNullAndEmptyArrays": false,
		}},
		// Verify teacher belongs to this supervisor
		{"$match": bson.M{
			"teacherInfo.subjects.supervisorId": _supervisorId,
		}},
		// Lookup coaching
		{"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "coachingProgramId",
			"foreignField": "_id",
			"as":           "coachingInfo",
		}},
		{"$unwind": bson.M{
			"path":                       "$coachingInfo",
			"preserveNullAndEmptyArrays": true,
		}},
		// Project desired fields
		{"$addFields": bson.M{
			"student": bson.M{
				"firstName": "$studentInfo.generalInfo.firstName",
				"lastName":  "$studentInfo.generalInfo.lastName",
				"picture":   "$studentInfo.generalInfo.picture",
			},
			"teacherFirstName": "$teacherInfo.firstName",
			"teacherLastName":  "$teacherInfo.lastName",
			"coachingTitle":    "$coachingInfo.title",
		}},
		{"$project": bson.M{
			"studentInfo":  0,
			"teacherInfo":  0,
			"coachingInfo": 0,
		}},
	}

	// Temporary struct to capture firstName and lastName separately
	type sessionDetailsTemp struct {
		models.StudentSession `bson:",inline"`
		Student               models.StudentInfo         `bson:"student" json:"student"`
		TeacherFirstName      transl.Localizable[string] `bson:"teacherFirstName" json:"teacherFirstName"`
		TeacherLastName       transl.Localizable[string] `bson:"teacherLastName" json:"teacherLastName"`
		CoachingTitle         transl.Localizable[string] `bson:"coachingTitle" json:"coachingTitle"`
	}

	var tempResult []sessionDetailsTemp
	err = c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &tempResult)
	})
	if err != nil {
		return nil, err
	}

	if len(tempResult) == 0 {
		return nil, errors.New("session not found or you don't have access")
	}

	temp := tempResult[0]

	// Merge teacher firstName and lastName into full name
	teacherName := mergeLocalizableNames(temp.TeacherFirstName, temp.TeacherLastName)

	sessionResponse := &models.SessionDetailsResponse{
		StudentSession: temp.StudentSession,
		Student:        temp.Student,
		TeacherName:    teacherName,
		CoachingTitle:  temp.CoachingTitle,
	}

	// Enrich justification object with requestedByObj and decisionByObj
	// SessionDetailsResponse embeds StudentSession, so we can access justification through it
	c.enrichJustification(ctx, &sessionResponse.StudentSession)

	return sessionResponse, nil
}

// mergeLocalizableNames combines firstName and lastName Localizable maps into a full name
// Preserves all language keys from both maps
func mergeLocalizableNames(firstName, lastName transl.Localizable[string]) transl.Localizable[string] {
	result := make(transl.Localizable[string])

	// Collect all language keys from both maps
	allLangs := make(map[string]bool)
	for k := range firstName {
		allLangs[k] = true
	}
	for k := range lastName {
		allLangs[k] = true
	}

	// Merge for each language
	for lang := range allLangs {
		fn := firstName[lang]
		ln := lastName[lang]
		if fn == "" && ln == "" {
			continue
		}
		// Combine with space, trim if needed
		fullName := fn
		if fn != "" && ln != "" {
			fullName = strings.TrimSpace(fn + " " + ln)
		} else if ln != "" {
			fullName = strings.TrimSpace(ln)
		} else {
			fullName = strings.TrimSpace(fn)
		}
		if fullName != "" {
			result[lang] = fullName
		}
	}

	// If no languages found, return empty map
	return result
}

// GetEnrolledCoachings retrieves all coachings a student is enrolled in
func (c *coachingsvcs) GetEnrolledCoachings(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*models.EnrolledCoachingPagination, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	// Base pipeline: group by coachingProgramId to get unique coachings
	basePipeline := []bson.M{
		{"$match": bson.M{"studentId": _studentId, "trash": false}},
		{"$group": bson.M{
			"_id":           "$coachingProgramId",
			"numOfSessions": bson.M{"$sum": 1},
			"completedSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", enums.StudentSessionStatusCompleted}},
					1,
					0,
				},
			}},
			"startDate":       bson.M{"$min": "$date"},
			"lastSessionDate": bson.M{"$max": "$date"},
			"allSessions":     bson.M{"$push": "$$ROOT"},
		}},
		{"$addFields": bson.M{
			"progress": bson.M{
				"$toInt": bson.M{
					"$multiply": bson.A{
						bson.M{"$divide": bson.A{"$completedSessions", "$numOfSessions"}},
						100,
					},
				},
			},
		}},
	}

	// Lookup coaching details
	coachingLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "coaching",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$coaching",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$match": bson.M{"coaching.trash": false},
		},
	}

	pipeline := append(basePipeline, coachingLookup...)

	// Lookup school details
	schoolLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "coaching.schoolId",
				"foreignField": "_id",
				"as":           "schoolObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$schoolObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
	}

	pipeline = append(pipeline, schoolLookup...)

	// Apply user-provided filters
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Count total coachings
	count, err := c.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Add pagination
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	// Lookup teachers
	teacherLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from": "academyTeachers",
				"let":  bson.M{"teacherIds": "$coaching.coachs"},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{"$in": []string{"$_id", "$$teacherIds"}},
						"trash": false,
					}},
					{"$project": bson.M{
						"_id":       1,
						"firstName": 1,
						"lastName":  1,
						"email":     1,
						"picture":   1,
					}},
				},
				"as": "teachers",
			},
		},
	}

	pipeline = append(pipeline, teacherLookup...)

	// Get upcoming and recent sessions
	now := time.Now()
	sessionLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from": "academyCoachingStudentSessions",
				"let":  bson.M{"coachingId": "$_id", "sid": _studentId},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$eq": []string{"$coachingProgramId", "$$coachingId"}},
								bson.M{"$eq": []string{"$studentId", "$$sid"}},
								bson.M{"$eq": []interface{}{"$trash", false}},
							},
						},
						"date": bson.M{"$gte": now},
					}},
					{"$sort": bson.M{"date": 1}},
					{"$limit": 10}, // Get next 10 upcoming sessions
				},
				"as": "upcomingSessions",
			},
		},
		{
			"$lookup": bson.M{
				"from": "academyCoachingStudentSessions",
				"let":  bson.M{"coachingId": "$_id", "sid": _studentId},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$eq": []string{"$coachingProgramId", "$$coachingId"}},
								bson.M{"$eq": []string{"$studentId", "$$sid"}},
								bson.M{"$eq": []interface{}{"$trash", false}},
								bson.M{"$in": bson.A{
									"$status",
									bson.A{enums.StudentSessionStatusCompleted, enums.StudentSessionStatusMissed},
								}},
							},
						},
					}},
					{"$sort": bson.M{"date": -1}},
					{"$limit": 5}, // Get last 5 completed sessions
				},
				"as": "recentSessions",
			},
		},
	}

	pipeline = append(pipeline, sessionLookup...)

	// Project final structure
	projectStage := bson.M{
		"$project": bson.M{
			"coaching": bson.M{
				"_id":             "$coaching._id",
				"title":           "$coaching.title",
				"description":     "$coaching.description",
				"thumbnail":       "$coaching.thumbnail",
				"programType":     "$coaching.programType",
				"level":           "$coaching.level",
				"class":           bson.M{"$arrayElemAt": bson.A{"$coaching.class", 0}},
				"schoolType":      "$coaching.schoolType",
				"schoolId":        "$coaching.schoolId",
				"schoolObj":       "$schoolObj",
				"country":         "$coaching.country",
				"minSessionNum":   "$coaching.minSessionNum",
				"maxSessionNum":   "$coaching.maxSessionNum",
				"sessionDuration": "$coaching.sessionDuration",
				"createdAt":       "$coaching.createdAt",
				"updatedAt":       "$coaching.updatedAt",
			},
			"progress":          1,
			"numOfSessions":     1,
			"completedSessions": 1,
			"startDate":         1,
			"lastSessionDate":   1,
			"nextSession": bson.M{
				"$arrayElemAt": bson.A{
					"$upcomingSessions",
					0,
				},
			},
			"upcomingSessions": bson.M{
				"$slice": bson.A{
					"$upcomingSessions",
					5,
				},
			},
			"recentSessions": bson.M{
				"$slice": bson.A{"$recentSessions", 5},
			},
			"teachers": bson.M{
				"$map": bson.M{
					"input": "$teachers",
					"as":    "teacher",
					"in": bson.M{
						"_id":       "$$teacher._id",
						"firstName": "$$teacher.firstName",
						"lastName":  "$$teacher.lastName",
						"email":     "$$teacher.email",
						"picture":   "$$teacher.picture",
					},
				},
			},
		},
	}

	pipeline = append(pipeline, projectStage)

	var result []models.EnrolledCoachingResponse
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.EnrolledCoachingPagination{
		Coachings:  result,
		Pagination: pagination,
	}, nil
}

// handleSessionStatusChange handles automatic recording/rejection of teacher hours based on session status changes
func (c *coachingsvcs) handleSessionStatusChange(ctx context.Context, oldStatus, newStatus enums.StudentSessionStatus, session *models.StudentSession) {
	// Record hours when status changes to completed
	if oldStatus != enums.StudentSessionStatusCompleted && newStatus == enums.StudentSessionStatusCompleted {
		// Calculate occurredAt from Date + From time
		occurredAt := session.Date
		if session.From != "" {
			// Parse time from "HH:MM" format and add to date
			fromParts := strings.Split(session.From, ":")
			if len(fromParts) == 2 {
				hour, _ := strconv.Atoi(fromParts[0])
				min, _ := strconv.Atoi(fromParts[1])
				occurredAt = time.Date(session.Date.Year(), session.Date.Month(), session.Date.Day(), hour, min, 0, 0, session.Date.Location())
			}
		}
		durationMinutes := float64(session.DurationMinutes)
		if durationMinutes == 0 {
			// Default to 60 minutes if not set
			durationMinutes = 60
		}
		_ = c.compensationSvcs.RecordHoursFromSession(ctx, session.Id, session.TeacherId, session.CoachingProgramId, occurredAt, durationMinutes)
	}

	// Reject hours when status changes from completed to cancelled or rescheduled
	if oldStatus == enums.StudentSessionStatusCompleted && (newStatus == enums.StudentSessionStatusCancelled || newStatus == enums.StudentSessionStatusRescheduled) {
		reason := "Session " + string(newStatus)
		_ = c.compensationSvcs.RejectHoursBySessionId(ctx, session.Id, reason)
	}
}

// GetAllStudentSessions retrieves all student sessions by student ID regardless of coaching ID with dynamic filtering
func (c *coachingsvcs) GetAllStudentSessions(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*models.StudentSessionListPagination, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	// Base filter
	baseFilter := bson.M{
		"studentId": _studentId,
		"trash":     false,
	}

	// Convert query to MongoDB filter
	var queryFilter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		queryFilter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("invalid convert to mongo filters")
		}

		// Merge with base filter
		if len(queryFilter) > 0 {
			baseFilter = bson.M{
				"$and": []bson.M{
					baseFilter,
					queryFilter,
				},
			}
		}
	}

	pipeline := []bson.M{
		{"$match": baseFilter},
	}

	count, err := c.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"date": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	if err := c.enrichSessionsWithZoom(ctx, result); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.StudentSessionListPagination{
		Sessions:   result,
		Pagination: pagination,
	}, nil
}

// GetStudentCoachingStats retrieves all coaching info with comprehensive stats for a student
func (c *coachingsvcs) GetStudentCoachingStats(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*models.StudentCoachingStatsPagination, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	var queryFilter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		queryFilter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("invalid convert to mongo filters")
		}
	}

	now := time.Now()

	// Base pipeline: group by coachingProgramId
	basePipeline := []bson.M{
		{"$match": bson.M{"studentId": _studentId, "trash": false}},
		{"$group": bson.M{
			"_id":           "$coachingProgramId",
			"numOfSessions": bson.M{"$sum": 1},
			"numOfCompletedSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", enums.StudentSessionStatusCompleted}},
					1,
					0,
				},
			}},
			"numOfAbsenceSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", enums.StudentSessionStatusMissed}},
					1,
					0,
				},
			}},
			"numOfPositiveRatings": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"$gte": bson.A{"$rating", 3}},
						bson.M{"$gt": bson.A{"$rating", 0}},
					}},
					1,
					0,
				},
			}},
			"numOfNegativeRatings": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"$lt": bson.A{"$rating", 3}},
						bson.M{"$gt": bson.A{"$rating", 0}},
					}},
					1,
					0,
				},
			}},
			"allSessions": bson.M{"$push": "$$ROOT"},
		}},
	}

	// Lookup coaching details
	coachingLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "coaching",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$coaching",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$match": bson.M{"coaching.trash": false},
		},
		{
			"$addFields": bson.M{
				"coachingId":        "$coaching._id",
				"coachingTitle":     "$coaching.title",
				"coachingThumbnail": "$coaching.thumbnail",
			},
		},
	}

	pipeline := append(basePipeline, coachingLookup...)

	// Apply user-provided filters
	if len(queryFilter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": queryFilter})
	}

	// Count total coachings
	count, err := c.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Add pagination
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	// Get upcoming sessions for each coaching
	upcomingSessionsLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from": "academyCoachingStudentSessions",
				"let":  bson.M{"coachingId": "$_id", "sid": _studentId},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$eq": []string{"$coachingProgramId", "$$coachingId"}},
								bson.M{"$eq": []string{"$studentId", "$$sid"}},
								bson.M{"$eq": []interface{}{"$trash", false}},
								bson.M{"$eq": []string{"$status", string(enums.StudentSessionStatusScheduled)}},
								bson.M{"$gte": []interface{}{"$date", now}},
							},
						},
					}},
					{"$sort": bson.M{"date": 1}},
					{"$limit": 10}, // Get next 10 upcoming sessions
				},
				"as": "restSessions",
			},
		},
	}

	pipeline = append(pipeline, upcomingSessionsLookup...)

	// Lookup assignments for each coaching
	assignmentsLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from": "academyCoachingStudentAssignments",
				"let":  bson.M{"coachingId": "$_id", "sid": _studentId},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{
							"$and": bson.A{
								bson.M{"$eq": []string{"$coachingId", "$$coachingId"}},
								bson.M{"$eq": []string{"$studentId", "$$sid"}},
								bson.M{"$eq": []interface{}{"$trash", false}},
							},
						},
					}},
					{"$group": bson.M{
						"_id":                 nil,
						"numOfAllAssignments": bson.M{"$sum": 1},
						"numOfDoneAssignments": bson.M{"$sum": bson.M{
							"$cond": bson.A{
								bson.M{"$eq": bson.A{"$status", "submitted"}},
								1,
								0,
							},
						}},
						"numOfNotDoneAssignments": bson.M{"$sum": bson.M{
							"$cond": bson.A{
								bson.M{"$eq": bson.A{"$status", "pending"}},
								1,
								0,
							},
						}},
					}},
				},
				"as": "assignments",
			},
		},
		{
			"$addFields": bson.M{
				"numOfAllAssignments": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$assignments.numOfAllAssignments", 0}},
						0,
					},
				},
				"numOfDoneAssignments": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$assignments.numOfDoneAssignments", 0}},
						0,
					},
				},
				"numOfNotDoneAssignments": bson.M{
					"$ifNull": []interface{}{
						bson.M{"$arrayElemAt": []interface{}{"$assignments.numOfNotDoneAssignments", 0}},
						0,
					},
				},
			},
		},
	}

	pipeline = append(pipeline, assignmentsLookup...)

	// Project final structure
	projectStage := bson.M{
		"$project": bson.M{
			"coachingId":              1,
			"coachingTitle":           1,
			"coachingThumbnail":       1,
			"numOfSessions":           1,
			"numOfCompletedSessions":  1,
			"restSessions":            bson.M{"$slice": bson.A{"$restSessions", 10}},
			"numOfAbsenceSessions":    1,
			"numOfAllAssignments":     1,
			"numOfDoneAssignments":    1,
			"numOfNotDoneAssignments": 1,
			"numOfPositiveRatings":    1,
			"numOfNegativeRatings":    1,
		},
	}

	pipeline = append(pipeline, projectStage)

	var result []models.StudentCoachingStatsResponse
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	for i := range result {
		if err := c.enrichSessionsWithZoom(ctx, result[i].RestSessions); err != nil {
			return nil, err
		}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.StudentCoachingStatsPagination{
		Stats:      result,
		Pagination: pagination,
	}, nil
}

// GetStudentSessionsForCoaching returns a student's sessions for one coaching program with date, from, to, status, sessionNum.
// requesterId must be the student or a parent of the student.
func (c *coachingsvcs) GetStudentSessionsForCoaching(ctx context.Context, coachingProgramId string, studentId string, requesterId string) (*models.StudentCoachingSessionsResponse, error) {
	_cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}
	_requesterId, err := primitive.ObjectIDFromHex(requesterId)
	if err != nil {
		return nil, errors.New("invalid requester id")
	}

	// Authorization: requester must be the student or a parent of the student
	if _requesterId != _studentId {
		isParent, err := c.parentSvcs.IsUserParentOfStudent(ctx, _requesterId, _studentId)
		if err != nil {
			return nil, fmt.Errorf("error checking parent relationship: %w", err)
		}
		if !isParent {
			return nil, errors.New("unauthorized: requester is not the student or a parent")
		}
	}

	// Verify coaching exists and student is enrolled (has at least one session)
	filter := bson.M{
		"coachingProgramId": _cpId,
		"studentId":         _studentId,
		"trash":             false,
	}
	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"date": 1}},
	}

	var sessions []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	}); err != nil {
		return nil, err
	}

	slots := make([]models.StudentCoachingSessionSlot, 0, len(sessions))
	var nextSession *models.StudentCoachingSessionSlot
	now := time.Now()

	for i := range sessions {
		s := &sessions[i]
		slot := models.StudentCoachingSessionSlot{
			Id:         s.Id,
			Date:       s.Date,
			From:       s.From,
			To:         s.To,
			Status:     s.Status,
			SessionNum: s.SessionNum,
		}
		slots = append(slots, slot)
		// Next session: first upcoming (date >= today) with status pending, scheduled, or rescheduled
		if nextSession == nil && !s.Date.Before(now) &&
			(s.Status == enums.StudentSessionStatusPending || s.Status == enums.StudentSessionStatusScheduled || s.Status == enums.StudentSessionStatusRescheduled) {
			nextSession = &slot
		}
	}

	return &models.StudentCoachingSessionsResponse{
		Sessions:    slots,
		NextSession: nextSession,
	}, nil
}

// CalculateCommitmentProgress calculates and updates commitment progress for a student in a coaching
func (c *coachingsvcs) RemoveCoachingProgramMember(ctx context.Context, coachingProgramId string, studentId string) error {
	cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return errors.New("invalid coaching program id")
	}
	_sId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return errors.New("invalid student id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Authorization
	isAdmin, err := util.HasCapability(ctx, CapabilityManageAllCoachingPrograms)
	if err != nil {
		return err
	}
	if !isAdmin {
		isAdmin, err = util.HasCapability(ctx, CapabilityRemoveAnyMemberFromCoaching)
		if err != nil {
			return err
		}
	}

	if !isAdmin {
		hasManaged, err := util.HasCapability(ctx, CapabilityRemoveManagedMemberFromCoaching)
		if err != nil {
			return err
		}
		if !hasManaged {
			return errors.New("you are not authorized to remove members from this coaching")
		}

		// Verify user is assigned to this coaching (teacher or supervisor)
		teacherIds, err := c.teacherAssignmentSvc.GetCoachingTeachers(ctx, cpId)
		if err != nil {
			return errors.New("coaching not found")
		}
		assigned := false
		for _, tid := range teacherIds {
			if tid == cfg.User.Id {
				assigned = true
				break
			}
		}
		if !assigned {
			coaching, err := c.coachingRepo.GetByFilter(ctx, bson.M{"_id": cpId, "trash": false})
			if err != nil {
				return errors.New("coaching not found")
			}
			for _, sup := range coaching.SupervisorIds {
				if sup == cfg.User.Id {
					assigned = true
					break
				}
			}
		}
		if !assigned {
			return errors.New("you are not assigned to this coaching")
		}
	}

	// Find all sessions for this student in the coaching program
	var sessions []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"coachingProgramId": cpId, "studentId": _sId, "trash": false}}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	}); err != nil {
		return err
	}

	if len(sessions) == 0 {
		return errors.New("no sessions found for this student in this coaching")
	}

	// Build bulk operations: separate completed/missed (preserve status) from cancellable sessions
	writeOps := []mongo.WriteModel{}
	cancellable := make([]models.StudentSession, 0)
	now := time.Now()
	for _, sess := range sessions {
		// If session is completed or missed, preserve its status and just add removal metadata
		if sess.Status == enums.StudentSessionStatusCompleted || sess.Status == enums.StudentSessionStatusMissed {
			upd := mongo.NewUpdateOneModel()
			upd.SetFilter(bson.M{"_id": sess.Id, "trash": false})
			upd.SetUpdate(bson.M{"$set": bson.M{
				"membershipRemoved":   true,
				"membershipRemovedAt": now,
				"membershipRemovedBy": cfg.User.Id,
				"updatedAt":           now,
				"updatedBy":           cfg.User.Id,
			}})
			writeOps = append(writeOps, upd)
		} else {
			// Future or actionable sessions: cancel/soft-delete them and mark removal
			upd := mongo.NewUpdateOneModel()
			upd.SetFilter(bson.M{"_id": sess.Id, "trash": false})
			upd.SetUpdate(bson.M{"$set": bson.M{
				"trash":               true,
				"status":              enums.StudentSessionStatusCancelled,
				"membershipRemoved":   true,
				"membershipRemovedAt": now,
				"membershipRemovedBy": cfg.User.Id,
				"updatedAt":           now,
				"updatedBy":           cfg.User.Id,
			}})
			writeOps = append(writeOps, upd)
			cancellable = append(cancellable, sess)
		}
	}

	if len(writeOps) > 0 {
		if _, err := c.studentSessionRepo.BulkWrite(ctx, writeOps); err != nil {
			return err
		}
	}

	// Soft-delete CoachingStudent record if exists
	filter := bson.M{"studentId": _sId, "coachingId": cpId, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": now, "updatedBy": cfg.User.Id}}
	if _, err := c.coachingStudentRepo.Patch(ctx, filter, update); err != nil {
		// Ignore not found
		if !errors.Is(err, mongo.ErrNoDocuments) {
			return err
		}
	}

	// Handle compensation/audit side-effects only for sessions that were cancelled now
	for _, sess := range cancellable {
		temp := sess
		temp.Status = enums.StudentSessionStatusCancelled
		c.handleSessionStatusChange(ctx, sess.Status, enums.StudentSessionStatusCancelled, &temp)
	}

	// Recalculate commitment progress for the student in this coaching
	_ = c.CalculateCommitmentProgress(ctx, studentId, coachingProgramId)

	return nil
}

func (c *coachingsvcs) CalculateCommitmentProgress(ctx context.Context, studentId string, coachingId string) error {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return errors.New("invalid student id")
	}

	_coachingId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return errors.New("invalid coaching id")
	}

	// Get CoachingStudent record
	coachingStudent, err := c.coachingStudentRepo.GetByFilter(ctx, bson.M{
		"studentId":  _studentId,
		"coachingId": _coachingId,
		"trash":      false,
	})
	if err != nil {
		// If CoachingStudent doesn't exist, create it
		coachingStudent = &models.CoachingStudent{
			Id:           primitive.NewObjectID(),
			StudentId:    _studentId,
			CoachingId:   _coachingId,
			RenewMonthly: false,
			CreatedAt:    time.Now(),
			UpdatedAt:    time.Now(),
			Trash:        false,
		}
		if err := c.coachingStudentRepo.Add(ctx, coachingStudent); err != nil {
			return err
		}
	}

	// Calculate session commitment
	// Total booked sessions = sessions with status != "pending"
	sessionPipeline := []bson.M{
		{"$match": bson.M{
			"studentId":         _studentId,
			"coachingProgramId": _coachingId,
			"trash":             false,
			"status":            bson.M{"$ne": enums.StudentSessionStatusPending},
		}},
		{"$group": bson.M{
			"_id":         nil,
			"totalBooked": bson.M{"$sum": 1},
			"completed": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", enums.StudentSessionStatusCompleted}},
					1,
					0,
				},
			}},
		}},
	}

	var sessionStats []bson.M
	if err := c.studentSessionRepo.Aggregate(ctx, sessionPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessionStats)
	}); err != nil {
		return err
	}

	var sessionCommitment float64
	if len(sessionStats) > 0 {
		totalBooked := 0
		completed := 0
		if totalBookedVal, ok := sessionStats[0]["totalBooked"].(int32); ok {
			totalBooked = int(totalBookedVal)
		} else if totalBookedVal, ok := sessionStats[0]["totalBooked"].(int64); ok {
			totalBooked = int(totalBookedVal)
		}
		if completedVal, ok := sessionStats[0]["completed"].(int32); ok {
			completed = int(completedVal)
		} else if completedVal, ok := sessionStats[0]["completed"].(int64); ok {
			completed = int(completedVal)
		}

		if totalBooked > 0 {
			sessionCommitment = (float64(completed) / float64(totalBooked)) * 100
		}
	}

	// Calculate assignment commitment
	assignmentPipeline := []bson.M{
		{"$match": bson.M{
			"studentId":  _studentId,
			"coachingId": _coachingId,
			"trash":      false,
		}},
		{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": 1},
			"submitted": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", "submitted"}},
					1,
					0,
				},
			}},
		}},
	}

	var assignmentStats []bson.M
	if err := c.coachingStudentAssignmentRepo.Aggregate(ctx, assignmentPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignmentStats)
	}); err != nil {
		return err
	}

	var assignmentCommitment float64
	if len(assignmentStats) > 0 {
		total := 0
		submitted := 0
		if totalVal, ok := assignmentStats[0]["total"].(int32); ok {
			total = int(totalVal)
		} else if totalVal, ok := assignmentStats[0]["total"].(int64); ok {
			total = int(totalVal)
		}
		if submittedVal, ok := assignmentStats[0]["submitted"].(int32); ok {
			submitted = int(submittedVal)
		} else if submittedVal, ok := assignmentStats[0]["submitted"].(int64); ok {
			submitted = int(submittedVal)
		}

		if total > 0 {
			assignmentCommitment = (float64(submitted) / float64(total)) * 100
		}
	}

	// Overall commitment = (session + assignment) / 2
	overallCommitment := (sessionCommitment + assignmentCommitment) / 2

	// Update CoachingStudent
	coachingStudent.CommitmentProgress = overallCommitment
	coachingStudent.UpdatedAt = time.Now()

	filter := bson.M{"_id": coachingStudent.Id}
	update := bson.M{"$set": bson.M{
		"commitmentProgress": overallCommitment,
		"updatedAt":          coachingStudent.UpdatedAt,
	}}

	_, err = c.coachingStudentRepo.Patch(ctx, filter, update)
	return err
}

func (c *coachingsvcs) enrichSessionsWithZoom(ctx context.Context, sessions []models.StudentSession) error {
	if len(sessions) == 0 {
		return nil
	}

	zoomMeetingIdsToFetch := []primitive.ObjectID{}
	zoomMeetingIdMap := make(map[primitive.ObjectID]primitive.ObjectID)

	for i := range sessions {
		if sessions[i].ZoomMeetingId != nil {
			zoomMeetingIdsToFetch = append(zoomMeetingIdsToFetch, *sessions[i].ZoomMeetingId)
			zoomMeetingIdMap[sessions[i].Id] = *sessions[i].ZoomMeetingId
		}
	}

	if len(zoomMeetingIdsToFetch) == 0 {
		return nil
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": bson.M{"$in": zoomMeetingIdsToFetch}}},
	}

	var meetings []zoommodels.ZoomMeeting
	if err := c.zoomMeetingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &meetings)
	}); err != nil {
		return err
	}

	meetingsMap := make(map[primitive.ObjectID]*models.ZoomMeetingDetails)
	for _, m := range meetings {
		meetingsMap[m.Id] = &models.ZoomMeetingDetails{
			Id:              m.Id,
			ZoomMeetingId:   m.ZoomMeetingId,
			JoinUrl:         m.JoinUrl,
			Password:        m.Password,
			StartTime:       m.StartTime,
			DurationMinutes: m.DurationMinutes,
			Status:          m.Status,
			ActualStartTime: m.ActualStartTime,
			ActualEndTime:   m.ActualEndTime,
			TeacherJoined:   m.TeacherJoined,
			RecordingUrl:    m.RecordingUrl,
		}
	}

	for i := range sessions {
		if sessions[i].ZoomMeetingId != nil {
			if zoomDetails, ok := meetingsMap[*sessions[i].ZoomMeetingId]; ok {
				sessions[i].ZoomMeeting = zoomDetails
			}
		}
	}

	return nil
}
