package contact

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/gateway"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/query"
	"log/slog"

	// "larsa-academy-microservice/pkg/services/contact/constant"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentEnums "larsa-academy-microservice/pkg/services/content/enums"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	points_enums "larsa-academy-microservice/pkg/services/points/enums"
	points_models "larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/services/sorting"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type StudentRegistrationResult struct {
	Student      *models.Student
	UserResponse []byte
	Headers      map[string][]string
}

type StudentSvcs interface {
	GetOne(ctx context.Context, id string) (*models.StudentWithStats, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentsPagination, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]*models.Student, error)

	Add(ctx context.Context, data *models.StudentDto) (*models.Student, error)
	Update(ctx context.Context, id string, data *models.StudentDto) (*models.Student, error)
	UpdateMe(ctx context.Context, id string, data *models.StudentProfileUpdateDto) (*models.Student, error) // Public profile update
	Delete(ctx context.Context, id string) error
	GetByParentPhone(ctx context.Context, phone models.PhoneNumber) ([]models.Student, error)
	Register(ctx context.Context, data *models.StudentRegistrationDto) (*StudentRegistrationResult, error)
	GetSummary(ctx context.Context, id string) (*models.StudentSummary, error)
	GetSessionsLog(ctx context.Context, coachingProgramId string, studentId string, skip, limit int64, query *query.Conditions) (*content_models.StudentSessionPagination, error)
	UpdateFavorites(ctx context.Context, studentId string, favorites []models.StudentFavorite) (*models.Student, error)
	AddFavorite(ctx context.Context, studentId string, favorite models.StudentFavorite) (*models.Student, error)
	RemoveFavorite(ctx context.Context, studentId string, favoriteType string, refId primitive.ObjectID) (*models.Student, error)
	GetEnrichedFavorites(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions, enrichPrice bool) (*models.StudentFavoritePagination, error)
	GetSuggestions(ctx context.Context, studentId string, data *models.StudentSuggestionRequestDto) (*models.StudentSuggestionResponseDto, error)
}

type studentSvcs struct {
	repo                  db.MainRepo[models.Student]
	parentrepo            db.MainRepo[models.Parent]
	courseRepo            db.MainRepo[content_models.Course]
	coachingRepo          db.MainRepo[content_models.Coaching]
	bundleRepo            db.MainRepo[content_models.Bundle]
	studentAssignmentRepo db.MainRepo[content_models.StudentAssignment]
	redeemRequestRepo     db.MainRepo[points_models.RedeemRequest]
	studentPointRepo      db.MainRepo[points_models.StudentPoint]
	studentCourseRepo     db.MainRepo[content_models.StudentCourse]
	studentSessionRepo    db.MainRepo[content_models.StudentSession]
	studentCoachingRepo   db.MainRepo[content_models.CoachingStudent]
	relationRepo          db.MainRepo[models.ParentStudentRelation]
	pricingSvcs           sales.PricingSvcs
	// parentSvcs                ParentSvcs
	withtxn     *db.WithTxn
	credSvcs    CredSvcs
	sortingsvcs sorting.SortingSvcs
	gateway     gateway.Gateway
}

func NewStudentSvcs(i *do.Injector) (StudentSvcs, error) {
	return &studentSvcs{
		repo:                  db.NewRepo[models.Student](i),
		parentrepo:            db.NewRepo[models.Parent](i),
		courseRepo:            db.NewRepo[content_models.Course](i),
		coachingRepo:          db.NewRepo[content_models.Coaching](i),
		bundleRepo:            db.NewRepo[content_models.Bundle](i),
		studentAssignmentRepo: db.NewRepo[content_models.StudentAssignment](i),
		redeemRequestRepo:     db.NewRepo[points_models.RedeemRequest](i),
		studentPointRepo:      db.NewRepo[points_models.StudentPoint](i),
		studentCourseRepo:     db.NewRepo[content_models.StudentCourse](i),
		studentSessionRepo:    db.NewRepo[content_models.StudentSession](i),
		studentCoachingRepo:   db.NewRepo[content_models.CoachingStudent](i),
		relationRepo:          db.NewRepo[models.ParentStudentRelation](i),
		pricingSvcs:           do.MustInvoke[sales.PricingSvcs](i),
		// parentSvcs:                do.MustInvoke[ParentSvcs](i),
		withtxn:     do.MustInvoke[*db.WithTxn](i),
		credSvcs:    do.MustInvoke[CredSvcs](i),
		sortingsvcs: do.MustInvoke[sorting.SortingSvcs](i),
		gateway:     do.MustInvoke[gateway.Gateway](i),
	}, nil
}

// Define the teacher lookup stage
// var assignTeacherLookup =[]bson.M{

// 	bson.M{
// 		"$lookup": bson.M{
// 			"from":         "academyTeachers", // Collection name
// 			"localField":   "assignTeacher",   // Field in Student collection
// 			"foreignField": "_id",             // Field in Teacher collection
// 			"as":           "assignTeacherObj", // Output array field
// 		},
// 	},
// 	bson.M{
// 		"$unwind": bson.M{
// 			"path":                       "$assignTeacherObj",
// 			"preserveNullAndEmptyArrays": true, // Important: keep documents even if no teacher found
// 		},
// 	},
// }

