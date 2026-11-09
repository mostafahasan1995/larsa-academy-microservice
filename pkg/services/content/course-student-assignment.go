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

var populateCourseStudentData = []bson.M{
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

func (c *coursesvcs) SubmitStudentCourseAssignment(ctx context.Context, assignmentId string, data *models.CourseStudentAssignmentDto) (*models.CourseStudentAssignment, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the assignment to find the course and verify the student can submit
	_id, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	var assignment *models.CourseAssignment
	assignment, err = c.courseAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("assignment not found")
	}

	// Add authorization check - students can submit their own assignments
	// First check if student can access this course
	canView, err := c.authSvc.CanViewStudentCourseData(ctx, cfg.User.Id.Hex())
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to submit assignments")
	}

	if assignment.Deadline.Before(time.Now()) {
		return nil, errors.New("assignment deadline has passed")
	}
	if assignment.Status == "submitted" {
		return nil, errors.New("assignment already submitted")
	}

	courseStudentAssignment := &models.CourseStudentAssignment{
		Id:                         primitive.NewObjectID(),
		AssignmentId:               assignment.Id,
		CourseId:                   assignment.CourseId,
		StudentId:                  cfg.User.Id,
		CourseStudentAssignmentDto: *data,
		SubmissionDate:             time.Now(),
		CreatedAt:                  time.Now(),
		CreatedBy:                  cfg.User.Id,
		UpdatedAt:                  time.Now(),
		UpdatedBy:                  cfg.User.Id,
	}

	if err := c.courseStudentAssignmentRepo.Add(ctx, courseStudentAssignment); err != nil {
		return nil, err
	}

	return courseStudentAssignment, nil
}

func (c *coursesvcs) GetAllStudentCourseAssignment(ctx context.Context, query *query.Conditions) ([]models.CourseStudentAssignment, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	// Add authorization filter - users can only see assignments they're authorized to view
	baseFilter := bson.M{"trash": false}

	// Students can only see their own assignments
	// Admins with ViewAllStudentCourseData can see all assignments
	canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllStudentCourseData)
	if !canViewAll {
		// Non-admin users can only see their own assignments
		baseFilter["studentId"] = cfg.User.Id
	}

	pipeline := []bson.M{
		{"$match": baseFilter},
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
	}

	var result []models.CourseStudentAssignment
	if err := c.courseStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *coursesvcs) GradeCourseStudentAssignment(ctx context.Context, studentAssignmentId string, data *models.GradeCourseStudentAssignmentDto) (*models.CourseStudentAssignment, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	canGrade, _ := util.HasCapability(ctx, CapabilityAccessTeacherEndpoints)
	canManageAll, _ := util.HasCapability(ctx, CapabilityManageAllCourses)
	if !canGrade && !canManageAll {
		return nil, errors.New("unauthorized to grade assignments")
	}

	_id, err := primitive.ObjectIDFromHex(studentAssignmentId)
	if err != nil {
		return nil, errors.New("invalid student assignment id")
	}

	existingAssignment, err := c.courseStudentAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("student assignment not found")
	}

	now := time.Now()
	existingAssignment.Grade = &data.Grade
	existingAssignment.Feedback = data.Feedback
	existingAssignment.GradedAt = &now
	existingAssignment.GradedBy = cfg.User.Id
	existingAssignment.UpdatedAt = now
	existingAssignment.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": existingAssignment}

	updatedAssignment, err := c.courseStudentAssignmentRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	_ = c.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{updatedAssignment.StudentId}, &messageModels.Notification{
		ServiceName: "academy",
		Title:       "Assignment graded",
		Content:     "Your teacher has graded your assignment.",
		Recipient:   updatedAssignment.StudentId.Hex(),
	})

	return updatedAssignment, nil
}

