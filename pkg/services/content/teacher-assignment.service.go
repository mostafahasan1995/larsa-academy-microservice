package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"log"
	"math"
	"sync"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// TeacherAssignmentSvcs defines the interface for teacher assignment operations
type TeacherAssignmentSvcs interface {
	// CRUD operations
	List(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherAssignmentPagination, error)
	Get(ctx context.Context, id string) (*models.TeacherAssignmentWithDetails, error)
	BulkCreateOrUpdate(ctx context.Context, data *models.BulkTeacherAssignmentDto) error
	Update(ctx context.Context, id string, data *models.TeacherAssignmentDto) (*models.TeacherAssignment, error)
	Delete(ctx context.Context, id string) error

	// Statistics
	GetStats(ctx context.Context) (*models.TeacherAssignmentStats, error)

	// Auto-creation helpers
	CreateFromCourse(ctx context.Context, courseId primitive.ObjectID, teacherIds []primitive.ObjectID) error
	CreateFromCoaching(ctx context.Context, coachingId primitive.ObjectID, coachIds []primitive.ObjectID) error

	// Query helpers
	GetCourseTeachers(ctx context.Context, courseId primitive.ObjectID) ([]primitive.ObjectID, error)
	GetCoachingTeachers(ctx context.Context, coachingId primitive.ObjectID) ([]primitive.ObjectID, error)
	BatchGetCourseTeachers(ctx context.Context, courseIds []primitive.ObjectID) (map[primitive.ObjectID][]primitive.ObjectID, error)
	BatchGetCoachingTeachers(ctx context.Context, coachingIds []primitive.ObjectID) (map[primitive.ObjectID][]primitive.ObjectID, error)
	GetCoursesByTeacher(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error)
	GetCoachingsByTeacher(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error)
}

type teacherAssignmentSvcs struct {
	i                       *do.Injector
	teacherAssignmentRepo   db.MainRepo[models.TeacherAssignment]
	courseRepo              db.MainRepo[models.Course]
	coachingRepo            db.MainRepo[models.Coaching]
	teacherRepo             db.MainRepo[contactmodels.Teacher]
	subjectRepo             db.MainRepo[models.Subject]
	supervisorAssignmentSvc SupervisorAssignmentSvcs
	initSupervisorSvc       sync.Once
}

func NewTeacherAssignmentSvcs(i *do.Injector) (TeacherAssignmentSvcs, error) {
	return &teacherAssignmentSvcs{
		i:                     i,
		teacherAssignmentRepo: db.NewRepo[models.TeacherAssignment](i),
		courseRepo:            db.NewRepo[models.Course](i),
		coachingRepo:          db.NewRepo[models.Coaching](i),
		teacherRepo:           db.NewRepo[contactmodels.Teacher](i),
		subjectRepo:           db.NewRepo[models.Subject](i),
	}, nil
}

func (t *teacherAssignmentSvcs) getSupervisorSvc() SupervisorAssignmentSvcs {
	t.initSupervisorSvc.Do(func() {
		svc, err := NewSupervisorAssignmentSvcs(t.i)
		if err != nil {
			log.Printf("Failed to initialize supervisor assignment service: %v", err)
			return
		}
		t.supervisorAssignmentSvc = svc
	})
	return t.supervisorAssignmentSvc
}

// convertPrimitiveDToM converts BSON primitive.D (and nested D/A) to bson.M so that
// JSON encoding produces a normal object instead of [{"Key":"...","Value":...}].
func convertPrimitiveDToM(v interface{}) interface{} {
	switch x := v.(type) {
	case primitive.D:
		m := make(bson.M)
		for _, e := range x {
			m[e.Key] = convertPrimitiveDToM(e.Value)
		}
		return m
	case primitive.A:
		sl := make([]interface{}, len(x))
		for i, item := range x {
			sl[i] = convertPrimitiveDToM(item)
		}
		return sl
	default:
		return v
	}
}

func (t *teacherAssignmentSvcs) validateTeacherIds(ctx context.Context, ids []primitive.ObjectID) error {
	if len(ids) == 0 {
		return nil
	}

	var teachers []contactmodels.Teacher
	pipeline := []bson.M{{"$match": bson.M{"_id": bson.M{"$in": ids}, "trash": false}}}
	if err := t.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error { return cur.All(ctx, &teachers) }); err != nil {
		return err
	}
	if len(teachers) != len(ids) {
		found := make(map[primitive.ObjectID]bool)
		for _, th := range teachers {
			found[th.Id] = true
		}
		var missing []string
		for _, id := range ids {
			if !found[id] {
				missing = append(missing, id.Hex())
			}
		}
		return fmt.Errorf("invalid teacher ids: %v", missing)
	}
	return nil
}

