package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/helpers"
	querypkg "larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/messaging"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"log"
	"math"
	"net/http"
	"strconv"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CourseSvcs defines the interface for course-related operations
type CourseSvcs interface {
	// Core course operations
	GetById(ctx context.Context, id string) (*models.EnrichedCourse, error)
	Get(ctx context.Context, skip, limit int64, query *querypkg.Conditions, enrichSubjects bool, includeStudentCount bool, enrichTeachers bool, enrichPrice bool) (*models.CoursePagination, error)
	GetAll(ctx context.Context, query *querypkg.Conditions) ([]models.EnrichedCourse, error)
	Add(ctx context.Context, data *models.CourseDto) (*models.Course, error)
	Update(ctx context.Context, id string, data *models.CourseDto) (*models.Course, error)
	Delete(ctx context.Context, id string) error

	// Course summary operations
	GetAllCourseSummaries(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseSummary, error)
	GetCourseSummary(ctx context.Context, courseId, summaryId string) (*models.CourseSummary, error)
	AddCourseSummary(ctx context.Context, courseId string, data *models.CourseSummaryDto) (*models.CourseSummary, error)
	UpdateCourseSummary(ctx context.Context, courseId, summaryId string, data *models.CourseSummaryDto) (*models.CourseSummary, error)
	DeleteCourseSummary(ctx context.Context, courseId, summaryId string) error

	// Course test operations
	GetAllCourseTests(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseTest, error)
	GetCourseTest(ctx context.Context, courseId, testId string) (*models.CourseTest, error)
	AddCourseTest(ctx context.Context, courseId string, data *models.CourseTestDto) (*models.CourseTest, error)
	UpdateCourseTest(ctx context.Context, courseId, testId string, data *models.CourseTestDto) (*models.CourseTest, error)
	DeleteCourseTest(ctx context.Context, courseId, testId string) error

	// Course FAQ operations
	GetAllCourseFaqs(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseFaq, error)
	GetCourseFaq(ctx context.Context, courseId, faqId string) (*models.CourseFaq, error)
	AddCourseFaq(ctx context.Context, courseId string, data *models.CourseFaqDto) (*models.CourseFaq, error)
	UpdateCourseFaq(ctx context.Context, courseId, faqId string, data *models.CourseFaqDto) (*models.CourseFaq, error)
	DeleteCourseFaq(ctx context.Context, courseId, faqId string) error

	// Course assignment operations
	GetAllCourseAssignments(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseAssignment, error)
	GetCourseAssignment(ctx context.Context, courseId, assignmentId string) (*models.CourseAssignment, error)
	AddCourseAssignment(ctx context.Context, courseId string, data *models.CourseAssignmentDto) (*models.CourseAssignment, error)
	UpdateCourseAssignment(ctx context.Context, courseId, assignmentId string, data *models.CourseAssignmentDto) (*models.CourseAssignment, error)
	DeleteCourseAssignment(ctx context.Context, courseId, assignmentId string) error

	// Course study note operations
	GetAllCourseStudyNotes(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseStudyNote, error)
	AddCourseStudyNote(ctx context.Context, courseId string, data *models.CourseStudyNoteDto) (*models.CourseStudyNote, error)
	UpdateCourseStudyNote(ctx context.Context, courseId, studyNoteId string, data *models.CourseStudyNoteDto) (*models.CourseStudyNote, error)
	DeleteCourseStudyNote(ctx context.Context, courseId, studyNoteId string) error

	// Course study note category operations
	GetAllCourseStudyNoteCategories(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseStudyNoteCategory, error)
	AddCourseStudyNoteCategory(ctx context.Context, courseId string, data *models.CourseStudyNoteCategoryDto) (*models.CourseStudyNoteCategory, error)
	UpdateCourseStudyNoteCategory(ctx context.Context, courseId, categoryId string, data *models.CourseStudyNoteCategoryDto) (*models.CourseStudyNoteCategory, error)
	DeleteCourseStudyNoteCategory(ctx context.Context, courseId, categoryId string) error

	// Course flashcard operations
	GetAllCourseFlashcards(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseFlashcard, error)
	AddCourseFlashcard(ctx context.Context, courseId string, data *models.CourseFlashcardDto) (*models.CourseFlashcard, error)
	UpdateCourseFlashcard(ctx context.Context, courseId, flashcardId string, data *models.CourseFlashcardDto) (*models.CourseFlashcard, error)
	DeleteCourseFlashcard(ctx context.Context, courseId, flashcardId string) error

	// Course flashcard category operations
	GetAllCourseFlashcardCategories(ctx context.Context, courseId string, query *querypkg.Conditions) ([]models.CourseFlashcardCategory, error)
	AddCourseFlashcardCategory(ctx context.Context, courseId string, data *models.CourseFlashcardCategoryDto) (*models.CourseFlashcardCategory, error)
	UpdateCourseFlashcardCategory(ctx context.Context, courseId, categoryId string, data *models.CourseFlashcardCategoryDto) (*models.CourseFlashcardCategory, error)
	DeleteCourseFlashcardCategory(ctx context.Context, courseId, categoryId string) error

	// Course category operations (tags)
	GetAllCourseCategories(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.CourseCategoryPagination, error)
	GetCourseCategory(ctx context.Context, id string) (*models.CourseCategory, error)
	AddCourseCategory(ctx context.Context, data *models.CourseCategoryDto) (*models.CourseCategory, error)
	UpdateCourseCategory(ctx context.Context, id string, data *models.CourseCategoryDto) (*models.CourseCategory, error)
	DeleteCourseCategory(ctx context.Context, id string) error

	//student assignments
	SubmitStudentCourseAssignment(ctx context.Context, assignmentId string, data *models.CourseStudentAssignmentDto) (*models.CourseStudentAssignment, error)
	GetAllStudentCourseAssignment(ctx context.Context, query *querypkg.Conditions) ([]models.CourseStudentAssignment, error)
	GetCourseAssignmentsForStudent(ctx context.Context, courseId string) ([]models.CourseAssignmentWithAttempt, error)
	GradeCourseStudentAssignment(ctx context.Context, studentAssignmentId string, data *models.GradeCourseStudentAssignmentDto) (*models.CourseStudentAssignment, error)
	GetCourseAssignmentStudentSubmissions(ctx context.Context, courseId, assignmentId string, query *querypkg.Conditions) ([]models.CourseStudentAssignmentRes, error)
	GetAllCourseAssignmentSubmissions(ctx context.Context, assignmentId string, query *querypkg.Conditions) ([]models.CourseStudentAssignmentRes, error)
	DeleteCourseStudentAssignment(ctx context.Context, submissionId, courseId string) error

	//student tests
	StartCourseStudentTest(ctx context.Context, testId string) (*models.CourseStudentTest, error)
	SubmitCourseStudentTest(ctx context.Context, studentTestId string, data *models.StudentCourseTestAnswersDto) (*models.CourseStudentTest, error)
	GetCourseStudentTest(ctx context.Context, studentTestId string) (*models.CourseStudentTest, error)
	GetAllCourseStudentTests(ctx context.Context, testId string, query *querypkg.Conditions) ([]models.CourseStudentTest, error)
	GetCourseTestsForStudent(ctx context.Context, courseId string) ([]models.CourseTestWithAttempt, error)
	GetCourseTestStudentSubmissions(ctx context.Context, courseId, testId string, query *querypkg.Conditions) ([]models.CourseStudentTestRes, error)
	DeleteCourseStudentTest(ctx context.Context, submissionId, courseId string) error

	//student courses
	GetAllCourseMembers(ctx context.Context, courseId string, skip, limit int64, query *querypkg.Conditions) (*models.MemberCoursePagination, error)
	GetEnrolledCourses(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions) (*models.EnrolledCoursePagination, error)
	GetEnrolledBundles(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions) (*models.EnrolledBundlePagination, error)
	CanViewStudentCourseData(ctx context.Context, studentId string) (bool, error)

	// Course pricing operations
	GetCoursePricing(ctx context.Context, courseId string, skip, limit int64) (*models.CoursePricingPagination, error)
	AddCoursePricing(ctx context.Context, courseId string, data *models.CoursePricingDto) (*models.CoursePricing, error)
	UpdateCoursePricing(ctx context.Context, courseId, pricingId string, data *models.CoursePricingDto) (*models.CoursePricing, error)
	DeleteCoursePricing(ctx context.Context, courseId, pricingId string) error

	// Calendar event management - Live session
	UpdateCourseLiveSessionTime(ctx context.Context, courseId string, schedule time.Time) (*models.Course, error)
	DeleteCourseLiveSession(ctx context.Context, courseId string) error

	// Course statistics
	GetCourseSummaryStats(ctx context.Context, courseId string) (*models.CourseSummaryStats, error)

	// Student facing list operations
	GetStudentList(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions, enrichSubjects bool, includeStudentCount bool, enrichTeachers bool, enrichPrice bool) (*models.StudentCoursePagination, error)
	MarkLessonAsCompleted(ctx context.Context, courseId, lessonId string) (*models.EnrolledCourseResponse, error)
	GetEnrolledCourseById(ctx context.Context, id string) (*models.StudentCourseWithDetails, error)
	GetStudentCourseStats(ctx context.Context, studentId string) (*models.StudentCourseStatsResponse, error)

	// Teacher facing operations
	GetTeacherCourses(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.TeacherCoursePagination, error)
	GetTeacherCourseMembers(ctx context.Context, courseId string, skip, limit int64, query *querypkg.Conditions) (*models.MemberCoursePagination, error)
}

