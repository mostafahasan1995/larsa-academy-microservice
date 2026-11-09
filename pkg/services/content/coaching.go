package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/gateway"
	"larsa-academy-microservice/pkg/helpers"
	querypkg "larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/compensation"
	contactsvcs "larsa-academy-microservice/pkg/services/contact"
	teachermodels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/messaging"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/services/zoom"
	zoommodels "larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"log"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CoachingSvcs interface {
	// DASHBOARD
	GetById(ctx context.Context, id string, enrichCoachs bool) (*models.EnrichedCoaching, error)
	Get(ctx context.Context, skip, limit int64, query *querypkg.Conditions, includePlacementTests bool, enrichCoachs bool, enrichPrice bool) (*models.EnrichedCoachingListPagination, error)
	GetAll(ctx context.Context, query *querypkg.Conditions, enrichCoachs bool) ([]models.EnrichedCoaching, error)
	Add(ctx context.Context, data *models.CoachingDto) (*models.Coaching, error)
	Update(ctx context.Context, coachingId string, data *models.CoachingDto) (*models.Coaching, error)
	Delete(ctx context.Context, coachingId string) error
	GetCoachingProgramMembers(ctx context.Context, coachingProgramId string, skip, limit int64, query *querypkg.Conditions) (*models.CoachingProgramMemberPagination, error)
	// Remove a member (soft-delete all sessions for the student in the coaching program)
	RemoveCoachingProgramMember(ctx context.Context, coachingProgramId string, studentId string) error
	GetEnrolledCoachings(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions) (*models.EnrolledCoachingPagination, error)
	GetStudentSessionResources(ctx context.Context, coachingProgramId, resourceType string, sessionNum int) ([]models.StudentSessionResourceResponse, error)
	// Coaching Assignment methods
	GetCoachingAllAssignments(ctx context.Context, coachingId string, query *querypkg.Conditions) ([]models.CoachingAssignment, error)
	AddCoachingAssignment(ctx context.Context, coachingId string, data *models.CoachingAssignmentDto) (*models.CoachingAssignment, error)
	UpdateCoachingAssignment(ctx context.Context, coachingId, assignmentId string, data *models.CoachingAssignmentDto) (*models.CoachingAssignment, error)
	DeleteCoachingAssignment(ctx context.Context, assignmentId string) error
	GetCoachingAssignmentStudentSubmissions(ctx context.Context, coachingId, assignmentId string, query *querypkg.Conditions) ([]models.CoachingStudentAssignmentRes, error)
	GetCoachingStudentAssignment(ctx context.Context, studentAssignmentId string) (*models.CoachingStudentAssignmentRes, error)
	// Coaching Test methods
	GetCoachingAllTests(ctx context.Context, coachingId string, query *querypkg.Conditions) ([]models.CoachingTest, error)
	AddCoachingTest(ctx context.Context, coachingId string, data *models.CoachingTestDto) (*models.CoachingTest, error)
	UpdateCoachingTest(ctx context.Context, coachingId, testId string, data *models.CoachingTestDto) (*models.CoachingTest, error)
	DeleteCoachingTest(ctx context.Context, testId string) error
	GetCoachingTestStudentSubmissions(ctx context.Context, coachingId, studentTestId string, query *querypkg.Conditions) ([]models.CoachingStudentTestRes, error)
	GetCoachingStudentTest(ctx context.Context, studentTestId string) (*models.CoachingStudentTestRes, error)

	// pricing
	AddCoachingPricing(ctx context.Context, coachingId string, data *models.CoachingPricingDto) (*models.CoachingPricing, error)
	UpdateCoachingPricing(ctx context.Context, coachingId string, pricingId string, data *models.CoachingPricingDto) (*models.CoachingPricing, error)
	DeleteCoachingPricing(ctx context.Context, coachingId string, pricingId string) error
	GetCoachingPricing(ctx context.Context, coachingId string, skip, limit int64, query *querypkg.Conditions) (*models.CoachingPricingPagination, error)

	// MOBILE
	// coaching student assignment
	GetCoachingAssignmentsForStudent(ctx context.Context, coachingId string) ([]models.CoachingAssignmentWithAttempt, error)
	StartCoachingAssignment(ctx context.Context, assignmentId string) (*models.CoachingStudentAssignment, error)
	SubmitCoachingStudentAssignment(ctx context.Context, studentAssignmentId string, data *models.StudentCoachingAssignmentAnswers) (*models.CoachingStudentAssignment, error)
	UpdateCoachingStudentAssignmentReviewStatus(ctx context.Context, studentAssignmentId string, reviewStatus string) (*models.CoachingStudentAssignment, error)
	// coaching student test
	StartCoachingStudentTest(ctx context.Context, testId string) (*models.CoachingStudentTest, error)
	SubmitCoachingStudentTest(ctx context.Context, studentTestId string, data *models.StudentCoachingTestAnswers) (*models.CoachingStudentTest, error)
	UpdateCoachingStudentTestReviewStatus(ctx context.Context, studentTestId string, reviewStatus string) (*models.CoachingStudentTest, error)

	// student session resources
	UpdateStudentSessionNotes(ctx context.Context, studentSessionId string, data *models.UpdateSessionNotesDto) (*models.StudentSession, error)
	UpdateStudentSessionAgendas(ctx context.Context, studentSessionId string, data *models.UpdateSessionAgendasDto) (*models.StudentSession, error)
	UpdateStudentSessionResources(ctx context.Context, studentSessionId string, data *models.UpdateSessionResourcesDto) (*models.StudentSession, error)
	UpdateStudentSessionFeedback(ctx context.Context, studentSessionId string, data *models.UpdateSessionFeedbackDto) (*models.StudentSession, error)
	//
	GetStudentsPrograms(ctx context.Context, studentId string, requesterId string) ([]models.MyCoachingProgramResponse, error)
	GetSessionsByStage(ctx context.Context, coachingProgramId string, stage string) ([]models.StudentSession, error)
	// justifications
	AbsenceJustificationRequest(ctx context.Context, studentSessionId string, data *models.AbsenceJustificationDto) (*models.StudentSession, error)
	JustificationDecision(ctx context.Context, StudentSessionId string, data *models.JustificationDecision) (*models.StudentSession, error)
	Reschedule(ctx context.Context, studentSessionId string, data *models.RescheduleSessionData) (*models.StudentSession, error)
	RescheduleRequest(ctx context.Context, studentSessionId string, data *models.RescheduleJustificationDto) (*models.StudentSession, error)
	BookingSessions(ctx context.Context, data *models.BookingSessions) (*models.BookingSessions, error)
	GetTeacherAvailability(ctx context.Context, teacherId string, coachingProgramId string) (*models.TeacherAvailabilityResponse, error)
	GetCoachingTeachersAvailability(ctx context.Context, coachingProgramId string) (*models.CoachingTeachersAvailabilityResponse, error)
	GetCoachingGroupedAvailability(ctx context.Context, coachingProgramId string) (*models.GroupedAvailabilityResponse, error)
	// supervisor session details
	GetSessionDetailsForSupervisor(ctx context.Context, sessionId string, supervisorId string) (*models.SessionDetailsResponse, error)
	// Calendar event management
	UpdateStudentSessionTime(ctx context.Context, studentSessionId string, data *models.UpdateSessionTimeData) (*models.StudentSession, error)
	DeleteStudentSession(ctx context.Context, studentSessionId string) error
	GetStudentSessionById(ctx context.Context, studentSessionId string) (*models.StudentSession, error)
	GetSessionById(ctx context.Context, sessionId string) (*models.StudentSession, error)
	BulkUpdateStudentSessions(ctx context.Context, data *models.BulkUpdateSessionsDto) error

	// TEACHER-FACING
	GetTeacherCoachings(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.TeacherCoachingPagination, error)
	GetTeacherCoachingSessions(ctx context.Context, coachingId string, skip, limit int64, query *querypkg.Conditions) (*models.StudentSessionPagination, error)
	GetTeacherCoachingMembers(ctx context.Context, coachingId string, skip, limit int64, query *querypkg.Conditions) (*models.CoachingProgramMemberPagination, error)

	// SESSION DETAILS
	GetSessionDetails(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.SessionDetailsPagination, error)
	// ADMIN absence requests
	GetTeacherAbsenceRequests(ctx context.Context, teacherId string, skip, limit int64) (*models.AbsenceRequestPagination, error)
	// ADMIN unified justification requests (absence/reschedule) with filters
	GetTeacherJustificationRequests(ctx context.Context, teacherId string, skip, limit int64, query *querypkg.Conditions) (*models.JustificationRequestPagination, error)
	// STUDENT SESSIONS AND STATS
	GetAllStudentSessions(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions) (*models.StudentSessionListPagination, error)
	GetStudentCoachingStats(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions) (*models.StudentCoachingStatsPagination, error)
	GetStudentSessionsForCoaching(ctx context.Context, coachingProgramId string, studentId string, requesterId string) (*models.StudentCoachingSessionsResponse, error)
	CalculateCommitmentProgress(ctx context.Context, studentId string, coachingId string) error

	// Student facing list operations
	GetStudentList(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions, includePlacementTests bool, enrichCoachs bool, enrichPrice bool) (*models.StudentCoachingPagination, error)
}