func (s *studentSvcs) GetOne(ctx context.Context, id string) (*models.StudentWithStats, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	// Use aggregation instead of simple find to include the teacher lookup
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}
	// pipeline = append(pipeline, assignTeacherLookup...)

	// Lookup for parent-student relations
	pipeline = append(pipeline, bson.M{
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

	var result []models.Student
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("student not found")
	}

	student := result[0]
	stats := models.StudentWithStats{
		Student: student,
	}

	// Calculate Total Enrolled Courses
	stats.TotalEnrolledCourses = len(student.AcademicInfo.EnrolledCourse)

	// Get Total Points from StudentPoint collection
	studentPoint, err := s.studentPointRepo.GetByFilter(ctx, bson.M{"studentId": student.Id, "trash": false})
	if err == nil && studentPoint != nil {
		stats.TotalPoints = studentPoint.TotalPoints
	} else {
		stats.TotalPoints = 0
	}

	// Calculate Course Completion Rate
	if stats.TotalEnrolledCourses > 0 {
		// Count completed courses from StudentCourse collection
		completedCoursesPipeline := []bson.M{
			{"$match": bson.M{
				"studentId": student.Id,
				"status":    "completed",
				"trash":     false,
			}},
			{"$count": "count"},
		}
		var completedCountResult []struct {
			Count int `bson:"count"`
		}
		err = s.studentCourseRepo.Aggregate(ctx, completedCoursesPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &completedCountResult)
		})
		completedCount := 0
		if err == nil && len(completedCountResult) > 0 {
			completedCount = completedCountResult[0].Count
		}
		// Calculate percentage: (completed / total enrolled) * 100
		stats.CourseCompletionRate = (float64(completedCount) / float64(stats.TotalEnrolledCourses)) * 100.0
	} else {
		stats.CourseCompletionRate = 0.0
	}

	// Calculate average course progress across all enrollments (0-100)
	var enrollments []content_models.StudentCourse
	err = s.studentCourseRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"studentId": student.Id,
			"trash":     false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	})
	if err == nil && len(enrollments) > 0 {
		sum := 0
		for _, e := range enrollments {
			sum += e.Progress
		}
		stats.AverageCourseProgress = float64(sum) / float64(len(enrollments))
	}

	// Enrich with parents info (similar to supervisor service)
	if len(student.ParentStudentRelations) > 0 {
		parentIdsSet := make(map[primitive.ObjectID]bool)
		for _, relation := range student.ParentStudentRelations {
			if relation.FirstType == "parent" {
				parentIdsSet[relation.FirstId] = true
			} else if relation.SecondType == "parent" {
				parentIdsSet[relation.SecondId] = true
			}
		}

		if len(parentIdsSet) > 0 {
			parentIds := make([]primitive.ObjectID, 0, len(parentIdsSet))
			for pid := range parentIdsSet {
				parentIds = append(parentIds, pid)
			}

			var parents []models.Parent
			err = s.parentrepo.Aggregate(ctx, []bson.M{
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

				var enrichedParents []models.EnrichedParent
				for _, rel := range student.ParentStudentRelations {
					var parentId primitive.ObjectID
					if rel.FirstType == "parent" {
						parentId = rel.FirstId
					} else if rel.SecondType == "parent" {
						parentId = rel.SecondId
					}

					if info, ok := parentInfoMap[parentId]; ok {
						enrichedParents = append(enrichedParents, models.EnrichedParent{
							ParentShortInfo: info,
							RelationType:    rel.RelationType,
						})
					}
				}
				stats.Parents = enrichedParents
			}
		}
	}

	return &stats, nil
}
func (s *studentSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentsPagination, error) {
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

	// Add teacher lookup
	// pipeline = append(pipeline, assignTeacherLookup...)

	count, err := s.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.Student
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	// Enrich students with stats
	studentsWithStats := make([]models.StudentWithStats, 0, len(result))

	// Collect all student IDs to fetch relations in bulk
	studentIds := make([]primitive.ObjectID, 0, len(result))
	for _, st := range result {
		studentIds = append(studentIds, st.Id)
	}

	// Fetch all parent-student relations for these students
	relationsMap := make(map[primitive.ObjectID][]*models.ParentStudentRelation)
	parentIdsSet := make(map[primitive.ObjectID]bool)
	if len(studentIds) > 0 {
		var allRelations []*models.ParentStudentRelation
		relPipeline := []bson.M{
			{"$match": bson.M{
				"$and": []interface{}{
					bson.M{"$or": []interface{}{
						bson.M{"firstId": bson.M{"$in": studentIds}},
						bson.M{"secondId": bson.M{"$in": studentIds}},
					}},
					bson.M{"trash": false},
				},
			}},
		}
		err = s.relationRepo.Aggregate(ctx, relPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &allRelations)
		})
		if err == nil {
			for _, rel := range allRelations {
				// Add to relations map for each student ID involved
				if rel.FirstType == "student" {
					relationsMap[rel.FirstId] = append(relationsMap[rel.FirstId], rel)
				}
				if rel.SecondType == "student" {
					relationsMap[rel.SecondId] = append(relationsMap[rel.SecondId], rel)
				}

				// Collect parent IDs
				if rel.FirstType == "parent" {
					parentIdsSet[rel.FirstId] = true
				}
				if rel.SecondType == "parent" {
					parentIdsSet[rel.SecondId] = true
				}
			}
		}
	}

	// Fetch all parents in bulk
	parentInfoMap := make(map[primitive.ObjectID]models.ParentShortInfo)
	if len(parentIdsSet) > 0 {
		parentIds := make([]primitive.ObjectID, 0, len(parentIdsSet))
		for pid := range parentIdsSet {
			parentIds = append(parentIds, pid)
		}

		var parents []models.Parent
		err = s.parentrepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": parentIds}, "trash": false}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &parents)
		})

		if err == nil {
			for _, p := range parents {
				parentInfoMap[p.Id] = models.ParentShortInfo{
					Id:        p.Id,
					FirstName: p.ParentGeneralInfo.FirstName,
					LastName:  p.ParentGeneralInfo.LastName,
					Phone:     p.ParentContactInfo.MobilePhoneNumber,
				}
			}
		}
	}

	// Pre-compute average course progress for all students in the page
	type progressAggregate struct {
		Sum   int
		Count int
	}
	progressByStudent := make(map[primitive.ObjectID]progressAggregate)
	if len(studentIds) > 0 {
		var enrollments []content_models.StudentCourse
		err = s.studentCourseRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"studentId": bson.M{"$in": studentIds},
				"trash":     false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		})
		if err != nil {
			slog.Error("failed to aggregate student course progress", "err", err)
		} else {
			for _, e := range enrollments {
				agg := progressByStudent[e.StudentId]
				agg.Sum += e.Progress
				agg.Count++
				progressByStudent[e.StudentId] = agg
			}
		}
	}

	for _, student := range result {
		stats := models.StudentWithStats{
			Student: student,
		}

		// Calculate Total Enrolled Courses
		stats.TotalEnrolledCourses = len(student.AcademicInfo.EnrolledCourse)

		// Get Total Points from StudentPoint collection
		studentPoint, err := s.studentPointRepo.GetByFilter(ctx, bson.M{"studentId": student.Id, "trash": false})
		if err == nil && studentPoint != nil {
			stats.TotalPoints = studentPoint.TotalPoints
		} else {
			stats.TotalPoints = 0
		}

		// Calculate Course Completion Rate
		if stats.TotalEnrolledCourses > 0 {
			// Count completed courses from StudentCourse collection
			completedCoursesPipeline := []bson.M{
				{"$match": bson.M{
					"studentId": student.Id,
					"status":    "completed",
					"trash":     false,
				}},
				{"$count": "count"},
			}
			var completedCountResult []struct {
				Count int `bson:"count"`
			}
			err = s.studentCourseRepo.Aggregate(ctx, completedCoursesPipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &completedCountResult)
			})
			completedCount := 0
			if err == nil && len(completedCountResult) > 0 {
				completedCount = completedCountResult[0].Count
			}
			// Calculate percentage: (completed / total enrolled) * 100
			stats.CourseCompletionRate = (float64(completedCount) / float64(stats.TotalEnrolledCourses)) * 100.0
		} else {
			stats.CourseCompletionRate = 0.0
		}

		// Calculate average course progress across all enrollments (0-100)
		if agg, ok := progressByStudent[student.Id]; ok && agg.Count > 0 {
			stats.AverageCourseProgress = float64(agg.Sum) / float64(agg.Count)
		}

		// Populate parents for each student
		if studentRelations, ok := relationsMap[student.Id]; ok {
			var enrichedParents []models.EnrichedParent
			for _, rel := range studentRelations {
				var parentId primitive.ObjectID
				if rel.FirstType == "parent" {
					parentId = rel.FirstId
				} else if rel.SecondType == "parent" {
					parentId = rel.SecondId
				}

				if info, ok := parentInfoMap[parentId]; ok {
					enrichedParents = append(enrichedParents, models.EnrichedParent{
						ParentShortInfo: info,
						RelationType:    rel.RelationType,
					})
				}
			}
			stats.Parents = enrichedParents
		}

		studentsWithStats = append(studentsWithStats, stats)
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.StudentsPagination{
		Students:   studentsWithStats,
		Pagination: pagination,
	}, nil
}

