package contact

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/services/contact/models"
	content_enums "larsa-academy-microservice/pkg/services/content/enums"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	messagesvcs "larsa-academy-microservice/pkg/services/messaging"
	messagingenums "larsa-academy-microservice/pkg/services/messaging/enums"
	messagingmodels "larsa-academy-microservice/pkg/services/messaging/models"
	messagingtpls "larsa-academy-microservice/pkg/services/messaging/template"
	pointsmodels "larsa-academy-microservice/pkg/services/points/models"
	support_models "larsa-academy-microservice/pkg/services/support/models"

	"larsa-academy-microservice/pkg/services/sorting"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func hasValidWorkingHours(shifts []models.Shift) bool {
	for _, shift := range shifts {
		if shift.From != "" && shift.To != "" {
			return true
		}
	}
	return false
}

func filterValidWorkingHours(shifts []models.Shift) []models.Shift {
	var valid []models.Shift
	for _, shift := range shifts {
		if shift.From != "" && shift.To != "" {
			valid = append(valid, shift)
		}
	}
	return valid
}

func syncDefaultWorkingHoursForTeacher(teacher *models.Teacher) {
	if teacher.Availability.DefaultWorkingHours == nil || len(teacher.Availability.DefaultWorkingHours) == 0 {
		return
	}

	defaultWorkingHours := filterValidWorkingHours(teacher.Availability.DefaultWorkingHours)
	if len(defaultWorkingHours) == 0 {
		return
	}

	for day, availability := range teacher.Availability.DayOfWeekAvailability {
		if !availability.IsCustom {
			availability.WorkingHours = filterValidWorkingHours(availability.WorkingHours)
			if len(availability.WorkingHours) == 0 {
				newWorkingHours := make([]models.Shift, len(defaultWorkingHours))
				copy(newWorkingHours, defaultWorkingHours)
				availability.WorkingHours = newWorkingHours
			}
			teacher.Availability.DayOfWeekAvailability[day] = availability
		}
	}
}

type TeacherSvcs interface {
	GetById(ctx context.Context, id string) (*models.Teacher, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherPagination, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]*models.TeacherWithStats, error)
	Add(ctx context.Context, data *models.TeacherDto) (*models.Teacher, error)
	AddWithInvite(ctx context.Context, data *models.TeacherDto) (*models.Teacher, error)
	Update(ctx context.Context, id string, data *models.TeacherDto) (*models.Teacher, error)
	Delete(ctx context.Context, id string) error
	GetSummary(ctx context.Context, id string) (*models.TeacherSummary, error)
	GetTodaySummary(ctx context.Context) (*models.TeacherTodaySummary, error)
	GetReviews(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*content_models.ReviewPagination, error)
	GetMyReviews(ctx context.Context, skip, limit int64, query *query.Conditions) (*content_models.ReviewPagination, error)
	GetMe(ctx context.Context) (*models.Teacher, error)
	UpdateMe(ctx context.Context, id string, data *models.TeacherProfileUpdateDto) (*models.Teacher, error)
	GetTeacherStudents(ctx context.Context, skip, limit int64, query *query.Conditions) (*content_models.TeacherStudentPagination, error)
	GetTeacherStudentProfile(ctx context.Context, studentId string) (*content_models.TeacherStudentProfile, error)
	GetTeacherStudentCoachingIds(ctx context.Context, studentId string) ([]primitive.ObjectID, error)
	GetStudentTeachers(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*content_models.StudentTeacherPagination, error)
}

type teachersvcs struct {
	repo                      db.MainRepo[models.Teacher]
	parentStudentRelationRepo db.MainRepo[models.ParentStudentRelation]
	studentRepo               db.MainRepo[models.Student]
	coachingRepo              db.MainRepo[content_models.Coaching]
	courseRepo                db.MainRepo[content_models.Course]
	courseAssignmentRepo      db.MainRepo[content_models.CourseAssignment]
	coachingAssignmentRepo    db.MainRepo[content_models.CoachingAssignment]
	feedbackRepo              db.MainRepo[support_models.Feedback]
	reviewRepo                db.MainRepo[content_models.Review]
	studentSessionRepo        db.MainRepo[content_models.StudentSession]
	studentPointRepo          db.MainRepo[pointsmodels.StudentPoint]
	studentCourseRepo         db.MainRepo[content_models.StudentCourse]
	parentRepo                db.MainRepo[models.Parent]
	bundleRepo                db.MainRepo[content_models.Bundle]
	withtxn                   *db.WithTxn
	credSvcs                  CredSvcs
	sortingsvcs               sorting.SortingSvcs
	messagesvcs               messagesvcs.MessageSvcs
	teacherAssignmentRepo     db.MainRepo[content_models.TeacherAssignment]
	// Helper interface registered via DI to avoid importing content package
	teacherAssignmentHelper TeacherAssignmentBatchHelper
}

func NewTeacherSvcs(i *do.Injector) (TeacherSvcs, error) {
	return &teachersvcs{
		repo:                      db.NewRepo[models.Teacher](i),
		parentStudentRelationRepo: db.NewRepo[models.ParentStudentRelation](i),
		studentRepo:               db.NewRepo[models.Student](i),
		coachingRepo:              db.NewRepo[content_models.Coaching](i),
		courseRepo:                db.NewRepo[content_models.Course](i),
		courseAssignmentRepo:      db.NewRepo[content_models.CourseAssignment](i),
		coachingAssignmentRepo:    db.NewRepo[content_models.CoachingAssignment](i),
		feedbackRepo:              db.NewRepo[support_models.Feedback](i),
		reviewRepo:                db.NewRepo[content_models.Review](i),
		studentSessionRepo:        db.NewRepo[content_models.StudentSession](i),
		studentPointRepo:          db.NewRepo[pointsmodels.StudentPoint](i),
		studentCourseRepo:         db.NewRepo[content_models.StudentCourse](i),
		parentRepo:                db.NewRepo[models.Parent](i),
		bundleRepo:                db.NewRepo[content_models.Bundle](i),
		withtxn:                   do.MustInvoke[*db.WithTxn](i),
		credSvcs:                  do.MustInvoke[CredSvcs](i),
		sortingsvcs:               do.MustInvoke[sorting.SortingSvcs](i),
		messagesvcs:               do.MustInvoke[messagesvcs.MessageSvcs](i),
		teacherAssignmentHelper:   do.MustInvoke[TeacherAssignmentBatchHelper](i),
	}, nil
}
func (t *teachersvcs) GetById(ctx context.Context, id string) (*models.Teacher, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	return t.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
}