type coachingsvcs struct {
	i                      *do.Injector
	coachingRepo           db.MainRepo[models.Coaching]
	coachingAssignmentRepo db.MainRepo[models.CoachingAssignment]
	coachingTestRepo       db.MainRepo[models.CoachingTest]
	withtxn                *db.WithTxn
	//
	studentSessionRepo            db.MainRepo[models.StudentSession]
	teacherRepo                   db.MainRepo[teachermodels.Teacher]
	coachingStudentAssignmentRepo db.MainRepo[models.CoachingStudentAssignment]
	coachingStudentTestRepo       db.MainRepo[models.CoachingStudentTest]
	coachingPricingRepo           db.MainRepo[models.CoachingPricing]

	coachingStudentRepo db.MainRepo[models.CoachingStudent]

	parentSvcs    contactsvcs.ParentSvcs
	userLookupSvc contactsvcs.UserLookupSvc

	usersGw               *gateway.UsersGw
	compensationSvcs      compensation.CompensationSvcs
	teacherAssignmentSvc  TeacherAssignmentSvcs
	teacherAssignmentRepo db.MainRepo[models.TeacherAssignment]
	authSvc               CoachingAuthService
	zoomService           zoom.ZoomService
	zoomMeetingRepo       db.MainRepo[zoommodels.ZoomMeeting]
	pricingSvcs           sales.PricingSvcs
	messageSvcs           messaging.MessageSvcs
}

func NewCoachingSvcs(i *do.Injector) (CoachingSvcs, error) {
	pricingSvcs, err := do.Invoke[sales.PricingSvcs](i)
	if err != nil {
		return nil, err
	}

	return &coachingsvcs{
		i:                             i,
		coachingRepo:                  db.NewRepo[models.Coaching](i),
		coachingAssignmentRepo:        db.NewRepo[models.CoachingAssignment](i),
		coachingTestRepo:              db.NewRepo[models.CoachingTest](i),
		withtxn:                       do.MustInvoke[*db.WithTxn](i),
		studentSessionRepo:            db.NewRepo[models.StudentSession](i),
		teacherRepo:                   db.NewRepo[teachermodels.Teacher](i),
		coachingStudentAssignmentRepo: db.NewRepo[models.CoachingStudentAssignment](i),
		coachingStudentTestRepo:       db.NewRepo[models.CoachingStudentTest](i),
		coachingPricingRepo:           db.NewRepo[models.CoachingPricing](i),
		coachingStudentRepo:           db.NewRepo[models.CoachingStudent](i),
		parentSvcs:                    do.MustInvoke[contactsvcs.ParentSvcs](i),
		userLookupSvc:                 do.MustInvoke[contactsvcs.UserLookupSvc](i),
		usersGw:                       do.MustInvoke[*gateway.UsersGw](i),
		compensationSvcs:              do.MustInvoke[compensation.CompensationSvcs](i),
		teacherAssignmentSvc:          do.MustInvoke[TeacherAssignmentSvcs](i),
		teacherAssignmentRepo:         db.NewRepo[models.TeacherAssignment](i),
		authSvc:                       do.MustInvoke[CoachingAuthService](i),
		zoomService:                   do.MustInvoke[zoom.ZoomService](i),
		zoomMeetingRepo:               db.NewRepo[zoommodels.ZoomMeeting](i),
		pricingSvcs:                   pricingSvcs,
		messageSvcs:                   do.MustInvoke[messaging.MessageSvcs](i),
	}, nil
}

// coaching program
func (c *coachingsvcs) GetById(ctx context.Context, id string, enrichCoachs bool) (*models.EnrichedCoaching, error) {
	if canView, err := c.authSvc.CanViewCoaching(ctx, id); err != nil || !canView {
		return nil, helpers.Forbidden("unauthorized to view coaching")
	}

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}

	// Add teacher enrichment if requested
	if enrichCoachs {
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyTeachers",
				"let":  bson.M{"coachIds": "$coachs"},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{"$in": []interface{}{"$_id", "$$coachIds"}},
					}},
					{"$project": bson.M{
						"_id":       1,
						"firstName": 1,
						"lastName":  1,
						"picture":   1,
					}},
				},
				"as": "coachsObjs",
			},
		})
	}

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academySchools",
			"localField":   "schoolId",
			"foreignField": "_id",
			"as":           "schoolData",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"schoolObj": bson.M{
				"$cond": []interface{}{
					bson.M{"$gt": []interface{}{bson.M{"$size": "$schoolData"}, 0}},
					bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
					nil,
				},
			},
		},
	})

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"schoolData": 0,
		},
	})

	var result []bson.M
	if err := c.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("coaching not found")
	}

	doc := result[0]

	coaching, err := util.Converter[models.EnrichedCoaching](doc)
	if err != nil {
		return nil, err
	}
	return &coaching, nil
}