func (s *studentSvcs) GetAll(ctx context.Context, query *query.Conditions) ([]*models.Student, error) {
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

	var result []*models.Student
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	return result, nil
}
func (s *studentSvcs) Add(ctx context.Context, data *models.StudentDto) (*models.Student, error) {
	var deleteUserOnErr func() error
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {

		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return nil, err
		}

		student := &models.Student{
			StudentDto: *data,
			CreatedAt:  time.Now(),
			CreatedBy:  cfg.User.Id,
			UpdatedAt:  time.Now(),
			UpdatedBy:  cfg.User.Id,
		}

		result, err := s.credSvcs.Add(ctx, student, "student")
		if err != nil {
			return nil, err
		}
		deleteUserOnErr = result.DeleteOnErr

		student.Id = result.UserData.Id

		seq, err := s.sortingsvcs.GetAndUpdateSourceSeq(ctx, "students")
		if err != nil {
			return nil, err
		}

		student.Seq = fmt.Sprintf("STU-%d", seq)

		if err := s.repo.Add(ctx, student); err != nil {
			return nil, err
		}

		return student, nil
	})

	if err != nil {
		if deleteUserOnErr != nil {
			deleteUserOnErr()
		}
		return nil, err
	}

	return result.(*models.Student), nil
}
func (s *studentSvcs) Update(ctx context.Context, id string, data *models.StudentDto) (*models.Student, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldStudent, err := s.GetOne(ctx, id)

		if err != nil {
			return nil, err
		}

		student := &models.Student{
			StudentDto: *data,
			CreatedAt:  oldStudent.CreatedAt,
			CreatedBy:  oldStudent.CreatedBy,
			UpdatedAt:  time.Now(),
			UpdatedBy:  cfg.User.Id,
		}

		student.Id = _id
		student.Seq = oldStudent.Seq // Preserve seq

		// Sync phone if provided in update
		if data.ContactInfo.MobilePhoneNumber.Content != "" && data.ContactInfo.MobilePhoneNumber.Prefix != "" {
			student.ContactInfo.MobilePhoneNumber = data.ContactInfo.MobilePhoneNumber
		}

		filter := bson.M{"_id": _id}

		update := bson.M{"$set": student}

		updatedStudent, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if err := s.credSvcs.Update(ctx, id, student); err != nil {
			return nil, err
		}

		return updatedStudent, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Student), nil
}

