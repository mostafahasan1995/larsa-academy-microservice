package content

import (
	"context"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
	content_constant "larsa-academy-microservice/pkg/services/content/constant"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

var populateStudentField = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "studentData",
		},
	},
	{
		"$unwind": bson.M{
			"path":                       "$studentData",
			"preserveNullAndEmptyArrays": true,
		},
	},
	{
		"$set": bson.M{
			"student": bson.M{
				"$cond": []interface{}{
					bson.M{"$ne": []interface{}{"$studentData", nil}},
					bson.M{
						"firstName": "$studentData.generalInfo.firstName",
						"lastName":  "$studentData.generalInfo.lastName",
						"email":     "$studentData.accountStudentSetting.email",
						"picture":   "$studentData.generalInfo.picture",
					},
					nil,
				},
			},
		},
	},
}

var populateReplyTeacherField = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "academyTeachers",
			"localField":   "reply.userId",
			"foreignField": "_id",
			"as":           "replyTeacherData",
		},
	},
	{
		"$unwind": bson.M{
			"path":                       "$replyTeacherData",
			"preserveNullAndEmptyArrays": true,
		},
	},
	{
		"$set": bson.M{
			"replyTeacher": bson.M{
				"$cond": []interface{}{
					bson.M{"$ne": []interface{}{"$replyTeacherData", nil}},
					bson.M{
						"firstName": "$replyTeacherData.firstName",
						"lastName":  "$replyTeacherData.lastName",
						"email":     "$replyTeacherData.accountSettings.email",
						"picture":   "$replyTeacherData.picture",
					},
					nil,
				},
			},
		},
	},
}

var populateRefField = []bson.M{
	// Lookup course by refId
	{
		"$lookup": bson.M{
			"from":         "academyCourses",
			"localField":   "refId",
			"foreignField": "_id",
			"as":           "courseData",
		},
	},
	{
		"$unwind": bson.M{
			"path":                       "$courseData",
			"preserveNullAndEmptyArrays": true,
		},
	},
	// Lookup coaching by refId
	{
		"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "refId",
			"foreignField": "_id",
			"as":           "coachingData",
		},
	},
	{
		"$unwind": bson.M{
			"path":                       "$coachingData",
			"preserveNullAndEmptyArrays": true,
		},
	},
	{
		"$set": bson.M{
			"ref": bson.M{
				"$cond": []interface{}{
					bson.M{"$eq": []interface{}{"$refType", "course"}},
					// Course refType
					bson.M{
						"$cond": []interface{}{
							bson.M{"$ne": []interface{}{"$courseData", nil}},
							bson.M{
								"course": bson.M{
									"_id":  "$courseData._id",
									"name": "$courseData.basicInfo.name",
								},
								"lesson": bson.M{
									"$cond": []interface{}{
										bson.M{"$and": []interface{}{
											bson.M{"$ne": []interface{}{"$lessonId", nil}},
											bson.M{"$ne": []interface{}{"$courseData", nil}},
										}},
										// Flatten all lessons from sections and sub-sections, then find matching lesson
										bson.M{
											"$let": bson.M{
												"vars": bson.M{
													"allLessons": bson.M{
														"$reduce": bson.M{
															"input":        bson.M{"$ifNull": []interface{}{"$courseData.content", bson.A{}}},
															"initialValue": bson.A{},
															"in": bson.M{
																"$concatArrays": []interface{}{
																	"$$value",
																	bson.M{
																		"$concatArrays": []interface{}{
																			bson.M{"$ifNull": []interface{}{"$$this.lessons", bson.A{}}},
																			bson.M{
																				"$reduce": bson.M{
																					"input":        bson.M{"$ifNull": []interface{}{"$$this.subSections", bson.A{}}},
																					"initialValue": bson.A{},
																					"in": bson.M{
																						"$concatArrays": []interface{}{
																							"$$value",
																							bson.M{"$ifNull": []interface{}{"$$this.lessons", bson.A{}}},
																						},
																					},
																				},
																			},
																		},
																	},
																},
															},
														},
													},
												},
												"in": bson.M{
													"$let": bson.M{
														"vars": bson.M{
															"matchedLesson": bson.M{
																"$arrayElemAt": []interface{}{
																	bson.M{
																		"$filter": bson.M{
																			"input": "$$allLessons",
																			"as":    "lesson",
																			"cond":  bson.M{"$eq": []interface{}{"$$lesson._id", "$lessonId"}},
																		},
																	},
																	0,
																},
															},
														},
														"in": bson.M{
															"$cond": []interface{}{
																bson.M{"$ne": []interface{}{"$$matchedLesson", nil}},
																bson.M{
																	"_id":  "$$matchedLesson._id",
																	"name": "$$matchedLesson.name",
																},
																nil,
															},
														},
													},
												},
											},
										},
										nil,
									},
								},
							},
							nil,
						},
					},
					// Coaching refType
					bson.M{
						"$cond": []interface{}{
							bson.M{"$ne": []interface{}{"$coachingData", nil}},
							bson.M{
								"coaching": bson.M{
									"_id":  "$coachingData._id",
									"name": "$coachingData.title",
								},
							},
							nil,
						},
					},
				},
			},
		},
	},
}