func (c *coachingsvcs) Get(ctx context.Context, skip, limit int64, query *querypkg.Conditions, includePlacementTests bool, enrichCoachs bool, enrichPrice bool) (*models.EnrichedCoachingListPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	// Separate coachs filter if present since it's a derived field
	var initialColumns []querypkg.Column
	for _, col := range query.Columns {
		initialColumns = append(initialColumns, col)
	}

	initialQuery := &querypkg.Conditions{Columns: initialColumns}
	filter, err := initialQuery.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := c.coachingRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	if includePlacementTests {
		cfg, _ := util.GetReqAppCfg(ctx)

		pipeline = append(pipeline,
			bson.M{"$lookup": bson.M{
				"from": "academyPlacementTests",
				"let":  bson.M{"coachingId": "$_id"},
				"pipeline": []bson.M{
					{
						"$match": bson.M{
							"$expr": bson.M{"$and": []bson.M{
								{"$eq": []any{"$refId", "$$coachingId"}},
								{"$eq": []any{"$refType", "coaching"}},
								{"$eq": []any{"$trash", false}},
							}},
						},
					},
				},
				"as": "placementTests",
			}},
			bson.M{"$addFields": bson.M{
				"placementTests": bson.M{
					"$map": bson.M{
						"input": "$placementTests",
						"as":    "pt",
						"in": bson.M{
							"_id":            "$$pt._id",
							"title":          "$$pt.title",
							"status":         "$$pt.status",
							"type":           "$$pt.type",
							"testMark":       "$$pt.testMark",
							"settings":       "$$pt.settings",
							"questionsCount": bson.M{"$size": bson.M{"$ifNull": []interface{}{"$$pt.questions", []bson.M{}}}},
							"updatedAt":      "$$pt.updatedAt",
						},
					},
				},
			}},
		)

		if cfg != nil && cfg.User != nil && !cfg.User.Id.IsZero() {
			pipeline = append(pipeline,
				bson.M{"$lookup": bson.M{
					"from": "academyStudentPlacementTests",
					"let": bson.M{
						"coachingId": "$_id",
						"placementTestIds": bson.M{"$map": bson.M{
							"input": "$placementTests",
							"as":    "pt",
							"in":    "$$pt._id",
						}},
					},
					"pipeline": []bson.M{
						{
							"$match": bson.M{
								"$expr": bson.M{"$and": []bson.M{
									{"$eq": []any{"$refId", "$$coachingId"}},
									{"$eq": []any{"$studentId", cfg.User.Id}},
									{"$eq": []any{"$trash", false}},
								}},
							},
						},
						{
							"$lookup": bson.M{
								"from":         "academyZoomMeetings",
								"localField":   "zoomMeetingId",
								"foreignField": "_id",
								"as":           "zoomMeeting",
							},
						},
						{
							"$unwind": bson.M{
								"path":                       "$zoomMeeting",
								"preserveNullAndEmptyArrays": true,
							},
						},
						{
							"$addFields": bson.M{
								"zoomMeeting": bson.M{
									"$cond": bson.A{
										bson.M{"$ifNull": bson.A{"$zoomMeeting", false}},
										bson.M{"$mergeObjects": bson.A{"$zoomMeeting", bson.M{"id": "$zoomMeeting._id"}}},
										nil,
									},
								},
							},
						},
					},
					"as": "studentPlacementTests",
				}},
				bson.M{"$addFields": bson.M{
					"placementTests": bson.M{
						"$map": bson.M{
							"input": "$placementTests",
							"as":    "pt",
							"in": bson.M{
								"$let": bson.M{
									"vars": bson.M{
										"studentData": bson.M{"$first": bson.M{"$filter": bson.M{
											"input": "$studentPlacementTests",
											"as":    "spt",
											"cond":  bson.M{"$eq": []any{"$$spt.placementTestId", "$$pt._id"}},
										}}},
									},
									"in": bson.M{
										"_id":            "$$pt._id",
										"title":          "$$pt.title",
										"status":         "$$pt.status",
										"type":           "$$pt.type",
										"testMark":       "$$pt.testMark",
										"settings":       "$$pt.settings",
										"questionsCount": "$$pt.questionsCount",
										"updatedAt":      "$$pt.updatedAt",
										"studentState": bson.M{
											"$let": bson.M{
												"vars": bson.M{
													"scheduleDate": bson.M{"$ifNull": []any{
														"$$studentData.sessionDate",
														"$$studentData.date",
													}},
												},
												"in": bson.M{
													"hasSubmission": bson.M{"$cond": []any{
														bson.M{"$ifNull": []any{"$$studentData", false}},
														true,
														false,
													}},
													"submissionId": bson.M{"$cond": []any{
														bson.M{"$ifNull": []any{"$$studentData._id", false}},
														"$$studentData._id",
														nil,
													}},
													"score":         "$$studentData.score",
													"result":        "$$studentData.result",
													"submissionNum": "$$studentData.submissionNum",
													"status":        "$$studentData.status",
													"scheduled": bson.M{"$in": []any{
														"$$studentData.status",
														[]string{"scheduled", "pending"},
													}},
													"scheduleDate": "$$scheduleDate",
													"from": bson.M{"$cond": []any{
														bson.M{"$ne": []any{"$$scheduleDate", nil}},
														"$$studentData.from",
														nil,
													}},
													"to": bson.M{"$cond": []any{
														bson.M{"$ne": []any{"$$scheduleDate", nil}},
														"$$studentData.to",
														nil,
													}},
													"zoomMeeting": "$$studentData.zoomMeeting",
												},
											},
										},
									},
								},
							},
						},
					},
				}},
				bson.M{"$project": bson.M{"studentPlacementTests": 0}},
			)
		}
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	// Add teacher enrichment if requested
	if enrichCoachs {
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyTeachers",
				"let":  bson.M{"coachIds": "$coachs"},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{"$in": []interface{}{"$_id", "$$coachIds"}},
					}},
					{"$project": bson.M{
						"_id":       1,
						"firstName": bson.M{"$ifNull": []interface{}{"$firstName", bson.M{}}},
						"lastName":  bson.M{"$ifNull": []interface{}{"$lastName", bson.M{}}},
						"picture":   1,
					}},
				},
				"as": "coachsObjs",
			},
		})
	}

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academySchools",
			"localField":   "schoolId",
			"foreignField": "_id",
			"as":           "schoolData",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"schoolObj": bson.M{
				"$cond": []interface{}{
					bson.M{"$gt": []interface{}{bson.M{"$size": "$schoolData"}, 0}},
					bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
					nil,
				},
			},
		},
	})

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"schoolData": 0,
		},
	})

	// Add price enrichment if requested
	if enrichPrice {
		stages, err := c.pricingSvcs.GetPriceEnrichmentStages(ctx, enums.CartTypeCoaching, "$_id")
		if err == nil {
			pipeline = append(pipeline, stages...)
		}
		pipeline = append(pipeline, bson.M{"$match": bson.M{"pricing": bson.M{"$ne": nil}}})
	}

	var result []bson.M
	if err := c.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	coachings_ := make([]models.Coaching, len(result))
	for i, doc := range result {
		item, err := util.Converter[models.Coaching](doc)
		if err != nil {
			return nil, err
		}
		coachings_[i] = item
	}

	filteredCoachings, err := c.authSvc.FilterCoachingsByPermissions(ctx, coachings_)
	if err != nil {
		return nil, err
	}

	// Keep count from pipeline (includes isEnrolled filter) - do not overwrite with len(filteredCoachings)
	// which would incorrectly set TotalCount to page size and break pagination

	// Re-convert filtered to EnrichedCoachingListItem
	coachings := make([]models.EnrichedCoachingListItem, len(filteredCoachings))
	for i, c := range filteredCoachings {
		// Find the original doc to preserve enrichment (like placementTests, coachsObjs)
		var doc bson.M
		for _, d := range result {
			if id, ok := d["_id"].(primitive.ObjectID); ok && id == c.Id {
				doc = d
				break
			}
		}
		if doc == nil {
			// Fallback to converting from the filtered model if doc not found (shouldn't happen)
			item, _ := util.Converter[models.EnrichedCoachingListItem](c)
			coachings[i] = item
			continue
		}

		item, err := util.Converter[models.EnrichedCoachingListItem](doc)
		if err != nil {
			return nil, err
		}
		coachings[i] = item
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.EnrichedCoachingListPagination{
		Coachings:  coachings,
		Pagination: pagination,
	}, nil
}