// coursesvcs is the implementation of CourseSvcs interface
type coursesvcs struct {
	i                           *do.Injector
	authSvc                     CourseAuthService
	repo                        db.MainRepo[models.Course]
	courseSummaryRepo           db.MainRepo[models.CourseSummary]
	courseAssignmentRepo        db.MainRepo[models.CourseAssignment]
	courseTestRepo              db.MainRepo[models.CourseTest]
	courseFaqRepo               db.MainRepo[models.CourseFaq]
	courseStudyNoteRepo         db.MainRepo[models.CourseStudyNote]
	courseStudyNoteCategoryRepo db.MainRepo[models.CourseStudyNoteCategory]
	courseFlashcardRepo         db.MainRepo[models.CourseFlashcard]
	courseFlashcardCategoryRepo db.MainRepo[models.CourseFlashcardCategory]
	courseCategoryRepo          db.MainRepo[models.CourseCategory]
	//cartRepo                    db.MainRepo[models.CartItem]
	studentCourseRepo           db.MainRepo[models.StudentCourse]
	courseStudentAssignmentRepo db.MainRepo[models.CourseStudentAssignment]
	courseStudentTestRepo       db.MainRepo[models.CourseStudentTest]
	coursePricingRepo           db.MainRepo[models.CoursePricing]
	studentBundleRepo           db.MainRepo[models.StudentBundle]
	bundleRepo                  db.MainRepo[models.Bundle]
	coachingRepo                db.MainRepo[models.Coaching]
	studentSessionRepo          db.MainRepo[models.StudentSession]
	teacherAssignmentRepo       db.MainRepo[models.TeacherAssignment]
	withtxn                     *db.WithTxn
	pricingSvcs                 sales.PricingSvcs
	messageSvcs                 messaging.MessageSvcs
}