// List retrieves teacher assignments with pagination and filters
func (t *teacherAssignmentSvcs) List(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherAssignmentPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	// Build aggregation pipeline with population
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
		{
			"$lookup": bson.M{
				"from":         "academyTeachers",
				"localField":   "teacherIds",
				"foreignField": "_id",
				"as":           "teacherObjs",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academySubjects",
				"localField":   "subjectIds",
				"foreignField": "_id",
				"as":           "subjectObjs",
			},
		},
		// Lookup course by targetId (non-trashed only)
		{
			"$lookup": bson.M{
				"from": "academyCourses",
				"let":  bson.M{"tid": "$targetId"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": bson.A{
									bson.M{"$eq": bson.A{"$_id", "$$tid"}},
									bson.M{"$eq": bson.A{"$trash", false}},
								},
							},
						},
					},
					{"$limit": 1},
				},
				"as": "courseLookup",
			},
		},
		// Lookup coaching by targetId (non-trashed only)
		{
			"$lookup": bson.M{
				"from": "academyCoachings",
				"let":  bson.M{"tid": "$targetId"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": bson.A{
									bson.M{"$eq": bson.A{"$_id", "$$tid"}},
									bson.M{"$eq": bson.A{"$trash", false}},
								},
							},
						},
					},
					{"$limit": 1},
				},
				"as": "coachingLookup",
			},
		},
		// Set targetObj from course or coaching based on type
		{
			"$addFields": bson.M{
				"targetObj": bson.M{
					"$cond": bson.A{
						bson.M{"$eq": bson.A{"$type", "course"}},
						bson.M{"$arrayElemAt": bson.A{"$courseLookup", 0}},
						bson.M{"$arrayElemAt": bson.A{"$coachingLookup", 0}},
					},
				},
			},
		},
		// Drop temporary lookup arrays from output
		{
			"$project": bson.M{
				"courseLookup":   0,
				"coachingLookup": 0,
			},
		},
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
	err = t.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return &models.TeacherAssignmentPagination{
			Assignments: []models.TeacherAssignmentWithDetails{},
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

	assignments := make([]models.TeacherAssignmentWithDetails, 0, len(data))
	for _, item := range data {
		var assignment models.TeacherAssignmentWithDetails
		bsonBytes, _ := bson.Marshal(item)
		bson.Unmarshal(bsonBytes, &assignment)

		// Populate teacher objects (query directly to ensure proper struct unmarshaling)
		if len(assignment.TeacherIds) > 0 {
			teachersPipeline := []bson.M{
				{"$match": bson.M{"_id": bson.M{"$in": assignment.TeacherIds}, "trash": false}},
			}
			var teachers []contactmodels.Teacher
			t.teacherRepo.Aggregate(ctx, teachersPipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &teachers)
			})
			teacherObjs := make([]interface{}, len(teachers))
			for i := range teachers {
				teacherObjs[i] = teachers[i]
			}
			assignment.TeacherObjs = teacherObjs
		}

		// targetObj is already set by aggregation pipeline; convert from primitive.D
		// to bson.M so JSON encoding produces a normal object, not [{"Key","Value"}].
		if assignment.TargetObj != nil {
			assignment.TargetObj = convertPrimitiveDToM(assignment.TargetObj)
		}

		// Populate subject objects
		if len(assignment.SubjectIds) > 0 {
			subjectsPipeline := []bson.M{
				{"$match": bson.M{"_id": bson.M{"$in": assignment.SubjectIds}, "trash": false}},
			}
			var subjects []models.Subject
			t.subjectRepo.Aggregate(ctx, subjectsPipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &subjects)
			})
			subjectObjs := make([]interface{}, len(subjects))
			for i := range subjects {
				subjectObjs[i] = subjects[i]
			}
			assignment.SubjectObjs = subjectObjs
		}

		assignments = append(assignments, assignment)
	}

	totalPage := int64(math.Ceil(float64(totalCount) / float64(limit)))

	return &models.TeacherAssignmentPagination{
		Assignments: assignments,
		Pagination: types.Pagination{
			TotalPages: float64(totalPage),
			PerPage:    limit,
			TotalCount: totalCount,
		},
	}, nil
}