func (c *coachingsvcs) GetAll(ctx context.Context, query *querypkg.Conditions, enrichCoachs bool) ([]models.EnrichedCoaching, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	// Separate coachs filter if present since it's a derived field
	var initialColumns []querypkg.Column
	for _, col := range query.Columns {
		initialColumns = append(initialColumns, col)
	}

	initialQuery := &querypkg.Conditions{Columns: initialColumns}
	filter, err := initialQuery.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
	}

	// Add teacher enrichment if requested
	if enrichCoachs {
		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from": "academyTeachers",
				"let":  bson.M{"coachIds": "$coachs"},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{"$in": []interface{}{"$_id", "$$coachIds"}},
					}},
					{"$project": bson.M{
						"_id":       1,
						"firstName": 1,
						"lastName":  1,
						"picture":   1,
					}},
				},
				"as": "coachsObjs",
			},
		})
	}

	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academySchools",
			"localField":   "schoolId",
			"foreignField": "_id",
			"as":           "schoolData",
		},
	})

	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"schoolObj": bson.M{
				"$cond": []interface{}{
					bson.M{"$gt": []interface{}{bson.M{"$size": "$schoolData"}, 0}},
					bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
					nil,
				},
			},
		},
	})

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"schoolData": 0,
		},
	})

	var result []bson.M
	if err := c.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	var coachingModels []models.Coaching
	for _, doc := range result {
		cModel, err := util.Converter[models.Coaching](doc)
		if err != nil {
			return nil, err
		}
		coachingModels = append(coachingModels, cModel)
	}

	filteredCoachings, err := c.authSvc.FilterCoachingsByPermissions(ctx, coachingModels)
	if err != nil {
		return nil, err
	}

	coachings := make([]models.EnrichedCoaching, len(filteredCoachings))
	for i, fc := range filteredCoachings {
		// Find original doc for enrichment
		var doc bson.M
		for _, d := range result {
			if id, ok := d["_id"].(primitive.ObjectID); ok && id == fc.Id {
				doc = d
				break
			}
		}

		item, err := util.Converter[models.EnrichedCoaching](doc)
		if err != nil {
			return nil, err
		}
		coachings[i] = item
	}
	return coachings, nil
}

func (c *coachingsvcs) Add(ctx context.Context, data *models.CoachingDto) (*models.Coaching, error) {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, ""); err != nil || !canManage {
		return nil, errors.New("unauthorized to add coaching")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Store coachs separately before clearing
	coachs := data.Coachs

	// Ensure coachs is not nil for persistence (empty array instead of null)
	if data.Coachs == nil {
		data.Coachs = make([]primitive.ObjectID, 0)
	}

	// Create coaching
	coaching := &models.Coaching{
		Id:          primitive.NewObjectID(),
		CoachingDto: *data,
		CreatedAt:   time.Now(),
		CreatedBy:   cfg.User.Id,
		UpdatedAt:   time.Now(),
	}

	if err := c.coachingRepo.Add(ctx, coaching); err != nil {
		return nil, err
	}

	// Sync supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[SupervisorAssignmentSvcs](c.i)
	supervisorIds, err := supervisorAssignmentSvc.CheckSupervisorAssignment(ctx, coaching.SchoolType, coaching.Country, coaching.Level, []primitive.ObjectID{coaching.Subject})
	if err != nil {
		// Log error but don't fail coaching creation
		log.Printf("Failed to check supervisor assignment for coaching: %v", err)
	} else {
		// Update coaching with supervisor IDs
		supervisorUpdate := bson.M{"$set": bson.M{"supervisorIds": supervisorIds}}
		updatedCoaching, err := c.coachingRepo.Patch(ctx, bson.M{"_id": coaching.Id}, supervisorUpdate)
		if err != nil {
			// Log error but don't fail coaching creation
			log.Printf("Failed to update supervisor IDs for coaching: %v", err)
		} else {
			coaching = updatedCoaching
		}
	}

	// Sync teacher assignment
	teacherAssignmentSvc := do.MustInvoke[TeacherAssignmentSvcs](c.i)
	err = teacherAssignmentSvc.CreateFromCoaching(ctx, coaching.Id, coachs)
	if err != nil {
		// Log error but don't fail coaching creation
		log.Printf("Failed to create teacher assignment for coaching: %v", err)
	}

	return coaching, nil
}