// NewCourseSvcs creates a new instance of CourseSvcs
func NewCourseSvcs(i *do.Injector) (CourseSvcs, error) {
	pricingSvcs, err := do.Invoke[sales.PricingSvcs](i)
	if err != nil {
		return nil, err
	}

	return &coursesvcs{
		i:                           i,
		authSvc:                     do.MustInvoke[CourseAuthService](i),
		repo:                        db.NewRepo[models.Course](i),
		courseSummaryRepo:           db.NewRepo[models.CourseSummary](i),
		courseAssignmentRepo:        db.NewRepo[models.CourseAssignment](i),
		courseTestRepo:              db.NewRepo[models.CourseTest](i),
		courseFaqRepo:               db.NewRepo[models.CourseFaq](i),
		courseFlashcardRepo:         db.NewRepo[models.CourseFlashcard](i),
		courseFlashcardCategoryRepo: db.NewRepo[models.CourseFlashcardCategory](i),
		courseCategoryRepo:          db.NewRepo[models.CourseCategory](i),
		courseStudyNoteRepo:         db.NewRepo[models.CourseStudyNote](i),
		courseStudyNoteCategoryRepo: db.NewRepo[models.CourseStudyNoteCategory](i),
		studentCourseRepo:           db.NewRepo[models.StudentCourse](i),
		courseStudentAssignmentRepo: db.NewRepo[models.CourseStudentAssignment](i),
		courseStudentTestRepo:       db.NewRepo[models.CourseStudentTest](i),
		coursePricingRepo:           db.NewRepo[models.CoursePricing](i),
		studentBundleRepo:           db.NewRepo[models.StudentBundle](i),
		bundleRepo:                  db.NewRepo[models.Bundle](i),
		coachingRepo:                db.NewRepo[models.Coaching](i),
		studentSessionRepo:          db.NewRepo[models.StudentSession](i),
		teacherAssignmentRepo:       db.NewRepo[models.TeacherAssignment](i),
		withtxn:                     do.MustInvoke[*db.WithTxn](i),
		pricingSvcs:                 pricingSvcs,
		messageSvcs:                 do.MustInvoke[messaging.MessageSvcs](i),
	}, nil
}

// GetById retrieves a course by ID with enriched teachers
func (c *coursesvcs) GetById(ctx context.Context, id string) (*models.EnrichedCourse, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	// Use aggregation to enrich with teachers from TeacherAssignment
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{
			"$lookup": bson.M{
				"from": "academyContentReviews",
				"let":  bson.M{"courseId": "$_id"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": []bson.M{
									{"$eq": []interface{}{"$type", "course"}},
									{"$eq": []interface{}{"$refId", "$$courseId"}},
									{"$eq": []interface{}{"$trash", false}},
								},
							},
						},
					},
				},
				"as": "reviews",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "basicInfo.schoolId",
				"foreignField": "_id",
				"as":           "schoolData",
			},
		},
		{
			"$lookup": bson.M{
				"from": "academyCourseCategories",
				"let":  bson.M{"tagIds": "$basicInfo.tags"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{"$in": []interface{}{"$_id", bson.M{"$ifNull": []interface{}{"$$tagIds", bson.A{}}}}},
							"trash": false,
						},
					},
				},
				"as": "tagsobj",
			},
		},
		{
			"$addFields": bson.M{
				"rating": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$reviews", bson.A{}}}}, 0}},
						bson.M{"$avg": "$reviews.rating"},
						nil,
					},
				},
				"schoolObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
						nil,
					},
				},
			},
		},
		{
			"$project": bson.M{
				"reviews":    0,
				"schoolData": 0,
			},
		},
	}

	var courses []models.EnrichedCourse
	if err := c.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	}); err != nil {
		return nil, err
	}

	if len(courses) == 0 {
		return nil, errors.New("course not found")
	}

	// Add authorization check
	canView, err := c.authSvc.CanViewCourse(ctx, id)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to view this course"))
	}

	return &courses[0], nil
}

// GetEnrolledCourseById retrieves a course by ID with enrollment details for the current student
func (c *coursesvcs) GetEnrolledCourseById(ctx context.Context, id string) (*models.StudentCourseWithDetails, error) {
	// First get the course using existing GetById logic
	course, err := c.GetById(ctx, id)
	if err != nil {
		return nil, err
	}

	// Get the current student ID from context
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Find the student's enrollment for this course
	_courseId, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	enrollment, err := c.studentCourseRepo.GetByFilter(ctx, bson.M{
		"courseId":  _courseId,
		"studentId": cfg.User.Id,
		"trash":     false,
	})
	if err != nil {
		return nil, errors.New("enrollment not found")
	}

	// Calculate total lessons
	totalLessons := c.countLessons(course.Content)

	// Calculate completed lessons count
	completedLessonsCount := len(enrollment.CompletedLessons)

	// Calculate completed sessions by section
	completedSessionsBySection := c.calculateCompletedBySection(course.Content, enrollment.CompletedLessons)

	return &models.StudentCourseWithDetails{
		Course:                     course.Course,
		Name:                       course.BasicInfo.Name,
		Description:                course.BasicInfo.Description,
		Thumbnail:                  course.BasicInfo.Thumbnail,
		Status:                     course.BasicInfo.Status,
		Type:                       course.BasicInfo.Type,
		Duration:                   course.BasicInfo.Duration,
		Content:                    course.Content,
		Rating:                     course.Rating,
		Enrollment:                 *enrollment,
		StudentStatus:              string(enrollment.Status),
		Teachers:                   course.TeachersObjs,
		Supervisors:                nil, // Need to fetch separately if needed
		TotalLessons:               totalLessons,
		CompletedLessons:           completedLessonsCount,
		CompletedSessionsBySection: completedSessionsBySection,
	}, nil
}

// getCourseTeachersAndSupervisors fetches teacher and supervisor details
func (c *coursesvcs) getCourseTeachersAndSupervisors(ctx context.Context, teacherIds, supervisorIds []string) ([]models.TeacherBasicInfo, []models.SupervisorBasicInfo, error) {
	var teachers []models.TeacherBasicInfo
	var supervisors []models.SupervisorBasicInfo

	// Convert string IDs to ObjectIDs
	teacherObjectIDs := make([]primitive.ObjectID, len(teacherIds))
	for i, id := range teacherIds {
		objID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		teacherObjectIDs[i] = objID
	}

	supervisorObjectIDs := make([]primitive.ObjectID, len(supervisorIds))
	for i, id := range supervisorIds {
		objID, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			continue
		}
		supervisorObjectIDs[i] = objID
	}

	// Lookup teachers
	if len(teacherObjectIDs) > 0 {
		teacherPipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": teacherObjectIDs}, "trash": false}},
			{"$project": bson.M{
				"_id":       1,
				"firstName": 1,
				"lastName":  1,
				"email":     1,
				"picture":   1,
			}},
		}

		if err := c.teacherAssignmentRepo.Aggregate(ctx, teacherPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teachers)
		}); err != nil {
			return nil, nil, err
		}
	}

	// Lookup supervisors
	if len(supervisorObjectIDs) > 0 {
		// This would need supervisor repo lookup - simplified for now
	}

	return teachers, supervisors, nil
}

