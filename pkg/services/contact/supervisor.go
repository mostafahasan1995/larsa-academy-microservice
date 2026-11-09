package contact

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/services/contact/models"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/sorting"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"strconv"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SupervisorSvc interface {
	// Get(ctx context.Context) (*models.Supervisor, error)
	GetOne(ctx context.Context, id string) (*models.Supervisor, error)
	GetOneWithStats(ctx context.Context, id string) (*models.SupervisorWithStats, error)
	GetDashboardWithFilter(ctx context.Context, id string, statusFilter string, todayOnly bool) (*content_models.SupervisorDashboard, error)
	GetDashboard(ctx context.Context, id string) (*content_models.SupervisorDashboard, error)
	Add(ctx context.Context, data *models.SupervisorDTO) (*models.Supervisor, error)
	Update(ctx context.Context, id string, data *models.SupervisorDTO) (*models.Supervisor, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]models.SupervisorWithStats, error)
	GetPaginated(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SupervisorWithPagination, error)
	Delete(ctx context.Context, id string) error
	GetSummary(ctx context.Context, id string) (*models.SupervisorSummary, error)
	GetAssignedStudents(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.StudentsPagination, error)
	GetOverseenTeachers(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.TeacherPagination, error)
	GetCoveredCourses(ctx context.Context, id string, skip, limit int64, query *query.Conditions, enrichTeachers bool) (*content_models.CoursePagination, error)
	GetCoveredCoaching(ctx context.Context, id string, skip, limit int64, query *query.Conditions, enrichTeachers bool) (*content_models.CoachingPagination, error)
	GetStudentCountsByCourse(ctx context.Context, supervisorId string) ([]content_models.StudentCountByCourseRow, error)
	GetStudentCountsByCoaching(ctx context.Context, supervisorId string) ([]content_models.StudentCountByCoachingRow, error)
	GetStudentCountsBySubject(ctx context.Context) ([]content_models.StudentCountBySubjectRow, error)
	GetManagedClassesGrades(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.ClassGradeInfoPagination, error)
	// New methods for correct relationships
	GetTeachersBySupervisorId(ctx context.Context, id string, query *query.Conditions) ([]models.Teacher, error)
	GetStudentsBySupervisorId(ctx context.Context, id string, query *query.Conditions) ([]models.Student, error)
	IsTeacherUnderSupervisor(ctx context.Context, teacherId, supervisorId primitive.ObjectID) (bool, error)
	GetManagedUsers(ctx context.Context, id string, withStudent bool) (*models.ManagedUsersResponse, error)
	// Extended endpoints with direct/indirect separation
	GetTeachersExtended(ctx context.Context, id string, query *query.Conditions) (*models.SupervisorTeachersExtended, error)
	GetStudentsExtended(ctx context.Context, id string, query *query.Conditions) (*models.SupervisorStudentsExtended, error)
	UpdateMe(ctx context.Context, id string, data *models.SupervisorProfileUpdateDto) (*models.Supervisor, error)
}

// TeacherAssignmentBatchHelper defines the interface for batch teacher assignment lookups
// This avoids importing the content package directly to prevent import cycles
type TeacherAssignmentBatchHelper interface {
	BatchGetCourseTeachers(ctx context.Context, courseIds []primitive.ObjectID) (map[primitive.ObjectID][]primitive.ObjectID, error)
	BatchGetCoachingTeachers(ctx context.Context, coachingIds []primitive.ObjectID) (map[primitive.ObjectID][]primitive.ObjectID, error)
	// Get lists of course/coaching IDs for a given teacher
	GetCoursesByTeacher(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error)
	GetCoachingsByTeacher(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error)
}

type supervisorSvcs struct {
	repo                      db.MainRepo[models.Supervisor]
	parentStudentRelationRepo db.MainRepo[models.ParentStudentRelation]
	studentRepo               db.MainRepo[models.Student]
	parentRepo                db.MainRepo[models.Parent]
	teacherRepo               db.MainRepo[models.Teacher]
	courseRepo                db.MainRepo[content_models.Course]
	studentCourseRepo         db.MainRepo[content_models.StudentCourse]
	studentSessionRepo        db.MainRepo[content_models.StudentSession]
	coachingRepo              db.MainRepo[content_models.Coaching]
	supervisorAssignmentRepo  db.MainRepo[content_models.SupervisorAssignment]
	teacherAssignmentRepo     db.MainRepo[content_models.TeacherAssignment]
	coachingStudentRepo       db.MainRepo[content_models.CoachingStudent]
	withtxn                   *db.WithTxn
	credSvcs                  CredSvcs
	sortingsvcs               sorting.SortingSvcs
	teacherAssignmentHelper   TeacherAssignmentBatchHelper
}

func NewSupervisorSvcs(i *do.Injector) (SupervisorSvc, error) {
	return &supervisorSvcs{
		repo:                      db.NewRepo[models.Supervisor](i),
		parentStudentRelationRepo: db.NewRepo[models.ParentStudentRelation](i),
		studentRepo:               db.NewRepo[models.Student](i),
		parentRepo:                db.NewRepo[models.Parent](i),
		teacherRepo:               db.NewRepo[models.Teacher](i),
		courseRepo:                db.NewRepo[content_models.Course](i),
		studentCourseRepo:         db.NewRepo[content_models.StudentCourse](i),
		studentSessionRepo:        db.NewRepo[content_models.StudentSession](i),
		coachingRepo:              db.NewRepo[content_models.Coaching](i),
		supervisorAssignmentRepo:  db.NewRepo[content_models.SupervisorAssignment](i),
		teacherAssignmentRepo:     db.NewRepo[content_models.TeacherAssignment](i),
		coachingStudentRepo:       db.NewRepo[content_models.CoachingStudent](i),
		withtxn:                   do.MustInvoke[*db.WithTxn](i),
		credSvcs:                  do.MustInvoke[CredSvcs](i),
		sortingsvcs:               do.MustInvoke[sorting.SortingSvcs](i),
		teacherAssignmentHelper:   do.MustInvoke[TeacherAssignmentBatchHelper](i),
	}, nil
}

func (s *supervisorSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	_, err = s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
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
		updatedSupervisor, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if updatedSupervisor == nil {
			return nil, errors.New("supervisor not found or already deleted")
		}
		if err := s.credSvcs.Delete(ctx, id); err != nil {
			return nil, err
		}

		return updatedSupervisor, nil
	})

	return err
}

func (s supervisorSvcs) GetPaginated(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SupervisorWithPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	countPipeline := make([]bson.M, len(pipeline))
	copy(countPipeline, pipeline)
	count, err := s.repo.Count(ctx, countPipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	// Extract unique subject names and stages from subjects array
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"subjectAssigned": bson.M{
				"$setUnion": []interface{}{
					bson.M{
						"$map": bson.M{
							"input": bson.M{"$ifNull": []interface{}{"$subjects", []interface{}{}}},
							"as":    "subject",
							"in":    "$$subject.subjectName",
						},
					},
					[]interface{}{},
				},
			},
			"assignedStage": bson.M{
				"$setUnion": []interface{}{
					bson.M{
						"$map": bson.M{
							"input": bson.M{"$ifNull": []interface{}{"$subjects", []interface{}{}}},
							"as":    "subject",
							"in":    "$$subject.stage",
						},
					},
					[]interface{}{},
				},
			},
		},
	})

	var result []models.SupervisorWithStats
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	// Calculate stats for each supervisor in Go
	if err := s.populateStats(ctx, result); err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pg := common.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}
	return &models.SupervisorWithPagination{
		Supervisors: result,
		Pagination:  pg,
	}, nil
}

func (s supervisorSvcs) GetAll(ctx context.Context, query *query.Conditions) ([]models.SupervisorWithStats, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	// Extract unique subject names and stages from subjects array
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"subjectAssigned": bson.M{
				"$setUnion": []interface{}{
					bson.M{
						"$map": bson.M{
							"input": bson.M{"$ifNull": []interface{}{"$subjects", []interface{}{}}},
							"as":    "subject",
							"in":    "$$subject.subjectName",
						},
					},
					[]interface{}{},
				},
			},
			"assignedStage": bson.M{
				"$setUnion": []interface{}{
					bson.M{
						"$map": bson.M{
							"input": bson.M{"$ifNull": []interface{}{"$subjects", []interface{}{}}},
							"as":    "subject",
							"in":    "$$subject.stage",
						},
					},
					[]interface{}{},
				},
			},
		},
	})

	// Remove pipeline stats calculation and just use basic filters
	var result []models.SupervisorWithStats
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})

	if err != nil {
		return nil, err
	}

	// Calculate stats for each supervisor in Go
	if err := s.populateStats(ctx, result); err != nil {
		return nil, err
	}

	return result, nil
}

func (s supervisorSvcs) Add(ctx context.Context, data *models.SupervisorDTO) (*models.Supervisor, error) {
	var deleteUserOnErr func() error
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return nil, err
		}

		sup := &models.Supervisor{
			SupervisorDTO: *data,
			CreatedAt:     time.Now(),
			CreatedBy:     cfg.User.Id,
			UpdatedAt:     time.Now(),
			UpdatedBy:     cfg.User.Id,
		}

		if sup.Status == "" {
			sup.Status = enums.SupervisorStatusActive
		}

		result, err := s.credSvcs.Add(ctx, sup, "supervisor")
		if err != nil {
			return nil, err
		}

		deleteUserOnErr = result.DeleteOnErr

		sup.Id = result.UserData.Id
		seq, err := s.sortingsvcs.GetAndUpdateSourceSeq(ctx, "supervisors")
		if err != nil {
			return nil, err
		}
		sup.Seq = fmt.Sprintf("SUP-%d", seq)
		if err := s.repo.Add(ctx, sup); err != nil {
			return nil, err
		}

		// Update disabled status based on supervisor status
		disabled := sup.Status == enums.SupervisorStatusInactive
		if err := s.credSvcs.UpdateRaw(ctx, sup.Id.Hex(), map[string]any{"disabled": disabled}); err != nil {
			return nil, fmt.Errorf("failed to update user disabled status: %w", err)
		}

		return sup, nil
	})

	if err != nil {
		if deleteUserOnErr != nil {
			deleteUserOnErr()
		}
		return nil, err
	}

	return result.(*models.Supervisor), nil
}

func (s *supervisorSvcs) Update(ctx context.Context, id string, data *models.SupervisorDTO) (*models.Supervisor, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldSupervisor, err := s.GetOne(ctx, id)

		if err != nil {
			return nil, err
		}
		sup := &models.Supervisor{
			SupervisorDTO: *data,
			CreatedAt:     oldSupervisor.CreatedAt,
			CreatedBy:     oldSupervisor.CreatedBy,
			UpdatedAt:     time.Now(),
			UpdatedBy:     cfg.User.Id,
		}

		sup.Id = _id

		// Sync phone if provided in update
		if data.MobileNumber.Content != "" && data.MobileNumber.Prefix != "" {
			sup.MobileNumber = data.MobileNumber
		}

		if sup.Status == "" {
			sup.Status = oldSupervisor.Status
		}

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": sup}

		updatedSup, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if err := s.credSvcs.Update(ctx, id, sup); err != nil {
			return nil, err
		}

		// Update disabled status based on supervisor status
		disabled := sup.Status == enums.SupervisorStatusInactive
		if err := s.credSvcs.UpdateRaw(ctx, id, map[string]any{"disabled": disabled}); err != nil {
			return nil, fmt.Errorf("failed to update user disabled status: %w", err)
		}

		return updatedSup, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Supervisor), nil
}