func (c *coachingsvcs) Update(ctx context.Context, coachingId string, data *models.CoachingDto) (*models.Coaching, error) {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, coachingId); err != nil || !canManage {
		return nil, errors.New("unauthorized to update coaching")
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Store coachs separately before clearing
	coachs := data.Coachs

	// Ensure coachs is not nil for persistence (empty array instead of null)
	if data.Coachs == nil {
		data.Coachs = make([]primitive.ObjectID, 0)
	}

	// Update coaching
	coaching := &models.Coaching{
		Id:          _id,
		CoachingDto: *data,
		UpdatedAt:   time.Now(),
		UpdatedBy:   cfg.User.Id,
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": coaching}

	updatedCoaching, err := c.coachingRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Sync teacher assignment if coachs provided
	if coachs != nil {
		teacherAssignmentSvc := do.MustInvoke[TeacherAssignmentSvcs](c.i)
		err = teacherAssignmentSvc.BulkCreateOrUpdate(ctx, &models.BulkTeacherAssignmentDto{
			Type:       "coaching",
			TargetId:   _id,
			TeacherIds: coachs,
		})
		if err != nil {
			// Log error but don't fail coaching update
			log.Printf("Failed to update teacher assignment: %v", err)
		}
	}

	return updatedCoaching, nil
}

func (c *coachingsvcs) Delete(ctx context.Context, coachingId string) error {
	if canManage, err := c.authSvc.CanManageCoaching(ctx, coachingId); err != nil || !canManage {
		return errors.New("unauthorized to delete coaching")
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return errors.New("invalid coaching id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = c.coachingRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

// TEACHER-FACING METHODS

// GetTeacherCoachings retrieves all coachings assigned to the authenticated teacher
func (c *coachingsvcs) GetTeacherCoachings(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.TeacherCoachingPagination, error) {
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

	// Get all coachings where teacher is assigned using teacher assignment service
	var teacherAssignments []models.TeacherAssignment
	err = c.teacherAssignmentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"teacherIds": teacherId,
			"type":       "coaching",
			"trash":      false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teacherAssignments)
	})
	if err != nil && err != mongo.ErrNoDocuments {
		return nil, err
	}

	// Collect coaching IDs
	coachingIds := make([]primitive.ObjectID, 0, len(teacherAssignments))
	for _, assignment := range teacherAssignments {
		coachingIds = append(coachingIds, assignment.TargetId)
	}

	if len(coachingIds) == 0 {
		// No coachings assigned, return empty result
		return &models.TeacherCoachingPagination{
			Coachings: []models.TeacherCoachingResponse{},
			Pagination: types.Pagination{
				TotalPages: 0,
				PerPage:    limit,
				TotalCount: 0,
			},
		}, nil
	}

	// Build filter for coachings
	teacherFilter := bson.M{
		"trash": false,
		"_id":   bson.M{"$in": coachingIds},
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

	count, err := c.coachingRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	// Add lookup to count students for each coaching
	pipeline = append(pipeline, bson.M{
		"$lookup": bson.M{
			"from":         "academyCoachingStudents",
			"localField":   "_id",
			"foreignField": "coachingId",
			"as":           "students",
		},
	})
	pipeline = append(pipeline, bson.M{
		"$addFields": bson.M{
			"studentCount": bson.M{
				"$size": bson.M{
					"$filter": bson.M{
						"input": "$students",
						"cond":  bson.M{"$eq": bson.A{"$$this.trash", false}},
					},
				},
			},
		},
	})

	// Use bson.M to hold the aggregation result
	var result []bson.M
	if err := c.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Map to response structs using util.Converter
	coachings := make([]models.TeacherCoachingResponse, len(result))
	for i, doc := range result {
		coaching, err := util.Converter[models.Coaching](doc)
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

		coachings[i] = models.TeacherCoachingResponse{
			Coaching:     coaching,
			StudentCount: studentCount,
		}
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.TeacherCoachingPagination{
		Coachings:  coachings,
		Pagination: pagination,
	}, nil
}

// GetTeacherCoachingSessions retrieves sessions for a specific coaching, filtered by teacher
func (c *coachingsvcs) GetTeacherCoachingSessions(ctx context.Context, coachingId string, skip, limit int64, query *querypkg.Conditions) (*models.StudentSessionPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id

	coachingObjectId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	// Verify teacher is assigned to this coaching using teacher assignment service
	teacherIds, err := c.teacherAssignmentSvc.GetCoachingTeachers(ctx, coachingObjectId)
	if err != nil {
		return nil, errors.New("coaching not found")
	}

	teacherAssigned := false
	for _, tid := range teacherIds {
		if tid == teacherId {
			teacherAssigned = true
			break
		}
	}

	if !teacherAssigned {
		return nil, errors.New("you are not assigned to this coaching")
	}

	// Check and convert query if provided, otherwise use empty filter
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

	// Base filter: coaching ID and teacher ID
	baseFilter := bson.M{
		"coachingProgramId": coachingObjectId,
		"teacherId":         teacherId,
		"trash":             false,
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

	count, err := c.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"date": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	if err := c.enrichSessionsWithZoom(ctx, result); err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.StudentSessionPagination{
		Sessions:   result,
		Pagination: pagination,
	}, nil
}

// GetTeacherAbsenceRequests returns absence justifications for all sessions taught by the given teacher
func (c *coachingsvcs) GetTeacherAbsenceRequests(ctx context.Context, teacherId string, skip, limit int64) (*models.AbsenceRequestPagination, error) {
	teacherObjId, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, errors.New("invalid teacher id")
	}

	baseMatch := bson.M{
		"teacherId":          teacherObjId,
		"trash":              false,
		"justification.type": enums.JustificationTypeAbsence,
	}

	count, err := c.studentSessionRepo.Count(ctx, []bson.M{{"$match": baseMatch}})
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": baseMatch},
		{"$sort": bson.M{"date": -1}},
		{"$skip": skip},
		{"$limit": limit},
		// student lookup
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "student",
		}},
		{"$unwind": "$student"},
		// coaching lookup
		{"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "coachingProgramId",
			"foreignField": "_id",
			"as":           "coaching",
		}},
		{"$unwind": bson.M{"path": "$coaching", "preserveNullAndEmptyArrays": true}},
		// duration from from/to (HH:mm)
		{"$addFields": bson.M{
			"durationMinutes": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"from": "$from",
						"to":   "$to",
					},
					"in": bson.M{
						"$cond": []interface{}{
							bson.M{"$and": bson.A{
								bson.M{"$regexMatch": bson.M{"input": "$$from", "regex": "^\\d{2}:\\d{2}$"}},
								bson.M{"$regexMatch": bson.M{"input": "$$to", "regex": "^\\d{2}:\\d{2}$"}},
							}},
							bson.M{
								"$let": bson.M{
									"vars": bson.M{
										"fromParts": bson.M{
											"$map": bson.M{
												"input": bson.M{"$split": bson.A{"$$from", ":"}},
												"as":    "p",
												"in":    bson.M{"$toInt": "$$p"},
											},
										},
										"toParts": bson.M{
											"$map": bson.M{
												"input": bson.M{"$split": bson.A{"$$to", ":"}},
												"as":    "p",
												"in":    bson.M{"$toInt": "$$p"},
											},
										},
									},
									"in": bson.M{
										"$subtract": bson.A{
											bson.M{
												"$add": bson.A{
													bson.M{"$multiply": bson.A{60, bson.M{"$arrayElemAt": bson.A{"$$toParts", 0}}}},
													bson.M{"$arrayElemAt": bson.A{"$$toParts", 1}},
												},
											},
											bson.M{
												"$add": bson.A{
													bson.M{"$multiply": bson.A{60, bson.M{"$arrayElemAt": bson.A{"$$fromParts", 0}}}},
													bson.M{"$arrayElemAt": bson.A{"$$fromParts", 1}},
												},
											},
										},
									},
								},
							},
							0,
						},
					},
				},
			},
		}},
		{"$project": bson.M{
			"_id":             1,
			"studentId":       1,
			"date":            1,
			"from":            1,
			"to":              1,
			"durationMinutes": 1,
			"coachingTitle":   "$coaching.title",
			"student": bson.M{
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"picture":   "$student.generalInfo.picture",
			},
			"reason":             "$justification.reason",
			"documents":          "$justification.documents",
			"status":             "$justification.decision",
			"decisionNote":       "$justification.note",
			"decisionDate":       "$justification.decisionDate",
			"decisionBy":         "$justification.decisionBy",
			"prevJustifications": "$prevJustifications",
		}},
	}

	var rows []models.AbsenceRequestRow
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &rows)
	}); err != nil {
		return nil, err
	}

	// Enrich DecisionByObj using userLookup service
	var decisionByIds []primitive.ObjectID
	for _, row := range rows {
		if !row.DecisionBy.IsZero() {
			decisionByIds = append(decisionByIds, row.DecisionBy)
		}
	}

	userInfoMap := make(map[primitive.ObjectID]contactsvcs.UserBasicInfo)
	if len(decisionByIds) > 0 {
		lookupResult, err := c.userLookupSvc.GetUsersBasicInfo(ctx, decisionByIds)
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

	for idx := range rows {
		if !rows[idx].DecisionBy.IsZero() {
			if userInfo, ok := userInfoMap[rows[idx].DecisionBy]; ok {
				firstName, lastName := parseName(userInfo.Name)
				rows[idx].DecisionByObj = &models.AbsenceRequestReviewer{
					Id:        userInfo.Id,
					FirstName: firstName,
					LastName:  lastName,
					Email:     userInfo.Email,
					Picture:   "", // userLookup doesn't provide picture
				}
			}
		}
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(count) / float64(limit))
	}

	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.AbsenceRequestPagination{
		Requests:   rows,
		Pagination: pagination,
	}, nil
}