func (t *teachersvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := t.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	// Add lookup to calculate ratings
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyFeedbacks",
			"let":  bson.M{"teacherId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []interface{}{"$teacherId", "$$teacherId"}},
							{"$eq": []interface{}{"$trash", false}},
						},
					},
				}},
				{"$group": bson.M{
					"_id": nil,
					"avg": bson.M{"$avg": "$rate"},
				}},
			},
			"as": "ratings",
		},
	})

	// Add project to include rating field
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"rating": bson.M{
				"$ifNull": []interface{}{
					bson.M{"$arrayElemAt": []interface{}{"$ratings.avg", 0}},
					0.0,
				},
			},
		},
	})

	// Remove the ratings array from output
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"ratings": 0,
		},
	})

	var result []models.TeacherWithStats
	errAg := t.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.TeacherPagination{
		Teachers:   result,
		Pagination: pagination,
	}, nil
}

func (t *teachersvcs) GetAll(ctx context.Context, query *query.Conditions) ([]*models.TeacherWithStats, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	// Add lookup to calculate ratings
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyFeedbacks",
			"let":  bson.M{"teacherId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []interface{}{"$teacherId", "$$teacherId"}},
							{"$eq": []interface{}{"$trash", false}},
						},
					},
				}},
				{"$group": bson.M{
					"_id": nil,
					"avg": bson.M{"$avg": "$rate"},
				}},
			},
			"as": "ratings",
		},
	})

	// Add project to include rating field
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"rating": bson.M{
				"$ifNull": []interface{}{
					bson.M{"$arrayElemAt": []interface{}{"$ratings.avg", 0}},
					0.0,
				},
			},
		},
	})

	// Remove the ratings array from output
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"ratings": 0,
		},
	})

	var result []*models.TeacherWithStats
	errAg := t.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	return result, nil
}

func (t *teachersvcs) Add(ctx context.Context, data *models.TeacherDto) (*models.Teacher, error) {

	var deleteUserOnErr func() error

	//var msg *messagingmodels.Message

	result, err := t.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return nil, err
		}

		teacher := &models.Teacher{
			TeacherDto: *data,
			CreatedAt:  time.Now(),
			CreatedBy:  cfg.User.Id,
			UpdatedAt:  time.Now(),
			UpdatedBy:  cfg.User.Id,
		}

		syncDefaultWorkingHoursForTeacher(teacher)

		result, err := t.credSvcs.Add(ctx, teacher, "teacher")
		if err != nil {
			return nil, err
		}
		deleteUserOnErr = result.DeleteOnErr

		teacher.Id = result.UserData.Id

		seq, err := t.sortingsvcs.GetAndUpdateSourceSeq(ctx, "teachers")
		if err != nil {
			return nil, err
		}

		teacher.Seq = fmt.Sprintf("TCH-%d", seq)

		if err := t.repo.Add(ctx, teacher); err != nil {
			return nil, err
		}

		return teacher, nil
	})

	if err != nil {
		if deleteUserOnErr != nil {
			deleteUserOnErr()
		}
		return nil, err
	}

	return result.(*models.Teacher), nil

}
func (t *teachersvcs) AddWithInvite(ctx context.Context, data *models.TeacherDto) (*models.Teacher, error) {

	var deleteUserOnErr func() error
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacher := &models.Teacher{
		TeacherDto: *data,
		CreatedAt:  time.Now(),
		CreatedBy:  cfg.User.Id,
		UpdatedAt:  time.Now(),
		UpdatedBy:  cfg.User.Id,
	}

	syncDefaultWorkingHoursForTeacher(teacher)

	//var msg *messagingmodels.Message

	result, err := t.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {

		result, err := t.credSvcs.Add(ctx, teacher, "teacher")
		if err != nil {
			return nil, err
		}
		deleteUserOnErr = result.DeleteOnErr

		teacher.Id = result.UserData.Id

		seq, err := t.sortingsvcs.GetAndUpdateSourceSeq(ctx, "teachers")
		if err != nil {
			return nil, err
		}

		teacher.Seq = fmt.Sprintf("TCH-%d", seq)

		if err := t.repo.Add(ctx, teacher); err != nil {
			return nil, err
		}

		return teacher, nil
	})

	if err != nil {
		if deleteUserOnErr != nil {
			deleteUserOnErr()
		}
		return nil, err
	}

	msg, err := t.getInvitationEmail(ctx, teacher)
	err = t.messagesvcs.SendEmail(ctx, msg)
	if err != nil {
		return nil, err
	}

	return result.(*models.Teacher), nil

}

func (t *teachersvcs) Update(ctx context.Context, id string, data *models.TeacherDto) (*models.Teacher, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := t.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldTeacher, err := t.GetById(ctx, id)

		if err != nil {
			return nil, err
		}
		teacher := &models.Teacher{
			TeacherDto: *data,
			CreatedAt:  oldTeacher.CreatedAt,
			CreatedBy:  oldTeacher.CreatedBy,
			UpdatedAt:  time.Now(),
			UpdatedBy:  cfg.User.Id,
		}

		syncDefaultWorkingHoursForTeacher(teacher)

		teacher.Id = _id

		// Sync phone if provided in update
		if data.Mobile != "" {
			teacher.Mobile = data.Mobile
		}

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": teacher}

		updatedTeacher, err := t.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if err := t.credSvcs.Update(ctx, id, teacher); err != nil {
			return nil, err
		}

		return updatedTeacher, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Teacher), nil
}

func (t *teachersvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	_, err = t.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		// Update the student document to set trash=true and update timestamps
		filter := bson.M{"_id": _id, "trash": false}
		update := bson.M{
			"$set": bson.M{
				"trash":     true,
				"updatedAt": time.Now(),
				"updatedBy": cfg.User.Id,
			},
		}

		// Use the repo to update the document
		updatedTeacher, err := t.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if updatedTeacher == nil {
			return nil, errors.New("teacher not found or already deleted")
		}
		if err := t.credSvcs.Delete(ctx, id); err != nil {
			return nil, err
		}

		return updatedTeacher, nil
	})

	return err
}

