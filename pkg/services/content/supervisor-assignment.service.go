package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"log"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// SupervisorAssignmentSvcs defines interface for supervisor assignment operations
type SupervisorAssignmentSvcs interface {
	List(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SupervisorAssignmentPagination, error)
	Get(ctx context.Context, id string) (*models.SupervisorAssignmentWithDetails, error)
	Create(ctx context.Context, data *models.SupervisorAssignmentDto) (*models.SupervisorAssignment, error)
	Update(ctx context.Context, id string, data *models.SupervisorAssignmentDto) (*models.SupervisorAssignment, error)
	Delete(ctx context.Context, id string) error
	CheckSupervisorAssignment(ctx context.Context, schoolType, country, level string, subjectIds []primitive.ObjectID) ([]primitive.ObjectID, error)
	GetMatchedTeachers(ctx context.Context, data *models.SupervisorAssignmentDto) ([]contactmodels.Teacher, error)
	CascadeFromTeachersToCourses(ctx context.Context, courseId primitive.ObjectID, teacherIds []primitive.ObjectID) error
	CascadeFromTeachersToCoachings(ctx context.Context, coachingId primitive.ObjectID, teacherIds []primitive.ObjectID) error
	RemoveSupervisorsOfRemovedTeachers(ctx context.Context, courseId primitive.ObjectID, removedTeacherIds []primitive.ObjectID) error
	RemoveSupervisorsOfRemovedTeachersFromCoaching(ctx context.Context, coachingId primitive.ObjectID, removedTeacherIds []primitive.ObjectID) error
}

type supervisorAssignmentSvcs struct {
	i                        *do.Injector
	supervisorAssignmentRepo db.MainRepo[models.SupervisorAssignment]
	supervisorRepo           db.MainRepo[contactmodels.Supervisor]
	teacherRepo              db.MainRepo[contactmodels.Teacher]
	subjectRepo              db.MainRepo[models.Subject]
	courseRepo               db.MainRepo[models.Course]
	coachingRepo             db.MainRepo[models.Coaching]
	teacherAssignmentRepo    db.MainRepo[models.TeacherAssignment]
	teacherAssignmentSvc     TeacherAssignmentSvcs
}

func NewSupervisorAssignmentSvcs(i *do.Injector) (SupervisorAssignmentSvcs, error) {
	teacherAssignmentSvc, err := NewTeacherAssignmentSvcs(i)
	if err != nil {
		return nil, err
	}
	return &supervisorAssignmentSvcs{
		i:                        i,
		supervisorAssignmentRepo: db.NewRepo[models.SupervisorAssignment](i),
		supervisorRepo:           db.NewRepo[contactmodels.Supervisor](i),
		teacherRepo:              db.NewRepo[contactmodels.Teacher](i),
		subjectRepo:              db.NewRepo[models.Subject](i),
		courseRepo:               db.NewRepo[models.Course](i),
		coachingRepo:             db.NewRepo[models.Coaching](i),
		teacherAssignmentRepo:    db.NewRepo[models.TeacherAssignment](i),
		teacherAssignmentSvc:     teacherAssignmentSvc,
	}, nil
}

// List retrieves supervisor assignments with pagination
func (s *supervisorAssignmentSvcs) List(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SupervisorAssignmentPagination, error) {
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
		{
			"$facet": bson.M{
				"data": []bson.M{
					{"$skip": skip},
					{"$limit": limit},
				},
				"totalCount": []bson.M{
					{"$count": "count"},
				},
			},
		},
	}

	var result []bson.M
	err = s.supervisorAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return &models.SupervisorAssignmentPagination{
			Assignments: []models.SupervisorAssignmentWithDetails{},
			Pagination: types.Pagination{
				TotalPages: 0,
				PerPage:    limit,
				TotalCount: 0,
			},
		}, nil
	}

	data := result[0]["data"].(primitive.A)
	totalCount := int64(0)
	if len(result[0]["totalCount"].(primitive.A)) > 0 {
		totalCount = int64(result[0]["totalCount"].(primitive.A)[0].(bson.M)["count"].(int32))
	}

	assignments := make([]models.SupervisorAssignmentWithDetails, 0, len(data))
	for _, item := range data {
		var assignment models.SupervisorAssignmentWithDetails
		bsonBytes, _ := bson.Marshal(item)
		bson.Unmarshal(bsonBytes, &assignment)

		// Populate supervisor
		supervisor, _ := s.supervisorRepo.GetByFilter(ctx, bson.M{"_id": assignment.SupervisorId, "trash": false})
		if supervisor != nil {
			assignment.SupervisorObj = supervisor
		}

		// Populate refObjs based on refType
		if assignment.RefType == string(enums.SupervisorRefTypeTeacher) {
			pipeline := []bson.M{
				{"$match": bson.M{"_id": bson.M{"$in": assignment.RefIds}, "trash": false}},
			}
			var teachers []interface{}
			s.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &teachers)
			})
			assignment.RefObjs = teachers
		} else if assignment.RefType == string(enums.SupervisorRefTypeSubject) {
			pipeline := []bson.M{
				{"$match": bson.M{"_id": bson.M{"$in": assignment.RefIds}, "trash": false}},
			}
			var subjects []interface{}
			s.subjectRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &subjects)
			})
			assignment.RefObjs = subjects

			// For dynamic assignments, count affected teachers
			if assignment.AssignType == string(enums.SupervisorAssignmentTypeDynamic) {
				count, _ := s.teacherRepo.Count(ctx, bson.M{
					"subjectIds": bson.M{"$in": assignment.RefIds},
					"trash":      false,
				})
				assignment.AffectedTeachers = int(count)
			}
		}

		assignments = append(assignments, assignment)
	}

	totalPage := int64(math.Ceil(float64(totalCount) / float64(limit)))

	return &models.SupervisorAssignmentPagination{
		Assignments: assignments,
		Pagination: types.Pagination{
			TotalPages: float64(totalPage),
			PerPage:    limit,
			TotalCount: totalCount,
		},
	}, nil
}

// Get retrieves assignment with populated details
func (s *supervisorAssignmentSvcs) Get(ctx context.Context, id string) (*models.SupervisorAssignmentWithDetails, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	assignment, err := s.supervisorAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	result := &models.SupervisorAssignmentWithDetails{
		SupervisorAssignment: *assignment,
	}

	// Populate supervisor
	supervisor, _ := s.supervisorRepo.GetByFilter(ctx, bson.M{"_id": assignment.SupervisorId, "trash": false})
	if supervisor != nil {
		result.SupervisorObj = supervisor
	}

	// Populate refObjs based on refType
	if assignment.RefType == string(enums.SupervisorRefTypeTeacher) {
		pipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": assignment.RefIds}, "trash": false}},
		}
		var teachers []interface{}
		s.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teachers)
		})
		result.RefObjs = teachers
	} else if assignment.RefType == string(enums.SupervisorRefTypeSubject) {
		pipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": assignment.RefIds}, "trash": false}},
		}
		var subjects []interface{}
		s.subjectRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &subjects)
		})
		result.RefObjs = subjects

		// For dynamic, count affected teachers
		if assignment.AssignType == string(enums.SupervisorAssignmentTypeDynamic) {
			count, _ := s.teacherRepo.Count(ctx, bson.M{
				"subjectIds": bson.M{"$in": assignment.RefIds},
				"trash":      false,
			})
			result.AffectedTeachers = int(count)
		}
	}

	return result, nil
}