// GetTeacherJustificationRequests returns absence or reschedule justifications for sessions taught by the given teacher, with dynamic filters
func (c *coachingsvcs) GetTeacherJustificationRequests(ctx context.Context, teacherId string, skip, limit int64, query *querypkg.Conditions) (*models.JustificationRequestPagination, error) {
	teacherObjId, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, errors.New("invalid teacher id")
	}

	// base filter ensures teacher and trash=false and justification present
	baseFilter := bson.M{
		"teacherId":          teacherObjId,
		"trash":              false,
		"justification.type": bson.M{"$in": []enums.JustificationType{enums.JustificationTypeAbsence, enums.JustificationTypeReschedule}},
	}

	// merge user-provided filters via querypkg.Conditions
	var userFilter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
		userFilter, err = query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
	}

	var match bson.M
	if len(userFilter) > 0 {
		match = bson.M{"$and": []bson.M{baseFilter, userFilter}}
	} else {
		match = baseFilter
	}

	count, err := c.studentSessionRepo.Count(ctx, []bson.M{{"$match": match}})
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": match},
		{"$sort": bson.M{"date": -1}},
		{"$skip": skip},
		{"$limit": limit},
		// student lookup
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "student",
		}},
		{"$unwind": "$student"},
		// coaching lookup
		{"$lookup": bson.M{
			"from":         "academyCoachings",
			"localField":   "coachingProgramId",
			"foreignField": "_id",
			"as":           "coaching",
		}},
		{"$unwind": bson.M{"path": "$coaching", "preserveNullAndEmptyArrays": true}},
		// duration from from/to (HH:mm)
		{"$addFields": bson.M{
			"durationMinutes": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"from": "$from",
						"to":   "$to",
					},
					"in": bson.M{
						"$cond": []interface{}{
							bson.M{"$and": bson.A{
								bson.M{"$regexMatch": bson.M{"input": "$$from", "regex": "^\\d{2}:\\d{2}$"}},
								bson.M{"$regexMatch": bson.M{"input": "$$to", "regex": "^\\d{2}:\\d{2}$"}},
							}},
							bson.M{
								"$let": bson.M{
									"vars": bson.M{
										"fromParts": bson.M{
											"$map": bson.M{
												"input": bson.M{"$split": bson.A{"$$from", ":"}},
												"as":    "p",
												"in":    bson.M{"$toInt": "$$p"},
											},
										},
										"toParts": bson.M{
											"$map": bson.M{
												"input": bson.M{"$split": bson.A{"$$to", ":"}},
												"as":    "p",
												"in":    bson.M{"$toInt": "$$p"},
											},
										},
									},
									"in": bson.M{
										"$subtract": bson.A{
											bson.M{
												"$add": bson.A{
													bson.M{"$multiply": bson.A{60, bson.M{"$arrayElemAt": bson.A{"$$toParts", 0}}}},
													bson.M{"$arrayElemAt": bson.A{"$$toParts", 1}},
												},
											},
											bson.M{
												"$add": bson.A{
													bson.M{"$multiply": bson.A{60, bson.M{"$arrayElemAt": bson.A{"$$fromParts", 0}}}},
													bson.M{"$arrayElemAt": bson.A{"$$fromParts", 1}},
												},
											},
										},
									},
								},
							},
							0,
						},
					},
				},
			},
		}},
		{"$project": bson.M{
			"_id":             1,
			"studentId":       1,
			"date":            1,
			"from":            1,
			"to":              1,
			"durationMinutes": 1,
			"type":            "$justification.type",
			"coachingTitle":   "$coaching.title",
			"student": bson.M{
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"picture":   "$student.generalInfo.picture",
			},
			"reason":             "$justification.reason",
			"documents":          "$justification.documents",
			"status":             "$justification.decision",
			"decisionNote":       "$justification.note",
			"decisionDate":       "$justification.decisionDate",
			"decisionBy":         "$justification.decisionBy",
			"requestedBy":        "$updatedBy",
			"prevJustifications": "$prevJustifications",
			// reschedule requested slot
			"newDate": "$justification.data.date",
			"newFrom": "$justification.data.from",
			"newTo":   "$justification.data.to",
		}},
	}

	var rows []models.JustificationRequestRow
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &rows)
	}); err != nil {
		return nil, err
	}

	// Collect all user IDs for batch lookup
	var allUserIds []primitive.ObjectID
	for _, row := range rows {
		if !row.RequestedBy.IsZero() {
			allUserIds = append(allUserIds, row.RequestedBy)
		}
		if !row.DecisionBy.IsZero() {
			allUserIds = append(allUserIds, row.DecisionBy)
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

	// Enrich RequestedByObj and DecisionByObj
	for idx := range rows {
		// Enrich RequestedByObj
		if !rows[idx].RequestedBy.IsZero() {
			if userInfo, ok := userInfoMap[rows[idx].RequestedBy]; ok {
				firstName, lastName := parseName(userInfo.Name)
				rows[idx].RequestedByObj = &models.AbsenceRequestReviewer{
					Id:        userInfo.Id,
					FirstName: firstName,
					LastName:  lastName,
					Email:     userInfo.Email,
					Picture:   "", // userLookup doesn't provide picture
				}
			}
		}

		// Enrich DecisionByObj
		if !rows[idx].DecisionBy.IsZero() {
			if userInfo, ok := userInfoMap[rows[idx].DecisionBy]; ok {
				firstName, lastName := parseName(userInfo.Name)
				rows[idx].DecisionByObj = &models.AbsenceRequestReviewer{
					Id:        userInfo.Id,
					FirstName: firstName,
					LastName:  lastName,
					Email:     userInfo.Email,
					Picture:   "", // userLookup doesn't provide picture
				}
			}
		}
	}

	var totalPages float64 = 0
	if limit > 0 {
		totalPages = math.Ceil(float64(count) / float64(limit))
	}

	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.JustificationRequestPagination{
		Requests:   rows,
		Pagination: pagination,
	}, nil
}

// GetTeacherCoachingMembers retrieves members of a coaching, filtered to only show students the teacher teaches
func (c *coachingsvcs) GetTeacherCoachingMembers(ctx context.Context, coachingId string, skip, limit int64, query *querypkg.Conditions) (*models.CoachingProgramMemberPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	teacherId := cfg.User.Id

	coachingObjectId, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return nil, errors.New("invalid coaching id")
	}

	// Verify teacher is assigned to this coaching using teacher assignment service
	teacherIds, err := c.teacherAssignmentSvc.GetCoachingTeachers(ctx, coachingObjectId)
	if err != nil {
		return nil, errors.New("coaching not found")
	}

	teacherAssigned := false
	for _, tid := range teacherIds {
		if tid == teacherId {
			teacherAssigned = true
			break
		}
	}

	if !teacherAssigned {
		return nil, errors.New("you are not assigned to this coaching")
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

	// Base pipeline: find unique students in this program where the teacher teaches
	basePipeline := []bson.M{
		{"$match": bson.M{
			"coachingProgramId": coachingObjectId,
			"teacherId":         teacherId,
			"trash":             false,
		}},
	}
	// Verify teacher is assigned to this coaching using teacher assignment service
	// Actually, the teacher is already verified above
	// Check permissions using auth service
	canViewMembers, _ := c.authSvc.CanViewCoachingMembers(ctx, coachingId)
	if !canViewMembers {
		return nil, errors.New("unauthorized to view members")
	}

	pipeline := append(basePipeline, []bson.M{
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
			"lastSessionDate": bson.M{"$max": "$date"},
			"startDate":       bson.M{"$min": "$date"},
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
	}...)

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
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"email":     "$student.accountStudentSetting.email",
				"picture":   "$student.generalInfo.picture",
			},
		},
	}

	// Combine base + lookup
	pipeline = append(pipeline, studentLookup...)

	// Apply user-provided filters
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Count total students
	totalCount, err := c.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Pagination & Session Fetching
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
		// Re-lookup sessions only for the displayed students
		bson.M{
			"$lookup": bson.M{
				"from": "academyCoachingStudentSessions",
				"let":  bson.M{"sid": "$_id"},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr":             bson.M{"$eq": []string{"$studentId", "$$sid"}},
						"coachingProgramId": coachingObjectId,
						"teacherId":         teacherId,
						"trash":             false,
					}},
					{"$sort": bson.M{"date": -1}},
				},
				"as": "sessions",
			},
		},
	)

	var result []models.CoachingProgramMember
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(totalCount) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &models.CoachingProgramMemberPagination{
		Members:    result,
		Pagination: pagination,
	}, nil
}