// applyCourseAuthFilter adds permission-based filtering to a MongoDB aggregation pipeline
func (c *coursesvcs) applyCourseAuthFilter(ctx context.Context, pipeline []bson.M) ([]bson.M, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCourses)
	canViewAssigned, _ := util.HasCapability(ctx, CapabilityViewAssignedCourses)
	canViewPublic, _ := util.HasCapability(ctx, CapabilityViewPublicCourses)

	if canViewAll {
		return pipeline, nil
	}

	if canViewAssigned {
		// Teachers/supervisors: check if user is in teachers list or supervisor list
		pipeline = append(pipeline, bson.M{
			"$match": bson.M{
				"$or": []bson.M{
					{"basicInfo.teachers": cfg.User.Id},
					{"basicInfo.supervisorIds": cfg.User.Id},
				},
			},
		})
		return pipeline, nil
	}

	if canViewPublic {
		// Students: lookup enrollments and filter for active OR enrolled
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyCourseStudents",
				"let":  bson.M{"courseId": "$_id"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": []bson.M{
									{"$eq": []interface{}{"$courseId", "$$courseId"}},
									{"$eq": []interface{}{"$studentId", cfg.User.Id}},
									{"$eq": []interface{}{"$trash", false}},
								},
							},
						},
					},
				},
				"as": "enrollments",
			},
		})

		// Keep courses that are active OR have enrollment
		pipeline = append(pipeline, bson.M{
			"$match": bson.M{
				"$or": []bson.M{
					{"basicInfo.status": "active"},
					{"enrollments.0": bson.M{"$exists": true}},
				},
			},
		})
		return pipeline, nil
	}

	// No permissions - return empty
	pipeline = append(pipeline, bson.M{"$match": bson.M{"_id": primitive.NilObjectID}})
	return pipeline, nil
}

// Get retrieves a paginated list of courses
func (c *coursesvcs) Get(ctx context.Context, skip, limit int64, query *querypkg.Conditions, enrichSubjects bool, includeStudentCount bool, enrichTeachers bool, enrichPrice bool) (*models.CoursePagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	// Check user capabilities once for permission filtering
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}

	// Add query filters
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// // Add permission filtering
	// pipeline, err = c.applyCourseAuthFilter(ctx, pipeline)
	// if err != nil {
	// 	return nil, err
	// }

	// Sort, skip, limit for pagination
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyContentReviews",
			"let":  bson.M{"courseId": "$_id"},
			"pipeline": []bson.M{
				{
					"$match": bson.M{
						"$expr": bson.M{
							"$and": []bson.M{
								{"$eq": []interface{}{"$type", "course"}},
								{"$eq": []interface{}{"$refId", "$$courseId"}},
								{"$eq": []interface{}{"$trash", false}},
							},
						},
					},
				},
			},
			"as": "reviews",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academySchools",
			"localField":   "basicInfo.schoolId",
			"foreignField": "_id",
			"as":           "schoolData",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyCourseCategories",
			"let":  bson.M{"tagIds": "$basicInfo.tags"},
			"pipeline": []bson.M{
				{
					"$match": bson.M{
						"$expr": bson.M{"$in": []interface{}{"$_id", bson.M{"$ifNull": []interface{}{"$$tagIds", bson.A{}}}}},
						"trash": false,
					},
				},
			},
			"as": "tagsobj",
		},
	})

	if enrichSubjects {
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from":         "academySubjects",
				"localField":   "basicInfo.linkedSubjects",
				"foreignField": "_id",
				"as":           "subjects",
			},
		})
	}

	if includeStudentCount {
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyCourseStudents",
				"let":  bson.M{"courseId": "$_id"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{
								"$and": []bson.M{
									{"$eq": []interface{}{"$courseId", "$$courseId"}},
									{"$eq": []interface{}{"$trash", false}},
								},
							},
						},
					},
					{"$count": "count"},
				},
				"as": "studentCountData",
			},
		})
	}

	addFields := bson.M{
		"rating": bson.M{
			"$cond": []interface{}{
				bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$reviews", bson.A{}}}}, 0}},
				bson.M{"$avg": "$reviews.rating"},
				nil,
			},
		},
		"schoolObj": bson.M{
			"$cond": []interface{}{
				bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
				bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
				nil,
			},
		},
	}

	if includeStudentCount {
		addFields["studentCount"] = bson.M{
			"$cond": []interface{}{
				bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$studentCountData", bson.A{}}}}, 0}},
				bson.M{"$arrayElemAt": []interface{}{"$studentCountData.count", 0}},
				0,
			},
		}
	}

	if enrichTeachers {
		pipeline = append(pipeline, bson.M{
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

	if enrichPrice {
		stages, err := c.pricingSvcs.GetPriceEnrichmentStages(ctx, enums.CartTypeCourse, "$_id")
		if err == nil {
			pipeline = append(pipeline, stages...)
		}
		pipeline = append(pipeline, bson.M{"$match": bson.M{"pricing": bson.M{"$ne": nil}}})
	}

	pipeline = append(pipeline, bson.M{
		"$addFields": addFields,
	})
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"reviews":          0,
			"enrollments":      0, // From permission filtering
			"studentCountData": 0,
			"schoolData":       0,
		},
	})

	// Get total count for pagination (only include stages before pagination)
	countPipeline := make([]bson.M, 0)
	for _, stage := range pipeline {
		// Stop before pagination/sorting/enrichment
		if _, ok := stage["$sort"]; ok {
			break
		}
		if _, ok := stage["$skip"]; ok {
			break
		}
		if _, ok := stage["$limit"]; ok {
			break
		}
		countPipeline = append(countPipeline, stage)
	}
	countPipeline = append(countPipeline, bson.M{"$count": "total"})

	var countResults []struct {
		Total int64 `bson:"total"`
	}
	if err := c.repo.Aggregate(ctx, countPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &countResults)
	}); err != nil {
		// If count fails, assume 0
		countResults = []struct {
			Total int64 `bson:"total"`
		}{}
	}

	totalCount := int64(0)
	if len(countResults) > 0 {
		totalCount = countResults[0].Total
	}

	var result []models.EnrichedCourse
	if err := c.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Calculate pagination
	var totalPages float64 = math.Ceil(float64(totalCount) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &models.CoursePagination{
		Courses:    result,
		Pagination: pagination,
	}, nil
}