// UpdateProfile - Public profile update (restricted fields only)
func (s *studentSvcs) UpdateMe(ctx context.Context, id string, data *models.StudentProfileUpdateDto) (*models.Student, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		oldStudent, err := s.GetOne(ctx, id)
		if err != nil {
			return nil, err
		}

		// Only update allowed fields - preserve admin-controlled fields
		// Prevent email/phone updates for public endpoints - preserve old values
		contactInfo := data.ContactInfo
		// Preserve mobile phone number (used for user authentication) - public cannot update it
		contactInfo.MobilePhoneNumber = oldStudent.ContactInfo.MobilePhoneNumber

		// Update user service FIRST (before updating student profile)
		// This ensures if user service update fails, student profile won't be updated
		// Update user service with names and picture
		// Do NOT update email/phone - those are admin-only fields
		userUpdateData := map[string]any{
			"firstName": data.GeneralInfo.FirstName.GetContentByLang("ar"),
			"lastName":  data.GeneralInfo.LastName.GetContentByLang("ar"),
			"email":     oldStudent.AccountStudentSetting.Email, // Preserve existing email
		}

		// Preserve existing phone if available
		if oldStudent.ContactInfo.MobilePhoneNumber.Prefix != "" && oldStudent.ContactInfo.MobilePhoneNumber.Content != "" {
			userUpdateData["phone"] = oldStudent.ContactInfo.MobilePhoneNumber.Prefix + oldStudent.ContactInfo.MobilePhoneNumber.Content
		}

		// Add picture if provided
		if data.GeneralInfo.Picture.Path != "" {
			userUpdateData["picture"] = data.GeneralInfo.Picture.Path
		}

		if err := s.credSvcs.UpdateRaw(ctx, id, userUpdateData); err != nil {
			return nil, err
		}

		// Only update student profile after user service update succeeds
		student := &models.Student{
			StudentDto: models.StudentDto{
				GeneralInfo: models.GeneralInfo{
					FirstName:   data.GeneralInfo.FirstName,
					LastName:    data.GeneralInfo.LastName,
					Gender:      data.GeneralInfo.Gender,
					DateOfBirth: data.GeneralInfo.DateOfBirth,
					Nationality: data.GeneralInfo.Nationality,
					Status:      oldStudent.GeneralInfo.Status, // Preserve status (admin-only)
					Picture:     data.GeneralInfo.Picture,
				},
				ContactInfo:           contactInfo,             // Mobile phone preserved, but parent phone can be updated
				AcademicInfo:          oldStudent.AcademicInfo, // Preserve academic info (admin-controlled)
				Other:                 oldStudent.Other,        // Preserve notes (admin-only)
				AddressDetails:        data.AddressDetails,
				AccountStudentSetting: oldStudent.AccountStudentSetting, // Preserve all account settings including email (admin-only)
				Favorites:             oldStudent.Favorites,             // Preserve favorites (not updatable via PATCH /me)
			},
			Id:                     _id,
			Seq:                    oldStudent.Seq, // Preserve seq
			CreatedAt:              oldStudent.CreatedAt,
			CreatedBy:              oldStudent.CreatedBy,
			UpdatedAt:              time.Now(),
			UpdatedBy:              cfg.User.Id,
			Trash:                  oldStudent.Trash,
			ParentStudentRelations: oldStudent.ParentStudentRelations, // Preserve relations
		}

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": student}

		updatedStudent, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		return updatedStudent, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.Student), nil
}

func (s *studentSvcs) Delete(ctx context.Context, id string) error {
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
		updatedStudent, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		if updatedStudent == nil {
			return nil, errors.New("student not found or already deleted")
		}
		if err := s.credSvcs.Delete(ctx, id); err != nil {
			return nil, err
		}

		return updatedStudent, nil
	})

	return err
}
func (s *studentSvcs) GetByParentPhone(ctx context.Context, phone models.PhoneNumber) ([]models.Student, error) {
	filter := bson.M{
		"contactInfo.parentPhoneNumber.prefix":  phone.Prefix,
		"contactInfo.parentPhoneNumber.content": phone.Content,
		"trash":                                 false,
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}
	var result []models.Student
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}
	return result, nil
}