func (s *supervisorSvcs) UpdateMe(ctx context.Context, id string, data *models.SupervisorProfileUpdateDto) (*models.Supervisor, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldSupervisor, err := s.GetOne(ctx, id)
		if err != nil {
			return nil, err
		}

		// Update user service FIRST
		userUpdateData := map[string]any{
			"firstName": data.FirstName.GetContentByLang("ar"),
			"lastName":  data.LastName.GetContentByLang("ar"),
			"email":     oldSupervisor.AccountSettings.Email, // Preserve email
		}

		if oldSupervisor.MobileNumber.Content != "" && oldSupervisor.MobileNumber.Prefix != "" {
			userUpdateData["phone"] = oldSupervisor.MobileNumber.Prefix + oldSupervisor.MobileNumber.Content
		}

		if data.ProfilePicture.Path != "" {
			userUpdateData["picture"] = data.ProfilePicture.Path
		}

		if err := s.credSvcs.UpdateRaw(ctx, id, userUpdateData); err != nil {
			return nil, err
		}

		// Update local supervisor
		supervisor := *oldSupervisor
		supervisor.FirstName = data.FirstName
		supervisor.LastName = data.LastName
		supervisor.Gender = data.Gender
		supervisor.DateOfBirth = data.DateOfBirth
		supervisor.Nationality = data.Nationality
		supervisor.ProfilePicture = data.ProfilePicture
		supervisor.Address = data.Address
		supervisor.UpdatedAt = time.Now()
		supervisor.UpdatedBy = cfg.User.Id

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": supervisor}

		updatedSup, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		return updatedSup, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Supervisor), nil
}
func (s *supervisorSvcs) GetOne(ctx context.Context, id string) (*models.Supervisor, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Use aggregation instead of simple find to include the teacher lookup
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}
	// pipeline = append(pipeline, assignTeacherLookup...)

	var result []models.Supervisor
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("supervisor not found")
	}

	return &result[0], nil
}

func (s *supervisorSvcs) GetSummary(ctx context.Context, id string) (*models.SupervisorSummary, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	supervisor, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs using content-based scope
	teacherIdsSlice, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	// Step 2: Get students using helper
	studentsData, err := s.GetAssignedStudents(ctx, id, 0, 0, nil)
	if err != nil {
		return nil, err
	}
	students := studentsData.Students

	studentsInterface := make([]interface{}, len(students))
	for i := range students {
		studentsInterface[i] = students[i]
	}

	// Get full teacher objects
	var allTeachers []models.Teacher
	if len(teacherIdsSlice) > 0 {
		teachersPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": teacherIdsSlice},
				"trash": false,
			}},
			{"$sort": bson.M{"_id": -1}},
		}
		err = s.teacherRepo.Aggregate(ctx, teachersPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &allTeachers)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
	}

	teachersInterface := make([]interface{}, len(allTeachers))
	for i := range allTeachers {
		teachersInterface[i] = allTeachers[i]
	}

	return &models.SupervisorSummary{
		Supervisor:       *supervisor,
		AssignedTeachers: teachersInterface,
		AssignedStudents: studentsInterface,
	}, nil
}