func (c *coursesvcs) GetCourseAssignmentStudentSubmissions(ctx context.Context, courseId, assignmentId string, query *query.Conditions) ([]models.CourseStudentAssignmentRes, error) {
	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	asId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		filter, err := query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("invalid convert to mongo filters")
		}

		pipeline := []bson.M{
			{"$match": bson.M{"courseId": cId, "assignmentId": asId, "trash": false}},
			{"$match": filter},
			{"$sort": bson.M{"_id": -1}},
		}
		pipeline = append(pipeline, populateCourseStudentData...)

		var result []models.CourseStudentAssignmentRes
		if err := c.courseStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result)
		}); err != nil {
			return nil, err
		}

		return result, nil
	}

	pipeline := []bson.M{
		{"$match": bson.M{"courseId": cId, "assignmentId": asId, "trash": false}},
		{"$sort": bson.M{"_id": -1}},
	}
	pipeline = append(pipeline, populateCourseStudentData...)

	var result []models.CourseStudentAssignmentRes
	if err := c.courseStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *coursesvcs) GetAllCourseAssignmentSubmissions(ctx context.Context, assignmentId string, query *query.Conditions) ([]models.CourseStudentAssignmentRes, error) {
	asId, err := primitive.ObjectIDFromHex(assignmentId)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		filter, err := query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("invalid convert to mongo filters")
		}

		pipeline := []bson.M{
			{"$match": bson.M{"assignmentId": asId, "trash": false}},
			{"$match": filter},
			{"$sort": bson.M{"_id": -1}},
		}
		pipeline = append(pipeline, populateCourseStudentData...)

		var result []models.CourseStudentAssignmentRes
		if err := c.courseStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result)
		}); err != nil {
			return nil, err
		}

		return result, nil
	}

	pipeline := []bson.M{
		{"$match": bson.M{"assignmentId": asId, "trash": false}},
		{"$sort": bson.M{"_id": -1}},
	}
	pipeline = append(pipeline, populateCourseStudentData...)

	var result []models.CourseStudentAssignmentRes
	if err := c.courseStudentAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

func (c *coursesvcs) GetCourseAssignmentsForStudent(ctx context.Context, courseId string) ([]models.CourseAssignmentWithAttempt, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	cId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	// 1. Verify enrollment
	enrollment, err := c.studentCourseRepo.GetByFilter(ctx, bson.M{
		"courseId":  cId,
		"studentId": cfg.User.Id,
		"trash":     false,
	})
	if err != nil || enrollment == nil {
		return nil, errors.New("student not enrolled in this course")
	}

	// 2. Get all active assignments for the course
	var assignments []models.CourseAssignment
	err = c.courseAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"courseId": cId,
			"trash":    false,
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

	var attempts []models.CourseStudentAssignment
	if len(assignmentIds) > 0 {
		err = c.courseStudentAssignmentRepo.Aggregate(ctx, []bson.M{
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
	attemptMap := make(map[primitive.ObjectID]models.CourseStudentAssignment)
	for _, a := range attempts {
		attemptMap[a.AssignmentId] = a
	}

	results := make([]models.CourseAssignmentWithAttempt, len(assignments))
	for i, a := range assignments {
		res := models.CourseAssignmentWithAttempt{
			Id:          a.Id,
			Title:       a.Title,
			Description: a.Description,
			Deadline:    a.Deadline,
			Subject:     a.Subject,
			Chapter:     a.Chapter,
			Status:      a.Status,
		}
		if attempt, ok := attemptMap[a.Id]; ok {
			res.Attempt = &attempt
		}
		results[i] = res
	}

	return results, nil
}

func (c *coursesvcs) DeleteCourseStudentAssignment(ctx context.Context, submissionId, courseId string) error {
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

	submission, err := c.courseStudentAssignmentRepo.GetByFilter(ctx, bson.M{"_id": subId, "trash": false})
	if err != nil {
		return errors.New("submission not found")
	}

	if submission.CourseId.Hex() != courseId {
		return errors.New("submission does not belong to this course")
	}

	now := time.Now()
	filter := bson.M{"_id": subId}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": now, "updatedBy": cfg.User.Id}}

	_, err = c.courseStudentAssignmentRepo.Patch(ctx, filter, update)
	return err
}