// Get retrieves a single teacher assignment with populated details
func (t *teacherAssignmentSvcs) Get(ctx context.Context, id string) (*models.TeacherAssignmentWithDetails, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	assignment, err := t.teacherAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	result := &models.TeacherAssignmentWithDetails{
		TeacherAssignment: *assignment,
	}

	// Populate teachers
	if len(assignment.TeacherIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": assignment.TeacherIds}, "trash": false}},
		}
		var teachers []contactmodels.Teacher
		t.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teachers)
		})
		teacherObjs := make([]interface{}, len(teachers))
		for i := range teachers {
			teacherObjs[i] = teachers[i]
		}
		result.TeacherObjs = teacherObjs
	}

	// Populate subjects
	if len(assignment.SubjectIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": assignment.SubjectIds}, "trash": false}},
		}
		var subjects []interface{}
		t.subjectRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &subjects)
		})
		result.SubjectObjs = subjects
	}

	// Populate target object
	if assignment.Type == "course" {
		course, _ := t.courseRepo.GetByFilter(ctx, bson.M{"_id": assignment.TargetId, "trash": false})
		if course != nil {
			result.TargetObj = course
		}
	} else if assignment.Type == "coaching" {
		coaching, _ := t.coachingRepo.GetByFilter(ctx, bson.M{"_id": assignment.TargetId, "trash": false})
		if coaching != nil {
			result.TargetObj = coaching
		}
	}

	return result, nil
}