func (t *teachersvcs) getInvitationEmail(ctx context.Context, teacher *models.Teacher) (*messagingmodels.Message, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	val, err := common.GetOptionValue("BUSINESS_NAME", cfg.Hp)
	if err != nil {
		return nil, errors.New("error getting business name")
	}
	businessName, ok := val.(string)
	if !ok {
		businessName = "[Business Name]"
	}

	plat, err := common.GetOptionValue("PLATFORM_NAME", cfg.Hp)
	if err != nil {
		return nil, errors.New("error getting platform name")
	}
	platformName, ok := plat.(string)
	if !ok {
		platformName = "[Platform Name]"
	}

	fUrl, err := common.GetOptionValue("FRONTEND_URL", cfg.Hp)
	if err != nil {
		return nil, errors.New("error getting frontend url")
	}

	frontEndUrl, ok := fUrl.(string)
	if !ok {
		return nil, errors.New("error getting frontend url")
	}

	var invitationTplData *messagingtpls.InvetationTplData

	var email string
	// here

	// Extract default language for invitation template
	memberName := teacher.FirstName.GetContentByLang("en") + " " + teacher.LastName.GetContentByLang("en")
	invitationTplData = &messagingtpls.InvetationTplData{
		MemberName:   strings.TrimSpace(memberName),
		CompanyName:  businessName,
		PlatformName: platformName,
		Email:        teacher.Email,
		Password:     teacher.AccountSettings.InitialPass,
		Link:         frontEndUrl + "/auth/login",
	}
	email = teacher.Email

	message, subject, err := t.messagesvcs.GetTemplateMessage(ctx, messagingenums.INVITATION, invitationTplData)
	if err != nil {
		return nil, err
	}

	msg := &messagingmodels.Message{
		Type:        messagingenums.INVITATION,
		Email:       email,
		Subject:     subject,
		Message:     message,
		MessageHtml: message,
		Target:      "email", //todo: must set in message service
		Others:      map[string]any{},
	}

	return msg, nil
}

func (t *teachersvcs) GetSummary(ctx context.Context, id string) (*models.TeacherSummary, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	teacher, err := t.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get assigned students through StudentSession (similar to GetTeacherStudents)
	studentIds, err := t.GetTeacherStudentsIds(ctx, _id)
	if err != nil {
		return nil, err
	}

	// Convert to ParentStudentRelation format for compatibility
	studentRelations := make([]models.ParentStudentRelation, 0, len(studentIds))
	for _, studentId := range studentIds {
		studentRelations = append(studentRelations, models.ParentStudentRelation{
			Id: primitive.NewObjectID(),
			ParentStudentRelationDto: models.ParentStudentRelationDto{
				FirstId:    _id,
				FirstType:  enums.PersonTypeTeacher,
				SecondId:   studentId,
				SecondType: enums.PersonTypeStudent,
			},
		})
	}
	studentsInterface := make([]interface{}, len(studentRelations))
	for i := range studentRelations {
		studentsInterface[i] = studentRelations[i]
	}

	// Get total delivered sessions
	// Count sessions in coachings where coach = teacher ID and sessions have recordings
	coachingsPipeline := []bson.M{
		{"$match": bson.M{
			"coach": _id,
			"trash": false,
		}},
		{"$unwind": "$sessions"},
		{"$match": bson.M{
			"$or": []bson.M{
				{"sessions.recordings": bson.M{"$exists": true, "$ne": []interface{}{}}},
				{"sessions.notes": bson.M{"$exists": true, "$ne": []interface{}{}}},
			},
		}},
		{"$count": "total"},
	}
	var deliveredSessionsResult []bson.M
	err = t.coachingRepo.Aggregate(ctx, coachingsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &deliveredSessionsResult)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	totalDeliveredSessions := 0
	if len(deliveredSessionsResult) > 0 {
		if count, ok := deliveredSessionsResult[0]["total"].(int32); ok {
			totalDeliveredSessions = int(count)
		}
	}

	// Get assignments created count
	// 1. Course assignments - courses where teacher is in basicInfo.teachers
	coursesPipeline := []bson.M{
		{"$match": bson.M{
			"basicInfo.teachers": _id,
			"trash":              false,
		}},
		{"$project": bson.M{"_id": 1}},
	}
	var courseDocs []bson.M
	err = t.courseRepo.Aggregate(ctx, coursesPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courseDocs)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	courseIds := make([]primitive.ObjectID, 0, len(courseDocs))
	for _, doc := range courseDocs {
		if id, ok := doc["_id"].(primitive.ObjectID); ok {
			courseIds = append(courseIds, id)
		}
	}

	courseAssignmentCount := 0
	if len(courseIds) > 0 {
		courseAssignmentPipeline := []bson.M{
			{"$match": bson.M{
				"courseId": bson.M{"$in": courseIds},
				"trash":    false,
			}},
			{"$count": "total"},
		}
		var courseAssignmentResult []bson.M
		err = t.courseAssignmentRepo.Aggregate(ctx, courseAssignmentPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courseAssignmentResult)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		if len(courseAssignmentResult) > 0 {
			if count, ok := courseAssignmentResult[0]["total"].(int32); ok {
				courseAssignmentCount = int(count)
			}
		}
	}

	// 2. Coaching assignments - coachings where coach = teacher ID
	coachingAssignmentPipeline := []bson.M{
		{"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "coachingId",
			"foreignField": "_id",
			"as":           "coaching",
		}},
		{"$unwind": "$coaching"},
		{"$match": bson.M{
			"coaching.coach": _id,
			"trash":          false,
		}},
		{"$count": "total"},
	}
	var coachingAssignmentResult []bson.M
	err = t.coachingAssignmentRepo.Aggregate(ctx, coachingAssignmentPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachingAssignmentResult)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	coachingAssignmentCount := 0
	if len(coachingAssignmentResult) > 0 {
		if count, ok := coachingAssignmentResult[0]["total"].(int32); ok {
			coachingAssignmentCount = int(count)
		}
	}

	// 3. Subject assignments - subjects in teacher's Subjects array
	subjectIds := make([]primitive.ObjectID, 0)
	for _, subject := range teacher.Subjects {
		if subject.SubjectID != "" {
			if subjectId, err := primitive.ObjectIDFromHex(subject.SubjectID); err == nil {
				subjectIds = append(subjectIds, subjectId)
			}
		}
	}
	totalAssignmentsCreated := courseAssignmentCount + coachingAssignmentCount

	// Get ratings average
	feedbackPipeline := []bson.M{
		{"$match": bson.M{
			"teacherId": _id,
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id":   nil,
			"avg":   bson.M{"$avg": "$rate"},
			"count": bson.M{"$sum": 1},
		}},
	}
	var feedbackResult []bson.M
	err = t.feedbackRepo.Aggregate(ctx, feedbackPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &feedbackResult)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	ratingsAvg := 0.0
	if len(feedbackResult) > 0 {
		if avg, ok := feedbackResult[0]["avg"].(float64); ok {
			ratingsAvg = avg
		}
	}

	return &models.TeacherSummary{
		Teacher:                *teacher,
		AssignedStudents:       studentsInterface,
		TotalDeliveredSessions: totalDeliveredSessions,
		AssignmentsCreated:     totalAssignmentsCreated,
		RatingsAvg:             ratingsAvg,
	}, nil
}