func (s *studentSvcs) Register(ctx context.Context, data *models.StudentRegistrationDto) (*StudentRegistrationResult, error) {
	// Construct phone number from ContactInfo (validation already done in handler)
	phoneNumber := data.ContactInfo.MobilePhoneNumber.Prefix + data.ContactInfo.MobilePhoneNumber.Content

	// Register user using cred service
	var picture *string
	if data.GeneralInfo.Picture.Path != "" {
		picture = &data.GeneralInfo.Picture.Path
	}

	registerResult, err := s.credSvcs.Register(
		ctx,
		data.GeneralInfo.FirstName.GetContentByLang("ar"),
		data.GeneralInfo.LastName.GetContentByLang("ar"),
		data.Email,
		data.Password,
		data.PasswordConfirmation,
		phoneNumber,
		data.PhoneOTP,
		picture,
	)
	if err != nil {
		return nil, err
	}

	// Create student record
	result, err := s.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		// Convert registration DTO to StudentDto
		studentDto := data.ToStudentDto()
		studentDto.AccountStudentSetting = models.AccountStudentSetting{
			Email:             data.Email,
			IsActive:          false,
			ResetOnFirstLogin: false,
			JoinStatus:        "pending",
		}

		// Always set status to pending for public registration
		studentDto.GeneralInfo.Status = "pending"

		student := &models.Student{
			StudentDto: studentDto,
			Id:         registerResult.UserData.Id,
			CreatedAt:  time.Now(),
			UpdatedAt:  time.Now(),
		}

		// Generate sequence number
		seq, err := s.sortingsvcs.GetAndUpdateSourceSeq(ctx, "students")
		if err != nil {
			return nil, fmt.Errorf("failed to generate sequence number: %w", err)
		}

		student.Seq = fmt.Sprintf("STU-%d", seq)

		err = s.repo.Add(ctx, student)
		if err != nil {
			return nil, err
		}
		return student, err
	})

	if err != nil {
		// Attempt cleanup
		if cleanupErr := registerResult.DeleteOnErr(); cleanupErr != nil {
			slog.Error("Failed to cleanup user after transaction failure", "error", cleanupErr, "userId", registerResult.UserData.Id.Hex(), "email", data.Email)
		}

		slog.Error("Student registration failed in transaction", "error", err, "email", data.Email, "userId", registerResult.UserData.Id.Hex())
		return nil, helpers.InternalServerError("failed to create student record")
	}

	student := result.(*models.Student)
	slog.Info("Student registered successfully", "email", data.Email, "userId", student.Id.Hex(), "studentId", student.Id.Hex())

	return &StudentRegistrationResult{
		Student:      student,
		UserResponse: registerResult.UserResponse,
		Headers:      registerResult.Headers,
	}, nil
}

func (s *studentSvcs) GetSummary(ctx context.Context, id string) (*models.StudentSummary, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	student, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// TotalLearningHours: from actual enrollments (StudentCourse), sum course durations (Duration is in minutes)
	var totalLearningMinutes float64
	var enrollments []content_models.StudentCourse
	err = s.studentCourseRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"studentId": _id, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	if len(enrollments) > 0 {
		courseIds := make([]primitive.ObjectID, 0, len(enrollments))
		for _, e := range enrollments {
			courseIds = append(courseIds, e.CourseId)
		}
		durationPipeline := []bson.M{
			{"$match": bson.M{"_id": bson.M{"$in": courseIds}, "trash": false}},
			{"$group": bson.M{"_id": nil, "total": bson.M{"$sum": "$basicInfo.duration"}}},
		}
		var durationResult []struct {
			Total int `bson:"total"`
		}
		if err := s.courseRepo.Aggregate(ctx, durationPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &durationResult)
		}); err == nil && len(durationResult) > 0 {
			totalLearningMinutes = float64(durationResult[0].Total)
		}
	}
	totalLearningHours := totalLearningMinutes / 60.0

	// ActiveCoursesCount: count enrollments with status "active" (not from Course.basicInfo.status)
	activeCoursesCount := 0
	activeCoursesPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _id,
			"status":    contentEnums.EnrollmentStatusActive,
			"trash":     false,
		}},
		{"$count": "count"},
	}
	var activeCoursesResult []struct {
		Count int `bson:"count"`
	}
	if err := s.studentCourseRepo.Aggregate(ctx, activeCoursesPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &activeCoursesResult)
	}); err == nil && len(activeCoursesResult) > 0 {
		activeCoursesCount = activeCoursesResult[0].Count
	}

	// ActiveCoachingsCount: count coaching enrollments with status "active"
	activeCoachingsCount := 0
	activeCoachingsPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _id,
			"status":    contentEnums.EnrollmentStatusActive,
			"trash":     false,
		}},
		{"$count": "count"},
	}
	var activeCoachingsResult []struct {
		Count int `bson:"count"`
	}
	if err := s.studentCoachingRepo.Aggregate(ctx, activeCoachingsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &activeCoachingsResult)
	}); err == nil && len(activeCoachingsResult) > 0 {
		activeCoachingsCount = activeCoachingsResult[0].Count
	}

	// TotalSessionsCompleted: count StudentSession with status "completed"
	totalSessionsCompleted := 0
	sessionsCompletedPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _id,
			"status":    "completed",
			"trash":     false,
		}},
		{"$count": "count"},
	}
	var sessionsResult []struct {
		Count int `bson:"count"`
	}
	if err := s.studentSessionRepo.Aggregate(ctx, sessionsCompletedPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessionsResult)
	}); err == nil && len(sessionsResult) > 0 {
		totalSessionsCompleted = sessionsResult[0].Count
	}

	// Quiz score average (percentage)
	quizScoreAverage, err := s.calculateQuizScoreAverage(ctx, _id)
	if err != nil {
		return nil, err
	}

	// Badges earned (approved redeem requests)
	badgesCount := 0
	badgesPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": _id,
			"status":    points_enums.RedeemStatusApproved,
			"trash":     false,
		}},
		{"$count": "count"},
	}
	var badgesCountResult []struct {
		Count int `bson:"count"`
	}
	err = s.redeemRequestRepo.Aggregate(ctx, badgesPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &badgesCountResult)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	if len(badgesCountResult) > 0 {
		badgesCount = badgesCountResult[0].Count
	}

	messagesWithTeacherCount := 0

	return &models.StudentSummary{
		Student:                student.Student,
		TotalLearningHours:    totalLearningHours,
		ActiveCoursesCount:    activeCoursesCount,
		ActiveCoachingsCount:  activeCoachingsCount,
		TotalSessionsCompleted: totalSessionsCompleted,
		QuizScoreAverage:      quizScoreAverage,
		BadgesEarned:          badgesCount,
		MessagesWithTeacher:   messagesWithTeacherCount,
	}, nil
}