// BulkCreateOrUpdate handles bulk create or update operations
func (t *teacherAssignmentSvcs) BulkCreateOrUpdate(ctx context.Context, data *models.BulkTeacherAssignmentDto) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Extract subject IDs from course or coaching
	var subjectIds []primitive.ObjectID
	if data.Type == "course" {
		course, err := t.courseRepo.GetByFilter(ctx, bson.M{"_id": data.TargetId, "trash": false})
		if err != nil {
			return fmt.Errorf("course not found: %v", err)
		}
		subjectIds = course.BasicInfo.LinkedSubjects
	} else if data.Type == "coaching" {
		coaching, err := t.coachingRepo.GetByFilter(ctx, bson.M{"_id": data.TargetId, "trash": false})
		if err != nil {
			return fmt.Errorf("coaching not found: %v", err)
		}
		subjectIds = []primitive.ObjectID{coaching.Subject}
	} else {
		return errors.New("invalid type: must be 'course' or 'coaching'")
	}

	// Check if assignment already exists
	existing, _ := t.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
		"type":     data.Type,
		"targetId": data.TargetId,
		"trash":    false,
	})

	status := data.Status
	if status == "" {
		status = "assigned"
	}

	// Validate teacher ids before update
	if err := t.validateTeacherIds(ctx, data.TeacherIds); err != nil {
		return err
	}

	if existing != nil {
		// Update existing
		oldTeacherIds := existing.TeacherIds
		update := bson.M{
			"$set": bson.M{
				"teacherIds": data.TeacherIds,
				"subjectIds": subjectIds,
				"status":     status,
				"updatedAt":  time.Now(),
				"updatedBy":  cfg.User.Id,
			},
		}
		_, err = t.teacherAssignmentRepo.Patch(ctx, bson.M{"_id": existing.Id}, update)
		if err != nil {
			return err
		}

		// Update course or coaching document directly
		if data.Type == "course" {
			courseUpdate := bson.M{
				"$set": bson.M{
					"basicInfo.teachers": data.TeacherIds,
					"updatedAt":          time.Now(),
					"updatedBy":          cfg.User.Id,
				},
			}
			_, err = t.courseRepo.Patch(ctx, bson.M{"_id": data.TargetId, "trash": false}, courseUpdate)
			if err != nil {
				log.Printf("Failed to sync teachers to course: %v", err)
			}
		} else if data.Type == "coaching" {
			coachingUpdate := bson.M{
				"$set": bson.M{
					"coachs":    data.TeacherIds,
					"updatedAt": time.Now(),
					"updatedBy": cfg.User.Id,
				},
			}
			_, err = t.coachingRepo.Patch(ctx, bson.M{"_id": data.TargetId, "trash": false}, coachingUpdate)
			if err != nil {
				log.Printf("Failed to sync coaches to coaching: %v", err)
			}
		}

		// Trigger supervisor cascade for bulk update
		if svc := t.getSupervisorSvc(); svc != nil {
			oldTeacherMap := make(map[primitive.ObjectID]bool)
			for _, tid := range oldTeacherIds {
				oldTeacherMap[tid] = true
			}

			addedTeachers := make([]primitive.ObjectID, 0)
			removedTeachers := make([]primitive.ObjectID, 0)

			for _, newTid := range data.TeacherIds {
				if !oldTeacherMap[newTid] {
					addedTeachers = append(addedTeachers, newTid)
				}
			}
			for _, oldTid := range oldTeacherIds {
				if !oldTeacherMap[oldTid] {
					removedTeachers = append(removedTeachers, oldTid)
				}
			}

			if data.Type == "course" {
				if len(addedTeachers) > 0 {
					if err := svc.CascadeFromTeachersToCourses(ctx, data.TargetId, addedTeachers); err != nil {
						log.Printf("Failed to cascade supervisors for added teachers to course: %v", err)
					}
				}
				if len(removedTeachers) > 0 {
					if err := svc.RemoveSupervisorsOfRemovedTeachers(ctx, data.TargetId, removedTeachers); err != nil {
						log.Printf("Failed to remove supervisors of removed teachers from course: %v", err)
					}
				}
			} else if data.Type == "coaching" {
				if len(addedTeachers) > 0 {
					if err := svc.CascadeFromTeachersToCoachings(ctx, data.TargetId, addedTeachers); err != nil {
						log.Printf("Failed to cascade supervisors for added teachers to coaching: %v", err)
					}
				}
				if len(removedTeachers) > 0 {
					if err := svc.RemoveSupervisorsOfRemovedTeachersFromCoaching(ctx, data.TargetId, removedTeachers); err != nil {
						log.Printf("Failed to remove supervisors of removed teachers from coaching: %v", err)
					}
				}
			}
		}

		return nil
	}

	// Create new
	assignment := &models.TeacherAssignment{
		Id: primitive.NewObjectID(),
		TeacherAssignmentDto: models.TeacherAssignmentDto{
			Type:       data.Type,
			TargetId:   data.TargetId,
			TeacherIds: data.TeacherIds,
			SubjectIds: subjectIds,
			Status:     status,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: cfg.User.Id,
		UpdatedBy: cfg.User.Id,
	}

	// Validate teacher ids before create
	if err := t.validateTeacherIds(ctx, assignment.TeacherIds); err != nil {
		return err
	}

	if err := t.teacherAssignmentRepo.Add(ctx, assignment); err != nil {
		return err
	}

	// Update course or coaching document directly for new assignment
	if len(data.TeacherIds) > 0 {
		if data.Type == "course" {
			courseUpdate := bson.M{
				"$set": bson.M{
					"basicInfo.teachers": data.TeacherIds,
					"updatedAt":          time.Now(),
					"updatedBy":          cfg.User.Id,
				},
			}
			_, err = t.courseRepo.Patch(ctx, bson.M{"_id": data.TargetId, "trash": false}, courseUpdate)
			if err != nil {
				log.Printf("Failed to sync teachers to course: %v", err)
			}
		} else if data.Type == "coaching" {
			coachingUpdate := bson.M{
				"$set": bson.M{
					"coachs":    data.TeacherIds,
					"updatedAt": time.Now(),
					"updatedBy": cfg.User.Id,
				},
			}
			_, err = t.coachingRepo.Patch(ctx, bson.M{"_id": data.TargetId, "trash": false}, coachingUpdate)
			if err != nil {
				log.Printf("Failed to sync coaches to coaching: %v", err)
			}
		}
	}

	// Trigger supervisor cascade for new assignment
	if svc := t.getSupervisorSvc(); svc != nil && len(data.TeacherIds) > 0 {
		if data.Type == "course" {
			if err := svc.CascadeFromTeachersToCourses(ctx, data.TargetId, data.TeacherIds); err != nil {
				log.Printf("Failed to cascade supervisors for new teachers to course: %v", err)
			}
		} else if data.Type == "coaching" {
			if err := svc.CascadeFromTeachersToCoachings(ctx, data.TargetId, data.TeacherIds); err != nil {
				log.Printf("Failed to cascade supervisors for new teachers to coaching: %v", err)
			}
		}
	}

	return nil
}