type StudentQuestionSvcs interface {
	GetStudentQuestionsByRef(ctx context.Context, refType string, refId string, skip, limit int64, query *query.Conditions) (*models.StudentQuestionPagination, error)
	GetStudentQuestions(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentQuestionPagination, error)
	GetStudentQuestion(ctx context.Context, questionId string) (*models.StudentQuestionRes, error)
	GetTeacherQuestionStats(ctx context.Context, refType *string, refId *string) (*models.StudentQuestionTeacherStats, error)
	GetSupervisorQuestionStats(ctx context.Context, refType *string, refId *string, skip, limit int64) (*models.StudentQuestionSupervisorStats, error)
	AddStudentQuestion(ctx context.Context, refType string, data *models.StudentQuestionDto) (*models.StudentQuestion, error)
	UpdateStudentQuestion(ctx context.Context, questionId string, data *models.StudentQuestionDto) (*models.StudentQuestion, error)
	DeleteStudentQuestion(ctx context.Context, questionId string) error
	AddReply(ctx context.Context, questionId string, data *models.StudentQuestionReplyDto) (*models.StudentQuestionReply, error)
	UpdateReply(ctx context.Context, replyId string, data *models.StudentQuestionReplyDto) (*models.StudentQuestionReply, error)
	DeleteReply(ctx context.Context, replyId string) error
}

type studentquestionsvcs struct {
	questionRepo          db.MainRepo[models.StudentQuestion]
	courseRepo            db.MainRepo[models.Course]
	coachingRepo          db.MainRepo[models.Coaching]
	teacherAssignmentRepo db.MainRepo[models.TeacherAssignment]
	teacherAssignmentSvc  TeacherAssignmentSvcs
	teacherRepo           db.MainRepo[contactmodels.Teacher]
	supervisorSvc         contact.SupervisorSvc
	dbClient              *mongo.Client
}

func NewStudentQuestionSvcs(i *do.Injector) (StudentQuestionSvcs, error) {
	return &studentquestionsvcs{
		questionRepo:          db.NewRepo[models.StudentQuestion](i),
		courseRepo:            db.NewRepo[models.Course](i),
		coachingRepo:          db.NewRepo[models.Coaching](i),
		teacherAssignmentRepo: db.NewRepo[models.TeacherAssignment](i),
		teacherAssignmentSvc:  do.MustInvoke[TeacherAssignmentSvcs](i),
		teacherRepo:           db.NewRepo[contactmodels.Teacher](i),
		supervisorSvc:         do.MustInvoke[contact.SupervisorSvc](i),
		dbClient:              do.MustInvoke[*mongo.Client](i),
	}, nil
}

// canReplyToQuestion checks if user is a teacher assigned to the course or coaching
func (s *studentquestionsvcs) canReplyToQuestion(ctx context.Context, question *models.StudentQuestion) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	// Check if user is a teacher assigned through teacher assignment
	var teacherIds []primitive.ObjectID
	if question.RefType == "course" {
		teacherIds, err = s.teacherAssignmentSvc.GetCourseTeachers(ctx, question.RefId)
		if err != nil {
			return false, err
		}
	} else if question.RefType == "coaching" {
		teacherIds, err = s.teacherAssignmentSvc.GetCoachingTeachers(ctx, question.RefId)
		if err != nil {
			return false, err
		}
	}

	// Check if user is in the teacherIds array
	for _, teacherId := range teacherIds {
		if teacherId == cfg.User.Id {
			return true, nil
		}
	}

	return false, nil
}

// validateLessonInCourse checks if a lesson exists in a course's content structure
func (s *studentquestionsvcs) validateLessonInCourse(ctx context.Context, courseId, lessonId primitive.ObjectID) error {
	course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": courseId, "trash": false})
	if err != nil {
		return content_constant.ErrCourseNotFound
	}

	// Recursive function to check if lesson exists in sections and sub-sections
	var checkLesson func(sections []models.Section) bool
	checkLesson = func(sections []models.Section) bool {
		for _, section := range sections {
			for _, lesson := range section.Lessons {
				if lesson.Id == lessonId {
					return true
				}
			}
			if len(section.SubSections) > 0 && checkLesson(section.SubSections) {
				return true
			}
		}
		return false
	}

	if !checkLesson(course.Content) {
		return content_constant.ErrLessonNotFound
	}

	return nil
}

// GetStudentQuestionsByRef retrieves all questions for a specific ref (course or coaching)
func (s *studentquestionsvcs) GetStudentQuestionsByRef(ctx context.Context, refType string, refId string, skip, limit int64, query *query.Conditions) (*models.StudentQuestionPagination, error) {
	_refId, err := primitive.ObjectIDFromHex(refId)
	if err != nil {
		return nil, content_constant.ErrInvalidRefId
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{
			"refId":   _refId,
			"refType": refType,
			"trash":   false,
			"$and":    []bson.M{filter},
		}},
	}

	count, err := s.questionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateStudentField...)
	pipeline = append(pipeline, populateReplyTeacherField...)
	pipeline = append(pipeline, populateRefField...)

	var result []models.StudentQuestionRes
	if err := s.questionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Convert embedded reply to reply res format using populated teacher data
	for i := range result {
		if result[i].StudentQuestion.Reply != nil {
			result[i].Reply = &models.StudentQuestionReplyRes{
				StudentQuestionReply: *result[i].StudentQuestion.Reply,
				Teacher:              result[i].ReplyTeacher,
			}
		}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.StudentQuestionPagination{
		Questions:  result,
		Pagination: pagination,
	}, nil
}