func (s *supervisorSvcs) GetAssignedStudents(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.StudentsPagination, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs using content-based scope
	teacherIds, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	if len(teacherIds) == 0 {
		return &models.StudentsPagination{Students: []models.StudentWithStats{}, Pagination: types.Pagination{}}, nil
	}

	uniqueStudentIds := make(map[primitive.ObjectID]bool)

	// 1. Get students from Courses
	var courseAssignments []content_models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "course",
			"teacherIds": bson.M{"$in": teacherIds},
			"trash":      false,
		}},
	}
	err = s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courseAssignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	courseIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range courseAssignments {
		courseIdSet[assignment.TargetId] = true
	}
	courseIds := make([]primitive.ObjectID, 0, len(courseIdSet))
	for id := range courseIdSet {
		courseIds = append(courseIds, id)
	}

	if len(courseIds) > 0 {
		studentCoursePipeline := []bson.M{
			{"$match": bson.M{
				"courseId": bson.M{"$in": courseIds},
				"trash":    false,
			}},
		}
		var studentCourses []content_models.StudentCourse
		err = s.studentCourseRepo.Aggregate(ctx, studentCoursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentCourses)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		for _, sc := range studentCourses {
			uniqueStudentIds[sc.StudentId] = true
		}
	}

	// 2. Get students from Coachings
	var coachingAssignments []content_models.TeacherAssignment
	coachingPipeline := []bson.M{
		{"$match": bson.M{
			"type":       "coaching",
			"teacherIds": bson.M{"$in": teacherIds},
			"trash":      false,
		}},
	}
	err = s.teacherAssignmentRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachingAssignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	coachingIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range coachingAssignments {
		coachingIdSet[assignment.TargetId] = true
	}
	coachingIds := make([]primitive.ObjectID, 0, len(coachingIdSet))
	for id := range coachingIdSet {
		coachingIds = append(coachingIds, id)
	}

	if len(coachingIds) > 0 {
		studentSessionPipeline := []bson.M{
			{"$match": bson.M{
				"coachingProgramId": bson.M{"$in": coachingIds},
				"trash":             false,
			}},
			{"$group": bson.M{
				"_id": "$studentId",
			}},
		}
		var studentSessions []struct {
			StudentId primitive.ObjectID `bson:"_id"`
		}
		err = s.studentSessionRepo.Aggregate(ctx, studentSessionPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentSessions)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		for _, ss := range studentSessions {
			uniqueStudentIds[ss.StudentId] = true
		}
	}

	// 3. Get students from direct sessions (legacy/fallback)
	directSessionPipeline := []bson.M{
		{"$match": bson.M{
			"teacherId": bson.M{"$in": teacherIds},
			"trash":     false,
		}},
		{"$group": bson.M{
			"_id": "$studentId",
		}},
	}
	var directSessions []struct {
		StudentId primitive.ObjectID `bson:"_id"`
	}
	err = s.studentSessionRepo.Aggregate(ctx, directSessionPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &directSessions)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	for _, ds := range directSessions {
		uniqueStudentIds[ds.StudentId] = true
	}

	// Extract unique student IDs
	studentIds := make([]primitive.ObjectID, 0, len(uniqueStudentIds))
	for id := range uniqueStudentIds {
		studentIds = append(studentIds, id)
	}

	if len(studentIds) == 0 {
		return &models.StudentsPagination{Students: []models.StudentWithStats{}, Pagination: types.Pagination{}}, nil
	}

	// Build filter from query conditions
	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}
	}

	// Get actual student objects
	studentsFilter := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": studentIds},
			"trash": false,
		}},
	}
	// Apply query filter if provided
	if filter != nil {
		studentsFilter = append(studentsFilter, bson.M{"$match": filter})
	}

	studentsFilter = append(studentsFilter, bson.M{
		"$lookup": bson.M{
			"from": "academyParentsStudentRelations",
			"let":  bson.M{"studentId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []interface{}{
							bson.M{"$or": []interface{}{
								bson.M{"$eq": []interface{}{"$firstId", "$$studentId"}},
								bson.M{"$eq": []interface{}{"$secondId", "$$studentId"}},
							}},
							bson.M{"$eq": []interface{}{"$trash", false}},
						},
					},
				}},
			},
			"as": "parentStudentRelations",
		},
	})

	countPipeline := make([]bson.M, len(studentsFilter))
	copy(countPipeline, studentsFilter)
	count, err := s.studentRepo.Count(ctx, countPipeline)
	if err != nil {
		return nil, err
	}

	studentsFilter = append(studentsFilter, bson.M{"$sort": bson.M{"_id": -1}})
	if limit > 0 {
		studentsFilter = append(studentsFilter, bson.M{"$skip": skip})
		studentsFilter = append(studentsFilter, bson.M{"$limit": limit})
	}

	var students []models.StudentWithStats
	err = s.studentRepo.Aggregate(ctx, studentsFilter, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &students)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// Enrich parent student relations with basic parent info
	if len(students) > 0 {
		parentIdsSet := make(map[primitive.ObjectID]bool)
		for _, student := range students {
			for _, relation := range student.ParentStudentRelations {
				if relation.FirstType == enums.PersonTypeParent {
					parentIdsSet[relation.FirstId] = true
				} else if relation.SecondType == enums.PersonTypeParent {
					parentIdsSet[relation.SecondId] = true
				}
			}
		}

		if len(parentIdsSet) > 0 {
			parentIds := make([]primitive.ObjectID, 0, len(parentIdsSet))
			for pid := range parentIdsSet {
				parentIds = append(parentIds, pid)
			}

			var parents []models.Parent
			err = s.parentRepo.Aggregate(ctx, []bson.M{
				{"$match": bson.M{"_id": bson.M{"$in": parentIds}, "trash": false}},
			}, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &parents)
			})

			if err == nil {
				parentInfoMap := make(map[primitive.ObjectID]models.ParentShortInfo)
				for _, p := range parents {
					parentInfoMap[p.Id] = models.ParentShortInfo{
						Id:        p.Id,
						FirstName: p.ParentGeneralInfo.FirstName,
						LastName:  p.ParentGeneralInfo.LastName,
						Phone:     p.ParentContactInfo.MobilePhoneNumber,
					}
				}

				for i := range students {
					var enrichedParents []models.EnrichedParent
					for _, rel := range students[i].ParentStudentRelations {
						var parentId primitive.ObjectID
						if rel.FirstType == enums.PersonTypeParent {
							parentId = rel.FirstId
						} else if rel.SecondType == enums.PersonTypeParent {
							parentId = rel.SecondId
						} else {
							continue // Ignore non-parent relations
						}

						if info, ok := parentInfoMap[parentId]; ok {
							enrichedParents = append(enrichedParents, models.EnrichedParent{
								ParentShortInfo: info,
								RelationType:    rel.RelationType,
							})
						}
					}
					students[i].Parents = enrichedParents
					students[i].ParentStudentRelations = nil // Hide internal relations
				}
			}
		}
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(count) / float64(limit))
	}

	return &models.StudentsPagination{
		Students: students,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *supervisorSvcs) GetOverseenTeachers(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.TeacherPagination, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs using content-based scope
	teacherIdsSlice, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	if len(teacherIdsSlice) == 0 {
		return &models.TeacherPagination{Teachers: []models.TeacherWithStats{}, Pagination: types.Pagination{}}, nil
	}

	// Build filter from query conditions
	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}
	}

	// Get full teacher objects
	teachersPipeline := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": teacherIdsSlice},
			"trash": false,
		}},
	}
	// Apply query filter if provided
	if filter != nil {
		teachersPipeline = append(teachersPipeline, bson.M{"$match": filter})
	}

	countPipeline := make([]bson.M, len(teachersPipeline))
	copy(countPipeline, teachersPipeline)
	count, err := s.teacherRepo.Count(ctx, countPipeline)
	if err != nil {
		return nil, err
	}

	// Add rating calculation
	teachersPipeline = append(teachersPipeline, []bson.M{
		{"$lookup": bson.M{
			"from": "academyContentReviews",
			"let":  bson.M{"teacherId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{"$expr": bson.M{"$eq": []interface{}{"$userId", "$$teacherId"}}}},
				{"$group": bson.M{
					"_id": nil,
					"avg": bson.M{"$avg": "$rating"},
				}},
			},
			"as": "ratings",
		}},
		{"$addFields": bson.M{
			"rating": bson.M{
				"$ifNull": []interface{}{
					bson.M{"$arrayElemAt": []interface{}{"$ratings.avg", 0}},
					0,
				},
			},
		}},
		{"$project": bson.M{"ratings": 0}},
	}...)

	teachersPipeline = append(teachersPipeline, bson.M{"$sort": bson.M{"_id": -1}})
	if limit > 0 {
		teachersPipeline = append(teachersPipeline, bson.M{"$skip": skip})
		teachersPipeline = append(teachersPipeline, bson.M{"$limit": limit})
	}

	var allTeachers []models.TeacherWithStats
	err = s.teacherRepo.Aggregate(ctx, teachersPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &allTeachers)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(count) / float64(limit))
	}

	return &models.TeacherPagination{
		Teachers: allTeachers,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *supervisorSvcs) GetCoveredCourses(ctx context.Context, id string, skip, limit int64, query *query.Conditions, enrichTeachers bool) (*content_models.CoursePagination, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs first
	teacherIdsSlice, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	if len(teacherIdsSlice) == 0 {
		return &content_models.CoursePagination{Courses: []content_models.EnrichedCourse{}, Pagination: types.Pagination{}}, nil
	}

	// Build filter from query conditions
	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}
	}

	// Get all course assignments where any of these teachers are assigned
	var courseAssignments []content_models.TeacherAssignment
	assignmentPipeline := []bson.M{
		{"$match": bson.M{
			"type":       "course",
			"teacherIds": bson.M{"$in": teacherIdsSlice},
			"trash":      false,
		}},
	}
	err = s.teacherAssignmentRepo.Aggregate(ctx, assignmentPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courseAssignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// Collect unique course IDs
	courseIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range courseAssignments {
		courseIdSet[assignment.TargetId] = true
	}

	if len(courseIdSet) == 0 {
		return &content_models.CoursePagination{Courses: []content_models.EnrichedCourse{}, Pagination: types.Pagination{}}, nil
	}

	courseIds := make([]primitive.ObjectID, 0, len(courseIdSet))
	for cid := range courseIdSet {
		courseIds = append(courseIds, cid)
	}

	coursesPipeline := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": courseIds},
			"trash": false,
		}},
	}
	// Apply query filter if provided
	if filter != nil {
		coursesPipeline = append(coursesPipeline, bson.M{"$match": filter})
	}

	countPipeline := make([]bson.M, len(coursesPipeline))
	copy(countPipeline, coursesPipeline)
	count, err := s.courseRepo.Count(ctx, countPipeline)
	if err != nil {
		return nil, err
	}

	// Add rating calculation from reviews
	coursesPipeline = append(coursesPipeline, []bson.M{
		{"$lookup": bson.M{
			"from": "academyContentReviews",
			"let":  bson.M{"courseId": "$_id"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []interface{}{
							bson.M{"$eq": []interface{}{"$refId", "$$courseId"}},
							bson.M{"$eq": []interface{}{"$type", "course"}},
						},
					},
				}},
				{"$group": bson.M{
					"_id": nil,
					"avg": bson.M{"$avg": "$rating"},
				}},
			},
			"as": "reviews",
		}},
		{"$addFields": bson.M{
			"rating": bson.M{
				"$ifNull": []interface{}{
					bson.M{"$arrayElemAt": []interface{}{"$reviews.avg", 0}},
					nil,
				},
			},
		}},
		{"$project": bson.M{"reviews": 0}},
	}...)

	if enrichTeachers {
		coursesPipeline = append(coursesPipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyTeachers",
				"let":  bson.M{"teacherIds": "$basicInfo.teachers"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{"$in": []interface{}{"$_id", "$$teacherIds"}},
						},
					},
					{
						"$project": bson.M{
							"_id":       1,
							"firstName": 1,
							"lastName":  1,
							"picture":   1,
						},
					},
				},
				"as": "teachersObjs",
			},
		})
	}

	coursesPipeline = append(coursesPipeline, bson.M{"$sort": bson.M{"_id": -1}})
	if limit > 0 {
		coursesPipeline = append(coursesPipeline, bson.M{"$skip": skip})
		coursesPipeline = append(coursesPipeline, bson.M{"$limit": limit})
	}

	var assignedCourses []content_models.EnrichedCourse
	err = s.courseRepo.Aggregate(ctx, coursesPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignedCourses)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(count) / float64(limit))
	}

	return &content_models.CoursePagination{
		Courses: assignedCourses,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *supervisorSvcs) GetCoveredCoaching(ctx context.Context, id string, skip, limit int64, query *query.Conditions, enrichTeachers bool) (*content_models.CoachingPagination, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs first
	teacherIdsSlice, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	if len(teacherIdsSlice) == 0 {
		return &content_models.CoachingPagination{Coachings: []content_models.EnrichedCoaching{}, Pagination: types.Pagination{}}, nil
	}

	// Build filter from query conditions
	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}
	}

	// Get all coaching assignments where any of these teachers are assigned
	var coachingAssignments []content_models.TeacherAssignment
	assignmentPipeline := []bson.M{
		{"$match": bson.M{
			"type":       "coaching",
			"teacherIds": bson.M{"$in": teacherIdsSlice},
			"trash":      false,
		}},
	}
	err = s.teacherAssignmentRepo.Aggregate(ctx, assignmentPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachingAssignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// Collect unique coaching IDs
	coachingIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range coachingAssignments {
		coachingIdSet[assignment.TargetId] = true
	}

	if len(coachingIdSet) == 0 {
		return &content_models.CoachingPagination{Coachings: []content_models.EnrichedCoaching{}, Pagination: types.Pagination{}}, nil
	}

	coachingIds := make([]primitive.ObjectID, 0, len(coachingIdSet))
	for cid := range coachingIdSet {
		coachingIds = append(coachingIds, cid)
	}

	coachingsPipeline := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": coachingIds},
			"trash": false,
		}},
	}
	// Apply query filter if provided
	if filter != nil {
		coachingsPipeline = append(coachingsPipeline, bson.M{"$match": filter})
	}

	countPipeline := make([]bson.M, len(coachingsPipeline))
	copy(countPipeline, coachingsPipeline)
	count, err := s.coachingRepo.Count(ctx, countPipeline)
	if err != nil {
		return nil, err
	}

	if enrichTeachers {
		coachingsPipeline = append(coachingsPipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyTeachers",
				"let":  bson.M{"teacherIds": "$coachs"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{"$in": []interface{}{"$_id", "$$teacherIds"}},
						},
					},
					{
						"$project": bson.M{
							"_id":       1,
							"firstName": 1,
							"lastName":  1,
							"picture":   1,
						},
					},
				},
				"as": "coachsObjs",
			},
		})
	}

	coachingsPipeline = append(coachingsPipeline, bson.M{"$sort": bson.M{"_id": -1}})
	if limit > 0 {
		coachingsPipeline = append(coachingsPipeline, bson.M{"$skip": skip})
		coachingsPipeline = append(coachingsPipeline, bson.M{"$limit": limit})
	}

	var assignedCoachings []content_models.EnrichedCoaching
	err = s.coachingRepo.Aggregate(ctx, coachingsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignedCoachings)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(count) / float64(limit))
	}

	return &content_models.CoachingPagination{
		Coachings: assignedCoachings,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

// getCoveredCourseIds returns course IDs assigned to the supervisor's teachers. Empty slice if id invalid or no courses.
func (s *supervisorSvcs) getCoveredCourseIds(ctx context.Context, supervisorId string) ([]primitive.ObjectID, error) {
	_id, err := primitive.ObjectIDFromHex(supervisorId)
	if err != nil {
		return nil, err
	}
	teacherIdsSlice, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil || len(teacherIdsSlice) == 0 {
		return nil, err
	}
	var courseAssignments []content_models.TeacherAssignment
	err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"type": "course", "teacherIds": bson.M{"$in": teacherIdsSlice}, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courseAssignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	courseIdSet := make(map[primitive.ObjectID]bool)
	for _, a := range courseAssignments {
		courseIdSet[a.TargetId] = true
	}
	ids := make([]primitive.ObjectID, 0, len(courseIdSet))
	for cid := range courseIdSet {
		ids = append(ids, cid)
	}
	return ids, nil
}

// getCoveredCoachingIds returns coaching IDs assigned to the supervisor's teachers.
func (s *supervisorSvcs) getCoveredCoachingIds(ctx context.Context, supervisorId string) ([]primitive.ObjectID, error) {
	_id, err := primitive.ObjectIDFromHex(supervisorId)
	if err != nil {
		return nil, err
	}
	teacherIdsSlice, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil || len(teacherIdsSlice) == 0 {
		return nil, err
	}
	var coachingAssignments []content_models.TeacherAssignment
	err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"type": "coaching", "teacherIds": bson.M{"$in": teacherIdsSlice}, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachingAssignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	coachingIdSet := make(map[primitive.ObjectID]bool)
	for _, a := range coachingAssignments {
		coachingIdSet[a.TargetId] = true
	}
	ids := make([]primitive.ObjectID, 0, len(coachingIdSet))
	for cid := range coachingIdSet {
		ids = append(ids, cid)
	}
	return ids, nil
}

func (s *supervisorSvcs) GetStudentCountsByCourse(ctx context.Context, supervisorId string) ([]content_models.StudentCountByCourseRow, error) {
	var courseIds []primitive.ObjectID
	if supervisorId != "" {
		ids, err := s.getCoveredCourseIds(ctx, supervisorId)
		if err != nil {
			return nil, err
		}
		courseIds = ids
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}
	if len(courseIds) > 0 {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"courseId": bson.M{"$in": courseIds}}})
	}
	pipeline = append(pipeline,
		bson.M{"$group": bson.M{"_id": "$courseId", "studentCount": bson.M{"$sum": 1}}},
		bson.M{"$lookup": bson.M{
			"from": "academyCourses",
			"localField": "_id",
			"foreignField": "_id",
			"as": "course",
		}},
		bson.M{"$unwind": bson.M{"path": "$course", "preserveNullAndEmptyArrays": true}},
		bson.M{"$project": bson.M{
			"courseId":     "$_id",
			"studentCount": "$studentCount",
			"courseName":   "$course.basicInfo.name",
		}},
	)
	var results []struct {
		CourseId     primitive.ObjectID         `bson:"courseId"`
		StudentCount int                         `bson:"studentCount"`
		CourseName   transl.Localizable[string] `bson:"courseName"`
	}
	if err := s.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}
	out := make([]content_models.StudentCountByCourseRow, 0, len(results))
	for _, r := range results {
		out = append(out, content_models.StudentCountByCourseRow{
			CourseId:     r.CourseId,
			StudentCount: r.StudentCount,
			CourseName:   r.CourseName,
		})
	}
	return out, nil
}