// Update updates a teacher assignment
func (t *teacherAssignmentSvcs) Update(ctx context.Context, id string, data *models.TeacherAssignmentDto) (*models.TeacherAssignment, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid assignment id")
	}

	// Get existing assignment to check type
	existing, err := t.teacherAssignmentRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate teacher ids before update
	if err := t.validateTeacherIds(ctx, data.TeacherIds); err != nil {
		return nil, err
	}

	update := bson.M{
		"$set": bson.M{
			"teacherIds": data.TeacherIds,
			"subjectIds": data.SubjectIds,
			"status":     data.Status,
			"updatedAt":  time.Now(),
			"updatedBy":  cfg.User.Id,
		},
	}

	updated, err := t.teacherAssignmentRepo.Patch(ctx, bson.M{"_id": _id, "trash": false}, update)
	if err != nil {
		return nil, err
	}

	// Sync changes back to course or coaching
	if existing.Type == "course" {
		// Update course teachers
		courseUpdate := bson.M{
			"$set": bson.M{
				"basicInfo.teachers": data.TeacherIds,
				"updatedAt":          time.Now(),
				"updatedBy":          cfg.User.Id,
			},
		}
		_, err = t.courseRepo.Patch(ctx, bson.M{"_id": existing.TargetId, "trash": false}, courseUpdate)
		if err != nil {
			log.Printf("Failed to sync teachers to course: %v", err)
		}

		// Cascade supervisor sync - supervisors of added teachers should be added to course
		// Supervisors of removed teachers should be removed if they only had access through those teachers
		if svc := t.getSupervisorSvc(); svc != nil {
			// Find added and removed teachers
			oldTeacherMap := make(map[primitive.ObjectID]bool)
			for _, tid := range existing.TeacherIds {
				oldTeacherMap[tid] = true
			}

			addedTeachers := make([]primitive.ObjectID, 0)
			removedTeachers := make([]primitive.ObjectID, 0)

			for _, newTid := range data.TeacherIds {
				if !oldTeacherMap[newTid] {
					addedTeachers = append(addedTeachers, newTid)
				}
			}
			for _, oldTid := range existing.TeacherIds {
				if !oldTeacherMap[oldTid] {
					removedTeachers = append(removedTeachers, oldTid)
				}
			}

			// Add supervisors of new teachers to the course
			if len(addedTeachers) > 0 {
				if err := svc.CascadeFromTeachersToCourses(ctx, existing.TargetId, addedTeachers); err != nil {
					log.Printf("Failed to cascade supervisors for added teachers to course: %v", err)
				}
			}

			// Remove supervisors who only had access through removed teachers
			if len(removedTeachers) > 0 {
				if err := svc.RemoveSupervisorsOfRemovedTeachers(ctx, existing.TargetId, removedTeachers); err != nil {
					log.Printf("Failed to remove supervisors of removed teachers from course: %v", err)
				}
			}
		}
	} else if existing.Type == "coaching" {
		// Update coaching coaches
		coachingUpdate := bson.M{
			"$set": bson.M{
				"coachs":    data.TeacherIds,
				"updatedAt": time.Now(),
				"updatedBy": cfg.User.Id,
			},
		}
		_, err = t.coachingRepo.Patch(ctx, bson.M{"_id": existing.TargetId, "trash": false}, coachingUpdate)
		if err != nil {
			log.Printf("Failed to sync coaches to coaching: %v", err)
		}

		// Cascade supervisor sync for coaching
		if svc := t.getSupervisorSvc(); svc != nil {
			oldTeacherMap := make(map[primitive.ObjectID]bool)
			for _, tid := range existing.TeacherIds {
				oldTeacherMap[tid] = true
			}

			addedTeachers := make([]primitive.ObjectID, 0)
			removedTeachers := make([]primitive.ObjectID, 0)

			for _, newTid := range data.TeacherIds {
				if !oldTeacherMap[newTid] {
					addedTeachers = append(addedTeachers, newTid)
				}
			}
			for _, oldTid := range existing.TeacherIds {
				if !oldTeacherMap[oldTid] {
					removedTeachers = append(removedTeachers, oldTid)
				}
			}

			if len(addedTeachers) > 0 {
				if err := svc.CascadeFromTeachersToCoachings(ctx, existing.TargetId, addedTeachers); err != nil {
					log.Printf("Failed to cascade supervisors for added teachers to coaching: %v", err)
				}
			}

			if len(removedTeachers) > 0 {
				if err := svc.RemoveSupervisorsOfRemovedTeachersFromCoaching(ctx, existing.TargetId, removedTeachers); err != nil {
					log.Printf("Failed to remove supervisors of removed teachers from coaching: %v", err)
				}
			}
		}
	}

	return updated, nil
}