// GetStudentQuestions retrieves all questions from all refs with role-based filtering and pagination
func (s *studentquestionsvcs) GetStudentQuestions(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentQuestionPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user is admin
	isAdmin, err := util.HasCapability(ctx, CapabilityReplyToAllStudentQuestions)
	if err != nil {
		return nil, err
	}

	// Build base filter
	baseFilter := bson.M{
		"trash": false,
	}

	// Apply role-based filtering
	if !isAdmin {
		// Check if user is a teacher or supervisor
		// For teachers: only show questions where they are assigned
		// For supervisors: only show questions where their teachers are assigned

		// Collect all refIds that the user has access to
		accessibleRefIds := make(map[primitive.ObjectID]bool)

		// Get teacher assignments for the current user
		var teacherAssignments []models.TeacherAssignment
		err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"teacherIds": cfg.User.Id,
				"trash":      false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teacherAssignments)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}

		// Add refs from teacher assignments
		for _, assignment := range teacherAssignments {
			accessibleRefIds[assignment.TargetId] = true
		}

		// Check if user is a supervisor
		teachers, err := s.supervisorSvc.GetTeachersBySupervisorId(ctx, cfg.User.Id.Hex(), nil)
		if err != nil {
			// If error, assume not a supervisor
			teachers = []contactmodels.Teacher{}
		}

		// Add refs from supervised teachers
		if len(teachers) > 0 {
			teacherIds := make([]primitive.ObjectID, len(teachers))
			for i, teacher := range teachers {
				teacherIds[i] = teacher.Id
			}

			// Get teacher assignments for supervised teachers
			var supervisedAssignments []models.TeacherAssignment
			err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
				{"$match": bson.M{
					"teacherIds": bson.M{"$in": teacherIds},
					"trash":      false,
				}},
			}, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &supervisedAssignments)
			})
			if err != nil && err != mongo.ErrNoDocuments {
				return nil, err
			}

			for _, assignment := range supervisedAssignments {
				accessibleRefIds[assignment.TargetId] = true
			}
		}

		// If no accessible refs, return empty result
		if len(accessibleRefIds) == 0 {
			return &models.StudentQuestionPagination{
				Questions: []models.StudentQuestionRes{},
				Pagination: types.Pagination{
					TotalPages: 0,
					PerPage:    limit,
					TotalCount: 0,
				},
			}, nil
		}

		// Build filter for accessible refs
		refIdsList := make([]primitive.ObjectID, 0, len(accessibleRefIds))
		for refId := range accessibleRefIds {
			refIdsList = append(refIdsList, refId)
		}

		baseFilter["refId"] = bson.M{"$in": refIdsList}
	}

	// Apply query conditions
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		queryFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		if len(queryFilter) > 0 {
			// Merge query filter with base filter
			baseFilter["$and"] = []bson.M{queryFilter}
		}
	}

	// Build pipeline
	pipeline := []bson.M{
		{"$match": baseFilter},
	}

	count, err := s.questionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, populateStudentField...)
	pipeline = append(pipeline, populateReplyTeacherField...)
	pipeline = append(pipeline, populateRefField...)

	var result []models.StudentQuestionRes
	if err := s.questionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Convert embedded reply to reply res format using populated teacher data
	for i := range result {
		if result[i].StudentQuestion.Reply != nil {
			result[i].Reply = &models.StudentQuestionReplyRes{
				StudentQuestionReply: *result[i].StudentQuestion.Reply,
				Teacher:              result[i].ReplyTeacher,
			}
		}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.StudentQuestionPagination{
		Questions:  result,
		Pagination: pagination,
	}, nil
}

// GetStudentQuestion retrieves a single question by ID with replies
func (s *studentquestionsvcs) GetStudentQuestion(ctx context.Context, questionId string) (*models.StudentQuestionRes, error) {
	_id, err := primitive.ObjectIDFromHex(questionId)
	if err != nil {
		return nil, content_constant.ErrInvalidQuestionId
	}

	// Single aggregation to get question with student and teacher data
	pipeline := []bson.M{
		{"$match": bson.M{
			"_id":   _id,
			"trash": false,
		}},
	}
	pipeline = append(pipeline, populateStudentField...)
	pipeline = append(pipeline, populateReplyTeacherField...)
	pipeline = append(pipeline, populateRefField...)

	var questions []models.StudentQuestionRes
	if err := s.questionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &questions)
	}); err != nil {
		return nil, err
	}

	if len(questions) == 0 {
		return nil, content_constant.ErrQuestionNotFound
	}

	result := questions[0]

	// Convert embedded reply to reply res format using populated teacher data
	if result.StudentQuestion.Reply != nil {
		result.Reply = &models.StudentQuestionReplyRes{
			StudentQuestionReply: *result.StudentQuestion.Reply,
			Teacher:              result.ReplyTeacher,
		}
	}

	return &result, nil
}