func (c *coursesvcs) GetAll(ctx context.Context, query *querypkg.Conditions) ([]models.EnrichedCourse, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}

	// Add query filters
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Add permission filtering
	// pipeline, err = c.applyCourseAuthFilter(ctx, pipeline)
	// if err != nil {
	// 	return nil, err
	// }

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyContentReviews",
			"let":  bson.M{"courseId": "$_id"},
			"pipeline": []bson.M{
				{
					"$match": bson.M{
						"$expr": bson.M{
							"$and": []bson.M{
								{"$eq": []interface{}{"$type", "course"}},
								{"$eq": []interface{}{"$refId", "$$courseId"}},
								{"$eq": []interface{}{"$trash", false}},
							},
						},
					},
				},
			},
			"as": "reviews",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academySchools",
			"localField":   "basicInfo.schoolId",
			"foreignField": "_id",
			"as":           "schoolData",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from": "academyCourseCategories",
			"let":  bson.M{"tagIds": "$basicInfo.tags"},
			"pipeline": []bson.M{
				{
					"$match": bson.M{
						"$expr": bson.M{"$in": []interface{}{"$_id", bson.M{"$ifNull": []interface{}{"$$tagIds", bson.A{}}}}},
						"trash": false,
					},
				},
			},
			"as": "tagsobj",
		},
	})

	addFields := bson.M{
		"rating": bson.M{
			"$cond": []interface{}{
				bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$reviews", bson.A{}}}}, 0}},
				bson.M{"$avg": "$reviews.rating"},
				nil,
			},
		},
		"schoolObj": bson.M{
			"$cond": []interface{}{
				bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
				bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
				nil,
			},
		},
	}

	pipeline = append(pipeline, bson.M{
		"$addFields": addFields,
	})
	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"reviews":     0,
			"schoolData":  0,
			"enrollments": 0,
		},
	})

	var result []models.EnrichedCourse
	if err := c.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	return result, nil
}

// ensureContentIds ensures all content items in sections have IDs
func ensureContentIds(data *models.CourseDto) {
	for i := range data.Content {
		for j := range data.Content[i].Lessons {
			for k := range data.Content[i].Lessons[j].Content {
				if data.Content[i].Lessons[j].Content[k].Id.IsZero() {
					data.Content[i].Lessons[j].Content[k].Id = primitive.NewObjectID()
				}
			}
		}
		// Also check subSections recursively
		for j := range data.Content[i].SubSections {
			for k := range data.Content[i].SubSections[j].Lessons {
				for l := range data.Content[i].SubSections[j].Lessons[k].Content {
					if data.Content[i].SubSections[j].Lessons[k].Content[l].Id.IsZero() {
						data.Content[i].SubSections[j].Lessons[k].Content[l].Id = primitive.NewObjectID()
					}
				}
			}
		}
	}
}

// Add creates a new course
func (c *coursesvcs) Add(ctx context.Context, data *models.CourseDto) (*models.Course, error) {
	// Add authorization check - users with ManageAllCourses or ManageAssignedCourses can create courses
	// canManageAll, _ := util.HasCapability(ctx, CapabilityManageAllCourses)
	// canManageAssigned, _ := util.HasCapability(ctx, CapabilityManageAssignedCourses)
	// if !canManageAll && !canManageAssigned {
	// 	return nil, helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to create courses"))
	// }

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Store teachers separately before clearing
	teachers := data.BasicInfo.Teachers

	// Ensure teachers is not nil for persistence (empty array instead of null)
	if data.BasicInfo.Teachers == nil {
		data.BasicInfo.Teachers = make([]primitive.ObjectID, 0)
	}

	// Ensure all content items have IDs
	ensureContentIds(data)

	// Create course
	course := &models.Course{
		Id:        primitive.NewObjectID(),
		CourseDto: *data,
		CreatedAt: time.Now(),
		CreatedBy: cfg.User.Id,
		UpdatedAt: time.Now(),
	}

	if err := c.repo.Add(ctx, course); err != nil {
		return nil, err
	}

	// Sync supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[SupervisorAssignmentSvcs](c.i)
	supervisorIds, err := supervisorAssignmentSvc.CheckSupervisorAssignment(ctx, course.BasicInfo.SchoolType, course.BasicInfo.Country, course.BasicInfo.Level, course.BasicInfo.LinkedSubjects)
	if err != nil {
		// Log error but don't fail course creation
		log.Printf("Failed to check supervisor assignment: %v", err)
	} else {
		// Update course with supervisor IDs
		supervisorUpdate := bson.M{"$set": bson.M{"basicInfo.supervisorIds": supervisorIds}}
		updatedCourse, err := c.repo.Patch(ctx, bson.M{"_id": course.Id}, supervisorUpdate)
		if err != nil {
			// Log error but don't fail course creation
			log.Printf("Failed to update supervisor IDs: %v", err)
		} else {
			course = updatedCourse
		}
	}

	// Sync teacher assignment
	teacherAssignmentSvc := do.MustInvoke[TeacherAssignmentSvcs](c.i)
	err = teacherAssignmentSvc.CreateFromCourse(ctx, course.Id, teachers)
	if err != nil {
		// Log error but don't fail course creation
		log.Printf("Failed to create teacher assignment: %v", err)
	}

	return course, nil
}