// Delete soft deletes a teacher assignment
func (t *teacherAssignmentSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid assignment id")
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

	_, err = t.teacherAssignmentRepo.Patch(ctx, bson.M{"_id": _id}, update)
	return err
}

// GetStats calculates statistics for the dashboard
func (t *teacherAssignmentSvcs) GetStats(ctx context.Context) (*models.TeacherAssignmentStats, error) {
	stats := &models.TeacherAssignmentStats{}

	// Count total courses
	totalCourses, _ := t.teacherAssignmentRepo.Count(ctx, bson.M{"trash": false, "type": "course"})
	stats.TotalCourses = int(totalCourses)

	// Count total coaching programs
	totalCoaching, _ := t.teacherAssignmentRepo.Count(ctx, bson.M{"trash": false, "type": "coaching"})
	stats.TotalCoaching = int(totalCoaching)

	// Count assigned courses
	assignedCourses, _ := t.teacherAssignmentRepo.Count(ctx, bson.M{
		"type":   "course",
		"status": "assigned",
		"trash":  false,
	})
	stats.AssignedCourses = int(assignedCourses)

	// Count assigned coaching
	assignedCoaching, _ := t.teacherAssignmentRepo.Count(ctx, bson.M{
		"type":   "coaching",
		"status": "assigned",
		"trash":  false,
	})
	stats.AssignedCoaching = int(assignedCoaching)

	// Calculate waiting for assignment
	stats.WaitingForAssignment = stats.TotalCourses - stats.AssignedCourses
	stats.WaitingCoachingAssignment = stats.TotalCoaching - stats.AssignedCoaching

	return stats, nil
}

// CreateFromCourse auto-creates assignment when a course is created
func (t *teacherAssignmentSvcs) CreateFromCourse(ctx context.Context, courseId primitive.ObjectID, teacherIds []primitive.ObjectID) error {
	// Get course to extract subject IDs
	course, err := t.courseRepo.GetByFilter(ctx, bson.M{"_id": courseId, "trash": false})
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		// If we can't get user context, use system user
		log.Printf("Warning: Could not get user context for auto-assignment: %v", err)
	}

	// Determine status based on whether teachers are assigned
	status := "pending"
	if len(teacherIds) > 0 {
		status = "assigned"
	}

	assignment := &models.TeacherAssignment{
		Id: primitive.NewObjectID(),
		TeacherAssignmentDto: models.TeacherAssignmentDto{
			Type:       "course",
			TargetId:   courseId,
			TeacherIds: teacherIds,
			SubjectIds: course.BasicInfo.LinkedSubjects,
			SchoolType: course.BasicInfo.SchoolType,
			Level:      course.BasicInfo.Level,
			Country:    course.BasicInfo.Country,
			Status:     status,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if cfg != nil {
		assignment.CreatedBy = cfg.User.Id
		assignment.UpdatedBy = cfg.User.Id
	}

	// CreateFromCourse validates and adds assignment
	if err := t.validateTeacherIds(ctx, assignment.TeacherIds); err != nil {
		return err
	}

	return t.teacherAssignmentRepo.Add(ctx, assignment)
}

// CreateFromCoaching auto-creates assignment when a coaching program is created
func (t *teacherAssignmentSvcs) CreateFromCoaching(ctx context.Context, coachingId primitive.ObjectID, coachIds []primitive.ObjectID) error {
	// Get coaching to extract subject ID
	coaching, err := t.coachingRepo.GetByFilter(ctx, bson.M{"_id": coachingId, "trash": false})
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		log.Printf("Warning: Could not get user context for auto-assignment: %v", err)
	}

	// Determine status based on whether coaches are assigned
	status := "pending"
	if len(coachIds) > 0 {
		status = "assigned"
	}

	assignment := &models.TeacherAssignment{
		Id: primitive.NewObjectID(),
		TeacherAssignmentDto: models.TeacherAssignmentDto{
			Type:       "coaching",
			TargetId:   coachingId,
			TeacherIds: coachIds,
			SubjectIds: []primitive.ObjectID{coaching.Subject},
			SchoolType: coaching.SchoolType,
			Level:      coaching.Level,
			Country:    coaching.Country,
			Status:     status,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if cfg != nil {
		assignment.CreatedBy = cfg.User.Id
		assignment.UpdatedBy = cfg.User.Id
	}

	// Validate teacher ids before create
	if err := t.validateTeacherIds(ctx, assignment.TeacherIds); err != nil {
		return err
	}

	return t.teacherAssignmentRepo.Add(ctx, assignment)
}

// GetCourseTeachers gets teacher IDs for a course from TeacherAssignment
func (t *teacherAssignmentSvcs) GetCourseTeachers(ctx context.Context, courseId primitive.ObjectID) ([]primitive.ObjectID, error) {
	assignment, err := t.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
		"type":     "course",
		"targetId": courseId,
		"trash":    false,
	})
	if err != nil {
		// If no assignment found, return empty slice (not an error)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []primitive.ObjectID{}, nil
		}
		return nil, err
	}
	return assignment.TeacherIds, nil
}