// GetTeacherQuestionStats retrieves statistics for the current teacher's questions
func (s *studentquestionsvcs) GetTeacherQuestionStats(ctx context.Context, refType *string, refId *string) (*models.StudentQuestionTeacherStats, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get accessible course IDs (similar to GetStudentQuestions logic)
	accessibleRefIds := make(map[primitive.ObjectID]bool)

	// If refId is provided, validate and use it directly
	if refId != nil {
		_refId, err := primitive.ObjectIDFromHex(*refId)
		if err != nil {
			return nil, content_constant.ErrInvalidRefId
		}

		// Verify teacher has access to this ref using teacher assignment service
		hasAccess := false
		var teacherIds []primitive.ObjectID

		// Determine refType - default to "course" if not provided
		refTypeStr := "course"
		if refType != nil {
			refTypeStr = *refType
		}

		if refTypeStr == "course" {
			teacherIds, err = s.teacherAssignmentSvc.GetCourseTeachers(ctx, _refId)
		} else if refTypeStr == "coaching" {
			teacherIds, err = s.teacherAssignmentSvc.GetCoachingTeachers(ctx, _refId)
		}

		if err != nil {
			return nil, err
		}

		// Check if user is in the teacherIds array
		for _, teacherId := range teacherIds {
			if teacherId == cfg.User.Id {
				hasAccess = true
				break
			}
		}

		if !hasAccess {
			return nil, content_constant.ErrCannotReplyToQuestion
		}

		accessibleRefIds[_refId] = true
	} else {
		// Get all courses/coachings where teacher is assigned
		matchFilter := bson.M{
			"teacherIds": cfg.User.Id,
			"trash":      false,
		}

		// Filter by type if provided
		if refType != nil {
			matchFilter["type"] = *refType
		}

		var teacherAssignments []models.TeacherAssignment
		err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
			{"$match": matchFilter},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teacherAssignments)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}

		// Add refs from teacher assignments
		for _, assignment := range teacherAssignments {
			accessibleRefIds[assignment.TargetId] = true
		}
	}

	// If no accessible refs, return zero stats
	if len(accessibleRefIds) == 0 {
		return &models.StudentQuestionTeacherStats{
			TotalQuestions: 0,
			TotalReplied:   0,
		}, nil
	}

	// Build filter for accessible refs
	refIdsList := make([]primitive.ObjectID, 0, len(accessibleRefIds))
	for refId := range accessibleRefIds {
		refIdsList = append(refIdsList, refId)
	}

	// Build match filter
	matchFilter := bson.M{
		"refId": bson.M{"$in": refIdsList},
		"trash": false,
	}

	// Add refType filter if provided
	if refType != nil {
		matchFilter["refType"] = *refType
	} else {
		matchFilter["refType"] = "course" // Default to course
	}

	// Aggregation pipeline to count questions and replies
	pipeline := []bson.M{
		{"$match": matchFilter},
		{
			"$group": bson.M{
				"_id": nil,
				"totalQuestions": bson.M{
					"$sum": 1,
				},
				"totalReplied": bson.M{
					"$sum": bson.M{
						"$cond": []interface{}{
							bson.M{
								"$and": []interface{}{
									bson.M{"$ne": []interface{}{bson.M{"$type": []interface{}{"$reply"}}, "missing"}},
									bson.M{"$ne": []interface{}{"$reply", nil}},
								},
							},
							1,
							0,
						},
					},
				},
			},
		},
	}

	var statsResult []struct {
		TotalQuestions int64 `bson:"totalQuestions"`
		TotalReplied   int64 `bson:"totalReplied"`
	}

	err = s.questionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &statsResult)
	})
	if err != nil {
		return nil, err
	}

	// If no results, return zero stats
	if len(statsResult) == 0 {
		return &models.StudentQuestionTeacherStats{
			TotalQuestions: 0,
			TotalReplied:   0,
		}, nil
	}

	return &models.StudentQuestionTeacherStats{
		TotalQuestions: statsResult[0].TotalQuestions,
		TotalReplied:   statsResult[0].TotalReplied,
	}, nil
}