// GetSessionDetails retrieves session details with student and teacher info, filtered by querypkg.Conditions
func (c *coachingsvcs) GetSessionDetails(ctx context.Context, skip, limit int64, query *querypkg.Conditions) (*models.SessionDetailsPagination, error) {
	now := time.Now()

	// Base match filter
	baseMatch := bson.M{"trash": false}

	// Convert query to MongoDB filter
	var queryFilter bson.M
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}

		var err error
		queryFilter, err = query.ConvertToMongo()
		if err != nil {
			return nil, err
		}

		// Merge with base filter
		if len(queryFilter) > 0 {
			baseMatch = bson.M{
				"$and": []bson.M{
					baseMatch,
					queryFilter,
				},
			}
		}
	}

	// Base pipeline: Group by studentId + coachingProgramId
	basePipeline := []bson.M{
		{"$match": baseMatch},
		// Group by studentId + coachingProgramId to get unique combinations
		{"$group": bson.M{
			"_id": bson.M{
				"studentId":         "$studentId",
				"coachingProgramId": "$coachingProgramId",
			},
			"teacherId": bson.M{"$first": "$teacherId"}, // Get teacherId from first session (should be same for all)
			"startDate": bson.M{"$min": "$date"},
			"endDate":   bson.M{"$max": "$date"},
			"sessions":  bson.M{"$push": "$$ROOT"},
		}},
		// Split future and past sessions
		{"$addFields": bson.M{
			"futureSessions": bson.M{
				"$filter": bson.M{
					"input": "$sessions",
					"as":    "s",
					"cond": bson.M{
						"$gte": []interface{}{"$$s.date", now},
					},
				},
			},
			"pastSessions": bson.M{
				"$filter": bson.M{
					"input": "$sessions",
					"as":    "s",
					"cond": bson.M{
						"$lt": []interface{}{"$$s.date", now},
					},
				},
			},
		}},
		// Get next and previous sessions
		{"$addFields": bson.M{
			"nextSession": bson.M{
				"$reduce": bson.M{
					"input":        "$futureSessions",
					"initialValue": nil,
					"in": bson.M{
						"$cond": []interface{}{
							bson.M{
								"$or": []interface{}{
									bson.M{"$eq": []interface{}{"$$value", nil}},
									bson.M{"$lt": []interface{}{"$$this.date", "$$value.date"}},
								},
							},
							"$$this",
							"$$value",
						},
					},
				},
			},
			"previousSession": bson.M{
				"$reduce": bson.M{
					"input":        "$pastSessions",
					"initialValue": nil,
					"in": bson.M{
						"$cond": []interface{}{
							bson.M{
								"$or": []interface{}{
									bson.M{"$eq": []interface{}{"$$value", nil}},
									bson.M{"$gt": []interface{}{"$$this.date", "$$value.date"}},
								},
							},
							"$$this",
							"$$value",
						},
					},
				},
			},
		}},
	}

	// Count total records
	countPipeline := make([]bson.M, len(basePipeline))
	copy(countPipeline, basePipeline)
	countPipeline = append(countPipeline, bson.M{"$count": "total"})

	var countResult []bson.M
	err := c.studentSessionRepo.Aggregate(ctx, countPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &countResult)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	totalCount := int64(0)
	if len(countResult) > 0 {
		if count, ok := countResult[0]["total"].(int32); ok {
			totalCount = int64(count)
		} else if count, ok := countResult[0]["total"].(int64); ok {
			totalCount = count
		}
	}

	// Get paginated results with lookups
	sessionsPipeline := make([]bson.M, len(basePipeline))
	copy(sessionsPipeline, basePipeline)
	sessionsPipeline = append(sessionsPipeline,
		bson.M{"$sort": bson.M{"_id.studentId": 1, "_id.coachingProgramId": 1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
		// Lookup student info
		bson.M{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "_id.studentId",
				"foreignField": "_id",
				"as":           "student",
			},
		},
		bson.M{"$unwind": bson.M{
			"path":                       "$student",
			"preserveNullAndEmptyArrays": true,
		}},
		// Lookup teacher info
		bson.M{
			"$lookup": bson.M{
				"from":         "academyTeachers",
				"localField":   "teacherId",
				"foreignField": "_id",
				"as":           "teacher",
			},
		},
		bson.M{"$unwind": bson.M{
			"path":                       "$teacher",
			"preserveNullAndEmptyArrays": true,
		}},
		// Lookup coaching info
		bson.M{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "_id.coachingProgramId",
				"foreignField": "_id",
				"as":           "coaching",
			},
		},
		bson.M{"$unwind": bson.M{
			"path":                       "$coaching",
			"preserveNullAndEmptyArrays": true,
		}},
		// Project final structure
		bson.M{"$project": bson.M{
			"student": bson.M{
				"id":        "$_id.studentId",
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"picture":   "$student.generalInfo.picture",
			},
			"teacher": bson.M{
				"id":        "$teacherId",
				"firstName": "$teacher.firstName",
				"lastName":  "$teacher.lastName",
				"picture":   "$teacher.picture",
			},
			"sessionId": bson.M{
				"$ifNull": []interface{}{
					"$nextSession._id",
					"$previousSession._id",
				},
			},
			"nextSessionId":     "$nextSession._id",
			"previousSessionId": "$previousSession._id",
			"durationMinutes": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"activeSession": bson.M{
							"$cond": []interface{}{
								bson.M{"$ne": []interface{}{"$nextSession", nil}},
								"$nextSession",
								"$previousSession",
							},
						},
					},
					"in": bson.M{
						"$cond": []interface{}{
							bson.M{"$ne": []interface{}{"$$activeSession", nil}},
							bson.M{
								"$cond": []interface{}{
									bson.M{"$and": bson.A{
										bson.M{"$regexMatch": bson.M{"input": "$$activeSession.from", "regex": "^\\d{2}:\\d{2}$"}},
										bson.M{"$regexMatch": bson.M{"input": "$$activeSession.to", "regex": "^\\d{2}:\\d{2}$"}},
									}},
									bson.M{
										"$let": bson.M{
											"vars": bson.M{
												"fromParts": bson.M{
													"$map": bson.M{
														"input": bson.M{"$split": bson.A{"$$activeSession.from", ":"}},
														"as":    "p",
														"in":    bson.M{"$toInt": "$$p"},
													},
												},
												"toParts": bson.M{
													"$map": bson.M{
														"input": bson.M{"$split": bson.A{"$$activeSession.to", ":"}},
														"as":    "p",
														"in":    bson.M{"$toInt": "$$p"},
													},
												},
											},
											"in": bson.M{
												"$subtract": bson.A{
													bson.M{
														"$add": bson.A{
															bson.M{"$multiply": bson.A{60, bson.M{"$arrayElemAt": bson.A{"$$toParts", 0}}}},
															bson.M{"$arrayElemAt": bson.A{"$$toParts", 1}},
														},
													},
													bson.M{
														"$add": bson.A{
															bson.M{"$multiply": bson.A{60, bson.M{"$arrayElemAt": bson.A{"$$fromParts", 0}}}},
															bson.M{"$arrayElemAt": bson.A{"$$fromParts", 1}},
														},
													},
												},
											},
										},
									},
									0,
								},
							},
							0,
						},
					},
				},
			},
			"class":               "$student.academicInfo.class",
			"coachingId":          "$_id.coachingProgramId",
			"coachingTitle":       "$coaching.title",
			"nextSessionDate":     "$nextSession.date",
			"previousSessionDate": "$previousSession.date",
			"startDate":           "$startDate",
			"endDate":             "$endDate",
			"status": bson.M{
				"$ifNull": []interface{}{
					"$nextSession.status",
					"$previousSession.status",
				},
			},
		}},
	)

	var sessionsResult []models.SessionDetails
	err = c.studentSessionRepo.Aggregate(ctx, sessionsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessionsResult)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(totalCount) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &models.SessionDetailsPagination{
		Sessions:   sessionsResult,
		Pagination: pagination,
	}, nil
}