// calculateQuizScoreAverage calculates the average quiz score as a percentage
// It sums all student assignment scores and divides by the total possible score (number of questions)
func (s *studentSvcs) calculateQuizScoreAverage(ctx context.Context, studentId primitive.ObjectID) (float64, error) {
	// Get all student assignments to calculate total score and max possible score
	var assignments []content_models.StudentAssignment
	assignmentsPipeline := []bson.M{
		{"$match": bson.M{
			"studentId": studentId,
		}},
	}
	err := s.studentAssignmentRepo.Aggregate(ctx, assignmentsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	})
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return 0, err
	}

	if len(assignments) == 0 {
		return 0, nil
	}

	totalStudentScore := 0.0
	totalMaxScore := 0.0

	for _, assignment := range assignments {
		totalStudentScore += assignment.TotalScore
		// Calculate max possible score: assume each question is worth 1 point
		// This gives us the number of questions as the max score
		if len(assignment.Answers) > 0 {
			totalMaxScore += float64(len(assignment.Answers))
		}
	}

	// Calculate percentage: (total student score / total max score) * 100
	if totalMaxScore > 0 {
		return (totalStudentScore / totalMaxScore) * 100, nil
	}

	return 0, nil
}

func (s *studentSvcs) GetSessionsLog(ctx context.Context, coachingProgramId string, studentId string, skip, limit int64, query *query.Conditions) (*content_models.StudentSessionPagination, error) {
	_coachingProgramId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, err
	}

	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, err
	}

	// Base filter
	baseFilter := bson.M{
		"coachingProgramId": _coachingProgramId,
		"studentId":         _studentId,
		"trash":             false,
	}

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
	}

	// Count total sessions
	count, err := s.studentSessionRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Add pagination
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"date": 1}}, // Sort by date ascending
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var sessions []content_models.StudentSession
	err = s.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
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

	return &content_models.StudentSessionPagination{
		Sessions:   sessions,
		Pagination: pagination,
	}, nil
}

// UpdateFavorites replaces the entire favorites array for a student
func (s *studentSvcs) UpdateFavorites(ctx context.Context, studentId string, favorites []models.StudentFavorite) (*models.Student, error) {
	_id, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Deduplicate favorites and validate each one
	seenKeys := make(map[string]bool)
	deduplicatedFavorites := make([]models.StudentFavorite, 0, len(favorites))

	for _, fav := range favorites {
		key := fav.UniqueKey()

		// Skip duplicates
		if seenKeys[key] {
			continue
		}

		// Validate type
		if fav.Type != "course" && fav.Type != "coaching" && fav.Type != "bundle" {
			return nil, errors.New("invalid favorite type, must be 'course', 'coaching', or 'bundle'")
		}

		// Verify the referenced item exists
		var exists bool
		switch fav.Type {
		case "course":
			_, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": fav.RefId, "trash": false})
			exists = err == nil
		case "coaching":
			_, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": fav.RefId, "trash": false})
			exists = err == nil
		case "bundle":
			_, err := s.bundleRepo.GetByFilter(ctx, bson.M{"_id": fav.RefId, "trash": false})
			exists = err == nil
		}

		if !exists {
			return nil, fmt.Errorf("referenced %s with id %s does not exist", fav.Type, fav.RefId.Hex())
		}

		// Add to deduplicated list
		deduplicatedFavorites = append(deduplicatedFavorites, fav)
		seenKeys[key] = true
	}

	// Use the deduplicated favorites
	favorites = deduplicatedFavorites

	// Update the favorites array
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"favorites": favorites,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	updatedStudent, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	if updatedStudent == nil {
		return nil, errors.New("student not found")
	}

	return updatedStudent, nil
}