// calculateMatchedTeachers finds all teachers assigned to courses/coachings matching the assignment criteria
// Only works for dynamic subject-based assignments
func (s *supervisorAssignmentSvcs) calculateMatchedTeachers(ctx context.Context, assignment *models.SupervisorAssignment) ([]primitive.ObjectID, error) {
	// Only calculate for dynamic subject assignments
	if assignment.AssignType != string(enums.SupervisorAssignmentTypeDynamic) || assignment.RefType != string(enums.SupervisorRefTypeSubject) {
		return []primitive.ObjectID{}, nil
	}

	teacherSet := make(map[primitive.ObjectID]bool)

	// Build course filters matching assignment criteria
	var courseFilters []bson.M
	for _, country := range assignment.Country {
		for _, level := range assignment.Level {
			courseFilter := bson.M{
				"trash": false,
			}
			if assignment.SchoolType != "" {
				courseFilter["basicInfo.schoolType"] = assignment.SchoolType
			}
			courseFilter["basicInfo.country"] = country
			courseFilter["basicInfo.gradeLevel"] = level
			if len(assignment.RefIds) > 0 {
				courseFilter["basicInfo.linkedSubjects"] = bson.M{"$in": assignment.RefIds}
			}
			courseFilters = append(courseFilters, courseFilter)
		}
	}

	// Build coaching filters matching assignment criteria
	var coachingFilters []bson.M
	for _, country := range assignment.Country {
		for _, level := range assignment.Level {
			coachingFilter := bson.M{
				"trash": false,
			}
			if assignment.SchoolType != "" {
				coachingFilter["schoolType"] = assignment.SchoolType
			}
			coachingFilter["country"] = country
			coachingFilter["class"] = level
			if len(assignment.RefIds) > 0 {
				coachingFilter["subject"] = bson.M{"$in": assignment.RefIds}
			}
			coachingFilters = append(coachingFilters, coachingFilter)
		}
	}

	// Get matching courses and their teachers
	if len(courseFilters) > 0 {
		coursePipeline := []bson.M{
			{"$match": bson.M{"$or": courseFilters}},
		}
		var courses []models.Course
		err := s.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courses)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}

		if len(courses) > 0 {
			courseIds := make([]primitive.ObjectID, len(courses))
			for i, c := range courses {
				courseIds[i] = c.Id
			}
			courseTeachers, err := s.teacherAssignmentSvc.BatchGetCourseTeachers(ctx, courseIds)
			if err == nil {
				for _, teacherIds := range courseTeachers {
					for _, tid := range teacherIds {
						teacherSet[tid] = true
					}
				}
			}
		}
	}

	// Get matching coachings and their teachers
	if len(coachingFilters) > 0 {
		coachingPipeline := []bson.M{
			{"$match": bson.M{"$or": coachingFilters}},
		}
		var coachings []models.Coaching
		err := s.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachings)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}

		if len(coachings) > 0 {
			coachingIds := make([]primitive.ObjectID, len(coachings))
			for i, c := range coachings {
				coachingIds[i] = c.Id
			}
			coachingTeachers, err := s.teacherAssignmentSvc.BatchGetCoachingTeachers(ctx, coachingIds)
			if err == nil {
				for _, teacherIds := range coachingTeachers {
					for _, tid := range teacherIds {
						teacherSet[tid] = true
					}
				}
			}
		}
	}

	// Convert set to slice
	result := make([]primitive.ObjectID, 0, len(teacherSet))
	for id := range teacherSet {
		result = append(result, id)
	}

	return result, nil
}

// Create creates a supervisor assignment with validation and syncing
func (s *supervisorAssignmentSvcs) Create(ctx context.Context, data *models.SupervisorAssignmentDto) (*models.SupervisorAssignment, error) {
	// Validation: dynamic must be subject, direct must be teacher
	if data.AssignType == string(enums.SupervisorAssignmentTypeDynamic) && data.RefType != string(enums.SupervisorRefTypeSubject) {
		return nil, errors.New("dynamic assignment must have refType='subject'")
	}
	if data.AssignType == string(enums.SupervisorAssignmentTypeDirect) && data.RefType != string(enums.SupervisorRefTypeTeacher) {
		return nil, errors.New("direct assignment must have refType='teacher'")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	status := data.Status
	if status == "" {
		status = "assigned"
	}

	assignment := &models.SupervisorAssignment{
		Id:                      primitive.NewObjectID(),
		SupervisorAssignmentDto: *data,
		CreatedAt:               time.Now(),
		UpdatedAt:               time.Now(),
		CreatedBy:               cfg.User.Id,
		UpdatedBy:               cfg.User.Id,
	}
	assignment.Status = status

	// Calculate matchedTeachers for dynamic subject assignments
	if data.AssignType == string(enums.SupervisorAssignmentTypeDynamic) && data.RefType == string(enums.SupervisorRefTypeSubject) {
		matchedTeachers, err := s.calculateMatchedTeachers(ctx, assignment)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate matched teachers: %v", err)
		}
		assignment.MatchedTeachers = matchedTeachers
	}

	if err := s.supervisorAssignmentRepo.Add(ctx, assignment); err != nil {
		return nil, err
	}

	// Sync to teachers based on type and status
	if assignment.Status == string(enums.SupervisorAssignmentStatusAssigned) {
		if data.AssignType == string(enums.SupervisorAssignmentTypeDynamic) {
			// Dynamic: update all teachers with these subjects
			if err := s.syncDynamicAssignment(ctx, data.SupervisorId, data.RefIds); err != nil {
				return nil, fmt.Errorf("failed to sync dynamic assignment: %v", err)
			}
			// Sync to courses and coachings
			if err := s.syncSupervisorToCourses(ctx, assignment); err != nil {
				return nil, fmt.Errorf("failed to sync supervisor to courses: %v", err)
			}
			if err := s.syncSupervisorToCoachings(ctx, assignment); err != nil {
				return nil, fmt.Errorf("failed to sync supervisor to coachings: %v", err)
			}
		} else {
			// Direct: update specific teachers
			if err := s.syncDirectAssignment(ctx, data.SupervisorId, data.RefIds); err != nil {
				return nil, fmt.Errorf("failed to sync direct assignment: %v", err)
			}
		}
	}

	return assignment, nil
}

// syncDynamicAssignment updates all teachers with the given subjects to have the supervisor
func (s *supervisorAssignmentSvcs) syncDynamicAssignment(ctx context.Context, supervisorId primitive.ObjectID, subjectIds []primitive.ObjectID) error {
	// Find all teachers that have any of these subjects
	pipeline := []bson.M{
		{"$match": bson.M{
			"subjectIds": bson.M{"$in": subjectIds},
			"trash":      false,
		}},
	}

	var teachers []contactmodels.Teacher
	err := s.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teachers)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Update each teacher's subjects array
	for _, teacher := range teachers {
		// Update supervisorId in matching subjects
		for i := range teacher.Subjects {
			subjectObjId, _ := primitive.ObjectIDFromHex(teacher.Subjects[i].SubjectID)
			for _, sid := range subjectIds {
				if subjectObjId == sid {
					teacher.Subjects[i].SupervisorId = supervisorId
				}
			}
		}

		// Update the teacher
		update := bson.M{
			"$set": bson.M{
				"subjects":  teacher.Subjects,
				"updatedAt": time.Now(),
			},
		}
		_, err = s.teacherRepo.Patch(ctx, bson.M{"_id": teacher.Id}, update)
		if err != nil {
			return err
		}
	}

	return nil
}