// GetSupervisorQuestionStats retrieves statistics for supervisor's teachers with pagination
func (s *studentquestionsvcs) GetSupervisorQuestionStats(ctx context.Context, refType *string, refId *string, skip, limit int64) (*models.StudentQuestionSupervisorStats, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user is admin
	isAdmin, err := util.HasCapability(ctx, CapabilityReplyToAllStudentQuestions)
	if err != nil {
		return nil, err
	}

	var teachers []contactmodels.Teacher
	if isAdmin {
		// Admin: get all teachers (or filter by refId if provided)
		if refId != nil {
			// If refId provided, get teachers assigned to that course
			_refId, err := primitive.ObjectIDFromHex(*refId)
			if err != nil {
				return nil, content_constant.ErrInvalidRefId
			}

			// Get teachers from teacher assignment
			teacherIds, err := s.teacherAssignmentSvc.GetCourseTeachers(ctx, _refId)
			if err != nil {
				return nil, err
			}

			if len(teacherIds) > 0 {
				err = s.teacherRepo.Aggregate(ctx, []bson.M{
					{"$match": bson.M{
						"_id":   bson.M{"$in": teacherIds},
						"trash": false,
					}},
				}, func(cur *mongo.Cursor) error {
					return cur.All(ctx, &teachers)
				})
				if err != nil && err != mongo.ErrNoDocuments {
					return nil, err
				}
			}
		} else {
			// Get all teachers
			err = s.teacherRepo.Aggregate(ctx, []bson.M{
				{"$match": bson.M{"trash": false}},
			}, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &teachers)
			})
			if err != nil && err != mongo.ErrNoDocuments {
				return nil, err
			}
		}
	} else {
		// Supervisor: get supervised teachers
		teachers, err = s.supervisorSvc.GetTeachersBySupervisorId(ctx, cfg.User.Id.Hex(), nil)
		if err != nil {
			return nil, err
		}

		// If refId provided, filter teachers to only those assigned to that course
		if refId != nil {
			_refId, err := primitive.ObjectIDFromHex(*refId)
			if err != nil {
				return nil, content_constant.ErrInvalidRefId
			}

			// Verify course exists
			_, err = s.courseRepo.GetByFilter(ctx, bson.M{"_id": _refId, "trash": false})
			if err != nil {
				return nil, content_constant.ErrCourseNotFound
			}

			// Filter teachers to only those assigned to this course using TeacherAssignment
			assignedTeacherIds, err := s.teacherAssignmentSvc.GetCourseTeachers(ctx, _refId)
			if err != nil {
				return nil, err
			}

			assignedTeacherIdsMap := make(map[primitive.ObjectID]bool)
			for _, teacherId := range assignedTeacherIds {
				assignedTeacherIdsMap[teacherId] = true
			}

			// Filter teachers list
			filteredTeachers := []contactmodels.Teacher{}
			for _, teacher := range teachers {
				if assignedTeacherIdsMap[teacher.Id] {
					filteredTeachers = append(filteredTeachers, teacher)
				}
			}
			teachers = filteredTeachers
		}
	}

	if len(teachers) == 0 {
		return &models.StudentQuestionSupervisorStats{
			TotalQuestions: 0,
			TotalReplied:   0,
			AvgReplyTime:   0,
			Teachers:       []models.StudentQuestionTeacherBreakdown{},
			Pagination: types.Pagination{
				TotalPages: 0,
				PerPage:    limit,
				TotalCount: 0,
			},
		}, nil
	}

	// Get teacher IDs
	teacherIds := make([]primitive.ObjectID, len(teachers))
	for i, teacher := range teachers {
		teacherIds[i] = teacher.Id
	}

	// Get accessible course IDs
	accessibleRefIds := make(map[primitive.ObjectID]bool)

	if refId != nil {
		_refId, err := primitive.ObjectIDFromHex(*refId)
		if err != nil {
			return nil, content_constant.ErrInvalidRefId
		}
		accessibleRefIds[_refId] = true
	} else {
		// Get all courses where any of these teachers are assigned
		var teacherAssignments []models.TeacherAssignment
		err = s.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"teacherIds": bson.M{"$in": teacherIds},
				"type":       "course",
				"trash":      false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teacherAssignments)
		})
		if err != nil && err != mongo.ErrNoDocuments {
			return nil, err
		}

		for _, assignment := range teacherAssignments {
			accessibleRefIds[assignment.TargetId] = true
		}
	}

	// Note: We don't return early if accessibleRefIds is empty because we still want to show teachers
	// even if they have no courses (they'll have zero stats)

	refIdsList := make([]primitive.ObjectID, 0, len(accessibleRefIds))
	for refId := range accessibleRefIds {
		refIdsList = append(refIdsList, refId)
	}

	// Phase 1: Calculate aggregate stats (all questions)
	// If no accessible courses, stats will be zero
	var aggregatePipeline []bson.M
	if len(refIdsList) > 0 {
		// Build match filter
		matchFilter := bson.M{
			"refId": bson.M{"$in": refIdsList},
			"trash": false,
		}

		if refType != nil {
			matchFilter["refType"] = *refType
		} else {
			matchFilter["refType"] = "course"
		}

		aggregatePipeline = []bson.M{
			{"$match": matchFilter},
			{
				"$group": bson.M{
					"_id":            nil,
					"totalQuestions": bson.M{"$sum": 1},
					"totalReplied": bson.M{
						"$sum": bson.M{
							"$cond": []interface{}{
								bson.M{
									"$and": []interface{}{
										bson.M{"$ne": []interface{}{bson.M{"$type": []interface{}{"$reply"}}, "missing"}},
										bson.M{"$ne": []interface{}{"$reply", nil}},
									},
								},
								1,
								0,
							},
						},
					},
					"replyTimes": bson.M{
						"$push": bson.M{
							"$cond": []interface{}{
								bson.M{"$ne": []interface{}{"$reply", nil}},
								bson.M{
									"$divide": []interface{}{
										bson.M{"$subtract": []interface{}{"$reply.createdAt", "$createdAt"}},
										3600000, // Convert milliseconds to hours (1000ms * 3600s)
									},
								},
								nil,
							},
						},
					},
				},
			},
			{
				"$project": bson.M{
					"totalQuestions": 1,
					"totalReplied":   1,
					"avgReplyTime": bson.M{
						"$avg": bson.M{
							"$filter": bson.M{
								"input": "$replyTimes",
								"as":    "time",
								"cond":  bson.M{"$ne": []interface{}{"$$time", nil}},
							},
						},
					},
				},
			},
		}
	} else {
		// No accessible courses, so no questions - stats will be zero
		aggregatePipeline = []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": []primitive.ObjectID{}}}}, // Match nothing
			{
				"$group": bson.M{
					"_id":            nil,
					"totalQuestions": bson.M{"$sum": 1},
					"totalReplied":   bson.M{"$sum": 0},
					"replyTimes":     bson.M{"$push": nil},
				},
			},
			{
				"$project": bson.M{
					"totalQuestions": 1,
					"totalReplied":   1,
					"avgReplyTime":   bson.M{"$literal": 0},
				},
			},
		}
	}

	var aggregateResult []struct {
		TotalQuestions int64   `bson:"totalQuestions"`
		TotalReplied   int64   `bson:"totalReplied"`
		AvgReplyTime   float64 `bson:"avgReplyTime"`
	}

	if len(aggregatePipeline) > 0 {
		err = s.questionRepo.Aggregate(ctx, aggregatePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &aggregateResult)
		})
		if err != nil {
			return nil, err
		}
	}

	var totalQuestions int64
	var totalReplied int64
	var avgReplyTime float64

	if len(aggregateResult) > 0 {
		totalQuestions = aggregateResult[0].TotalQuestions
		totalReplied = aggregateResult[0].TotalReplied
		avgReplyTime = aggregateResult[0].AvgReplyTime
	}

	// Phase 2: Calculate per-teacher stats with pagination
	// Get paginated teachers
	teacherMap := make(map[primitive.ObjectID]contactmodels.Teacher)
	for _, teacher := range teachers {
		teacherMap[teacher.Id] = teacher
	}

	// Sort teachers by ID for consistent pagination
	teacherIdsSorted := make([]primitive.ObjectID, len(teachers))
	for i, teacher := range teachers {
		teacherIdsSorted[i] = teacher.Id
	}

	// Apply pagination
	totalTeachers := int64(len(teachers))
	totalPages := math.Ceil(float64(totalTeachers) / float64(limit))

	paginatedTeacherIds := teacherIdsSorted
	if int64(len(paginatedTeacherIds)) > skip {
		end := skip + limit
		if int64(len(paginatedTeacherIds)) < end {
			end = int64(len(paginatedTeacherIds))
		}
		paginatedTeacherIds = paginatedTeacherIds[skip:end]
	} else {
		paginatedTeacherIds = []primitive.ObjectID{}
	}

	// Calculate stats for each paginated teacher
	teacherBreakdowns := make([]models.StudentQuestionTeacherBreakdown, 0, len(paginatedTeacherIds))

	for _, teacherId := range paginatedTeacherIds {
		teacher := teacherMap[teacherId]

		// Get courses for this teacher using teacher assignment service
		teacherCourseIds := []primitive.ObjectID{}
		for refId := range accessibleRefIds {
			// Check if teacher is assigned to this course using teacher assignment service
			courseTeacherIds, err := s.teacherAssignmentSvc.GetCourseTeachers(ctx, refId)
			if err != nil {
				continue
			}

			// Check if this teacher is in the list
			for _, tid := range courseTeacherIds {
				if tid == teacherId {
					teacherCourseIds = append(teacherCourseIds, refId)
					break
				}
			}
		}

		// Get stats for this teacher's questions
		var teacherStatsPipeline []bson.M
		if len(teacherCourseIds) > 0 {
			teacherMatchFilter := bson.M{
				"refId": bson.M{"$in": teacherCourseIds},
				"trash": false,
			}
			if refType != nil {
				teacherMatchFilter["refType"] = *refType
			} else {
				teacherMatchFilter["refType"] = "course"
			}

			teacherStatsPipeline = []bson.M{
				{"$match": teacherMatchFilter},
				{
					"$group": bson.M{
						"_id":            nil,
						"totalQuestions": bson.M{"$sum": 1},
						"totalReplied": bson.M{
							"$sum": bson.M{
								"$cond": []interface{}{
									bson.M{
										"$and": []interface{}{
											bson.M{"$ne": []interface{}{bson.M{"$type": []interface{}{"$reply"}}, "missing"}},
											bson.M{"$ne": []interface{}{"$reply", nil}},
										},
									},
									1,
									0,
								},
							},
						},
						"replyTimes": bson.M{
							"$push": bson.M{
								"$cond": []interface{}{
									bson.M{
										"$and": []interface{}{
											bson.M{"$ne": []interface{}{bson.M{"$type": []interface{}{"$reply"}}, "missing"}},
											bson.M{"$ne": []interface{}{"$reply", nil}},
										},
									},
									bson.M{
										"$divide": []interface{}{
											bson.M{"$subtract": []interface{}{"$reply.createdAt", "$createdAt"}},
											3600000, // Convert milliseconds to hours (1000ms * 3600s)
										},
									},
									nil,
								},
							},
						},
					},
				},
				{
					"$project": bson.M{
						"totalQuestions": 1,
						"totalReplied":   1,
						"avgReplyTime": bson.M{
							"$avg": bson.M{
								"$filter": bson.M{
									"input": "$replyTimes",
									"as":    "time",
									"cond":  bson.M{"$ne": []interface{}{"$$time", nil}},
								},
							},
						},
					},
				},
			}
		}

		var teacherStatsResult []struct {
			TotalQuestions int64   `bson:"totalQuestions"`
			TotalReplied   int64   `bson:"totalReplied"`
			AvgReplyTime   float64 `bson:"avgReplyTime"`
		}

		var teacherTotalQuestions int64
		var teacherTotalReplied int64
		var teacherAvgReplyTime float64

		if len(teacherStatsPipeline) > 0 {
			err = s.questionRepo.Aggregate(ctx, teacherStatsPipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &teacherStatsResult)
			})
			if err != nil {
				return nil, err
			}

			if len(teacherStatsResult) > 0 {
				teacherTotalQuestions = teacherStatsResult[0].TotalQuestions
				teacherTotalReplied = teacherStatsResult[0].TotalReplied
				teacherAvgReplyTime = teacherStatsResult[0].AvgReplyTime
			}
		}

		teacherBreakdowns = append(teacherBreakdowns, models.StudentQuestionTeacherBreakdown{
			Id:             teacher.Id,
			FirstName:      teacher.FirstName,
			LastName:       teacher.LastName,
			TotalQuestions: teacherTotalQuestions,
			TotalReplied:   teacherTotalReplied,
			AvgReplyTime:   teacherAvgReplyTime,
		})
	}

	return &models.StudentQuestionSupervisorStats{
		TotalQuestions: totalQuestions,
		TotalReplied:   totalReplied,
		AvgReplyTime:   avgReplyTime,
		Teachers:       teacherBreakdowns,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: totalTeachers,
		},
	}, nil
}