func (c *coachingsvcs) GetStudentList(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions, includePlacementTests bool, enrichCoachs bool, enrichPrice bool) (*models.StudentCoachingPagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	var targetUserId primitive.ObjectID
	var studentIdForFavorites string

	if studentId != "" {
		_studentId, err := primitive.ObjectIDFromHex(studentId)
		if err != nil {
			return nil, errors.New("invalid student id")
		}

		if studentId != cfg.User.Id.Hex() {
			isParent, err := c.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, _studentId)
			if err != nil {
				return nil, fmt.Errorf("error checking parent relationship: %w", err)
			}
			if !isParent {
				return nil, errors.New("unauthorized: you can only view your own or your children's data")
			}
		}
		targetUserId = _studentId
		studentIdForFavorites = studentId
	} else if cfg.User != nil && !cfg.User.Id.IsZero() {
		targetUserId = cfg.User.Id
		studentIdForFavorites = cfg.User.Id.Hex()
	} else {
		coachingPagination, err := c.Get(ctx, skip, limit, query, includePlacementTests, enrichCoachs, enrichPrice)
		if err != nil {
			return nil, err
		}
		return &models.StudentCoachingPagination{
			Coachings:  mapToStudentCoachingResponse(coachingPagination.Coachings, nil, nil),
			Pagination: coachingPagination.Pagination,
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
	var enrollments []models.CoachingStudent
	err = c.coachingStudentRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"studentId": targetUserId, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	})
	if err == nil {
		for _, e := range enrollments {
			enrollmentsMap[e.CoachingId] = true
			enrollmentIds = append(enrollmentIds, e.CoachingId)
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

	coachingPagination, err := c.Get(ctx, skip, limit, query, includePlacementTests, enrichCoachs, enrichPrice)
	if err != nil {
		return nil, err
	}

	if coachingPagination.Coachings == nil {
		coachingPagination.Coachings = []models.EnrichedCoachingListItem{}
	}

	studentSvc := do.MustInvoke[contactsvcs.StudentSvcs](c.i)
	student, err := studentSvc.GetOne(ctx, studentIdForFavorites)
	favoritesMap := make(map[primitive.ObjectID]bool)
	if err == nil && student != nil {
		for _, fav := range student.Favorites {
			if fav.Type == "coaching" && !fav.RefId.IsZero() {
				favoritesMap[fav.RefId] = true
			}
		}
	}

	return &models.StudentCoachingPagination{
		Coachings:  mapToStudentCoachingResponse(coachingPagination.Coachings, favoritesMap, enrollmentsMap),
		Pagination: coachingPagination.Pagination,
	}, nil
}

func mapToStudentCoachingResponse(coachings []models.EnrichedCoachingListItem, favoritesMap map[primitive.ObjectID]bool, enrollmentsMap map[primitive.ObjectID]bool) []models.StudentCoachingResponse {
	if coachings == nil {
		return []models.StudentCoachingResponse{}
	}

	response := make([]models.StudentCoachingResponse, len(coachings))
	for i, coaching := range coachings {
		isFavorite := false
		if favoritesMap != nil && !coaching.Id.IsZero() {
			isFavorite = favoritesMap[coaching.Id]
		}
		isEnrolled := false
		if enrollmentsMap != nil && !coaching.Id.IsZero() {
			isEnrolled = enrollmentsMap[coaching.Id]
		}
		response[i] = models.StudentCoachingResponse{
			Id:             coaching.Id,
			CoachingDto:    coaching.CoachingDto,
			CreatedBy:      coaching.CreatedBy,
			CreatedAt:      coaching.CreatedAt,
			UpdatedBy:      coaching.UpdatedBy,
			UpdatedAt:      coaching.UpdatedAt,
			Trash:          coaching.Trash,
			CoachsObjs:     coaching.CoachsObjs,
			PlacementTests: coaching.PlacementTests,
			Pricing:        coaching.Pricing,
			IsFavorite:     isFavorite,
			IsEnrolled:     isEnrolled,
		}
	}

	return response
}