// syncDirectAssignment updates specific teachers (direct assignment)
// and cascades supervisor to all courses/coachings those teachers are assigned to
func (s *supervisorAssignmentSvcs) syncDirectAssignment(ctx context.Context, supervisorId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	// For direct assignments, we need to cascade supervisor to all courses/coachings
	// that the assigned teachers are part of
	if err := s.syncSupervisorToCoursesFromTeachers(ctx, supervisorId, teacherIds); err != nil {
		return fmt.Errorf("failed to sync supervisor to courses from teachers: %v", err)
	}
	if err := s.syncSupervisorToCoachingsFromTeachers(ctx, supervisorId, teacherIds); err != nil {
		return fmt.Errorf("failed to sync supervisor to coachings from teachers: %v", err)
	}
	return nil
}

// syncSupervisorToCoursesFromTeachers adds a supervisor to courses where the given teachers are assigned
func (s *supervisorAssignmentSvcs) syncSupervisorToCoursesFromTeachers(ctx context.Context, supervisorId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	if len(teacherIds) == 0 {
		return nil
	}

	// Get all courses where these teachers are assigned
	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "course",
			"teacherIds": bson.M{"$in": teacherIds},
			"trash":      false,
		}},
	}
	if err := s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Get unique course IDs
	courseIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range assignments {
		courseIdSet[assignment.TargetId] = true
	}

	// Update each course to add the supervisor
	for courseId := range courseIdSet {
		course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": courseId, "trash": false})
		if err != nil {
			continue
		}

		// Check if supervisor already exists
		supervisorExists := false
		for _, existingId := range course.BasicInfo.SupervisorIds {
			if existingId == supervisorId {
				supervisorExists = true
				break
			}
		}

		if !supervisorExists {
			course.BasicInfo.SupervisorIds = append(course.BasicInfo.SupervisorIds, supervisorId)
			update := bson.M{
				"$set": bson.M{
					"basicInfo.supervisorIds": course.BasicInfo.SupervisorIds,
					"updatedAt":               time.Now(),
				},
			}
			if _, err := s.courseRepo.Patch(ctx, bson.M{"_id": courseId}, update); err != nil {
				log.Printf("Failed to add supervisor %s to course %s: %v", supervisorId.Hex(), courseId.Hex(), err)
			}
		}
	}

	return nil
}

// syncSupervisorToCoachingsFromTeachers adds a supervisor to coachings where the given teachers are assigned
func (s *supervisorAssignmentSvcs) syncSupervisorToCoachingsFromTeachers(ctx context.Context, supervisorId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	if len(teacherIds) == 0 {
		return nil
	}

	// Get all coachings where these teachers are assigned
	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "coaching",
			"teacherIds": bson.M{"$in": teacherIds},
			"trash":      false,
		}},
	}
	if err := s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Get unique coaching IDs
	coachingIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range assignments {
		coachingIdSet[assignment.TargetId] = true
	}

	// Update each coaching to add the supervisor
	for coachingId := range coachingIdSet {
		coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": coachingId, "trash": false})
		if err != nil {
			continue
		}

		// Check if supervisor already exists
		supervisorExists := false
		for _, existingId := range coaching.SupervisorIds {
			if existingId == supervisorId {
				supervisorExists = true
				break
			}
		}

		if !supervisorExists {
			coaching.SupervisorIds = append(coaching.SupervisorIds, supervisorId)
			update := bson.M{
				"$set": bson.M{
					"supervisorIds": coaching.SupervisorIds,
					"updatedAt":     time.Now(),
				},
			}
			if _, err := s.coachingRepo.Patch(ctx, bson.M{"_id": coachingId}, update); err != nil {
				log.Printf("Failed to add supervisor %s to coaching %s: %v", supervisorId.Hex(), coachingId.Hex(), err)
			}
		}
	}

	return nil
}

// removeDirectAssignment removes supervisor from specific teachers
// and cascades removal from all courses/coachings those teachers are assigned to
func (s *supervisorAssignmentSvcs) removeDirectAssignment(ctx context.Context, supervisorId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	// Remove supervisor from all courses where these teachers are assigned
	if err := s.removeSupervisorFromCoursesByTeachers(ctx, supervisorId, teacherIds); err != nil {
		return fmt.Errorf("failed to remove supervisor from courses: %v", err)
	}
	if err := s.removeSupervisorFromCoachingsByTeachers(ctx, supervisorId, teacherIds); err != nil {
		return fmt.Errorf("failed to remove supervisor from coachings: %v", err)
	}
	return nil
}

// removeSupervisorFromCoursesByTeachers removes a supervisor from courses
// only if all teachers on that course have been removed from the supervisor's assignment
func (s *supervisorAssignmentSvcs) removeSupervisorFromCoursesByTeachers(ctx context.Context, supervisorId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	if len(teacherIds) == 0 {
		return nil
	}

	// Get all courses where these teachers are assigned
	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "course",
			"teacherIds": bson.M{"$in": teacherIds},
			"trash":      false,
		}},
	}
	if err := s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Get unique course IDs
	courseIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range assignments {
		courseIdSet[assignment.TargetId] = true
	}

	// For each course, check if any remaining teacher still has this supervisor
	for courseId := range courseIdSet {
		course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": courseId, "trash": false})
		if err != nil {
			continue
		}

		// Check if supervisor is in this course
		supervisorInCourse := false
		for _, sid := range course.BasicInfo.SupervisorIds {
			if sid == supervisorId {
				supervisorInCourse = true
				break
			}
		}
		if !supervisorInCourse {
			continue
		}

		// Check if any of the removed teachers are the ONLY reason this supervisor is on the course
		// Get all teacher assignments for this course
		courseAssignment, err := s.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
			"type":     "course",
			"targetId": courseId,
			"trash":    false,
		})
		if err != nil {
			continue
		}

		// Check if any remaining teacher on this course has this supervisor
		hasOtherTeacherWithSupervisor := false
		for _, teacherId := range courseAssignment.TeacherIds {
			// Skip removed teachers
			removed := false
			for _, rt := range teacherIds {
				if teacherId == rt {
					removed = true
					break
				}
			}
			if removed {
				continue
			}

			// Check if this teacher has this supervisor via direct assignment
			hasSupervisor, err := s.teacherHasDirectSupervisor(ctx, teacherId, supervisorId)
			if err != nil {
				continue
			}
			if hasSupervisor {
				hasOtherTeacherWithSupervisor = true
				break
			}
		}

		// Also check if supervisor should still be on course via dynamic assignment
		shouldRemainViaDynamic, _ := s.checkSupervisorShouldRemainOnCourse(ctx, course, supervisorId)

		if !hasOtherTeacherWithSupervisor && !shouldRemainViaDynamic {
			// Remove supervisor from course
			newSupervisorIds := make([]primitive.ObjectID, 0)
			for _, sid := range course.BasicInfo.SupervisorIds {
				if sid != supervisorId {
					newSupervisorIds = append(newSupervisorIds, sid)
				}
			}
			course.BasicInfo.SupervisorIds = newSupervisorIds

			update := bson.M{
				"$set": bson.M{
					"basicInfo.supervisorIds": course.BasicInfo.SupervisorIds,
					"updatedAt":               time.Now(),
				},
			}
			if _, err := s.courseRepo.Patch(ctx, bson.M{"_id": courseId}, update); err != nil {
				log.Printf("Failed to remove supervisor %s from course %s: %v", supervisorId.Hex(), courseId.Hex(), err)
			}
		}
	}

	return nil
}