func (s *supervisorSvcs) GetStudentCountsByCoaching(ctx context.Context, supervisorId string) ([]content_models.StudentCountByCoachingRow, error) {
	var coachingIds []primitive.ObjectID
	if supervisorId != "" {
		ids, err := s.getCoveredCoachingIds(ctx, supervisorId)
		if err != nil {
			return nil, err
		}
		coachingIds = ids
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}
	if len(coachingIds) > 0 {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"coachingId": bson.M{"$in": coachingIds}}})
	}
	pipeline = append(pipeline,
		bson.M{"$group": bson.M{"_id": "$coachingId", "studentCount": bson.M{"$sum": 1}}},
		bson.M{"$lookup": bson.M{
			"from": "academyCoachings",
			"localField": "_id",
			"foreignField": "_id",
			"as": "coaching",
		}},
		bson.M{"$unwind": bson.M{"path": "$coaching", "preserveNullAndEmptyArrays": true}},
		bson.M{"$project": bson.M{
			"coachingId":    "$_id",
			"studentCount":  "$studentCount",
			"subjectId":     "$coaching.subject",
			"coachingTitle": "$coaching.title",
		}},
	)
	var results []struct {
		CoachingId    primitive.ObjectID         `bson:"coachingId"`
		StudentCount  int                       `bson:"studentCount"`
		SubjectId     primitive.ObjectID        `bson:"subjectId"`
		CoachingTitle transl.Localizable[string] `bson:"coachingTitle"`
	}
	if err := s.coachingStudentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}
	out := make([]content_models.StudentCountByCoachingRow, 0, len(results))
	for _, r := range results {
		out = append(out, content_models.StudentCountByCoachingRow{
			CoachingId:    r.CoachingId,
			SubjectId:     r.SubjectId,
			StudentCount:  r.StudentCount,
			CoachingTitle: r.CoachingTitle,
		})
	}
	return out, nil
}

// GetStudentCountsBySubject returns number of students grouped by subject across courses and coachings (admin-wide)
func (s *supervisorSvcs) GetStudentCountsBySubject(ctx context.Context) ([]content_models.StudentCountBySubjectRow, error) {
	type subjectAgg struct {
		SubjectId    primitive.ObjectID `bson:"subjectId"`
		StudentCount int                `bson:"studentCount"`
	}

	subjectCounts := make(map[primitive.ObjectID]int)

	// 1) Aggregate from course enrollments
	coursePipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$lookup": bson.M{
			"from":         "academyCourses",
			"localField":   "courseId",
			"foreignField": "_id",
			"as":           "course",
		}},
		{"$unwind": bson.M{"path": "$course", "preserveNullAndEmptyArrays": true}},
		// Explode linkedSubjects
		{"$unwind": bson.M{"path": "$course.basicInfo.linkedSubjects", "preserveNullAndEmptyArrays": true}},
		{"$group": bson.M{
			"_id":          "$course.basicInfo.linkedSubjects",
			"studentCount": bson.M{"$sum": 1},
		}},
		{"$project": bson.M{
			"subjectId":    "$_id",
			"studentCount": 1,
		}},
	}

	var courseResults []subjectAgg
	if err := s.studentCourseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courseResults)
	}); err != nil {
		return nil, err
	}
	for _, r := range courseResults {
		if r.SubjectId.IsZero() {
			continue
		}
		subjectCounts[r.SubjectId] += r.StudentCount
	}

	// 2) Aggregate from coaching enrollments
	coachingPipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "coachingProgramId",
			"foreignField": "_id",
			"as":           "coaching",
		}},
		{"$unwind": bson.M{"path": "$coaching", "preserveNullAndEmptyArrays": true}},
		{"$group": bson.M{
			"_id":          "$coaching.subject",
			"studentCount": bson.M{"$sum": 1},
		}},
		{"$project": bson.M{
			"subjectId":    "$_id",
			"studentCount": 1,
		}},
	}

	var coachingResults []subjectAgg
	if err := s.coachingStudentRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachingResults)
	}); err != nil {
		return nil, err
	}
	for _, r := range coachingResults {
		if r.SubjectId.IsZero() {
			continue
		}
		subjectCounts[r.SubjectId] += r.StudentCount
	}

	if len(subjectCounts) == 0 {
		return []content_models.StudentCountBySubjectRow{}, nil
	}

	// 3) Load subject names
	subjectIds := make([]primitive.ObjectID, 0, len(subjectCounts))
	for id := range subjectCounts {
		subjectIds = append(subjectIds, id)
	}

	out := make([]content_models.StudentCountBySubjectRow, 0, len(subjectCounts))
	for id, count := range subjectCounts {
		out = append(out, content_models.StudentCountBySubjectRow{
			SubjectId:    id,
			// SubjectName left empty for now; can be enriched by caller if needed
			StudentCount: count,
		})
	}

	return out, nil
}

func (s *supervisorSvcs) GetManagedClassesGrades(ctx context.Context, id string, skip, limit int64, query *query.Conditions) (*models.ClassGradeInfoPagination, error) {
	supervisorID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// 1. Get all overseeable teachers to identify the full scope of managed content
	teacherIds, err := s.getTeacherIdsForSupervisor(ctx, supervisorID)
	if err != nil {
		return nil, err
	}

	// Map to store unique courses and coachings in scope
	courseMap := make(map[primitive.ObjectID]content_models.Course)
	coachingMap := make(map[primitive.ObjectID]content_models.Coaching)

	// Add directly assigned content
	directScope, _ := s.getSupervisorContentScope(ctx, supervisorID)
	if directScope != nil {
		for _, c := range directScope.courses {
			courseMap[c.Id] = c
		}
		for _, c := range directScope.coachings {
			coachingMap[c.Id] = c
		}
	}

	// Add content via overseen teachers
	if len(teacherIds) > 0 {
		var assignments []content_models.TeacherAssignment
		err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"teacherIds": bson.M{"$in": teacherIds}, "trash": false}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &assignments)
		})

		if err == nil {
			var courseIdsToFetch, coachingIdsToFetch []primitive.ObjectID
			for _, a := range assignments {
				if a.Type == "course" {
					if _, ok := courseMap[a.TargetId]; !ok {
						courseIdsToFetch = append(courseIdsToFetch, a.TargetId)
					}
				} else if a.Type == "coaching" {
					if _, ok := coachingMap[a.TargetId]; !ok {
						coachingIdsToFetch = append(coachingIdsToFetch, a.TargetId)
					}
				}
			}

			if len(courseIdsToFetch) > 0 {
				var extra []content_models.Course
				s.courseRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": courseIdsToFetch}, "trash": false}}}, func(cur *mongo.Cursor) error {
					return cur.All(ctx, &extra)
				})
				for _, c := range extra {
					courseMap[c.Id] = c
				}
			}
			if len(coachingIdsToFetch) > 0 {
				var extra []content_models.Coaching
				s.coachingRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": coachingIdsToFetch}, "trash": false}}}, func(cur *mongo.Cursor) error {
					return cur.All(ctx, &extra)
				})
				for _, c := range extra {
					coachingMap[c.Id] = c
				}
			}
		}
	}

	allCourseIds := make([]primitive.ObjectID, 0, len(courseMap))
	for id := range courseMap {
		allCourseIds = append(allCourseIds, id)
	}
	allCoachingIds := make([]primitive.ObjectID, 0, len(coachingMap))
	for id := range coachingMap {
		allCoachingIds = append(allCoachingIds, id)
	}

	// Batch get teacher assignments for counts
	courseTeachers, _ := s.teacherAssignmentHelper.BatchGetCourseTeachers(ctx, allCourseIds)
	coachingTeachers, _ := s.teacherAssignmentHelper.BatchGetCoachingTeachers(ctx, allCoachingIds)

	result := make([]models.ClassGradeInfo, 0)

	// 2. Process Courses
	if len(allCourseIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{"courseId": bson.M{"$in": allCourseIds}, "trash": false}},
			{"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "student",
			}},
			{"$unwind": "$student"},
			{"$match": bson.M{"student.trash": false}},
			{"$group": bson.M{
				"_id": bson.M{
					"courseId": "$courseId",
					"grade":    "$student.academicInfo.class",
				},
				"count": bson.M{"$sum": 1},
			}},
		}
		var counts []struct {
			ID struct {
				CourseId primitive.ObjectID `bson:"courseId"`
				Grade    string             `bson:"grade"`
			} `bson:"_id"`
			Count int `bson:"count"`
		}
		s.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &counts)
		})

		courseStudentCount := make(map[primitive.ObjectID]map[string]int)
		for _, c := range counts {
			if courseStudentCount[c.ID.CourseId] == nil {
				courseStudentCount[c.ID.CourseId] = make(map[string]int)
			}
			courseStudentCount[c.ID.CourseId][c.ID.Grade] = c.Count
		}

		for _, c := range courseMap {
			// Courses usually have a single grade level in basic info, but we can have students from different grades
			// However, for unique combinations, we should probably stick to what's defined in the course OR what students are actually there.
			// Requirement says unique combinations of Course + Class/Grade.

			// If we want all actual combinations:
			if m, ok := courseStudentCount[c.Id]; ok {
				for grade, sCount := range m {
					result = append(result, models.ClassGradeInfo{
						Grade:            grade,
						ContentName:      (c.BasicInfo.Name),
						NumberOfStudents: sCount,
						TeacherAssigned:  len(courseTeachers[c.Id]),
					})
				}
			} else if c.BasicInfo.GradeLevel != "" {
				// Fallback if no students yet
				result = append(result, models.ClassGradeInfo{
					Grade:            c.BasicInfo.GradeLevel,
					ContentName:      c.BasicInfo.Name,
					NumberOfStudents: 0,
					TeacherAssigned:  len(courseTeachers[c.Id]),
				})
			}
		}
	}

	// 3. Process Coachings (Grouped by Student Academic Class)
	if len(allCoachingIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{"coachingId": bson.M{"$in": allCoachingIds}, "trash": false}},
			{"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "student",
			}},
			{"$unwind": "$student"},
			{"$match": bson.M{"student.trash": false}},
			{"$group": bson.M{
				"_id": bson.M{
					"coachingId": "$coachingId",
					"class":      "$student.academicInfo.class",
				},
				"count": bson.M{"$sum": 1},
			}},
		}
		var coachingCounts []struct {
			ID struct {
				CoachingId primitive.ObjectID `bson:"coachingId"`
				Class      string             `bson:"class"`
			} `bson:"_id"`
			Count int `bson:"count"`
		}
		s.coachingStudentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachingCounts)
		})

		countMap := make(map[primitive.ObjectID]map[string]int)
		for _, item := range coachingCounts {
			if countMap[item.ID.CoachingId] == nil {
				countMap[item.ID.CoachingId] = make(map[string]int)
			}
			countMap[item.ID.CoachingId][item.ID.Class] = item.Count
		}

		for _, co := range coachingMap {
			tCount := len(coachingTeachers[co.Id])

			grades := co.Class
			if len(grades) == 0 && co.Level != "" {
				grades = []string{co.Level}
			}

			for _, g := range grades {
				sCount := 0
				if m, ok := countMap[co.Id]; ok {
					sCount = m[g]
				}
				result = append(result, models.ClassGradeInfo{
					Grade:            g,
					ContentName:      co.Title,
					NumberOfStudents: sCount,
					TeacherAssigned:  tCount,
				})
			}
		}
	}

	// 4. Filter & Paginate
	filteredResult := s.applyClassGradeFilters(result, query)
	totalCount := int64(len(filteredResult))
	start, end := skip, skip+limit
	if start > totalCount {
		start = totalCount
	}
	if end > totalCount {
		end = totalCount
	}

	paginatedResult := []models.ClassGradeInfo{}
	if start < totalCount {
		paginatedResult = filteredResult[start:end]
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(totalCount) / float64(limit))
	}

	return &models.ClassGradeInfoPagination{
		ClassGradeInfos: paginatedResult,
		Pagination: common.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: totalCount,
		},
	}, nil
}