func (t *teachersvcs) GetTodaySummary(ctx context.Context) (*models.TeacherTodaySummary, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id
	now := time.Now()

	// Get today's date range (start and end of day in UTC)
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
	endOfDay := time.Date(now.Year(), now.Month(), now.Day(), 23, 59, 59, 999999999, time.UTC)

	// Query today's sessions for this teacher
	pipeline := []bson.M{
		{
			"$match": bson.M{
				"teacherId": teacherId,
				"date": bson.M{
					"$gte": startOfDay,
					"$lte": endOfDay,
				},
				"trash": false,
			},
		},
		{"$sort": bson.M{"from": 1}}, // Sort by start time
	}

	var sessions []content_models.StudentSession
	if err := t.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	}); err != nil {
		return nil, err
	}

	// Collect IDs for batch enrichment
	studentIds := make(map[primitive.ObjectID]bool)
	coachingIds := make(map[primitive.ObjectID]bool)
	bundleIds := make(map[primitive.ObjectID]bool)

	for _, session := range sessions {
		studentIds[session.StudentId] = true
		coachingIds[session.CoachingProgramId] = true
		if !session.BundleId.IsZero() {
			bundleIds[session.BundleId] = true
		}
	}

	// Batch fetch students
	studentMap := make(map[primitive.ObjectID]*models.Student)
	if len(studentIds) > 0 {
		studentIdSlice := make([]primitive.ObjectID, 0, len(studentIds))
		for id := range studentIds {
			studentIdSlice = append(studentIdSlice, id)
		}
		studentPipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": studentIdSlice}, "trash": false}},
		}
		var students []models.Student
		if err := t.studentRepo.Aggregate(ctx, studentPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &students)
		}); err == nil {
			for i := range students {
				studentMap[students[i].Id] = &students[i]
			}
		}
	}

	// Batch fetch coachings
	coachingMap := make(map[primitive.ObjectID]*content_models.Coaching)
	if len(coachingIds) > 0 {
		coachingIdSlice := make([]primitive.ObjectID, 0, len(coachingIds))
		for id := range coachingIds {
			coachingIdSlice = append(coachingIdSlice, id)
		}
		coachingPipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": coachingIdSlice}, "trash": false}},
		}
		var coachings []content_models.Coaching
		if err := t.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachings)
		}); err == nil {
			for i := range coachings {
				coachingMap[coachings[i].Id] = &coachings[i]
			}
		}
	}

	// Batch fetch bundles
	bundleMap := make(map[primitive.ObjectID]*content_models.Bundle)
	if len(bundleIds) > 0 {
		bundleIdSlice := make([]primitive.ObjectID, 0, len(bundleIds))
		for id := range bundleIds {
			bundleIdSlice = append(bundleIdSlice, id)
		}
		bundlePipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": bundleIdSlice}, "trash": false}},
		}
		var bundles []content_models.Bundle
		if err := t.bundleRepo.Aggregate(ctx, bundlePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &bundles)
		}); err == nil {
			for i := range bundles {
				bundleMap[bundles[i].Id] = &bundles[i]
			}
		}
	}

	// Convert sessions to enriched calendar events
	enrichedEvents := make([]content_models.EnrichedCalendarEvent, 0, len(sessions))
	for _, session := range sessions {
		// Parse time strings to create start and end times
		startTime, err := parseSessionTime(session.Date, session.From)
		if err != nil {
			continue
		}
		endTime, err := parseSessionTime(session.Date, session.To)
		if err != nil {
			continue
		}

		event := content_models.EnrichedCalendarEvent{
			CalendarEvent: content_models.CalendarEvent{
				Id:                session.Id,
				Type:              "coaching_session",
				Title:             fmt.Sprintf("Session #%d", session.SessionNum),
				StartTime:         startTime,
				EndTime:           endTime,
				TeacherId:         teacherId,
				StudentId:         &session.StudentId,
				CoachingProgramId: &session.CoachingProgramId,
				Status:            string(session.Status),
				BundleId:          nil,
			},
		}

		// Set bundle ID if not zero
		if !session.BundleId.IsZero() {
			event.BundleId = &session.BundleId
		}

		// Enrich with student info
		if student, ok := studentMap[session.StudentId]; ok {
			firstName := student.GeneralInfo.FirstName.GetContentByLang("en")
			lastName := student.GeneralInfo.LastName.GetContentByLang("en")
			fullName := fmt.Sprintf("%s %s", firstName, lastName)
			event.Students = []content_models.EventStudent{
				{
					Id:   student.Id,
					Name: fullName,
				},
			}
		}

		// Enrich with coaching info
		if coaching, ok := coachingMap[session.CoachingProgramId]; ok {
			title := coaching.Title.GetContentByLang("en")
			event.CoachingProgram = &content_models.EventCoachingProgram{
				Id:   coaching.Id,
				Name: title,
			}
			event.Title = fmt.Sprintf("%s - Session #%d", title, session.SessionNum)
		}

		// Enrich with bundle info
		if !session.BundleId.IsZero() {
			if bundle, ok := bundleMap[session.BundleId]; ok {
				bundleName := bundle.Name.GetContentByLang("en")
				event.Bundle = &content_models.EventBundle{
					Id:   bundle.Id,
					Name: bundleName,
				}
			}
		}

		// Add session-specific details
		event.CoachingSessionDetails = &content_models.CoachingSessionDetails{
			SessionNum:  session.SessionNum,
			Name:        session.SessionName,
			Description: session.Description,
		}

		enrichedEvents = append(enrichedEvents, event)
	}

	// Calculate counts
	totalCount := len(enrichedEvents)
	completedCount := 0
	for _, event := range enrichedEvents {
		if event.Status == string(content_enums.StudentSessionStatusCompleted) {
			completedCount++
		}
	}
	notCompletedCount := totalCount - completedCount

	// Find current session (time is between StartTime and EndTime)
	var currentSession *content_models.EnrichedCalendarEvent
	for i := range enrichedEvents {
		event := &enrichedEvents[i]
		if now.After(event.StartTime) && now.Before(event.EndTime) {
			currentSession = event
			break // Assume only one current session
		}
	}

	// Find next session (first session that hasn't started yet)
	var nextSession *content_models.EnrichedCalendarEvent
	for i := range enrichedEvents {
		event := &enrichedEvents[i]
		if now.Before(event.StartTime) {
			if nextSession == nil || event.StartTime.Before(nextSession.StartTime) {
				nextSession = event
			}
		}
	}

	// Convert enrichedEvents to interface{} slice
	allSessionsInterface := make([]interface{}, len(enrichedEvents))
	for i := range enrichedEvents {
		allSessionsInterface[i] = enrichedEvents[i]
	}

	// Prepare pointers as interface{}
	var currentSessionInterface interface{}
	if currentSession != nil {
		currentSessionInterface = *currentSession
	}

	var nextSessionInterface interface{}
	if nextSession != nil {
		nextSessionInterface = *nextSession
	}

	return &models.TeacherTodaySummary{
		Date:                      startOfDay.Format("2006-01-02"),
		TotalSessionsCount:        totalCount,
		CompletedSessionsCount:    completedCount,
		NotCompletedSessionsCount: notCompletedCount,
		CurrentSession:            currentSessionInterface,
		NextSession:               nextSessionInterface,
		AllSessions:               allSessionsInterface,
	}, nil
}