// removeSupervisorFromCoachingsByTeachers removes a supervisor from coachings
// only if all teachers on that coaching have been removed from the supervisor's assignment
func (s *supervisorAssignmentSvcs) removeSupervisorFromCoachingsByTeachers(ctx context.Context, supervisorId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	if len(teacherIds) == 0 {
		return nil
	}

	// Get all coachings where these teachers are assigned
	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "coaching",
			"teacherIds": bson.M{"$in": teacherIds},
			"trash":      false,
		}},
	}
	if err := s.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Get unique coaching IDs
	coachingIdSet := make(map[primitive.ObjectID]bool)
	for _, assignment := range assignments {
		coachingIdSet[assignment.TargetId] = true
	}

	// For each coaching, check if supervisor should be removed
	for coachingId := range coachingIdSet {
		coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": coachingId, "trash": false})
		if err != nil {
			continue
		}

		// Check if supervisor is in this coaching
		supervisorInCoaching := false
		for _, sid := range coaching.SupervisorIds {
			if sid == supervisorId {
				supervisorInCoaching = true
				break
			}
		}
		if !supervisorInCoaching {
			continue
		}

		// Check if any remaining teacher on this coaching has this supervisor
		coachingAssignment, err := s.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
			"type":     "coaching",
			"targetId": coachingId,
			"trash":    false,
		})
		if err != nil {
			continue
		}

		hasOtherTeacherWithSupervisor := false
		for _, teacherId := range coachingAssignment.TeacherIds {
			// Skip removed teachers
			removed := false
			for _, rt := range teacherIds {
				if teacherId == rt {
					removed = true
					break
				}
			}
			if removed {
				continue
			}

			hasSupervisor, err := s.teacherHasDirectSupervisor(ctx, teacherId, supervisorId)
			if err != nil {
				continue
			}
			if hasSupervisor {
				hasOtherTeacherWithSupervisor = true
				break
			}
		}

		shouldRemainViaDynamic, _ := s.checkSupervisorShouldRemainOnCoaching(ctx, coaching, supervisorId)

		if !hasOtherTeacherWithSupervisor && !shouldRemainViaDynamic {
			// Remove supervisor from coaching
			newSupervisorIds := make([]primitive.ObjectID, 0)
			for _, sid := range coaching.SupervisorIds {
				if sid != supervisorId {
					newSupervisorIds = append(newSupervisorIds, sid)
				}
			}
			coaching.SupervisorIds = newSupervisorIds

			update := bson.M{
				"$set": bson.M{
					"supervisorIds": coaching.SupervisorIds,
					"updatedAt":     time.Now(),
				},
			}
			if _, err := s.coachingRepo.Patch(ctx, bson.M{"_id": coachingId}, update); err != nil {
				log.Printf("Failed to remove supervisor %s from coaching %s: %v", supervisorId.Hex(), coachingId.Hex(), err)
			}
		}
	}

	return nil
}

// teacherHasDirectSupervisor checks if a teacher has a specific supervisor via direct assignment
func (s *supervisorAssignmentSvcs) teacherHasDirectSupervisor(ctx context.Context, teacherId, supervisorId primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"supervisorId": supervisorId,
		"refType":      string(enums.SupervisorRefTypeTeacher),
		"refIds":       teacherId,
		"status":       string(enums.SupervisorAssignmentStatusAssigned),
		"trash":        false,
	}
	count, err := s.supervisorAssignmentRepo.Count(ctx, filter)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// checkSupervisorShouldRemainOnCourse checks if supervisor should remain on course via dynamic assignment
func (s *supervisorAssignmentSvcs) checkSupervisorShouldRemainOnCourse(ctx context.Context, course *models.Course, supervisorId primitive.ObjectID) (bool, error) {
	// Get all dynamic assignments for this supervisor
	filter := bson.M{
		"supervisorId": supervisorId,
		"assignType":   string(enums.SupervisorAssignmentTypeDynamic),
		"refType":      string(enums.SupervisorRefTypeSubject),
		"status":       string(enums.SupervisorAssignmentStatusAssigned),
		"trash":        false,
	}
	var assignments []models.SupervisorAssignment
	err := s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	// Check if any assignment matches this course
	for _, assignment := range assignments {
		// Check schoolType
		if assignment.SchoolType != "" && assignment.SchoolType != course.BasicInfo.SchoolType {
			continue
		}

		// Check country
		countryMatches := false
		for _, country := range assignment.Country {
			if country == course.BasicInfo.Country {
				countryMatches = true
				break
			}
		}
		if !countryMatches {
			continue
		}

		// Check level
		levelMatches := false
		for _, level := range assignment.Level {
			if level == course.BasicInfo.Level {
				levelMatches = true
				break
			}
		}
		if !levelMatches {
			continue
		}

		// Check subjects - course has linked subjects
		subjectMatches := false
		for _, subjectId := range course.BasicInfo.LinkedSubjects {
			for _, refId := range assignment.RefIds {
				if subjectId == refId {
					subjectMatches = true
					break
				}
			}
			if subjectMatches {
				break
			}
		}

		if subjectMatches {
			return true, nil
		}
	}

	return false, nil
}

// checkSupervisorShouldRemainOnCoaching checks if supervisor should remain on coaching via dynamic assignment
func (s *supervisorAssignmentSvcs) checkSupervisorShouldRemainOnCoaching(ctx context.Context, coaching *models.Coaching, supervisorId primitive.ObjectID) (bool, error) {
	filter := bson.M{
		"supervisorId": supervisorId,
		"assignType":   string(enums.SupervisorAssignmentTypeDynamic),
		"refType":      string(enums.SupervisorRefTypeSubject),
		"status":       string(enums.SupervisorAssignmentStatusAssigned),
		"trash":        false,
	}
	var assignments []models.SupervisorAssignment
	err := s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return false, err
	}

	for _, assignment := range assignments {
		if assignment.SchoolType != "" && assignment.SchoolType != coaching.SchoolType {
			continue
		}

		countryMatches := false
		for _, country := range assignment.Country {
			if country == coaching.Country {
				countryMatches = true
				break
			}
		}
		if !countryMatches {
			continue
		}

		levelMatches := false
		for _, level := range assignment.Level {
			if level == coaching.Level {
				levelMatches = true
				break
			}
		}
		if !levelMatches {
			continue
		}

		// Check subjects - coaching has single subject
		for _, refId := range assignment.RefIds {
			if refId == coaching.Subject {
				return true, nil
			}
		}
	}

	return false, nil
}