// Update updates an existing course
func (c *coursesvcs) Update(ctx context.Context, CourseId string, data *models.CourseDto) (*models.Course, error) {
	_id, err := primitive.ObjectIDFromHex(CourseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	// Add authorization check
	canManage, err := c.authSvc.CanManageCourse(ctx, CourseId)
	if err != nil {
		return nil, err
	}
	if !canManage {
		return nil, helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to update this course"))
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Store teachers separately before clearing
	teachers := data.BasicInfo.Teachers

	// Ensure teachers is not nil for persistence (empty array instead of null)
	if data.BasicInfo.Teachers == nil {
		data.BasicInfo.Teachers = make([]primitive.ObjectID, 0)
	}

	// Ensure all content items have IDs
	ensureContentIds(data)

	// Update course
	course := &models.Course{
		Id:        _id,
		CourseDto: *data,
		UpdatedAt: time.Now(),
		UpdatedBy: cfg.User.Id,
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": course}

	updatedCourse, err := c.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Sync supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[SupervisorAssignmentSvcs](c.i)
	supervisorIds, err := supervisorAssignmentSvc.CheckSupervisorAssignment(ctx, updatedCourse.BasicInfo.SchoolType, updatedCourse.BasicInfo.Country, updatedCourse.BasicInfo.Level, updatedCourse.BasicInfo.LinkedSubjects)
	if err != nil {
		// Log error but don't fail course update
		log.Printf("Failed to check supervisor assignment: %v", err)
	} else {
		// Update course with supervisor IDs
		supervisorUpdate := bson.M{"$set": bson.M{"basicInfo.supervisorIds": supervisorIds}}
		updatedCourse, err = c.repo.Patch(ctx, bson.M{"_id": _id, "trash": false}, supervisorUpdate)
		if err != nil {
			// Log error but don't fail course update
			log.Printf("Failed to update supervisor IDs: %v", err)
		}
	}

	// Sync teacher assignment if teachers provided
	if teachers != nil {
		teacherAssignmentSvc := do.MustInvoke[TeacherAssignmentSvcs](c.i)
		err = teacherAssignmentSvc.BulkCreateOrUpdate(ctx, &models.BulkTeacherAssignmentDto{
			Type:       "course",
			TargetId:   _id,
			TeacherIds: teachers,
		})
		if err != nil {
			// Log error but don't fail course update
			log.Printf("Failed to update teacher assignment: %v", err)
		}
	}

	return updatedCourse, nil
}

// Delete soft deletes a course
func (c *coursesvcs) Delete(ctx context.Context, courseId string) error {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}

	// Add authorization check
	canManage, err := c.authSvc.CanManageCourse(ctx, courseId)
	if err != nil {
		return err
	}
	if !canManage {
		return helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to delete this course"))
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

// UpdateCourseLiveSessionTime updates the schedule time of a course live session (for calendar events)
func (c *coursesvcs) UpdateCourseLiveSessionTime(ctx context.Context, courseId string, schedule time.Time) (*models.Course, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	course, err := c.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("course not found")
	}

	// Authorization: Only teacher assigned to course or admin can update live session time
	// Check if user is admin
	isAdmin, err := util.HasCapability(ctx, CapabilityViewAllCalendarEvents)
	if err != nil {
		return nil, err
	}

	if !isAdmin {
		// For non-admins, must be one of the teachers assigned to this course
		teacherAssignmentSvc := do.MustInvoke[TeacherAssignmentSvcs](c.i)
		teacherIds, err := teacherAssignmentSvc.GetCourseTeachers(ctx, course.Id)
		if err != nil {
			return nil, err
		}
		isTeacher := false
		for _, teacherId := range teacherIds {
			if teacherId == cfg.User.Id {
				isTeacher = true
				break
			}
		}
		if !isTeacher {
			return nil, errors.New("you are not authorized to update this course live session")
		}
	}

	// Update live session schedule
	course.Settings.LiveSession.Schedule = schedule
	course.UpdatedAt = time.Now()
	course.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{
		"settings.liveSession.schedule": schedule,
		"updatedAt":                     time.Now(),
		"updatedBy":                     cfg.User.Id,
	}}

	updatedCourse, err := c.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedCourse, nil
}

// DeleteCourseLiveSession deactivates/cancels a course live session (for calendar events)
func (c *coursesvcs) DeleteCourseLiveSession(ctx context.Context, courseId string) error {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return errors.New("invalid course id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	course, err := c.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return errors.New("course not found")
	}

	// Authorization: Only teacher assigned to course or admin can delete live session
	// Check if user is admin
	isAdmin, err := util.HasCapability(ctx, CapabilityViewAllCalendarEvents)
	if err != nil {
		return err
	}

	if !isAdmin {
		// For non-admins, must be one of the teachers assigned to this course
		teacherAssignmentSvc := do.MustInvoke[TeacherAssignmentSvcs](c.i)
		teacherIds, err := teacherAssignmentSvc.GetCourseTeachers(ctx, course.Id)
		if err != nil {
			return err
		}
		isTeacher := false
		for _, teacherId := range teacherIds {
			if teacherId == cfg.User.Id {
				isTeacher = true
				break
			}
		}
		if !isTeacher {
			return errors.New("you are not authorized to delete this course live session")
		}
	}

	// Deactivate live session
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{
		"settings.liveSession.active": false,
		"updatedAt":                   time.Now(),
		"updatedBy":                   cfg.User.Id,
	}}

	_, err = c.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

// GetCourseSummaryStats retrieves summary statistics for a course
func (c *coursesvcs) GetCourseSummaryStats(ctx context.Context, courseId string) (*models.CourseSummaryStats, error) {
	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	// Verify course exists
	course, err := c.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("course not found")
	}

	// Add authorization check
	canView, err := c.authSvc.CanViewCourse(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, helpers.NewApiError(http.StatusForbidden, errors.New("unauthorized to view this course"))
	}

	stats := &models.CourseSummaryStats{}

	// 1. Count enrolled students
	enrolledCount, err := c.studentCourseRepo.Count(ctx, []bson.M{
		{"$match": bson.M{"courseId": _id, "trash": false}},
	})
	if err != nil {
		return nil, err
	}
	stats.EnrolledStudentsCount = int(enrolledCount)

	// 2. Calculate completion rate (percentage of students with status "completed")
	if enrolledCount > 0 {
		completedCount, err := c.studentCourseRepo.Count(ctx, []bson.M{
			{"$match": bson.M{"courseId": _id, "status": "completed", "trash": false}},
		})
		if err != nil {
			return nil, err
		}
		stats.CompletionRate = (float64(completedCount) / float64(enrolledCount)) * 100.0
	} else {
		stats.CompletionRate = 0.0
	}

	// 3. Calculate average engagement rate (average progress percentage)
	if enrolledCount > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{"courseId": _id, "trash": false}},
			{"$group": bson.M{
				"_id":         nil,
				"avgProgress": bson.M{"$avg": "$progress"},
			}},
		}
		var result []struct {
			AvgProgress float64 `bson:"avgProgress"`
		}
		if err := c.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result)
		}); err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}
		if len(result) > 0 {
			stats.AvgEngagementRate = result[0].AvgProgress
		} else {
			stats.AvgEngagementRate = 0.0
		}
	} else {
		stats.AvgEngagementRate = 0.0
	}

	// 4. Get course rating (average rating from reviews)
	ratingPipeline := []bson.M{
		{"$match": bson.M{"refId": _id, "type": "course", "trash": false}},
		{"$group": bson.M{
			"_id":   nil,
			"avg":   bson.M{"$avg": "$rating"},
			"count": bson.M{"$sum": 1},
		}},
	}
	var ratingResult []struct {
		Avg   float64 `bson:"avg"`
		Count int     `bson:"count"`
	}
	reviewRepo := db.NewRepo[models.Review](c.i)
	if err := reviewRepo.Aggregate(ctx, ratingPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &ratingResult)
	}); err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}
	if len(ratingResult) > 0 && ratingResult[0].Count > 0 {
		stats.CourseRating = &ratingResult[0].Avg
	} else {
		stats.CourseRating = nil
	}

	// 5. Count total lessons (recursively count lessons in sections and subsections)
	stats.TotalLessons = c.countLessons(course.Content)

	return stats, nil
}