// Helper function to apply filters to ClassGradeInfo results
func (s *supervisorSvcs) applyClassGradeFilters(results []models.ClassGradeInfo, query *query.Conditions) []models.ClassGradeInfo {
	if query == nil || len(query.Columns) == 0 {
		return results
	}

	filtered := make([]models.ClassGradeInfo, 0)

	for _, item := range results {
		shouldInclude := true

		for _, col := range query.Columns {
			var matches bool
			switch col.Name {
			case "grade":
				matches = s.matchesFilter(item.Grade, col)
			case "contentName.en":
				matches = s.matchesFilter(item.ContentName.GetContentByLang("en"), col)
			case "contentName.ar":
				matches = s.matchesFilter(item.ContentName.GetContentByLang("ar"), col)
			case "numberOfStudents":
				studentCount := item.NumberOfStudents
				matches = s.matchesNumericFilter(studentCount, col)
			default:
				matches = true
			}

			if !matches {
				shouldInclude = false
				break
			}
		}

		if shouldInclude {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// Helper function to check if a string value matches the filter
func (s *supervisorSvcs) matchesFilter(value string, col query.Column) bool {
	if col.Value == nil {
		return true
	}

	filterValue := fmt.Sprintf("%v", col.Value)
	exp := strings.ToLower(col.Exp)

	switch exp {
	case "eq", "=", "":
		return value == filterValue
	case "neq", "!=":
		return value != filterValue
	case "like":
		return strings.Contains(strings.ToLower(value), strings.ToLower(filterValue))
	case "in":
		// Handle comma-separated values
		values := strings.Split(filterValue, ",")
		for _, v := range values {
			if strings.TrimSpace(v) == value {
				return true
			}
		}
		return false
	default:
		return true
	}
}

// Helper function to check if a numeric value matches the filter
func (s *supervisorSvcs) matchesNumericFilter(value int, col query.Column) bool {
	if col.Value == nil {
		return true
	}

	filterValue, ok := col.Value.(float64)
	if !ok {
		// Try to parse as string
		if strVal, ok := col.Value.(string); ok {
			parsed, err := strconv.ParseFloat(strVal, 64)
			if err != nil {
				return true
			}
			filterValue = parsed
		} else {
			return true
		}
	}

	exp := strings.ToLower(col.Exp)
	intFilterValue := int(filterValue)

	switch exp {
	case "eq", "=", "":
		return value == intFilterValue
	case "neq", "!=":
		return value != intFilterValue
	case "gt", ">":
		return value > intFilterValue
	case "gte", ">=":
		return value >= intFilterValue
	case "lt", "<":
		return value < intFilterValue
	case "lte", "<=":
		return value <= intFilterValue
	default:
		return true
	}
}

// Helper function to get teacher IDs for a supervisor.
// Preference is given to explicit SupervisorAssignment records (direct + dynamic),
// with a fallback to content-based scope for backward compatibility when no assignments exist.
func (s *supervisorSvcs) getTeacherIdsForSupervisor(ctx context.Context, supervisorId primitive.ObjectID) ([]primitive.ObjectID, error) {
	teacherSet := make(map[primitive.ObjectID]bool)

	// 1. Primary source: SupervisorAssignment-based scope (direct + dynamic)
	assignmentScope, err := s.getSupervisorScope(ctx, supervisorId)
	hasAssignments := err == nil && (len(assignmentScope.directTeacherIds) > 0 || len(assignmentScope.dynamicCriteria) > 0)

	if hasAssignments {
		// Add direct teacher assignments
		for _, id := range assignmentScope.directTeacherIds {
			teacherSet[id] = true
		}

		// Resolve teachers from dynamic criteria (courses/coachings matching criteria)
		if len(assignmentScope.dynamicCriteria) > 0 {
			resolvedTeachers, err := s.resolveTeachersFromScope(ctx, assignmentScope)
			if err == nil {
				for _, tid := range resolvedTeachers {
					teacherSet[tid] = true
				}
			}
		}
	} else {
		// 2. Fallback: Content-based scope when no explicit assignments exist
		scope, err := s.getSupervisorContentScope(ctx, supervisorId)
		if err == nil {
			// Get teachers from courses using batch lookup
			if len(scope.courses) > 0 {
				courseIds := make([]primitive.ObjectID, len(scope.courses))
				for i, c := range scope.courses {
					courseIds[i] = c.Id
				}
				courseTeachers, err := s.teacherAssignmentHelper.BatchGetCourseTeachers(ctx, courseIds)
				if err == nil {
					for _, teacherIds := range courseTeachers {
						for _, tid := range teacherIds {
							teacherSet[tid] = true
						}
					}
				}
			}

			// Get teachers from coachings using batch lookup
			if len(scope.coachings) > 0 {
				coachingIds := make([]primitive.ObjectID, len(scope.coachings))
				for i, c := range scope.coachings {
					coachingIds[i] = c.Id
				}
				coachingTeachers, err := s.teacherAssignmentHelper.BatchGetCoachingTeachers(ctx, coachingIds)
				if err == nil {
					for _, teacherIds := range coachingTeachers {
						for _, tid := range teacherIds {
							teacherSet[tid] = true
						}
					}
				}
			}
		}
	}

	teacherIds := make([]primitive.ObjectID, 0, len(teacherSet))
	for id := range teacherSet {
		teacherIds = append(teacherIds, id)
	}

	return teacherIds, nil
}

func (s *supervisorSvcs) populateStats(ctx context.Context, supervisors []models.SupervisorWithStats) error {
	if len(supervisors) == 0 {
		return nil
	}

	supIDs := make([]primitive.ObjectID, len(supervisors))
	for i, sup := range supervisors {
		supIDs[i] = sup.Id
	}

	// Map SupervisorID -> Set of TeacherIDs
	supTeacherMap := make(map[primitive.ObjectID]map[primitive.ObjectID]bool)
	for _, id := range supIDs {
		supTeacherMap[id] = make(map[primitive.ObjectID]bool)
	}

	// 1. Content Scope: Courses
	var courses []content_models.Course
	// Find courses where ANY of the supervisors is assigned
	err := s.courseRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"basicInfo.supervisorIds": bson.M{"$in": supIDs}, "trash": false}},
		{"$project": bson.M{"_id": 1, "basicInfo.supervisorIds": 1}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	})
	if err != nil {
		return err
	}

	// 2. Content Scope: Coachings
	var coachings []content_models.Coaching
	err = s.coachingRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"supervisorIds": bson.M{"$in": supIDs}, "trash": false}},
		{"$project": bson.M{"_id": 1, "supervisorIds": 1}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachings)
	})
	if err != nil {
		return err
	}

	// Collect Course/Coaching IDs to fetch teachers
	courseIDs := make([]primitive.ObjectID, len(courses))
	for i, c := range courses {
		courseIDs[i] = c.Id
	}

	coachingIDs := make([]primitive.ObjectID, len(coachings))
	for i, c := range coachings {
		coachingIDs[i] = c.Id
	}

	// Batch fetch teachers for these courses/coachings
	courseTeachers, _ := s.teacherAssignmentHelper.BatchGetCourseTeachers(ctx, courseIDs)
	coachingTeachers, _ := s.teacherAssignmentHelper.BatchGetCoachingTeachers(ctx, coachingIDs)

	// Map back to supervisors
	for _, c := range courses {
		tids := courseTeachers[c.Id]
		for _, supID := range c.BasicInfo.SupervisorIds {
			if _, ok := supTeacherMap[supID]; ok {
				for _, tid := range tids {
					supTeacherMap[supID][tid] = true
				}
			}
		}
	}
	for _, c := range coachings {
		tids := coachingTeachers[c.Id]
		for _, supID := range c.SupervisorIds {
			if _, ok := supTeacherMap[supID]; ok {
				for _, tid := range tids {
					supTeacherMap[supID][tid] = true
				}
			}
		}
	}

	// 3. Supervisor Assignments (Direct & Dynamic)
	var assignments []content_models.SupervisorAssignment
	err = s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"supervisorId": bson.M{"$in": supIDs},
			"status":       "assigned",
			"trash":        false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil {
		return err
	}

	// Identify supervisors with dynamic assignments
	dynamicSupIDs := make(map[primitive.ObjectID]bool)
	for _, assign := range assignments {
		if assign.RefType == "teacher" {
			for _, tid := range assign.RefIds {
				if _, ok := supTeacherMap[assign.SupervisorId]; ok {
					supTeacherMap[assign.SupervisorId][tid] = true
				}
			}
		} else {
			dynamicSupIDs[assign.SupervisorId] = true
		}
	}

	// Fallback for supervisors with dynamic assignments
	for supID := range dynamicSupIDs {
		tids, err := s.getTeacherIdsForSupervisor(ctx, supID)
		if err == nil {
			for _, tid := range tids {
				supTeacherMap[supID][tid] = true
			}
		}
	}

	// Now we have all teachers for all supervisors.
	// Collect ALL unique teachers to fetch their students.
	allTeacherIDsMap := make(map[primitive.ObjectID]bool)
	for _, tids := range supTeacherMap {
		for tid := range tids {
			allTeacherIDsMap[tid] = true
		}
	}
	allTeacherIDs := make([]primitive.ObjectID, 0, len(allTeacherIDsMap))
	for tid := range allTeacherIDsMap {
		allTeacherIDs = append(allTeacherIDs, tid)
	}

	// Fetch courses/coachings for these teachers
	var teacherAssignments []content_models.TeacherAssignment
	if len(allTeacherIDs) > 0 {
		err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"teacherIds": bson.M{"$in": allTeacherIDs}, "trash": false}},
			{"$project": bson.M{"targetId": 1, "type": 1, "teacherIds": 1}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teacherAssignments)
		})
		if err != nil {
			return err
		}
	}

	// Map TeacherID -> [CourseIDs] and TeacherID -> [CoachingIDs]
	teacherCourses := make(map[primitive.ObjectID][]primitive.ObjectID)
	teacherCoachings := make(map[primitive.ObjectID][]primitive.ObjectID)

	allCourseIDsMap := make(map[primitive.ObjectID]bool)
	allCoachingIDsMap := make(map[primitive.ObjectID]bool)

	for _, ta := range teacherAssignments {
		for _, tid := range ta.TeacherIds {
			if allTeacherIDsMap[tid] {
				if ta.Type == "course" {
					teacherCourses[tid] = append(teacherCourses[tid], ta.TargetId)
					allCourseIDsMap[ta.TargetId] = true
				} else if ta.Type == "coaching" {
					teacherCoachings[tid] = append(teacherCoachings[tid], ta.TargetId)
					allCoachingIDsMap[ta.TargetId] = true
				}
			}
		}
	}

	// Fetch Students for these courses/coachings
	allCourseIDs := make([]primitive.ObjectID, 0, len(allCourseIDsMap))
	for id := range allCourseIDsMap {
		allCourseIDs = append(allCourseIDs, id)
	}

	allCoachingIDs := make([]primitive.ObjectID, 0, len(allCoachingIDsMap))
	for id := range allCoachingIDsMap {
		allCoachingIDs = append(allCoachingIDs, id)
	}

	// Map CourseID -> [StudentIDs]
	courseStudents := make(map[primitive.ObjectID][]primitive.ObjectID)
	if len(allCourseIDs) > 0 {
		var scs []content_models.StudentCourse
		err = s.studentCourseRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"courseId": bson.M{"$in": allCourseIDs}, "trash": false}},
			{"$project": bson.M{"courseId": 1, "studentId": 1}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &scs)
		})
		if err == nil {
			for _, sc := range scs {
				courseStudents[sc.CourseId] = append(courseStudents[sc.CourseId], sc.StudentId)
			}
		}
	}

	// Map CoachingID -> [StudentIDs]
	coachingStudents := make(map[primitive.ObjectID][]primitive.ObjectID)
	if len(allCoachingIDs) > 0 {
		var sss []struct {
			CoachingProgramId primitive.ObjectID `bson:"coachingProgramId"`
			StudentId         primitive.ObjectID `bson:"studentId"`
		}
		err = s.studentSessionRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"coachingProgramId": bson.M{"$in": allCoachingIDs}, "trash": false}},
			{"$project": bson.M{"coachingProgramId": 1, "studentId": 1}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &sss)
		})
		if err == nil {
			for _, ss := range sss {
				coachingStudents[ss.CoachingProgramId] = append(coachingStudents[ss.CoachingProgramId], ss.StudentId)
			}
		}
	}

	// Verify students exist and are not trashed
	allStudentIDsMap := make(map[primitive.ObjectID]bool)
	for _, sids := range courseStudents {
		for _, sid := range sids {
			allStudentIDsMap[sid] = true
		}
	}
	for _, sids := range coachingStudents {
		for _, sid := range sids {
			allStudentIDsMap[sid] = true
		}
	}

	validStudentIDs := make(map[primitive.ObjectID]bool)
	if len(allStudentIDsMap) > 0 {
		allStudentIDs := make([]primitive.ObjectID, 0, len(allStudentIDsMap))
		for sid := range allStudentIDsMap {
			allStudentIDs = append(allStudentIDs, sid)
		}

		var validStudents []struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		err = s.studentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": allStudentIDs}, "trash": false}},
			{"$project": bson.M{"_id": 1}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &validStudents)
		})
		if err != nil {
			return err
		}
		for _, s := range validStudents {
			validStudentIDs[s.Id] = true
		}
	}

	// Final Aggregation
	for i := range supervisors {
		supID := supervisors[i].Id
		teachers := supTeacherMap[supID]

		supervisors[i].TotalNumTeachers = len(teachers)

		studentSet := make(map[primitive.ObjectID]bool)
		for tid := range teachers {
			// Add students from courses
			for _, cid := range teacherCourses[tid] {
				for _, sid := range courseStudents[cid] {
					if validStudentIDs[sid] {
						studentSet[sid] = true
					}
				}
			}
			// Add students from coachings
			for _, cid := range teacherCoachings[tid] {
				for _, sid := range coachingStudents[cid] {
					if validStudentIDs[sid] {
						studentSet[sid] = true
					}
				}
			}
		}
		supervisors[i].TotalNumStudents = len(studentSet)
	}

	return nil
}