// CascadeFromTeachersToCourses adds supervisors of the given teachers to their courses
// This is called when teachers are added to a course
func (s *supervisorAssignmentSvcs) CascadeFromTeachersToCourses(ctx context.Context, courseId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	if len(teacherIds) == 0 {
		return nil
	}

	course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": courseId, "trash": false})
	if err != nil {
		return err
	}

	// Get all supervisors assigned to these teachers (via direct assignment)
	supervisorMap := make(map[primitive.ObjectID]bool)

	filter := bson.M{
		"refType": string(enums.SupervisorRefTypeTeacher),
		"refIds":  bson.M{"$in": teacherIds},
		"status":  string(enums.SupervisorAssignmentStatusAssigned),
		"trash":   false,
	}
	var assignments []models.SupervisorAssignment
	err = s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	for _, assignment := range assignments {
		supervisorMap[assignment.SupervisorId] = true
	}

	// Add these supervisors to the course
	existingSupervisors := course.BasicInfo.SupervisorIds
	updated := false
	for supervisorId := range supervisorMap {
		exists := false
		for _, existingId := range existingSupervisors {
			if existingId == supervisorId {
				exists = true
				break
			}
		}
		if !exists {
			existingSupervisors = append(existingSupervisors, supervisorId)
			updated = true
		}
	}

	if updated {
		update := bson.M{
			"$set": bson.M{
				"basicInfo.supervisorIds": existingSupervisors,
				"updatedAt":               time.Now(),
			},
		}
		_, err = s.courseRepo.Patch(ctx, bson.M{"_id": courseId}, update)
		if err != nil {
			return fmt.Errorf("failed to update course supervisors: %v", err)
		}
	}

	return nil
}

// CascadeFromTeachersToCoachings adds supervisors of the given teachers to their coachings
// This is called when teachers are added to a coaching
func (s *supervisorAssignmentSvcs) CascadeFromTeachersToCoachings(ctx context.Context, coachingId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	if len(teacherIds) == 0 {
		return nil
	}

	coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": coachingId, "trash": false})
	if err != nil {
		return err
	}

	supervisorMap := make(map[primitive.ObjectID]bool)

	filter := bson.M{
		"refType": string(enums.SupervisorRefTypeTeacher),
		"refIds":  bson.M{"$in": teacherIds},
		"status":  string(enums.SupervisorAssignmentStatusAssigned),
		"trash":   false,
	}
	var assignments []models.SupervisorAssignment
	err = s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	for _, assignment := range assignments {
		supervisorMap[assignment.SupervisorId] = true
	}

	existingSupervisors := coaching.SupervisorIds
	updated := false
	for supervisorId := range supervisorMap {
		exists := false
		for _, existingId := range existingSupervisors {
			if existingId == supervisorId {
				exists = true
				break
			}
		}
		if !exists {
			existingSupervisors = append(existingSupervisors, supervisorId)
			updated = true
		}
	}

	if updated {
		update := bson.M{
			"$set": bson.M{
				"supervisorIds": existingSupervisors,
				"updatedAt":     time.Now(),
			},
		}
		_, err = s.coachingRepo.Patch(ctx, bson.M{"_id": coachingId}, update)
		if err != nil {
			return fmt.Errorf("failed to update coaching supervisors: %v", err)
		}
	}

	return nil
}

// RemoveSupervisorsOfRemovedTeachers removes supervisors from a course
// when the teachers they were following are removed
func (s *supervisorAssignmentSvcs) RemoveSupervisorsOfRemovedTeachers(ctx context.Context, courseId primitive.ObjectID, removedTeacherIds []primitive.ObjectID) error {
	if len(removedTeacherIds) == 0 {
		return nil
	}

	course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": courseId, "trash": false})
	if err != nil {
		return err
	}

	// Get the current teacher assignment for this course
	courseAssignment, err := s.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
		"type":     "course",
		"targetId": courseId,
		"trash":    false,
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	remainingTeacherIds := make(map[primitive.ObjectID]bool)
	if courseAssignment != nil {
		for _, tid := range courseAssignment.TeacherIds {
			remainingTeacherIds[tid] = true
		}
	}

	// For each supervisor that might be affected, check if they should be removed
	supervisorsToRemove := make([]primitive.ObjectID, 0)

	for _, supervisorId := range course.BasicInfo.SupervisorIds {
		// Check if this supervisor is ONLY on the course because of the removed teachers
		// Get all direct assignments where this supervisor is assigned to teachers
		filter := bson.M{
			"supervisorId": supervisorId,
			"refType":      string(enums.SupervisorRefTypeTeacher),
			"status":       string(enums.SupervisorAssignmentStatusAssigned),
			"trash":        false,
		}
		var assignments []models.SupervisorAssignment
		err = s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &assignments)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			continue
		}

		// Check if supervisor has any remaining teachers that are on this course
		hasRemainingTeacherWithSupervisor := false
		for _, assignment := range assignments {
			for _, teacherId := range assignment.RefIds {
				// Check if this teacher is on the course
				if remainingTeacherIds[teacherId] {
					hasRemainingTeacherWithSupervisor = true
					break
				}
			}
			if hasRemainingTeacherWithSupervisor {
				break
			}
		}

		if !hasRemainingTeacherWithSupervisor {
			// Check if supervisor should remain via dynamic assignment
			shouldRemain, _ := s.checkSupervisorShouldRemainOnCourse(ctx, course, supervisorId)
			if !shouldRemain {
				supervisorsToRemove = append(supervisorsToRemove, supervisorId)
			}
		}
	}

	// Remove supervisors
	if len(supervisorsToRemove) > 0 {
		newSupervisorIds := make([]primitive.ObjectID, 0)
		removeMap := make(map[primitive.ObjectID]bool)
		for _, sid := range supervisorsToRemove {
			removeMap[sid] = true
		}
		for _, sid := range course.BasicInfo.SupervisorIds {
			if !removeMap[sid] {
				newSupervisorIds = append(newSupervisorIds, sid)
			}
		}

		update := bson.M{
			"$set": bson.M{
				"basicInfo.supervisorIds": newSupervisorIds,
				"updatedAt":               time.Now(),
			},
		}
		_, err = s.courseRepo.Patch(ctx, bson.M{"_id": courseId}, update)
		if err != nil {
			return fmt.Errorf("failed to remove supervisors from course: %v", err)
		}
	}

	return nil
}