// GetCoachingTeachers gets teacher IDs for a coaching from TeacherAssignment
func (t *teacherAssignmentSvcs) GetCoachingTeachers(ctx context.Context, coachingId primitive.ObjectID) ([]primitive.ObjectID, error) {
	assignment, err := t.teacherAssignmentRepo.GetByFilter(ctx, bson.M{
		"type":     "coaching",
		"targetId": coachingId,
		"trash":    false,
	})
	if err != nil {
		// If no assignment found, return empty slice (not an error)
		if errors.Is(err, mongo.ErrNoDocuments) {
			return []primitive.ObjectID{}, nil
		}
		return nil, err
	}
	return assignment.TeacherIds, nil
}

// BatchGetCourseTeachers gets teacher IDs for multiple courses in batch
func (t *teacherAssignmentSvcs) BatchGetCourseTeachers(ctx context.Context, courseIds []primitive.ObjectID) (map[primitive.ObjectID][]primitive.ObjectID, error) {
	result := make(map[primitive.ObjectID][]primitive.ObjectID)
	if len(courseIds) == 0 {
		return result, nil
	}

	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":     "course",
			"targetId": bson.M{"$in": courseIds},
			"trash":    false,
		}},
	}
	if err := t.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	for _, assignment := range assignments {
		result[assignment.TargetId] = assignment.TeacherIds
	}

	// Initialize empty slices for courses without assignments
	for _, courseId := range courseIds {
		if _, exists := result[courseId]; !exists {
			result[courseId] = []primitive.ObjectID{}
		}
	}

	return result, nil
}

// BatchGetCoachingTeachers gets teacher IDs for multiple coachings in batch
func (t *teacherAssignmentSvcs) BatchGetCoachingTeachers(ctx context.Context, coachingIds []primitive.ObjectID) (map[primitive.ObjectID][]primitive.ObjectID, error) {
	result := make(map[primitive.ObjectID][]primitive.ObjectID)
	if len(coachingIds) == 0 {
		return result, nil
	}

	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":     "coaching",
			"targetId": bson.M{"$in": coachingIds},
			"trash":    false,
		}},
	}
	if err := t.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	for _, assignment := range assignments {
		result[assignment.TargetId] = assignment.TeacherIds
	}

	// Initialize empty slices for coachings without assignments
	for _, coachingId := range coachingIds {
		if _, exists := result[coachingId]; !exists {
			result[coachingId] = []primitive.ObjectID{}
		}
	}

	return result, nil
}

// GetCoursesByTeacher gets all course IDs where a teacher is assigned
func (t *teacherAssignmentSvcs) GetCoursesByTeacher(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error) {
	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "course",
			"teacherIds": teacherId,
			"trash":      false,
		}},
	}
	if err := t.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	courseIds := make([]primitive.ObjectID, len(assignments))
	for i, assignment := range assignments {
		courseIds[i] = assignment.TargetId
	}

	return courseIds, nil
}

// GetCoachingsByTeacher gets all coaching IDs where a teacher is assigned (uses TeacherAssignment)
func (t *teacherAssignmentSvcs) GetCoachingsByTeacher(ctx context.Context, teacherId primitive.ObjectID) ([]primitive.ObjectID, error) {
	var assignments []models.TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{
			"type":       "coaching",
			"teacherIds": teacherId,
			"trash":      false,
		}},
	}
	if err := t.teacherAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	coachingIds := make([]primitive.ObjectID, len(assignments))
	for i, assignment := range assignments {
		coachingIds[i] = assignment.TargetId
	}

	return coachingIds, nil
}