// AddStudentQuestion creates a new student question
func (s *studentquestionsvcs) AddStudentQuestion(ctx context.Context, refType string, data *models.StudentQuestionDto) (*models.StudentQuestion, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate refType
	if refType != "course" && refType != "coaching" {
		return nil, content_constant.ErrInvalidRefId
	}

	// Validate lessonId if provided (only valid when refType is "course")
	if data.LessonId != nil {
		if refType != "course" {
			return nil, content_constant.ErrLessonNotFound
		}
		if err := s.validateLessonInCourse(ctx, data.RefId, *data.LessonId); err != nil {
			return nil, err
		}
	}

	// Validate that refId exists
	if refType == "course" {
		_, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": data.RefId, "trash": false})
		if err != nil {
			return nil, content_constant.ErrCourseNotFound
		}
	} else if refType == "coaching" {
		_, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": data.RefId, "trash": false})
		if err != nil {
			return nil, content_constant.ErrCoachingNotFound
		}
	}

	question := &models.StudentQuestion{
		Id:                 primitive.NewObjectID(),
		StudentId:          cfg.User.Id,
		RefType:            refType,
		StudentQuestionDto: *data, // RefId and LessonId are included in the DTO
		Reply:              nil,   // Initialize with no reply
		CreatedAt:          time.Now(),
		CreatedBy:          cfg.User.Id,
		UpdatedAt:          time.Now(),
		UpdatedBy:          cfg.User.Id,
		Trash:              false,
	}

	if err := s.questionRepo.Add(ctx, question); err != nil {
		return nil, err
	}

	return question, nil
}