// RemoveSupervisorsOfRemovedTeachersFromCoaching removes supervisors from a coaching
// when the teachers they were following are removed
func (s *supervisorAssignmentSvcs) RemoveSupervisorsOfRemovedTeachersFromCoaching(ctx context.Context, coachingId primitive.ObjectID, removedTeacherIds []primitive.ObjectID) error {
	if len(removedTeacherIds) == 0 {
		return nil
	}

	coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": coachingId, "trash": false})
	if err != nil {
		return err
	}

	coachingAssignment, err := s.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
		"type":     "coaching",
		"targetId": coachingId,
		"trash":    false,
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	remainingTeacherIds := make(map[primitive.ObjectID]bool)
	if coachingAssignment != nil {
		for _, tid := range coachingAssignment.TeacherIds {
			remainingTeacherIds[tid] = true
		}
	}

	supervisorsToRemove := make([]primitive.ObjectID, 0)

	for _, supervisorId := range coaching.SupervisorIds {
		filter := bson.M{
			"supervisorId": supervisorId,
			"refType":      string(enums.SupervisorRefTypeTeacher),
			"status":       string(enums.SupervisorAssignmentStatusAssigned),
			"trash":        false,
		}
		var assignments []models.SupervisorAssignment
		err = s.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &assignments)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			continue
		}

		hasRemainingTeacherWithSupervisor := false
		for _, assignment := range assignments {
			for _, teacherId := range assignment.RefIds {
				if remainingTeacherIds[teacherId] {
					hasRemainingTeacherWithSupervisor = true
					break
				}
			}
			if hasRemainingTeacherWithSupervisor {
				break
			}
		}

		if !hasRemainingTeacherWithSupervisor {
			shouldRemain, _ := s.checkSupervisorShouldRemainOnCoaching(ctx, coaching, supervisorId)
			if !shouldRemain {
				supervisorsToRemove = append(supervisorsToRemove, supervisorId)
			}
		}
	}

	if len(supervisorsToRemove) > 0 {
		newSupervisorIds := make([]primitive.ObjectID, 0)
		removeMap := make(map[primitive.ObjectID]bool)
		for _, sid := range supervisorsToRemove {
			removeMap[sid] = true
		}
		for _, sid := range coaching.SupervisorIds {
			if !removeMap[sid] {
				newSupervisorIds = append(newSupervisorIds, sid)
			}
		}

		update := bson.M{
			"$set": bson.M{
				"supervisorIds": newSupervisorIds,
				"updatedAt":     time.Now(),
			},
		}
		_, err = s.coachingRepo.Patch(ctx, bson.M{"_id": coachingId}, update)
		if err != nil {
			return fmt.Errorf("failed to remove supervisors from coaching: %v", err)
		}
	}

	return nil
}

// removeDynamicAssignment removes supervisor from all teachers with the given subjects
func (s *supervisorAssignmentSvcs) removeDynamicAssignment(ctx context.Context, supervisorId primitive.ObjectID, subjectIds []primitive.ObjectID) error {
	// Find all teachers that have any of these subjects and THIS supervisor
	pipeline := []bson.M{
		{"$match": bson.M{
			"subjectIds":            bson.M{"$in": subjectIds},
			"subjects.supervisorId": supervisorId,
			"trash":                 false,
		}},
	}

	var teachers []contactmodels.Teacher
	err := s.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teachers)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Update each teacher's subjects array
	for _, teacher := range teachers {
		updated := false
		// Remove supervisorId in matching subjects
		for i := range teacher.Subjects {
			subjectObjId, _ := primitive.ObjectIDFromHex(teacher.Subjects[i].SubjectID)
			match := false
			for _, sid := range subjectIds {
				if subjectObjId == sid {
					match = true
					break
				}
			}

			if match && teacher.Subjects[i].SupervisorId == supervisorId {
				// We need to check if there are OTHER assignments for this supervisor and subject
				stillCovered, _ := s.isStillAssigned(ctx, supervisorId, subjectObjId, teacher)
				if !stillCovered {
					teacher.Subjects[i].SupervisorId = primitive.NilObjectID
					updated = true
				}
			}
		}

		if updated {
			update := bson.M{
				"$set": bson.M{
					"subjects":  teacher.Subjects,
					"updatedAt": time.Now(),
				},
			}
			_, err = s.teacherRepo.Patch(ctx, bson.M{"_id": teacher.Id}, update)
			if err != nil {
				return err
			}
		}
	}

	return nil
}

// isStillAssigned checks if a supervisor is still assigned to a subject for a teacher via another assignment
func (s *supervisorAssignmentSvcs) isStillAssigned(ctx context.Context, supervisorId, subjectId primitive.ObjectID, teacher contactmodels.Teacher) (bool, error) {
	// Check for other active assignments for this supervisor and subject
	filter := bson.M{
		"supervisorId": supervisorId,
		"status":       string(enums.SupervisorAssignmentStatusAssigned),
		"refType":      string(enums.SupervisorRefTypeSubject),
		"refIds":       subjectId,
		"trash":        false,
	}

	count, err := s.supervisorAssignmentRepo.Count(ctx, filter)
	if err != nil {
		return false, err
	}

	return count > 0, nil
}

// syncSupervisorToCourses updates courses with matching criteria to have the correct supervisor IDs
// This method re-calculates supervisor IDs for all courses based on current assignments
func (s *supervisorAssignmentSvcs) syncSupervisorToCourses(ctx context.Context, assignment *models.SupervisorAssignment) error {
	// Only sync for dynamic assignments with subject refType
	if assignment.RefType != string(enums.SupervisorRefTypeSubject) {
		return nil
	}

	// Get all courses that might match this assignment (have matching subjects)
	pipeline := []bson.M{
		{"$match": bson.M{
			"basicInfo.linkedSubjects": bson.M{"$in": assignment.RefIds},
			"trash":                    false,
		}},
	}

	var courses []models.Course
	err := s.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// For each course, recalculate supervisor IDs based on all matching assignments
	for _, course := range courses {
		// Use CheckSupervisorAssignment to find all matching supervisors
		supervisorIds, err := s.CheckSupervisorAssignment(
			ctx,
			course.BasicInfo.SchoolType,
			course.BasicInfo.Country,
			course.BasicInfo.Level,
			course.BasicInfo.LinkedSubjects,
		)
		if err != nil {
			return fmt.Errorf("failed to check supervisor assignment for course %s: %v", course.Id.Hex(), err)
		}

		// Update course with supervisor IDs
		update := bson.M{
			"$set": bson.M{
				"basicInfo.supervisorIds": supervisorIds,
				"updatedAt":               time.Now(),
			},
		}
		_, err = s.courseRepo.Patch(ctx, bson.M{"_id": course.Id}, update)
		if err != nil {
			return fmt.Errorf("failed to update course %s: %v", course.Id.Hex(), err)
		}
	}

	return nil
}

// syncSupervisorToCoachings updates coachings with matching criteria to have the correct supervisor IDs
// This method re-calculates supervisor IDs for all coachings based on current assignments
func (s *supervisorAssignmentSvcs) syncSupervisorToCoachings(ctx context.Context, assignment *models.SupervisorAssignment) error {
	// Only sync for dynamic assignments with subject refType
	if assignment.RefType != string(enums.SupervisorRefTypeSubject) {
		return nil
	}

	// Get all coachings that might match this assignment (have matching subject)
	pipeline := []bson.M{
		{"$match": bson.M{
			"subject": bson.M{"$in": assignment.RefIds},
			"trash":   false,
		}},
	}

	var coachings []models.Coaching
	err := s.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachings)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// For each coaching, recalculate supervisor IDs based on all matching assignments
	for _, coaching := range coachings {
		// Use CheckSupervisorAssignment to find all matching supervisors
		// Coaching has a single subject, so we pass it as an array
		supervisorIds, err := s.CheckSupervisorAssignment(
			ctx,
			coaching.SchoolType,
			coaching.Country,
			coaching.Level,
			[]primitive.ObjectID{coaching.Subject},
		)
		if err != nil {
			return fmt.Errorf("failed to check supervisor assignment for coaching %s: %v", coaching.Id.Hex(), err)
		}

		// Update coaching with supervisor IDs
		update := bson.M{
			"$set": bson.M{
				"supervisorIds": supervisorIds,
				"updatedAt":     time.Now(),
			},
		}
		_, err = s.coachingRepo.Patch(ctx, bson.M{"_id": coaching.Id}, update)
		if err != nil {
			return fmt.Errorf("failed to update coaching %s: %v", coaching.Id.Hex(), err)
		}
	}

	return nil
}