// parseSessionTime combines a date with a time string (HH:MM format) to create a full timestamp
func parseSessionTime(date time.Time, timeStr string) (time.Time, error) {
	parts := strings.Split(timeStr, ":")
	if len(parts) != 2 {
		return time.Time{}, fmt.Errorf("invalid time format: %s", timeStr)
	}

	var hours, minutes int
	if _, err := fmt.Sscanf(parts[0], "%d", &hours); err != nil {
		return time.Time{}, err
	}
	if _, err := fmt.Sscanf(parts[1], "%d", &minutes); err != nil {
		return time.Time{}, err
	}

	return time.Date(
		date.Year(), date.Month(), date.Day(),
		hours, minutes, 0, 0,
		date.Location(),
	), nil
}

func (t *teachersvcs) GetReviews(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*content_models.ReviewPagination, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get course and coaching IDs using teacherAssignmentHelper to avoid importing content package
	courseIds, err := t.teacherAssignmentHelper.GetCoursesByTeacher(ctx, _id)
	if err != nil {
		return nil, err
	}
	coachingIds, err := t.teacherAssignmentHelper.GetCoachingsByTeacher(ctx, _id)
	if err != nil {
		return nil, err
	}

	// Combine all ref IDs
	allRefIds := make([]primitive.ObjectID, 0)
	allRefIds = append(allRefIds, courseIds...)
	allRefIds = append(allRefIds, coachingIds...)

	if len(allRefIds) == 0 {
		var totalPages float64 = 0
		pagination := types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: 0,
		}
		return &content_models.ReviewPagination{
			Reviews:    []content_models.ReviewRes{},
			Pagination: pagination,
		}, nil
	}

	// Build review types array
	reviewTypes := []string{}
	if len(courseIds) > 0 {
		reviewTypes = append(reviewTypes, "course")
	}
	if len(coachingIds) > 0 {
		reviewTypes = append(reviewTypes, "coaching")
	}

	// Get reviews
	reviewsPipeline := []bson.M{
		{"$match": bson.M{
			"refId": bson.M{"$in": allRefIds},
			"type":  bson.M{"$in": reviewTypes},
			"trash": false,
		}},
	}

	// Apply query filters if provided
	if query != nil && len(query.Columns) > 0 {
		if err := query.CheckValid(); err == nil {
			filter, err := query.ConvertToMongo()
			if err == nil {
				// Merge query filter with existing match conditions
				reviewsPipeline = append(reviewsPipeline, bson.M{"$match": filter})
			}
		}
	}

	count, err := t.reviewRepo.Count(ctx, reviewsPipeline)
	if err != nil {
		return nil, err
	}

	reviewsPipeline = append(reviewsPipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	// Add user population pipeline (same as content service)
	populateUserField := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "userId",
				"foreignField": "_id",
				"as":           "userData",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$userData",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$set": bson.M{
				"firstName": "$userData.firstName",
				"lastName":  "$userData.lastName",
				"email":     "$userData.email",
				"picture":   "$userData.picture",
			},
		},
	}
	reviewsPipeline = append(reviewsPipeline, populateUserField...)

	var reviews []content_models.ReviewRes
	err = t.reviewRepo.Aggregate(ctx, reviewsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &reviews)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &content_models.ReviewPagination{
		Reviews:    reviews,
		Pagination: pagination,
	}, nil
}

// GetMyReviews retrieves reviews for the authenticated teacher
func (t *teachersvcs) GetMyReviews(ctx context.Context, skip, limit int64, query *query.Conditions) (*content_models.ReviewPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	teacherId := cfg.User.Id.Hex()
	return t.GetReviews(ctx, teacherId, skip, limit, query)
}