// GetOneWithStats gets a single supervisor with totalNumStudents and totalNumTeachers
func (s *supervisorSvcs) GetOneWithStats(ctx context.Context, id string) (*models.SupervisorWithStats, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get supervisor
	supervisor, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// 1. Get Scope (Courses & Coachings)
	scope, err := s.getSupervisorContentScope(ctx, _id)
	if err != nil {
		return nil, err
	}

	// 2. Count Teachers using TeacherAssignmentSvcs
	teacherSet := make(map[primitive.ObjectID]bool)

	// Get teachers from courses using batch lookup
	if len(scope.courses) > 0 {
		courseIds := make([]primitive.ObjectID, len(scope.courses))
		for i, c := range scope.courses {
			courseIds[i] = c.Id
		}
		courseTeachers, err := s.teacherAssignmentHelper.BatchGetCourseTeachers(ctx, courseIds)
		if err == nil {
			for _, teacherIds := range courseTeachers {
				for _, tid := range teacherIds {
					teacherSet[tid] = true
				}
			}
		}
	}

	// Get teachers from coachings using batch lookup
	if len(scope.coachings) > 0 {
		coachingIds := make([]primitive.ObjectID, len(scope.coachings))
		for i, c := range scope.coachings {
			coachingIds[i] = c.Id
		}
		coachingTeachers, err := s.teacherAssignmentHelper.BatchGetCoachingTeachers(ctx, coachingIds)
		if err == nil {
			for _, teacherIds := range coachingTeachers {
				for _, tid := range teacherIds {
					teacherSet[tid] = true
				}
			}
		}
	}
	teachersCount := int64(len(teacherSet))

	// 3. Count Students
	studentSet := make(map[primitive.ObjectID]bool)

	// A. From Courses
	if len(scope.courses) > 0 {
		courseIds := make([]primitive.ObjectID, len(scope.courses))
		for i, c := range scope.courses {
			courseIds[i] = c.Id
		}

		pipeline := []bson.M{
			{"$match": bson.M{
				"courseId": bson.M{"$in": courseIds},
				"trash":    false,
			}},
			{"$group": bson.M{"_id": "$studentId"}},
		}
		var studentids []struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		err = s.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentids)
		})
		if err == nil {
			for _, s := range studentids {
				studentSet[s.Id] = true
			}
		}
	}

	// B. From Coachings
	if len(scope.coachings) > 0 {
		coachingIds := make([]primitive.ObjectID, len(scope.coachings))
		for i, c := range scope.coachings {
			coachingIds[i] = c.Id
		}

		pipeline := []bson.M{
			{"$match": bson.M{
				"coachingId": bson.M{"$in": coachingIds},
				"trash":      false,
			}},
			{"$group": bson.M{"_id": "$studentId"}},
		}
		var studentids []struct {
			Id primitive.ObjectID `bson:"_id"`
		}
		err = s.coachingStudentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentids)
		})
		if err == nil {
			for _, s := range studentids {
				studentSet[s.Id] = true
			}
		}
	}
	studentsCount := int64(len(studentSet))

	return &models.SupervisorWithStats{
		Supervisor:       *supervisor,
		TotalNumStudents: int(studentsCount),
		TotalNumTeachers: int(teachersCount),
	}, nil
}

// GetTeachersBySupervisorId gets all teachers where any subject has this supervisorId
func (s *supervisorSvcs) GetTeachersBySupervisorId(ctx context.Context, id string, query *query.Conditions) ([]models.Teacher, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs using content-based scope
	teacherIds, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	if len(teacherIds) == 0 {
		return []models.Teacher{}, nil
	}

	// 3. Fetch full teacher objects
	pipeline := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": teacherIds},
			"trash": false,
		}},
	}

	// Apply filter if provided
	if query != nil {
		if err := query.CheckValid(); err == nil {
			filter, err := query.ConvertToMongo()
			if err == nil {
				pipeline = append(pipeline, bson.M{"$match": filter})
			}
		}
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	var teachers []models.Teacher
	err = s.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teachers)
	})
	if err != nil {
		return nil, err
	}

	return teachers, nil
}

// GetStudentsBySupervisorId gets all students enrolled in courses/coaching taught by supervisor's teachers
func (s *supervisorSvcs) GetStudentsBySupervisorId(ctx context.Context, id string, query *query.Conditions) ([]models.Student, error) {
	// Get all teachers for this supervisor
	teachers, err := s.GetTeachersBySupervisorId(ctx, id, nil)
	if err != nil {
		return nil, err
	}

	if len(teachers) == 0 {
		return []models.Student{}, nil
	}

	teacherIds := make([]primitive.ObjectID, len(teachers))
	for i, t := range teachers {
		teacherIds[i] = t.Id
	}

	uniqueStudentIds := make(map[primitive.ObjectID]bool)

	// Get courses taught by these teachers using TeacherAssignmentSvcs
	var courseIds []primitive.ObjectID
	if len(teacherIds) > 0 {
		// Get all course assignments where any of these teachers are assigned
		var courseAssignments []content_models.TeacherAssignment
		pipeline := []bson.M{
			{"$match": bson.M{
				"type":       "course",
				"teacherIds": bson.M{"$in": teacherIds},
				"trash":      false,
			}},
		}
		err = s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courseAssignments)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		courseIdSet := make(map[primitive.ObjectID]bool)
		for _, assignment := range courseAssignments {
			courseIdSet[assignment.TargetId] = true
		}
		courseIds = make([]primitive.ObjectID, 0, len(courseIdSet))
		for id := range courseIdSet {
			courseIds = append(courseIds, id)
		}
	}

	// Get students enrolled in these courses
	if len(courseIds) > 0 {
		studentCoursePipeline := []bson.M{
			{"$match": bson.M{
				"courseId": bson.M{"$in": courseIds},
				"trash":    false,
			}},
		}
		var studentCourses []content_models.StudentCourse
		err = s.studentCourseRepo.Aggregate(ctx, studentCoursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentCourses)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		for _, sc := range studentCourses {
			uniqueStudentIds[sc.StudentId] = true
		}
	}

	// Get coaching programs taught by these teachers using TeacherAssignmentSvcs
	var coachingIds []primitive.ObjectID
	if len(teacherIds) > 0 {
		// Get all coaching assignments where any of these teachers are assigned
		var coachingAssignments []content_models.TeacherAssignment
		pipeline := []bson.M{
			{"$match": bson.M{
				"type":       "coaching",
				"teacherIds": bson.M{"$in": teacherIds},
				"trash":      false,
			}},
		}
		err = s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachingAssignments)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		coachingIdSet := make(map[primitive.ObjectID]bool)
		for _, assignment := range coachingAssignments {
			coachingIdSet[assignment.TargetId] = true
		}
		coachingIds = make([]primitive.ObjectID, 0, len(coachingIdSet))
		for id := range coachingIdSet {
			courseIds = append(courseIds, id)
		}
	}

	// Get students enrolled in these coaching programs
	if len(coachingIds) > 0 {
		studentSessionPipeline := []bson.M{
			{"$match": bson.M{
				"coachingProgramId": bson.M{"$in": coachingIds},
				"trash":             false,
			}},
			{"$group": bson.M{
				"_id": "$studentId",
			}},
		}
		var studentSessions []struct {
			StudentId primitive.ObjectID `bson:"_id"`
		}
		err = s.studentSessionRepo.Aggregate(ctx, studentSessionPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentSessions)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		for _, ss := range studentSessions {
			uniqueStudentIds[ss.StudentId] = true
		}
	}

	// Extract unique student IDs
	studentIds := make([]primitive.ObjectID, 0, len(uniqueStudentIds))
	for id := range uniqueStudentIds {
		studentIds = append(studentIds, id)
	}

	if len(studentIds) == 0 {
		return []models.Student{}, nil
	}

	// 3. Fetch full student objects
	pipeline := []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": studentIds},
			"trash": false,
		}},
	}

	// Apply filter if provided
	if query != nil {
		if err := query.CheckValid(); err == nil {
			filter, err := query.ConvertToMongo()
			if err == nil {
				pipeline = append(pipeline, bson.M{"$match": filter})
			}
		}
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	var students []models.Student
	err = s.studentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &students)
	})
	if err != nil {
		return nil, err
	}

	return students, nil
}

// IsTeacherUnderSupervisor checks if a teacher is managed by a supervisor
func (s *supervisorSvcs) IsTeacherUnderSupervisor(ctx context.Context, teacherId, supervisorId primitive.ObjectID) (bool, error) {
	// Get teacher IDs using content-based scope
	teacherIds, err := s.getTeacherIdsForSupervisor(ctx, supervisorId)
	if err != nil {
		return false, err
	}

	for _, tid := range teacherIds {
		if tid == teacherId {
			return true, nil
		}
	}

	return false, nil
}