// syncAllCoursesAndCoachings re-syncs all courses and coachings with supervisor IDs
// This is used when an assignment is deleted to ensure all affected entities are updated
func (s *supervisorAssignmentSvcs) syncAllCoursesAndCoachings(ctx context.Context) error {
	// Get all active dynamic assignments with subject refType
	pipeline := []bson.M{
		{"$match": bson.M{
			"assignType": string(enums.SupervisorAssignmentTypeDynamic),
			"refType":    string(enums.SupervisorRefTypeSubject),
			"status":     "assigned",
			"trash":      false,
		}},
	}

	var assignments []models.SupervisorAssignment
	err := s.supervisorAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	// Collect all unique subject IDs from assignments
	subjectIdMap := make(map[primitive.ObjectID]bool)
	for _, assignment := range assignments {
		for _, subjectId := range assignment.RefIds {
			subjectIdMap[subjectId] = true
		}
	}

	// Sync courses
	coursePipeline := []bson.M{
		{"$match": bson.M{
			"basicInfo.linkedSubjects": bson.M{"$in": getKeysFromMap(subjectIdMap)},
			"trash":                    false,
		}},
	}

	var courses []models.Course
	err = s.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	for _, course := range courses {
		supervisorIds, err := s.CheckSupervisorAssignment(
			ctx,
			course.BasicInfo.SchoolType,
			course.BasicInfo.Country,
			course.BasicInfo.Level,
			course.BasicInfo.LinkedSubjects,
		)
		if err != nil {
			continue // Skip on error, continue with others
		}

		update := bson.M{
			"$set": bson.M{
				"basicInfo.supervisorIds": supervisorIds,
				"updatedAt":               time.Now(),
			},
		}
		s.courseRepo.Patch(ctx, bson.M{"_id": course.Id}, update)
	}

	// Sync coachings
	coachingPipeline := []bson.M{
		{"$match": bson.M{
			"subject": bson.M{"$in": getKeysFromMap(subjectIdMap)},
			"trash":   false,
		}},
	}

	var coachings []models.Coaching
	err = s.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachings)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return err
	}

	for _, coaching := range coachings {
		supervisorIds, err := s.CheckSupervisorAssignment(
			ctx,
			coaching.SchoolType,
			coaching.Country,
			coaching.Level,
			[]primitive.ObjectID{coaching.Subject},
		)
		if err != nil {
			continue // Skip on error, continue with others
		}

		update := bson.M{
			"$set": bson.M{
				"supervisorIds": supervisorIds,
				"updatedAt":     time.Now(),
			},
		}
		s.coachingRepo.Patch(ctx, bson.M{"_id": coaching.Id}, update)
	}

	return nil
}

// Helper function to get keys from map
func getKeysFromMap(m map[primitive.ObjectID]bool) []primitive.ObjectID {
	keys := make([]primitive.ObjectID, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// Update updates a supervisor assignment
func (s *supervisorAssignmentSvcs) Update(ctx context.Context, id string, data *models.SupervisorAssignmentDto) (*models.SupervisorAssignment, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	// Validation
	if data.AssignType == string(enums.SupervisorAssignmentTypeDynamic) && data.RefType != string(enums.SupervisorRefTypeSubject) {
		return nil, errors.New("dynamic assignment must have refType='subject'")
	}
	if data.AssignType == string(enums.SupervisorAssignmentTypeDirect) && data.RefType != string(enums.SupervisorRefTypeTeacher) {
		return nil, errors.New("direct assignment must have refType='teacher'")
	}

	// Get old assignment before updating to sync courses/coachings that matched old criteria
	oldAssignment, err := s.supervisorAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id})
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Calculate matchedTeachers for dynamic subject assignments
	var matchedTeachers []primitive.ObjectID
	if data.AssignType == string(enums.SupervisorAssignmentTypeDynamic) && data.RefType == string(enums.SupervisorRefTypeSubject) {
		tempAssignment := &models.SupervisorAssignment{
			SupervisorAssignmentDto: *data,
		}
		matchedTeachers, err = s.calculateMatchedTeachers(ctx, tempAssignment)
		if err != nil {
			return nil, fmt.Errorf("failed to calculate matched teachers: %v", err)
		}
	}

	update := bson.M{
		"$set": bson.M{
			"supervisorId":    data.SupervisorId,
			"assignType":      data.AssignType,
			"refType":         data.RefType,
			"refIds":          data.RefIds,
			"schoolType":      data.SchoolType,
			"country":         data.Country,
			"level":           data.Level,
			"matchedTeachers": matchedTeachers,
			"status":          data.Status,
			"updatedAt":       time.Now(),
			"updatedBy":       cfg.User.Id,
			"trash":           data.Trash,
		},
	}
	if update["supervisorId"] == "" {
		update["supervisorId"] = oldAssignment.SupervisorId
	}
	if update["assignType"] == "" {
		update["assignType"] = oldAssignment.AssignType
	}
	if update["refType"] == "" {
		update["refType"] = oldAssignment.RefType
	}
	if update["refIds"] == "" {
		update["refIds"] = oldAssignment.RefIds
	}
	if update["schoolType"] == "" {
		update["schoolType"] = oldAssignment.SchoolType
	}
	if update["country"] == "" {
		update["country"] = oldAssignment.Country
	}
	if update["level"] == "" {
		update["level"] = oldAssignment.Level
	}
	if update["matchedTeachers"] == "" {
		update["matchedTeachers"] = oldAssignment.MatchedTeachers
	}
	if update["status"] == "" {
		update["status"] = oldAssignment.Status
	}
	if update["trash"] == "" {
		update["trash"] = oldAssignment.Trash
	}
	updated, err := s.supervisorAssignmentRepo.Patch(ctx, bson.M{"_id": _id}, update)
	if err != nil {
		return nil, err
	}

	// Re-sync based on type
	if updated.Status == string(enums.SupervisorAssignmentStatusAssigned) {
		if updated.AssignType == string(enums.SupervisorAssignmentTypeDynamic) {
			if err := s.syncDynamicAssignment(ctx, updated.SupervisorId, updated.RefIds); err != nil {
				return nil, fmt.Errorf("failed to sync dynamic assignment: %v", err)
			}
			// Sync to courses and coachings matching NEW criteria
			if err := s.syncSupervisorToCourses(ctx, updated); err != nil {
				return nil, fmt.Errorf("failed to sync supervisor to courses: %v", err)
			}
			if err := s.syncSupervisorToCoachings(ctx, updated); err != nil {
				return nil, fmt.Errorf("failed to sync supervisor to coachings: %v", err)
			}
		} else {
			if err := s.syncDirectAssignment(ctx, updated.SupervisorId, updated.RefIds); err != nil {
				return nil, fmt.Errorf("failed to sync direct assignment: %v", err)
			}
		}
	} else {
		// If status is not assigned (e.g. pending or trash), we should remove supervisor from affected teachers/courses/coachings
		if updated.AssignType == string(enums.SupervisorAssignmentTypeDynamic) {
			if err := s.removeDynamicAssignment(ctx, updated.SupervisorId, updated.RefIds); err != nil {
				return nil, fmt.Errorf("failed to remove dynamic assignment: %v", err)
			}
			// Re-sync courses and coachings to remove this supervisor if no other assignment matches
			if err := s.syncSupervisorToCourses(ctx, updated); err != nil {
				return nil, fmt.Errorf("failed to remove supervisor from courses: %v", err)
			}
			if err := s.syncSupervisorToCoachings(ctx, updated); err != nil {
				return nil, fmt.Errorf("failed to remove supervisor from coachings: %v", err)
			}
		} else {
			if err := s.removeDirectAssignment(ctx, updated.SupervisorId, updated.RefIds); err != nil {
				return nil, fmt.Errorf("failed to remove direct assignment: %v", err)
			}
		}
	}

	// Also sync courses/coachings matching OLD criteria (in case criteria or status changed)
	if oldAssignment.AssignType == string(enums.SupervisorAssignmentTypeDynamic) && oldAssignment.RefType == string(enums.SupervisorRefTypeSubject) {
		if err := s.syncSupervisorToCourses(ctx, oldAssignment); err != nil {
			return nil, fmt.Errorf("failed to sync supervisor to courses (old criteria): %v", err)
		}
		if err := s.syncSupervisorToCoachings(ctx, oldAssignment); err != nil {
			return nil, fmt.Errorf("failed to sync supervisor to coachings (old criteria): %v", err)
		}

		// If status changed from assigned to something else, remove supervisor from old teachers
		if oldAssignment.Status == string(enums.SupervisorAssignmentStatusAssigned) && updated.Status != string(enums.SupervisorAssignmentStatusAssigned) {
			if err := s.removeDynamicAssignment(ctx, oldAssignment.SupervisorId, oldAssignment.RefIds); err != nil {
				return nil, fmt.Errorf("failed to remove old dynamic assignment: %v", err)
			}
		}
	} else if oldAssignment.AssignType == string(enums.SupervisorAssignmentTypeDirect) {
		if oldAssignment.Status == string(enums.SupervisorAssignmentStatusAssigned) && updated.Status != string(enums.SupervisorAssignmentStatusAssigned) {
			if err := s.removeDirectAssignment(ctx, oldAssignment.SupervisorId, oldAssignment.RefIds); err != nil {
				return nil, fmt.Errorf("failed to remove old direct assignment: %v", err)
			}
		}
	}

	return updated, nil
}