// countLessons recursively counts lessons in sections and subsections
func (c *coursesvcs) countLessons(sections []models.Section) int {
	total := 0
	for _, section := range sections {
		// Count lessons in this section
		total += len(section.Lessons)
		// Recursively count lessons in subsections
		if len(section.SubSections) > 0 {
			total += c.countLessons(section.SubSections)
		}
	}
	return total
}

func (c *coursesvcs) GetStudentList(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions, enrichSubjects bool, includeStudentCount bool, enrichTeachers bool, enrichPrice bool) (*models.StudentCoursePagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	var targetUserId primitive.ObjectID
	var isParentViewing bool

	if studentId != "" {
		_studentId, err := primitive.ObjectIDFromHex(studentId)
		if err != nil {
			return nil, errors.New("invalid student id")
		}

		if studentId != cfg.User.Id.Hex() {
			isParent, err := c.authSvc.CanViewStudentCourseData(ctx, studentId)
			if err != nil {
				return nil, fmt.Errorf("error checking authorization: %w", err)
			}
			if !isParent {
				return nil, errors.New("unauthorized: you can only view your own or your children's data")
			}
			isParentViewing = true
		}
		targetUserId = _studentId
	} else if cfg.User != nil && !cfg.User.Id.IsZero() {
		targetUserId = cfg.User.Id
	} else {
		res, err := c.Get(ctx, skip, limit, query, enrichSubjects, includeStudentCount, enrichTeachers, enrichPrice)
		if err != nil {
			return nil, err
		}
		return &models.StudentCoursePagination{
			Courses:    mapToStudentCourseResponse(res.Courses, nil, nil),
			Pagination: res.Pagination,
		}, nil
	}

	var isEnrolledRequested *bool
	if query != nil {
		newCols := []querypkg.Column{}
		for _, col := range query.Columns {
			if col.Name == "isEnrolled" {
				val, ok := col.Value.(bool)
				if !ok {
					if s, ok := col.Value.(string); ok {
						if b, err := strconv.ParseBool(s); err == nil {
							val = b
							ok = true
						}
					}
				}
				if ok {
					isEnrolledRequested = &val
				}
				continue
			}
			newCols = append(newCols, col)
		}
		query.Columns = newCols
	}

	enrollmentsMap := make(map[primitive.ObjectID]bool)
	enrollmentIds := []primitive.ObjectID{}
	var enrollments []models.StudentCourse
	err = c.studentCourseRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"studentId": targetUserId, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	})
	if err == nil {
		for _, e := range enrollments {
			enrollmentsMap[e.CourseId] = true
			enrollmentIds = append(enrollmentIds, e.CourseId)
		}
	}

	if isEnrolledRequested != nil {
		if query == nil {
			query = &querypkg.Conditions{}
		}
		if *isEnrolledRequested {
			query.Columns = append(query.Columns, querypkg.Column{
				Name:  "_id",
				Exp:   querypkg.In,
				Value: enrollmentIds,
			})
		} else {
			query.Columns = append(query.Columns, querypkg.Column{
				Name:  "_id",
				Exp:   querypkg.NotIn,
				Value: enrollmentIds,
			})
		}
	}

	res, err := c.Get(ctx, skip, limit, query, enrichSubjects, includeStudentCount, enrichTeachers, enrichPrice)
	if err != nil {
		return nil, err
	}

	studentSvc := do.MustInvoke[contact.StudentSvcs](c.i)
	var studentIdForFavorites string
	if isParentViewing {
		studentIdForFavorites = studentId
	} else {
		studentIdForFavorites = targetUserId.Hex()
	}
	student, err := studentSvc.GetOne(ctx, studentIdForFavorites)
	favoritesMap := make(map[primitive.ObjectID]bool)
	if err == nil && student != nil {
		for _, fav := range student.Favorites {
			if fav.Type == "course" && !fav.RefId.IsZero() {
				favoritesMap[fav.RefId] = true
			}
		}
	}

	return &models.StudentCoursePagination{
		Courses:    mapToStudentCourseResponse(res.Courses, favoritesMap, enrollmentsMap),
		Pagination: res.Pagination,
	}, nil
}