// UpdateStudentQuestion updates an existing student question
func (s *studentquestionsvcs) UpdateStudentQuestion(ctx context.Context, questionId string, data *models.StudentQuestionDto) (*models.StudentQuestion, error) {
	_id, err := primitive.ObjectIDFromHex(questionId)
	if err != nil {
		return nil, content_constant.ErrInvalidQuestionId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Check if question exists and belongs to the user
	question, err := s.questionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, content_constant.ErrQuestionNotFound
	}

	if question.StudentId != cfg.User.Id {
		return nil, content_constant.ErrCannotUpdateQuestion
	}

	// Validate lessonId if provided (only valid when refType is "course")
	// Use the existing question's refId since refId is not updated
	if data.LessonId != nil {
		if err := s.validateLessonInCourse(ctx, question.RefId, *data.LessonId); err != nil {
			return nil, err
		}
	}

	update := bson.M{
		"$set": bson.M{
			"title":       data.Title,
			"description": data.Description,
			"updatedAt":   time.Now(),
			"updatedBy":   cfg.User.Id,
		},
	}

	// Update lessonId if provided (can be set or unset)
	if data.LessonId != nil {
		update["$set"].(bson.M)["lessonId"] = *data.LessonId
	} else {
		// If lessonId is explicitly set to nil/omitted, we could unset it
		// For now, we'll only update if it's provided
	}

	updatedQuestion, err := s.questionRepo.Patch(ctx, bson.M{"_id": _id, "trash": false}, update)
	if err != nil {
		return nil, content_constant.ErrQuestionNotFound
	}

	return updatedQuestion, nil
}