// Delete soft deletes an assignment
func (s *supervisorAssignmentSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid assignment id")
	}

	// Get assignment before deleting to check if we need to sync
	assignment, err := s.supervisorAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	update := bson.M{
		"$set": bson.M{
			"trash":     true,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	_, err = s.supervisorAssignmentRepo.Patch(ctx, bson.M{"_id": _id}, update)
	if err != nil {
		return err
	}

	// Re-sync all courses and coachings since this assignment is now deleted
	// This ensures supervisor IDs are recalculated without the deleted assignment
	if assignment.AssignType == string(enums.SupervisorAssignmentTypeDynamic) && assignment.RefType == string(enums.SupervisorRefTypeSubject) {
		// If it was assigned, remove supervisor from teachers
		if assignment.Status == string(enums.SupervisorAssignmentStatusAssigned) {
			s.removeDynamicAssignment(ctx, assignment.SupervisorId, assignment.RefIds)
		}

		// First sync courses/coachings that matched the deleted assignment
		if err := s.syncSupervisorToCourses(ctx, assignment); err != nil {
			return fmt.Errorf("failed to re-sync courses after deletion: %v", err)
		}
		if err := s.syncSupervisorToCoachings(ctx, assignment); err != nil {
			return fmt.Errorf("failed to re-sync coachings after deletion: %v", err)
		}
		// Then sync all other courses/coachings that might be affected
		if err := s.syncAllCoursesAndCoachings(ctx); err != nil {
			return fmt.Errorf("failed to re-sync all courses and coachings after deletion: %v", err)
		}
	} else if assignment.AssignType == string(enums.SupervisorAssignmentTypeDirect) && assignment.RefType == string(enums.SupervisorRefTypeTeacher) {
		// For direct teacher assignments, cascade removal to courses/coachings
		if assignment.Status == string(enums.SupervisorAssignmentStatusAssigned) {
			if err := s.removeDirectAssignment(ctx, assignment.SupervisorId, assignment.RefIds); err != nil {
				return fmt.Errorf("failed to remove direct assignment from courses/coachings: %v", err)
			}
		}
	}

	return nil
}

// CheckSupervisorAssignment finds matching supervisors based on schoolType, country, level, and subjects
// It checks all combinations of country and level arrays against supervisor assignments
func (s *supervisorAssignmentSvcs) CheckSupervisorAssignment(ctx context.Context, schoolType, country, level string, subjectIds []primitive.ObjectID) ([]primitive.ObjectID, error) {
	var supervisorIds []primitive.ObjectID
	supervisorMap := make(map[primitive.ObjectID]bool)

	// Get all supervisor assignments with refType = "subject" and status = "assigned"
	pipeline := []bson.M{
		{"$match": bson.M{
			"refType": string(enums.SupervisorRefTypeSubject),
			"status":  "assigned",
			"trash":   false,
		}},
	}

	var assignments []models.SupervisorAssignment
	err := s.supervisorAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Check each subject against all supervisor assignments
	for _, subjectId := range subjectIds {
		for _, assignment := range assignments {
			// Check if schoolType matches
			if assignment.SchoolType != schoolType {
				continue
			}

			// Check if this subjectId is in the assignment's refIds
			subjectMatches := false
			for _, refId := range assignment.RefIds {
				if refId == subjectId {
					subjectMatches = true
					break
				}
			}
			if !subjectMatches {
				continue
			}

			// Check all combinations of country and level
			// If country matches any in the array AND level matches any in the array
			countryMatches := false
			for _, assignCountry := range assignment.Country {
				if assignCountry == country {
					countryMatches = true
					break
				}
			}

			levelMatches := false
			for _, assignLevel := range assignment.Level {
				if assignLevel == level {
					levelMatches = true
					break
				}
			}

			// If all conditions match, add supervisor
			if countryMatches && levelMatches {
				// Use map to avoid duplicates
				if !supervisorMap[assignment.SupervisorId] {
					supervisorMap[assignment.SupervisorId] = true
					supervisorIds = append(supervisorIds, assignment.SupervisorId)
				}
			}
		}
	}

	return supervisorIds, nil
}

// GetMatchedTeachers finds all teachers matching the criteria in the DTO
func (s *supervisorAssignmentSvcs) GetMatchedTeachers(ctx context.Context, data *models.SupervisorAssignmentDto) ([]contactmodels.Teacher, error) {
	// Assignment type is always dynamic for this operation as requested
	data.AssignType = string(enums.SupervisorAssignmentTypeDynamic)
	data.RefType = string(enums.SupervisorRefTypeSubject)

	tempAssignment := &models.SupervisorAssignment{
		SupervisorAssignmentDto: *data,
	}

	teacherIds, err := s.calculateMatchedTeachers(ctx, tempAssignment)
	if err != nil {
		return nil, err
	}

	if len(teacherIds) == 0 {
		return []contactmodels.Teacher{}, nil
	}

	// Fetch teacher details
	pipeline := []bson.M{
		{"$match": bson.M{"_id": bson.M{"$in": teacherIds}, "trash": false}},
	}

	var teachers []contactmodels.Teacher
	err = s.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teachers)
	})
	if err != nil {
		return nil, err
	}

	return teachers, nil
}