func (t *teachersvcs) GetMe(ctx context.Context) (*models.Teacher, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get teacher by user ID (teacher.Id is the user ID from users microservice)
	teacherId := cfg.User.Id.Hex()
	teacher, err := t.GetById(ctx, teacherId)
	if err != nil {
		return nil, err
	}

	return teacher, nil
}

// UpdateMe updates the authenticated teacher's profile (public update)
func (t *teachersvcs) UpdateMe(ctx context.Context, id string, data *models.TeacherProfileUpdateDto) (*models.Teacher, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Verify the teacher is updating their own profile
	if cfg.User.Id.Hex() != id {
		return nil, errors.New("you can only update your own profile")
	}

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid teacher id")
	}

	// Get existing teacher to preserve fields that shouldn't be updated
	existingTeacher, err := t.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	// Sync default working hours to day of week availability
	syncDefaultWorkingHoursForTeacher(existingTeacher)
	data.Availability = existingTeacher.Availability

	// Build update with only allowed fields
	update := bson.M{
		"$set": bson.M{
			"firstName":     data.FirstName,
			"lastName":      data.LastName,
			"gender":        data.Gender,
			"dateOfBirth":   data.DateOfBirth,
			"nationality":   data.Nationality,
			"picture":       data.Picture,
			"mobile":        data.Mobile,
			"qualification": data.Qualification,
			"address":       data.Address,
			"availability":  data.Availability,
			"updatedAt":     time.Now(),
			"updatedBy":     cfg.User.Id,
		},
	}

	filter := bson.M{"_id": _id, "trash": false}
	updatedTeacher, err := t.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Update user service with all relevant fields (firstName, lastName, email, picture)
	// Always sync to ensure picture and other fields stay in sync
	if err := t.credSvcs.Update(ctx, id, updatedTeacher); err != nil {
		return nil, err
	}

	return updatedTeacher, nil
}

// GetTeacherStudentsIds gets the student IDs that are assigned to a teacher
// Students are identified through StudentSession where teacherId matches
func (t *teachersvcs) GetTeacherStudentsIds(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error) {
	// Get unique student IDs from StudentSession
	studentsPipeline := []bson.M{
		{"$match": bson.M{
			"teacherId": teacherId,
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id": "$studentId",
		}},
	}

	var studentSessions []struct {
		StudentId primitive.ObjectID `bson:"_id"`
	}
	err := t.studentSessionRepo.Aggregate(ctx, studentsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &studentSessions)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// Extract student IDs
	studentIds := make([]primitive.ObjectID, 0, len(studentSessions))
	for _, session := range studentSessions {
		studentIds = append(studentIds, session.StudentId)
	}

	return studentIds, nil
}

// GetTeacherStudents retrieves all students assigned to the authenticated teacher across all coachings
func (t *teachersvcs) GetTeacherStudents(ctx context.Context, skip, limit int64, query *query.Conditions) (*content_models.TeacherStudentPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id

	// Get student IDs assigned to this teacher
	studentIds, err := t.GetTeacherStudentsIds(ctx, teacherId)
	if err != nil {
		return nil, err
	}

	if len(studentIds) == 0 {
		var totalPages float64 = 0
		pagination := types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: 0,
		}
		return &content_models.TeacherStudentPagination{
			Students:   []content_models.TeacherStudent{},
			Pagination: pagination,
		}, nil
	}

	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
	} else {
		filter = bson.M{}
	}

	// Base pipeline: group sessions by student to get unique students with statistics
	basePipeline := []bson.M{
		{"$match": bson.M{
			"teacherId": teacherId,
			"studentId": bson.M{"$in": studentIds},
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id":           "$studentId",
			"totalSessions": bson.M{"$sum": 1},
			"completedSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", content_enums.StudentSessionStatusCompleted}},
					1,
					0,
				},
			}},
			"coachings":        bson.M{"$addToSet": "$coachingProgramId"}, // Unique coaching IDs
			"lastSessionDate":  bson.M{"$max": "$date"},
			"firstSessionDate": bson.M{"$min": "$date"},
		}},
		{"$addFields": bson.M{
			"numOfCoachings": bson.M{"$size": "$coachings"},
			"progress": bson.M{
				"$toInt": bson.M{
					"$multiply": bson.A{
						bson.M{"$divide": bson.A{"$completedSessions", "$totalSessions"}},
						100,
					},
				},
			},
		}},
	}

	// Lookup student details
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
				"student": "$student",
			},
		},
	}

	// Combine base + lookup
	pipeline := append(basePipeline, studentLookup...)

	// Apply user-provided filters
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Count total students
	count, err := t.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Pagination
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"lastSessionDate": -1}}, // Sort by most recent session
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []content_models.TeacherStudent
	if err := t.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
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

	return &content_models.TeacherStudentPagination{
		Students:   result,
		Pagination: pagination,
	}, nil
}