// AddFavorite adds a new favorite to the student's favorites list while ensuring uniqueness
func (s *studentSvcs) AddFavorite(ctx context.Context, studentId string, favorite models.StudentFavorite) (*models.Student, error) {
	_id, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get current student to access existing favorites
	currentStudent, err := s.GetOne(ctx, studentId)
	if err != nil {
		return nil, err
	}

	// Validate type
	if favorite.Type != "course" && favorite.Type != "coaching" && favorite.Type != "bundle" {
		return nil, errors.New("invalid favorite type, must be 'course', 'coaching', or 'bundle'")
	}

	// Check for duplicates
	for _, fav := range currentStudent.Favorites {
		if fav.Equals(favorite) {
			return nil, errors.New("favorite already exists")
		}
	}

	// Verify the referenced item exists
	var exists bool
	switch favorite.Type {
	case "course":
		_, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": favorite.RefId, "trash": false})
		exists = err == nil
	case "coaching":
		_, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": favorite.RefId, "trash": false})
		exists = err == nil
	case "bundle":
		_, err := s.bundleRepo.GetByFilter(ctx, bson.M{"_id": favorite.RefId, "trash": false})
		exists = err == nil
	}

	if !exists {
		return nil, fmt.Errorf("referenced %s with id %s does not exist", favorite.Type, favorite.RefId.Hex())
	}

	// Add the favorite to the list
	updatedFavorites := make([]models.StudentFavorite, 0, len(currentStudent.Favorites)+1)
	updatedFavorites = append(updatedFavorites, currentStudent.Favorites...)
	updatedFavorites = append(updatedFavorites, favorite)

	// Update the favorites array
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"favorites": updatedFavorites,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	updatedStudent, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	if updatedStudent == nil {
		return nil, errors.New("student not found")
	}

	return updatedStudent, nil
}

// RemoveFavorite removes a specific favorite from the student's favorites list
func (s *studentSvcs) RemoveFavorite(ctx context.Context, studentId string, favoriteType string, refId primitive.ObjectID) (*models.Student, error) {
	_id, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get current student to access existing favorites
	currentStudent, err := s.GetOne(ctx, studentId)
	if err != nil {
		return nil, err
	}

	// Validate type
	if favoriteType != "course" && favoriteType != "coaching" && favoriteType != "bundle" {
		return nil, errors.New("invalid favorite type, must be 'course', 'coaching', or 'bundle'")
	}

	// Find and remove the favorite
	targetFavorite := models.StudentFavorite{Type: favoriteType, RefId: refId}
	updatedFavorites := make([]models.StudentFavorite, 0, len(currentStudent.Favorites))
	found := false

	for _, fav := range currentStudent.Favorites {
		if fav.Equals(targetFavorite) {
			found = true
			continue // Skip this favorite (remove it)
		}
		updatedFavorites = append(updatedFavorites, fav)
	}

	if !found {
		return nil, errors.New("favorite not found")
	}

	// Update the favorites array
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"favorites": updatedFavorites,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	updatedStudent, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	if updatedStudent == nil {
		return nil, errors.New("student not found")
	}

	return updatedStudent, nil
}

// GetEnrichedFavorites retrieves and enriches favorites with content data
func (s *studentSvcs) GetEnrichedFavorites(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions, enrichPrice bool) (*models.StudentFavoritePagination, error) {
	// Get student to retrieve favorites
	student, err := s.GetOne(ctx, studentId)
	if err != nil {
		return nil, err
	}

	favorites := student.Favorites

	// Apply filtering if query conditions provided
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, errors.New("query not valid")
		}

		filter, err := query.ConvertToMongo()
		if err != nil {
			return nil, errors.New("error converting query to mongo")
		}

		// Filter favorites in-memory (since they're embedded)
		filteredFavorites := make([]models.StudentFavorite, 0)
		for _, fav := range favorites {
			// Convert favorite to bson.M for filtering
			favMap := bson.M{
				"type":  fav.Type,
				"refId": fav.RefId,
			}
			// Add meta fields if they exist
			if fav.Meta != nil {
				for k, v := range fav.Meta {
					favMap["meta."+k] = v
				}
			}

			// Check if favorite matches filter
			matches := true
			for key, value := range filter {
				if favValue, ok := favMap[key]; !ok || favValue != value {
					matches = false
					break
				}
			}
			if matches {
				filteredFavorites = append(filteredFavorites, fav)
			}
		}
		favorites = filteredFavorites
	}

	// Count total before pagination
	totalCount := int64(len(favorites))

	// Apply pagination
	var paginatedFavorites []models.StudentFavorite
	if int(skip) < len(favorites) {
		end := int(skip + limit)
		if end > len(favorites) {
			end = len(favorites)
		}
		paginatedFavorites = favorites[skip:end]
	}

	// Enrich favorites with content data
	enrichedFavorites := make([]models.StudentFavorite, 0, len(paginatedFavorites))

	// Collect IDs by type for batch fetching
	courseIds := make([]primitive.ObjectID, 0)
	coachingIds := make([]primitive.ObjectID, 0)
	bundleIds := make([]primitive.ObjectID, 0)

	for _, fav := range paginatedFavorites {
		switch fav.Type {
		case "course":
			courseIds = append(courseIds, fav.RefId)
		case "coaching":
			coachingIds = append(coachingIds, fav.RefId)
		case "bundle":
			bundleIds = append(bundleIds, fav.RefId)
		}
	}

	// Batch fetch courses
	courseMap := make(map[primitive.ObjectID]*content_models.StudentCourseResponse)
	if len(courseIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": courseIds},
				"trash": false,
			}},
		}

		// Enrich with price
		if enrichPrice {
			stages, err := s.pricingSvcs.GetPriceEnrichmentStages(ctx, contentEnums.CartTypeCourse, "$_id")
			if err == nil {
				pipeline = append(pipeline, stages...)
			}
		}

		var courses []content_models.StudentCourseResponse
		if err := s.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courses)
		}); err == nil {
			for i := range courses {
				courseMap[courses[i].Id] = &courses[i]
			}
		}
	}

	// Batch fetch coachings
	coachingMap := make(map[primitive.ObjectID]*content_models.StudentCoachingResponse)
	if len(coachingIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": coachingIds},
				"trash": false,
			}},
		}

		// Enrich with price
		if enrichPrice {
			stages, err := s.pricingSvcs.GetPriceEnrichmentStages(ctx, contentEnums.CartTypeCoaching, "$_id")
			if err == nil {
				pipeline = append(pipeline, stages...)
			}
		}

		var coachings []content_models.StudentCoachingResponse
		if err := s.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachings)
		}); err == nil {
			for i := range coachings {
				coachingMap[coachings[i].Id] = &coachings[i]
			}
		}
	}

	// Batch fetch bundles
	bundleMap := make(map[primitive.ObjectID]*content_models.StudentBundleResponse)
	if len(bundleIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": bundleIds},
				"trash": false,
			}},
		}

		// Enrich with price
		if enrichPrice {
			stages, err := s.pricingSvcs.GetPriceEnrichmentStages(ctx, contentEnums.CartTypeBundle, "$_id")
			if err == nil {
				pipeline = append(pipeline, stages...)
			}
		}

		var bundles []content_models.StudentBundleResponse
		if err := s.bundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &bundles)
		}); err == nil {
			for i := range bundles {
				bundleMap[bundles[i].Id] = &bundles[i]
			}
		}
	}

	// Enrich each favorite with its data
	for _, fav := range paginatedFavorites {
		enriched := fav
		switch fav.Type {
		case "course":
			if course, ok := courseMap[fav.RefId]; ok {
				course.IsFavorite = true
				enriched.Data = course
			}
		case "coaching":
			if coaching, ok := coachingMap[fav.RefId]; ok {
				coaching.IsFavorite = true
				enriched.Data = coaching
			}
		case "bundle":
			if bundle, ok := bundleMap[fav.RefId]; ok {
				bundle.IsFavorite = true
				enriched.Data = bundle
			}
		}
		enrichedFavorites = append(enrichedFavorites, enriched)
	}

	var totalPages float64 = math.Ceil(float64(totalCount) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: totalCount,
	}

	return &models.StudentFavoritePagination{
		Favorites:  enrichedFavorites,
		Pagination: pagination,
	}, nil
}