// GetManagedUsers gets the IDs of teachers and students managed by a supervisor
func (s *supervisorSvcs) GetManagedUsers(ctx context.Context, id string, withStudent bool) (*models.ManagedUsersResponse, error) {
	supervisorID, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Get teacher IDs using content-based scope
	teacherIds, err := s.getTeacherIdsForSupervisor(ctx, supervisorID)
	if err != nil {
		return nil, err
	}

	resp := &models.ManagedUsersResponse{
		TeacherIds: teacherIds,
	}

	if withStudent {
		studentsData, err := s.GetAssignedStudents(ctx, id, 0, 0, nil)
		if err != nil {
			return nil, err
		}

		studentIds := make([]primitive.ObjectID, len(studentsData.Students))
		for i, student := range studentsData.Students {
			studentIds[i] = student.Id
		}
		resp.StudentIds = studentIds
	}

	return resp, nil
}

// GetDashboard gets the dashboard data for a supervisor
func (s *supervisorSvcs) GetDashboard(ctx context.Context, id string) (*content_models.SupervisorDashboard, error) {
	return s.GetDashboardWithFilter(ctx, id, "all", false)
}

// GetDashboardWithFilter gets the dashboard data for a supervisor with status filter
func (s *supervisorSvcs) GetDashboardWithFilter(ctx context.Context, id string, statusFilter string, todayOnly bool) (*content_models.SupervisorDashboard, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// 1. Get scope and teachers
	scope, err := s.getSupervisorContentScope(ctx, _id)
	if err != nil {
		return nil, err
	}

	teacherIds, err := s.getTeacherIdsForSupervisor(ctx, _id)
	if err != nil {
		return nil, err
	}

	// 1.5 Subjects count from content scope (courses + coachings)
	// Here, a \"subject\" is considered as the combination of subjectId + schoolType + level + country.
	// This matches the domain rule that (math, beginner, private, USA) and (math, intermediate, public, USA)
	// are treated as two distinct subject assignments.
	subjectGroupSet := make(map[string]bool)
	for _, c := range scope.courses {
		for _, sid := range c.BasicInfo.LinkedSubjects {
			key := fmt.Sprintf("%s|%s|%s|%s", sid.Hex(), c.BasicInfo.SchoolType, c.BasicInfo.Level, c.BasicInfo.Country)
			subjectGroupSet[key] = true
		}
	}
	for _, c := range scope.coachings {
		if !c.Subject.IsZero() {
			key := fmt.Sprintf("%s|%s|%s|%s", c.Subject.Hex(), c.SchoolType, c.Level, c.Country)
			subjectGroupSet[key] = true
		}
	}
	assignedSubjectsCount := len(subjectGroupSet)
	approvedSubjectsCount := assignedSubjectsCount

	// 2. Prepare result structure
	dashboard := &content_models.SupervisorDashboard{
		TotalTeachers:    len(teacherIds),
		AssignedSubjects: assignedSubjectsCount,
		ApprovedSubjects: approvedSubjectsCount,
		StatusSummary: content_models.StatusSummary{
			Active:    0,
			Completed: 0,
			Late:      0,
		},
		UpcomingSessions: []content_models.UpcomingSession{},
		RecentFeedbacks:  []content_models.RecentFeedback{},
	}

	if len(teacherIds) == 0 {
		return dashboard, nil
	}

	// 2.5 Calculate Total Students
	uniqueStudentIds := make(map[primitive.ObjectID]bool)

	// A. Students from Courses
	if len(scope.courses) > 0 {
		courseIds := make([]primitive.ObjectID, len(scope.courses))
		for i, c := range scope.courses {
			courseIds[i] = c.Id
		}
		var studentCourses []content_models.StudentCourse
		err = s.studentCourseRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"courseId": bson.M{"$in": courseIds}, "trash": false}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentCourses)
		})
		if err == nil {
			for _, sc := range studentCourses {
				uniqueStudentIds[sc.StudentId] = true
			}
		}
	}

	// B. Students from Coachings
	if len(scope.coachings) > 0 {
		coachingIds := make([]primitive.ObjectID, len(scope.coachings))
		for i, c := range scope.coachings {
			coachingIds[i] = c.Id
		}
		var studentSessions []struct {
			StudentId primitive.ObjectID `bson:"studentId"`
		}
		err = s.studentSessionRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"coachingProgramId": bson.M{"$in": coachingIds}, "trash": false}},
			{"$group": bson.M{"_id": "$studentId", "studentId": bson.M{"$first": "$studentId"}}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentSessions)
		})
		if err == nil {
			for _, ss := range studentSessions {
				uniqueStudentIds[ss.StudentId] = true
			}
		}
	}

	// C. Direct Teacher Students (legacy/fallback)
	var directSessions []struct {
		StudentId primitive.ObjectID `bson:"studentId"`
	}
	err = s.studentSessionRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"teacherId": bson.M{"$in": teacherIds}, "trash": false}},
		{"$group": bson.M{"_id": "$studentId", "studentId": bson.M{"$first": "$studentId"}}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &directSessions)
	})
	if err == nil {
		for _, ds := range directSessions {
			uniqueStudentIds[ds.StudentId] = true
		}
	}

	dashboard.TotalStudents = len(uniqueStudentIds)

	// 3. Get Student Sessions status summary
	// For now, we'll implement a basic version.
	// In a real scenario, we'd need more complex queries across collections.

	sessionMatch := bson.M{
		"teacherId": bson.M{"$in": teacherIds},
		"trash":     false,
	}

	// Today's range for TodaySessions
	now := time.Now()
	startOfDay := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	endOfDay := startOfDay.Add(24 * time.Hour)

	if todayOnly {
		sessionMatch["startTime"] = bson.M{
			"$gte": startOfDay,
			"$lt":  endOfDay,
		}
	}

	pipeline := []bson.M{
		{"$match": sessionMatch},
		{"$facet": bson.M{
			"statusStats": []bson.M{
				{"$group": bson.M{
					"_id":   "$status",
					"count": bson.M{"$sum": 1},
				}},
			},
			"todayCount": []bson.M{
				{"$match": bson.M{
					"startTime": bson.M{
						"$gte": startOfDay,
						"$lt":  endOfDay,
					},
				}},
				{"$count": "count"},
			},
		}},
	}

	var results []struct {
		StatusStats []struct {
			Status string `bson:"_id"`
			Count  int    `bson:"count"`
		} `bson:"statusStats"`
		TodayCount []struct {
			Count int `bson:"count"`
		} `bson:"todayCount"`
	}

	err = s.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	})

	if err == nil && len(results) > 0 {
		for _, stat := range results[0].StatusStats {
			switch strings.ToLower(stat.Status) {
			case "active", "scheduled":
				dashboard.StatusSummary.Active += stat.Count
			case "completed":
				dashboard.StatusSummary.Completed += stat.Count
			case "late", "missed":
				dashboard.StatusSummary.Late += stat.Count
			}
		}
		if len(results[0].TodayCount) > 0 {
			dashboard.TodaySessions = results[0].TodayCount[0].Count
		}
	}

	// 4. Get Upcoming Sessions
	upcomingPipeline := []bson.M{
		{"$match": bson.M{
			"teacherId": bson.M{"$in": teacherIds},
			"trash":     false,
			"startTime": bson.M{"$gte": time.Now()},
		}},
		{"$sort": bson.M{"startTime": 1}},
		{"$limit": 5},
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "student",
		}},
		{"$unwind": "$student"},
		{"$lookup": bson.M{
			"from":         "academyTeachers",
			"localField":   "teacherId",
			"foreignField": "_id",
			"as":           "teacher",
		}},
		{"$unwind": "$teacher"},
	}

	err = s.studentSessionRepo.Aggregate(ctx, upcomingPipeline, func(cur *mongo.Cursor) error {
		var sessions []struct {
			StartTime time.Time `bson:"startTime"`
			Status    string    `bson:"status"`
			Student   struct {
				GeneralInfo struct {
					FirstName transl.Localizable[string] `bson:"firstName"`
					LastName  transl.Localizable[string] `bson:"lastName"`
				} `bson:"generalInfo"`
			} `bson:"student"`
			Teacher struct {
				FirstName transl.Localizable[string] `bson:"firstName"`
				LastName  transl.Localizable[string] `bson:"lastName"`
			} `bson:"teacher"`
		}
		if err := cur.All(ctx, &sessions); err != nil {
			return err
		}

		for _, s := range sessions {
			dashboard.UpcomingSessions = append(dashboard.UpcomingSessions, content_models.UpcomingSession{
				StudentName: s.Student.GeneralInfo.FirstName.GetContentByLang("ar") + " " + s.Student.GeneralInfo.LastName.GetContentByLang("ar"),
				TeacherName: s.Teacher.FirstName.GetContentByLang("ar") + " " + s.Teacher.LastName.GetContentByLang("ar"),
				Time:        s.StartTime,
				Status:      s.Status,
			})
		}
		return nil
	})

	// 5. Get Recent Feedbacks
	feedbackPipeline := []bson.M{
		{"$match": bson.M{
			"teacherId":              bson.M{"$in": teacherIds},
			"trash":                  false,
			"feedback.teacherRating": bson.M{"$ne": nil},
		}},
		{"$sort": bson.M{"updatedAt": -1}},
		{"$limit": 5},
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "student",
		}},
		{"$unwind": "$student"},
	}

	_ = s.studentSessionRepo.Aggregate(ctx, feedbackPipeline, func(cur *mongo.Cursor) error {
		var feedbacks []struct {
			UpdatedAt time.Time `bson:"updatedAt"`
			Feedback  struct {
				TeacherRating   int    `bson:"teacherRating"`
				TeacherFeedback string `bson:"teacherFeedback"`
			} `bson:"feedback"`
			Student struct {
				GeneralInfo struct {
					FirstName transl.Localizable[string] `bson:"firstName"`
					LastName  transl.Localizable[string] `bson:"lastName"`
				} `bson:"generalInfo"`
			} `bson:"student"`
		}
		if err := cur.All(ctx, &feedbacks); err != nil {
			return err
		}

		for _, f := range feedbacks {
			dashboard.RecentFeedbacks = append(dashboard.RecentFeedbacks, content_models.RecentFeedback{
				StudentName: f.Student.GeneralInfo.FirstName.GetContentByLang("ar") + " " + f.Student.GeneralInfo.LastName.GetContentByLang("ar"),
				Rating:      f.Feedback.TeacherRating,
				Comment:     f.Feedback.TeacherFeedback,
				Date:        f.UpdatedAt,
			})
		}
		return nil
	})

	return dashboard, nil
}

// Helper to get supervisor scope from matching Courses and Coachings
type supervisorContentScope struct {
	courses   []content_models.Course
	coachings []content_models.Coaching
}

func (s *supervisorSvcs) getSupervisorContentScope(ctx context.Context, supervisorId primitive.ObjectID) (*supervisorContentScope, error) {
	// 1. Get Courses where supervisor is in basicInfo.supervisorIds
	coursePipeline := []bson.M{
		{"$match": bson.M{
			"basicInfo.supervisorIds": supervisorId,
			"trash":                   false,
		}},
	}
	var courses []content_models.Course
	err := s.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	// 2. Get Coachings where supervisor is in supervisorIds
	coachingPipeline := []bson.M{
		{"$match": bson.M{
			"supervisorIds": supervisorId,
			"trash":         false,
		}},
	}
	var coachings []content_models.Coaching
	err = s.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachings)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	return &supervisorContentScope{
		courses:   courses,
		coachings: coachings,
	}, nil
}