// GetTeacherStudentProfile retrieves comprehensive student profile for the authenticated teacher
func (t *teachersvcs) GetTeacherStudentProfile(ctx context.Context, studentId string) (*content_models.TeacherStudentProfile, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	// 1. Verify teacher is assigned to at least one coaching or course with this student
	coachingIds := make([]primitive.ObjectID, 0)
	courseIds := make([]primitive.ObjectID, 0)

	// 1a. Check Coachings
	verificationPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _studentId,
			"teacherId": teacherId,
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id":         nil,
			"coachingIds": bson.M{"$addToSet": "$coachingProgramId"},
		}},
	}
	var verificationResult []bson.M
	if err := t.studentSessionRepo.Aggregate(ctx, verificationPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &verificationResult)
	}); err != nil {
		return nil, err
	}
	if len(verificationResult) > 0 {
		if ids, ok := verificationResult[0]["coachingIds"].(primitive.A); ok {
			for _, id := range ids {
				if objId, ok := id.(primitive.ObjectID); ok {
					coachingIds = append(coachingIds, objId)
				}
			}
		}
	}

	// 1b. Check Courses
	// Find all courses assigned to this teacher
	var assignedCourses []content_models.Course
	err = t.courseRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"basicInfo.teachers": teacherId,
			"trash":              false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignedCourses)
	})
	if err == nil && len(assignedCourses) > 0 {
		assignedCourseIds := make([]primitive.ObjectID, len(assignedCourses))
		for i, c := range assignedCourses {
			assignedCourseIds[i] = c.Id
		}

		// Check if student is enrolled in any of these courses
		var enrollments []content_models.StudentCourse
		err = t.studentCourseRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"studentId": _studentId,
				"courseId":  bson.M{"$in": assignedCourseIds},
				"trash":     false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		})
		if err == nil {
			for _, e := range enrollments {
				courseIds = append(courseIds, e.CourseId)
			}
		}
	}

	if len(coachingIds) == 0 && len(courseIds) == 0 {
		return nil, errors.New("student not found or you are not assigned to any coaching/course with this student")
	}

	// 2. Get student profile
	student, err := t.studentRepo.GetByFilter(ctx, bson.M{"_id": _studentId, "trash": false})
	if err != nil {
		return nil, errors.New("student not found")
	}

	// Build student profile info
	studentInfo := content_models.TeacherStudentProfileInfo{
		Id:          student.Id,
		FirstName:   student.GeneralInfo.FirstName,
		LastName:    student.GeneralInfo.LastName,
		Email:       student.AccountStudentSetting.Email,
		Picture:     student.GeneralInfo.Picture,
		Gender:      student.GeneralInfo.Gender,
		DateOfBirth: student.GeneralInfo.DateOfBirth,
		Nationality: student.GeneralInfo.Nationality,
		Status:      string(student.GeneralInfo.Status),
		Class:       student.AcademicInfo.Class,
		Stage:       student.AcademicInfo.Stage,
	}
	if student.ContactInfo.MobilePhoneNumber.Prefix != "" && student.ContactInfo.MobilePhoneNumber.Content != "" {
		studentInfo.Phone = student.ContactInfo.MobilePhoneNumber.Prefix + student.ContactInfo.MobilePhoneNumber.Content
	}

	// 3. Get coachings with student count
	coachingsPipeline := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": coachingIds},
			"trash": false,
		}},
		// Add lookup to count students for each coaching
		{"$lookup": bson.M{
			"from":         "academyCoachingStudents",
			"localField":   "_id",
			"foreignField": "coachingId",
			"as":           "students",
		}},
		{"$addFields": bson.M{
			"studentCount": bson.M{
				"$size": bson.M{
					"$filter": bson.M{
						"input": "$students",
						"cond":  bson.M{"$eq": bson.A{"$$this.trash", false}},
					},
				},
			},
		}},
		{"$project": bson.M{
			"_id":          1,
			"title":        1,
			"description":  1,
			"studentCount": 1,
		}},
	}
	var coachings []content_models.CoachingBasicInfo
	if len(coachingIds) > 0 {
		if err := t.coachingRepo.Aggregate(ctx, coachingsPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachings)
		}); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
	}

	// 3b. Get courses with student count
	var courses []content_models.CoachingBasicInfo
	if len(courseIds) > 0 {
		coursesPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": courseIds},
				"trash": false,
			}},
			{"$lookup": bson.M{
				"from":         "academyCourseStudents",
				"localField":   "_id",
				"foreignField": "courseId",
				"as":           "students",
			}},
			{"$addFields": bson.M{
				"studentCount": bson.M{
					"$size": bson.M{
						"$filter": bson.M{
							"input": "$students",
							"cond":  bson.M{"$eq": bson.A{"$$this.trash", false}},
						},
					},
				},
				"title":       "$basicInfo.name",
				"description": "$basicInfo.description",
			}},
			{"$project": bson.M{
				"_id":          1,
				"title":        1,
				"description":  1,
				"studentCount": 1,
			}},
		}
		if err := t.courseRepo.Aggregate(ctx, coursesPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courses)
		}); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
	}

	// 4. Get points
	pointsSummary := content_models.PointsSummary{
		TotalPoints:     0,
		RemainingPoints: 0,
		RedeemedPoints:  0,
	}
	studentPoint, err := t.studentPointRepo.GetByFilter(ctx, bson.M{"studentId": _studentId, "trash": false})
	if err == nil && studentPoint != nil {
		pointsSummary.TotalPoints = studentPoint.TotalPoints
		pointsSummary.RemainingPoints = studentPoint.RemainingPoints
		pointsSummary.RedeemedPoints = studentPoint.RedeemedPoints
	}

	// 5. Calculate attendance rate
	attendancePipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _studentId,
			"teacherId": teacherId,
			"trash":     false,
			"status": bson.M{
				"$in": []content_enums.StudentSessionStatus{
					content_enums.StudentSessionStatusCompleted,
					content_enums.StudentSessionStatusMissed,
					content_enums.StudentSessionStatusCancelled,
				},
			},
		}},
		{"$group": bson.M{
			"_id":   "$status",
			"count": bson.M{"$sum": 1},
		}},
	}
	var attendanceCounts []struct {
		Id    content_enums.StudentSessionStatus `bson:"_id"`
		Count int                                `bson:"count"`
	}
	if err := t.studentSessionRepo.Aggregate(ctx, attendancePipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &attendanceCounts)
	}); err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	attendanceRate := 0.0
	totalConcluded := 0
	completedCount := 0
	for _, ac := range attendanceCounts {
		totalConcluded += ac.Count
		if ac.Id == content_enums.StudentSessionStatusCompleted {
			completedCount = ac.Count
		}
	}
	if totalConcluded > 0 {
		attendanceRate = (float64(completedCount) / float64(totalConcluded)) * 100.0
	}

	// 6. Get review summary - only for reviews related to teacher's coachings
	reviewSummary := content_models.ReviewSummary{
		TotalCount:    0,
		AverageRating: 0.0,
	}
	if len(coachingIds) > 0 {
		reviewPipeline := []bson.M{
			{"$match": bson.M{
				"userId": _studentId,
				"type":   "coaching",
				"refId":  bson.M{"$in": coachingIds},
				"trash":  false,
			}},
			{"$group": bson.M{
				"_id":   nil,
				"count": bson.M{"$sum": 1},
				"avg":   bson.M{"$avg": "$rating"},
			}},
		}
		var reviewResult []bson.M
		if err := t.reviewRepo.Aggregate(ctx, reviewPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &reviewResult)
		}); err == nil && len(reviewResult) > 0 {
			if count, ok := reviewResult[0]["count"].(int32); ok {
				reviewSummary.TotalCount = int(count)
			}
			if avg, ok := reviewResult[0]["avg"].(float64); ok {
				reviewSummary.AverageRating = avg
			}
		}
	}

	// 7. Get parent info from student's ParentStudentRelations
	parentInfo := make([]content_models.ParentBasicInfo, 0)
	if len(student.ParentStudentRelations) > 0 {
		// Collect parent IDs
		parentIds := make([]primitive.ObjectID, 0)
		parentRelationMap := make(map[primitive.ObjectID]string) // map[parentId]relationType
		for _, rel := range student.ParentStudentRelations {
			if rel == nil {
				continue
			}
			var parentId primitive.ObjectID
			// Student can be either first or second in the relation
			if rel.FirstType == "parent" {
				parentId = rel.FirstId
			} else if rel.SecondType == "parent" {
				parentId = rel.SecondId
			}
			if !parentId.IsZero() {
				parentIds = append(parentIds, parentId)
				parentRelationMap[parentId] = rel.RelationType
			}
		}

		// Lookup parent details
		if len(parentIds) > 0 {
			parentPipeline := []bson.M{
				{"$match": bson.M{
					"_id":   bson.M{"$in": parentIds},
					"trash": false,
				}},
			}
			var parents []models.Parent
			if err := t.parentRepo.Aggregate(ctx, parentPipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &parents)
			}); err == nil {
				for _, parent := range parents {
					parentBasic := content_models.ParentBasicInfo{
						Id:           parent.Id,
						FirstName:    parent.ParentGeneralInfo.FirstName,
						LastName:     parent.ParentGeneralInfo.LastName,
						Email:        parent.AccountParentSetting.Email,
						RelationType: parentRelationMap[parent.Id],
					}
					if parent.ParentContactInfo.MobilePhoneNumber.Prefix != "" && parent.ParentContactInfo.MobilePhoneNumber.Content != "" {
						parentBasic.Phone = parent.ParentContactInfo.MobilePhoneNumber.Prefix + parent.ParentContactInfo.MobilePhoneNumber.Content
					}
					parentInfo = append(parentInfo, parentBasic)
				}
			}
		}
	}

	return &content_models.TeacherStudentProfile{
		Student:        studentInfo,
		Coachings:      coachings,
		Courses:        courses,
		Points:         pointsSummary,
		AttendanceRate: attendanceRate,
		ReviewSummary:  reviewSummary,
		ParentInfo:     parentInfo,
	}, nil
}