// DeleteStudentQuestion soft deletes a student question
func (s *studentquestionsvcs) DeleteStudentQuestion(ctx context.Context, questionId string) error {
	_id, err := primitive.ObjectIDFromHex(questionId)
	if err != nil {
		return content_constant.ErrInvalidQuestionId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Check if question exists and belongs to the user
	question, err := s.questionRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return content_constant.ErrQuestionNotFound
	}

	if question.StudentId != cfg.User.Id {
		return content_constant.ErrCannotDeleteQuestion
	}

	update := bson.M{
		"$set": bson.M{
			"trash":     true,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	if _, err := s.questionRepo.Patch(ctx, bson.M{"_id": _id, "trash": false}, update); err != nil {
		return content_constant.ErrQuestionNotFound
	}

	return nil
}

// AddReply adds a reply to a student question (for teachers/supervisors/admins)
func (s *studentquestionsvcs) AddReply(ctx context.Context, questionId string, data *models.StudentQuestionReplyDto) (*models.StudentQuestionReply, error) {
	_questionId, err := primitive.ObjectIDFromHex(questionId)
	if err != nil {
		return nil, content_constant.ErrInvalidQuestionId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Verify question exists
	question, err := s.questionRepo.GetByFilter(ctx, bson.M{"_id": _questionId, "trash": false})
	if err != nil {
		return nil, content_constant.ErrQuestionNotFound
	}

	// Check authorization: admin can reply to all, teacher/supervisor can reply if assigned to course
	canReply, err := s.canReplyToQuestion(ctx, question)
	if err != nil {
		return nil, err
	}
	if !canReply {
		return nil, content_constant.ErrCannotReplyToQuestion
	}

	reply := models.StudentQuestionReply{
		Id:                      primitive.NewObjectID(),
		UserId:                  cfg.User.Id,
		StudentQuestionReplyDto: *data,
		CreatedAt:               time.Now(),
		CreatedBy:               cfg.User.Id,
		UpdatedAt:               time.Now(),
		UpdatedBy:               cfg.User.Id,
		Trash:                   false,
	}

	// Set/update the single reply using $set
	update := bson.M{
		"$set": bson.M{
			"reply":     reply,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	if _, err := s.questionRepo.Patch(ctx, bson.M{"_id": _questionId, "trash": false}, update); err != nil {
		return nil, err
	}

	return &reply, nil
}

// UpdateReply updates an existing reply
func (s *studentquestionsvcs) UpdateReply(ctx context.Context, replyId string, data *models.StudentQuestionReplyDto) (*models.StudentQuestionReply, error) {
	_id, err := primitive.ObjectIDFromHex(replyId)
	if err != nil {
		return nil, content_constant.ErrInvalidReplyId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Find the question containing this reply
	question, err := s.questionRepo.GetByFilter(ctx, bson.M{
		"reply._id": _id,
		"trash":     false,
	})
	if err != nil {
		return nil, content_constant.ErrReplyNotFound
	}

	// Check if reply exists and belongs to the user
	if question.Reply == nil || question.Reply.Id != _id {
		return nil, content_constant.ErrReplyNotFound
	}

	if question.Reply.UserId != cfg.User.Id {
		return nil, content_constant.ErrCannotUpdateReply
	}

	// Update the single reply
	update := bson.M{
		"$set": bson.M{
			"reply.content":   data.Content,
			"reply.updatedAt": time.Now(),
			"reply.updatedBy": cfg.User.Id,
			"updatedAt":       time.Now(),
			"updatedBy":       cfg.User.Id,
		},
	}

	if _, err := s.questionRepo.Patch(ctx, bson.M{"_id": question.Id, "trash": false}, update); err != nil {
		return nil, err
	}

	// Fetch updated reply
	updatedQuestion, err := s.questionRepo.GetByFilter(ctx, bson.M{"_id": question.Id, "trash": false})
	if err != nil {
		return nil, err
	}

	if updatedQuestion.Reply != nil && updatedQuestion.Reply.Id == _id {
		return updatedQuestion.Reply, nil
	}

	return nil, content_constant.ErrReplyNotFound
}

// DeleteReply hard deletes a reply by removing it from the question
func (s *studentquestionsvcs) DeleteReply(ctx context.Context, replyId string) error {
	_id, err := primitive.ObjectIDFromHex(replyId)
	if err != nil {
		return content_constant.ErrInvalidReplyId
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Find the question containing this reply
	question, err := s.questionRepo.GetByFilter(ctx, bson.M{
		"reply._id": _id,
		"trash":     false,
	})
	if err != nil {
		return content_constant.ErrReplyNotFound
	}

	// Check if reply exists and belongs to the user
	if question.Reply == nil || question.Reply.Id != _id {
		return content_constant.ErrReplyNotFound
	}

	if question.Reply.UserId != cfg.User.Id {
		return content_constant.ErrCannotDeleteReply
	}

	// Hard delete the reply by removing it completely
	update := bson.M{
		"$unset": bson.M{
			"reply": "",
		},
		"$set": bson.M{
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	if _, err := s.questionRepo.Patch(ctx, bson.M{"_id": question.Id, "trash": false}, update); err != nil {
		return err
	}

	return nil
}