// getSupervisorScope fetches supervisor assignments from SupervisorAssignment collection.
func (s *supervisorSvcs) getSupervisorScope(ctx context.Context, supervisorId primitive.ObjectID, statusFilter ...string) (*supervisorScope, error) {
	// Fetch all assignments for this supervisor from the CONTENT service repo
	match := bson.M{
		"supervisorId": supervisorId,
		"trash":        false,
	}

	if len(statusFilter) > 0 {
		match["status"] = statusFilter[0]
	}

	pipeline := []bson.M{
		{"$match": match},
	}
	var assignments []content_models.SupervisorAssignment
	err := s.supervisorAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil {
		return nil, err
	}

	scope := &supervisorScope{
		directTeacherIds: []primitive.ObjectID{},
		dynamicCriteria:  []dynamicScope{},
	}

	for _, assignment := range assignments {
		// If RefType == "teacher" -> Direct assignment
		// If RefType == "subject" -> Dynamic assignment

		if assignment.RefType == "teacher" {
			scope.directTeacherIds = append(scope.directTeacherIds, assignment.RefIds...)
		} else if assignment.RefType == "subject" {
			scope.dynamicCriteria = append(scope.dynamicCriteria, dynamicScope{
				schoolType: assignment.SchoolType,
				countries:  assignment.Country,
				levels:     assignment.Level,
				subjectIds: assignment.RefIds,
			})
		}
	}

	return scope, nil
}

// resolveTeachersFromScope finds all unique teachers that fall under the supervisor's scope
func (s *supervisorSvcs) resolveTeachersFromScope(ctx context.Context, scope *supervisorScope) ([]primitive.ObjectID, error) {
	teacherSet := make(map[primitive.ObjectID]bool)

	// 1. Add direct teachers
	for _, id := range scope.directTeacherIds {
		teacherSet[id] = true
	}

	// 2. Find teachers from dynamic criteria (Courses and Coachings)
	if len(scope.dynamicCriteria) > 0 {
		// Build filters for Courses and Coachings
		var courseFilters []bson.M
		var coachingFilters []bson.M

		for _, criteria := range scope.dynamicCriteria {
			// Base filter
			baseFilter := bson.M{
				"trash": false,
			}
			if criteria.schoolType != "" {
				baseFilter["basicInfo.schoolType"] = criteria.schoolType // Course field
			}
			if len(criteria.countries) > 0 {
				baseFilter["basicInfo.country"] = bson.M{"$in": criteria.countries}
			}
			if len(criteria.levels) > 0 {
				baseFilter["basicInfo.level"] = bson.M{"$in": criteria.levels}
			}
			if len(criteria.subjectIds) > 0 {
				baseFilter["basicInfo.linkedSubjects"] = bson.M{"$in": criteria.subjectIds}
			}
			courseFilters = append(courseFilters, baseFilter)

			// Coaching filter (fields are at root)
			coachingFilter := bson.M{
				"trash": false,
			}
			if criteria.schoolType != "" {
				coachingFilter["schoolType"] = criteria.schoolType
			}
			if len(criteria.countries) > 0 {
				coachingFilter["country"] = bson.M{"$in": criteria.countries}
			}
			if len(criteria.levels) > 0 {
				coachingFilter["level"] = bson.M{"$in": criteria.levels}
			}
			if len(criteria.subjectIds) > 0 {
				coachingFilter["subject"] = bson.M{"$in": criteria.subjectIds} // Coaching has single subject matches any of the list
			}
			coachingFilters = append(coachingFilters, coachingFilter)
		}

		// Execute Course query and get teachers using TeacherAssignmentSvcs
		if len(courseFilters) > 0 {
			coursePipeline := []bson.M{
				{"$match": bson.M{"$or": courseFilters}},
				{"$project": bson.M{"_id": 1}},
			}
			var courses []content_models.Course
			err := s.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &courses)
			})
			if err != nil {
				return nil, err
			}

			// Get teachers for these courses using batch lookup
			if len(courses) > 0 {
				courseIds := make([]primitive.ObjectID, len(courses))
				for i, c := range courses {
					courseIds[i] = c.Id
				}
				courseTeachers, err := s.teacherAssignmentHelper.BatchGetCourseTeachers(ctx, courseIds)
				if err == nil {
					for _, teacherIds := range courseTeachers {
						for _, tid := range teacherIds {
							teacherSet[tid] = true
						}
					}
				}
			}
		}

		// Execute Coaching query and get teachers using TeacherAssignmentSvcs
		if len(coachingFilters) > 0 {
			coachingPipeline := []bson.M{
				{"$match": bson.M{"$or": coachingFilters}},
				{"$project": bson.M{"_id": 1}},
			}
			var coachings []content_models.Coaching
			err := s.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &coachings)
			})
			if err != nil {
				return nil, err
			}

			// Get teachers for these coachings using batch lookup
			if len(coachings) > 0 {
				coachingIds := make([]primitive.ObjectID, len(coachings))
				for i, c := range coachings {
					coachingIds[i] = c.Id
				}
				coachingTeachers, err := s.teacherAssignmentHelper.BatchGetCoachingTeachers(ctx, coachingIds)
				if err == nil {
					for _, teacherIds := range coachingTeachers {
						for _, tid := range teacherIds {
							teacherSet[tid] = true
						}
					}
				}
			}
		}
	}

	result := make([]primitive.ObjectID, 0, len(teacherSet))
	for id := range teacherSet {
		result = append(result, id)
	}

	return result, nil
}

// GetTeachersExtended gets teachers separated into direct and indirect based on supervisor assignments
func (s *supervisorSvcs) GetTeachersExtended(ctx context.Context, id string, query *query.Conditions) (*models.SupervisorTeachersExtended, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result := &models.SupervisorTeachersExtended{
		DirectTeachers:   []models.Teacher{},
		IndirectTeachers: []models.Teacher{},
		DirectCount:      0,
		IndirectCount:    0,
	}

	// Get supervisor scope to separate direct and indirect
	scope, err := s.getSupervisorScope(ctx, _id)
	if err != nil {
		return nil, err
	}

	// Apply filter if provided
	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err == nil {
			f, err := query.ConvertToMongo()
			if err == nil {
				filter = f
			}
		}
	}

	// Get direct teachers
	if len(scope.directTeacherIds) > 0 {
		directPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": scope.directTeacherIds},
				"trash": false,
			}},
		}
		if filter != nil {
			directPipeline = append(directPipeline, bson.M{"$match": filter})
		}
		directPipeline = append(directPipeline, bson.M{"$sort": bson.M{"_id": -1}})

		err = s.teacherRepo.Aggregate(ctx, directPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result.DirectTeachers)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		result.DirectCount = len(result.DirectTeachers)
	}

	// Get indirect teachers (from courses/coachings)
	indirectTeacherIds, err := s.resolveTeachersFromScope(ctx, scope)
	if err != nil {
		return nil, err
	}

	// Remove direct teachers from indirect list
	indirectSet := make(map[primitive.ObjectID]bool)
	for _, tid := range indirectTeacherIds {
		indirectSet[tid] = true
	}
	for _, tid := range scope.directTeacherIds {
		delete(indirectSet, tid)
	}

	indirectIds := make([]primitive.ObjectID, 0, len(indirectSet))
	for id := range indirectSet {
		indirectIds = append(indirectIds, id)
	}

	if len(indirectIds) > 0 {
		indirectPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": indirectIds},
				"trash": false,
			}},
		}
		if filter != nil {
			indirectPipeline = append(indirectPipeline, bson.M{"$match": filter})
		}
		indirectPipeline = append(indirectPipeline, bson.M{"$sort": bson.M{"_id": -1}})

		err = s.teacherRepo.Aggregate(ctx, indirectPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result.IndirectTeachers)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}
		result.IndirectCount = len(result.IndirectTeachers)
	}

	return result, nil
}

// GetStudentsExtended gets students from courses and coachings that have this supervisor
func (s *supervisorSvcs) GetStudentsExtended(ctx context.Context, id string, query *query.Conditions) (*models.SupervisorStudentsExtended, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	result := &models.SupervisorStudentsExtended{
		Students: []models.Student{},
		Count:    0,
	}

	// Get courses and coachings with this supervisor
	scope, err := s.getSupervisorContentScope(ctx, _id)
	if err != nil {
		return nil, err
	}

	uniqueStudentIds := make(map[primitive.ObjectID]bool)

	// Get students from courses
	if len(scope.courses) > 0 {
		courseIds := make([]primitive.ObjectID, len(scope.courses))
		for i, c := range scope.courses {
			courseIds[i] = c.Id
		}

		studentCoursePipeline := []bson.M{
			{"$match": bson.M{
				"courseId": bson.M{"$in": courseIds},
				"trash":    false,
			}},
			{"$group": bson.M{"_id": "$studentId"}},
		}
		var studentCourses []struct {
			StudentId primitive.ObjectID `bson:"_id"`
		}
		err = s.studentCourseRepo.Aggregate(ctx, studentCoursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentCourses)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		for _, sc := range studentCourses {
			uniqueStudentIds[sc.StudentId] = true
		}
	}

	// Get students from coachings
	if len(scope.coachings) > 0 {
		coachingIds := make([]primitive.ObjectID, len(scope.coachings))
		for i, c := range scope.coachings {
			coachingIds[i] = c.Id
		}

		studentSessionPipeline := []bson.M{
			{"$match": bson.M{
				"coachingProgramId": bson.M{"$in": coachingIds},
				"trash":             false,
			}},
			{"$group": bson.M{"_id": "$studentId"}},
		}
		var studentSessions []struct {
			StudentId primitive.ObjectID `bson:"_id"`
		}
		err = s.studentSessionRepo.Aggregate(ctx, studentSessionPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &studentSessions)
		})
		if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
			return nil, err
		}

		for _, ss := range studentSessions {
			uniqueStudentIds[ss.StudentId] = true
		}
	}

	// Convert to slice and load full student objects
	studentIdsSlice := make([]primitive.ObjectID, 0, len(uniqueStudentIds))
	for id := range uniqueStudentIds {
		studentIdsSlice = append(studentIdsSlice, id)
	}

	if len(studentIdsSlice) > 0 {
		studentsPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": studentIdsSlice},
				"trash": false,
			}},
		}

		// Apply filter if provided
		if query != nil {
			if err := query.CheckValid(); err == nil {
				filter, err := query.ConvertToMongo()
				if err == nil {
					studentsPipeline = append(studentsPipeline, bson.M{"$match": filter})
				}
			}
		}

		studentsPipeline = append(studentsPipeline, bson.M{"$sort": bson.M{"_id": -1}})

		err = s.studentRepo.Aggregate(ctx, studentsPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result.Students)
		})
		if err != nil {
			return nil, err
		}
	}

	result.Count = len(result.Students)
	return result, nil
}

type supervisorScope struct {
	directTeacherIds []primitive.ObjectID
	dynamicCriteria  []dynamicScope
}

type dynamicScope struct {
	schoolType string
	countries  []string
	levels     []string
	subjectIds []primitive.ObjectID
}