// GetTeacherStudentCoachingIds retrieves the coaching IDs where the teacher is assigned with the student
func (t *teachersvcs) GetTeacherStudentCoachingIds(ctx context.Context, studentId string) ([]primitive.ObjectID, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	// Verify teacher is assigned to at least one coaching with this student
	verificationPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _studentId,
			"teacherId": teacherId,
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id":         nil,
			"coachingIds": bson.M{"$addToSet": "$coachingProgramId"},
		}},
	}
	var verificationResult []bson.M
	if err := t.studentSessionRepo.Aggregate(ctx, verificationPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &verificationResult)
	}); err != nil {
		return nil, err
	}

	if len(verificationResult) == 0 {
		return nil, errors.New("student not found or you are not assigned to any coaching with this student")
	}

	coachingIds := make([]primitive.ObjectID, 0)
	if ids, ok := verificationResult[0]["coachingIds"].(primitive.A); ok {
		for _, id := range ids {
			if objId, ok := id.(primitive.ObjectID); ok {
				coachingIds = append(coachingIds, objId)
			}
		}
	}

	return coachingIds, nil
}

// GetStudentTeachers retrieves all teachers assigned to a student across all coachings
func (t *teachersvcs) GetStudentTeachers(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*content_models.StudentTeacherPagination, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	// Base pipeline: group sessions by teacher to get unique teachers with statistics
	basePipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _studentId,
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id":           "$teacherId",
			"totalSessions": bson.M{"$sum": 1},
			"completedSessions": bson.M{"$sum": bson.M{
				"$cond": bson.A{
					bson.M{"$eq": bson.A{"$status", content_enums.StudentSessionStatusCompleted}},
					1,
					0,
				},
			}},
			"coachings":        bson.M{"$addToSet": "$coachingProgramId"},
			"lastSessionDate":  bson.M{"$max": "$date"},
			"firstSessionDate": bson.M{"$min": "$date"},
		}},
		{"$addFields": bson.M{
			"numOfCoachings": bson.M{"$size": "$coachings"},
			"progress": bson.M{
				"$toInt": bson.M{
					"$multiply": bson.A{
						bson.M{"$divide": bson.A{"$completedSessions", "$totalSessions"}},
						100,
					},
				},
			},
		}},
	}

	// Lookup teacher details
	teacherLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "users",
				"localField":   "_id",
				"foreignField": "_id",
				"as":           "teacher",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$teacher",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$set": bson.M{
				"teacher": "$teacher",
			},
		},
	}

	// Combine base + lookup
	pipeline := append(basePipeline, teacherLookup...)

	// Get total count before pagination
	var totalCount int64
	countPipeline := append(basePipeline, bson.M{"$count": "total"})
	var countResult []bson.M
	if err := t.studentSessionRepo.Aggregate(ctx, countPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &countResult)
	}); err != nil {
		return nil, err
	}
	if len(countResult) > 0 {
		if count, ok := countResult[0]["total"].(int32); ok {
			totalCount = int64(count)
		}
	}

	// Apply pagination
	paginationStage := bson.M{
		"$skip": skip,
	}
	pipeline = append(pipeline, paginationStage)

	paginationStage = bson.M{
		"$limit": limit,
	}
	pipeline = append(pipeline, paginationStage)

	// Execute aggregation
	var results []content_models.StudentTeacher
	if err := t.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	var totalPages float64
	if totalCount > 0 && limit > 0 {
		totalPages = math.Ceil(float64(totalCount) / float64(limit))
	}

	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &content_models.StudentTeacherPagination{
		Teachers:   results,
		Pagination: pagination,
	}, nil
}