func (s *studentSvcs) GetSuggestions(ctx context.Context, studentId string, data *models.StudentSuggestionRequestDto) (*models.StudentSuggestionResponseDto, error) {
	var class string
	var country string
	limit := 10

	if data != nil {
		class = data.Class
		country = data.Country
		if data.Limit > 0 {
			limit = data.Limit
		}
	}

	// If not provided in request, get from student profile
	if class == "" || country == "" {
		student, err := s.GetOne(ctx, studentId)
		if err == nil && student != nil {
			if class == "" {
				class = student.AcademicInfo.Class
			}
			if country == "" {
				country = student.AddressDetails.Country
			}
		}
	}

	// Build filters
	courseFilter := bson.M{"trash": false, "basicInfo.status": "active"}
	coachingFilter := bson.M{"trash": false}

	if class != "" {
		courseFilter["basicInfo.gradeLevel"] = class
		coachingFilter["class"] = class
	}
	if country != "" {
		courseFilter["basicInfo.country"] = country
		coachingFilter["country"] = country
	}

	// Fetch random courses
	coursePipeline := []bson.M{
		{"$match": courseFilter},
		{"$sample": bson.M{"size": limit}},
	}
	// Enrich with price
	stages, err := s.pricingSvcs.GetPriceEnrichmentStages(ctx, contentEnums.CartTypeCourse, "$_id")
	if err == nil {
		coursePipeline = append(coursePipeline, stages...)
	}

	var courses []content_models.StudentCourseResponse
	_ = s.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	})

	// Fetch random coachings
	coachingPipeline := []bson.M{
		{"$match": coachingFilter},
		{"$sample": bson.M{"size": limit}},
	}
	// Enrich with price
	coachingStages, err := s.pricingSvcs.GetPriceEnrichmentStages(ctx, contentEnums.CartTypeCoaching, "$_id")
	if err == nil {
		coachingPipeline = append(coachingPipeline, coachingStages...)
	}

	var coachings []content_models.StudentCoachingResponse
	_ = s.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachings)
	})

	// Fallback: If no results with class/country, try broader search if we had filters
	if len(courses) == 0 && (class != "" || country != "") {
		delete(courseFilter, "basicInfo.gradeLevel")
		delete(courseFilter, "basicInfo.country")
		coursePipeline = []bson.M{
			{"$match": courseFilter},
			{"$sample": bson.M{"size": limit}},
		}
		if err == nil {
			coursePipeline = append(coursePipeline, stages...)
		}
		_ = s.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courses)
		})
	}

	if len(coachings) == 0 && (class != "" || country != "") {
		delete(coachingFilter, "class")
		delete(coachingFilter, "country")
		coachingPipeline = []bson.M{
			{"$match": coachingFilter},
			{"$sample": bson.M{"size": limit}},
		}
		if err == nil {
			coachingPipeline = append(coachingPipeline, coachingStages...)
		}
		_ = s.coachingRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachings)
		})
	}

	// Convert to any slice for response
	courseAnys := make([]any, len(courses))
	for i, v := range courses {
		courseAnys[i] = v
	}
	coachingAnys := make([]any, len(coachings))
	for i, v := range coachings {
		coachingAnys[i] = v
	}

	return &models.StudentSuggestionResponseDto{
		Courses:   courseAnys,
		Coachings: coachingAnys,
	}, nil
}