func mapToStudentCourseResponse(courses []models.EnrichedCourse, favoritesMap map[primitive.ObjectID]bool, enrollmentsMap map[primitive.ObjectID]bool) []models.StudentCourseResponse {
	response := make([]models.StudentCourseResponse, len(courses))
	for i, course := range courses {
		isFavorite := false
		if favoritesMap != nil {
			isFavorite = favoritesMap[course.Id]
		}
		isEnrolled := false
		if enrollmentsMap != nil {
			isEnrolled = enrollmentsMap[course.Id]
		}
		response[i] = models.StudentCourseResponse{
			Course:       course.Course,
			Subjects:     course.Subjects,
			StudentCount: course.StudentCount,
			TeachersObjs: course.TeachersObjs,
			Pricing:      course.Pricing,
			IsFavorite:   isFavorite,
			IsEnrolled:   isEnrolled,
		}
	}
	return response
}

func (c *coursesvcs) GetTeacherCourses(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.TeacherCoursePagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id

	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		var err error
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
	} else {
		filter = bson.M{}
	}

	// Get all courses where teacher is assigned using teacher assignment repo
	var teacherAssignments []models.TeacherAssignment
	err = c.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"teacherIds": teacherId,
			"type":       "course",
			"trash":      false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teacherAssignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Collect course IDs
	courseIds := make([]primitive.ObjectID, 0, len(teacherAssignments))
	for _, assignment := range teacherAssignments {
		courseIds = append(courseIds, assignment.TargetId)
	}

	if len(courseIds) == 0 {
		// No courses assigned, return empty result
		return &models.TeacherCoursePagination{
			Courses: []models.TeacherCourseResponse{},
			Pagination: types.Pagination{
				TotalPages: 0,
				PerPage:    limit,
				TotalCount: 0,
			},
		}, nil
	}

	// Build filter for courses
	teacherFilter := bson.M{
		"trash": false,
		"_id":   bson.M{"$in": courseIds},
	}

	// Merge with user-provided filters
	if len(filter) > 0 {
		teacherFilter = bson.M{
			"$and": []bson.M{
				teacherFilter,
				filter,
			},
		}
	}

	pipeline := []bson.M{
		{"$match": teacherFilter},
	}

	count, err := c.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	// Add lookup to count students for each course
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academyCourseStudents",
			"localField":   "_id",
			"foreignField": "courseId",
			"as":           "students",
		},
	})
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"studentCount": bson.M{
				"$size": bson.M{
					"$filter": bson.M{
						"input": bson.M{"$ifNull": bson.A{"$students", bson.A{}}},
						"cond":  bson.M{"$eq": bson.A{"$$this.trash", false}},
					},
				},
			},
		},
	})

	// Use bson.M to hold the aggregation result
	var result []bson.M
	if err := c.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Map to response structs using util.Converter
	courses := make([]models.TeacherCourseResponse, len(result))
	for i, doc := range result {
		course, err := util.Converter[models.Course](doc)
		if err != nil {
			return nil, err
		}

		studentCount := 0
		if countVal, ok := doc["studentCount"]; ok {
			if countInt, ok := countVal.(int32); ok {
				studentCount = int(countInt)
			} else if countInt, ok := countVal.(int); ok {
				studentCount = countInt
			}
		}

		courses[i] = models.TeacherCourseResponse{
			Course:       course,
			StudentCount: studentCount,
		}
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.TeacherCoursePagination{
		Courses:    courses,
		Pagination: pagination,
	}, nil
}

func (c *coursesvcs) GetTeacherCourseMembers(ctx context.Context, courseId string, skip, limit int64, query *querypkg.Conditions) (*models.MemberCoursePagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id

	courseObjectId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	// Verify teacher is assigned to this course using teacher assignment repo
	count, err := c.teacherAssignmentRepo.Count(ctx, []bson.M{
		{"$match": bson.M{
			"targetId":   courseObjectId,
			"teacherIds": teacherId,
			"type":       "course",
			"trash":      false,
		}},
	})
	if err != nil {
		return nil, err
	}
	if count == 0 {
		return nil, errors.New("you are not assigned to this course")
	}

	// Check and convert query if provided, otherwise use empty filter
	var filter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("invalid convert to mongo filters")
		}
	} else {
		filter = bson.M{}
	}

	// Base filter: course ID and not trashed
	baseFilter := bson.M{
		"courseId": courseObjectId,
		"trash":    false,
	}

	// Merge with user-provided filters
	if len(filter) > 0 {
		baseFilter = bson.M{
			"$and": []bson.M{
				baseFilter,
				filter,
			},
		}
	}

	pipeline := []bson.M{
		{"$match": baseFilter},
	}

	totalCount, err := c.studentCourseRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	// Lookup student details
	pipeline = append(pipeline, []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
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
			"$addFields": bson.M{
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"email":     "$student.accountStudentSetting.email",
			},
		},
		{
			"$project": bson.M{
				"student": 0,
			},
		},
	}...)

	var result []models.StudentCourseRes
	if err := c.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(totalCount) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &models.MemberCoursePagination{
		StudentCourses: result,
		Pagination:     pagination,
	}, nil
}
