package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func (c *coursesvcs) GetAllCourseMembers(ctx context.Context, courseId string, skip, limit int64, query *query.Conditions) (*models.MemberCoursePagination, error) {
	// Add authorization check - users who can view course members
	canView, err := c.authSvc.CanViewCourseMembers(ctx, courseId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view course members")
	}

	_id, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"courseId": _id, "trash": false}},
		{"$match": filter},
	}

	count, err := c.studentCourseRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	// Populate student
	pipeline = append(pipeline, []bson.M{
		{
			"$lookup": bson.M{
				"from": "academyStudents",
				"let":  bson.M{"studentId": "$studentId"},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{"$eq": []interface{}{"$_id", "$$studentId"}}}},
				},
				"as": "student",
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
			},
		},
	}...)

	var result []models.StudentCourseRes
	if err := c.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.MemberCoursePagination{
		StudentCourses: result,
		Pagination:     pagination,
	}, nil
}

func (c *coursesvcs) GetEnrolledCourses(ctx context.Context, studentId string, skip, limit int64, queryCond *query.Conditions) (*models.EnrolledCoursePagination, error) {
	// Add authorization check - users who can view the student's course data
	canView, err := c.authSvc.CanViewStudentCourseData(ctx, studentId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view this student's course data")
	}

	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	// Normalize nil queries to avoid panics when calling methods on it.
	if queryCond == nil {
		queryCond = &query.Conditions{}
	}

	if err := queryCond.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := queryCond.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	// Base pipeline: match student enrollments
	basePipeline := []bson.M{
		{"$match": bson.M{"studentId": _studentId, "trash": false}},
	}

	// Lookup course details
	courseLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academyCourses",
				"localField":   "courseId",
				"foreignField": "_id",
				"as":           "course",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$course",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$match": bson.M{"course.trash": false},
		},
		// Lookup reviews to calculate rating
		{
			"$lookup": bson.M{
				"from": "academyContentReviews",
				"let":  bson.M{"courseId": "$course._id"},
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
			"$addFields": bson.M{
				"course.rating": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$reviews", bson.A{}}}}, 0}},
						bson.M{"$avg": "$reviews.rating"},
						nil,
					},
				},
			},
		},
	}

	pipeline := append(basePipeline, courseLookup...)

	// Apply user-provided filters AFTER course is available in pipeline so course.basicInfo fields can be filtered.
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Count total enrollments
	count, err := c.studentCourseRepo.Count(ctx, pipeline)
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
				"let":  bson.M{"teacherIds": bson.M{"$ifNull": bson.A{"$course.basicInfo.teachers", bson.A{}}}},
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

	// Lookup supervisors using SupervisorIds field
	supervisorLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from": "academySupervisors",
				"let":  bson.M{"supervisorIds": bson.M{"$ifNull": bson.A{"$course.basicInfo.supervisorIds", bson.A{}}}},
				"pipeline": []bson.M{
					{"$match": bson.M{
						"$expr": bson.M{"$in": []string{"$_id", "$$supervisorIds"}},
						"trash": false,
					}},
					{"$project": bson.M{
						"_id":       1,
						"firstName": 1,
						"lastName":  1,
						"email":     "$accountSettings.email",
						"picture":   "$profilePicture",
					}},
				},
				"as": "supervisors",
			},
		},
	}

	pipeline = append(pipeline, supervisorLookup...)

	// Project final structure
	projectStage := bson.M{
		"$project": bson.M{
			"enrollment": bson.M{
				"_id":               "$_id",
				"studentId":         "$studentId",
				"courseId":          "$courseId",
				"bundleId":          "$bundleId",
				"startDate":         "$startDate",
				"endDate":           "$endDate",
				"status":            "$status",
				"certificate":       "$certificate",
				"certificateStatus": "$certificateStatus",
				"progress":          "$progress",
				"logins":            "$logins",
				"lastActivity":      "$lastActivity",
				"completedLessons":  "$completedLessons",
				"createdBy":         "$createdBy",
				"updatedBy":         "$updatedBy",
				"createdAt":         "$createdAt",
				"updatedAt":         "$updatedAt",
				"trash":             "$trash",
			},
			"studentStatus": "$status", // Student status from StudentCourse.Status
			"course": bson.M{
				"_id":         "$course._id",
				"basicInfo":   "$course.basicInfo",
				"content":     "$course.content",
				"thumbnail":   "$course.basicInfo.thumbnail",
				"description": "$course.basicInfo.description",
				"status":      "$course.basicInfo.status",
				"type":        "$course.basicInfo.type",
				"duration":    "$course.basicInfo.duration",
				"rating":      "$course.rating",
				"createdAt":   "$course.createdAt",
				"updatedAt":   "$course.updatedAt",
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
			"supervisors": bson.M{ // Added supervisors projection
				"$map": bson.M{
					"input": "$supervisors",
					"as":    "supervisor",
					"in": bson.M{
						"_id":       "$$supervisor._id",
						"firstName": "$$supervisor.firstName",
						"lastName":  "$$supervisor.lastName",
						"email":     "$$supervisor.email",
						"picture":   "$$supervisor.picture",
					},
				},
			},
			"totalLessons": bson.M{
				"$size": bson.M{
					"$reduce": bson.M{
						"input":        bson.M{"$ifNull": bson.A{"$course.content", bson.A{}}},
						"initialValue": bson.A{},
						"in": bson.M{
							"$concatArrays": bson.A{
								"$$value",
								bson.M{
									"$concatArrays": bson.A{
										bson.M{"$ifNull": bson.A{"$$this.lessons", bson.A{}}},
										bson.M{
											"$reduce": bson.M{
												"input":        bson.M{"$ifNull": bson.A{"$$this.subSections", bson.A{}}},
												"initialValue": bson.A{},
												"in": bson.M{
													"$concatArrays": bson.A{
														"$$value",
														bson.M{"$ifNull": bson.A{"$$this.lessons", bson.A{}}},
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
			"completedLessons": bson.M{"$size": bson.M{"$ifNull": bson.A{"$completedLessons", bson.A{}}}},
			"upcomingLiveSession": bson.M{
				"$cond": bson.A{
					bson.M{"$and": bson.A{
						bson.M{"$eq": bson.A{"$course.settings.liveSession.active", true}},
						bson.M{"$gte": bson.A{"$course.settings.liveSession.schedule", time.Now()}},
					}},
					bson.M{
						"title":     "$course.settings.liveSession.title",
						"schedule":  "$course.settings.liveSession.schedule",
						"thumbnail": "$course.settings.liveSession.thumbnail",
						"url":       "$course.settings.liveSession.url",
						"active":    "$course.settings.liveSession.active",
					},
					nil,
				},
			},
		},
	}

	pipeline = append(pipeline, projectStage)

	var result []models.EnrolledCourseResponse
	if err := c.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	for i := range result {
		result[i].CompletedSessionsBySection = c.calculateCompletedBySection(result[i].Course.Content, result[i].Enrollment.CompletedLessons)
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.EnrolledCoursePagination{
		Courses:    result,
		Pagination: pagination,
	}, nil
}

func (c *coursesvcs) GetEnrolledBundles(ctx context.Context, studentId string, skip, limit int64, query *query.Conditions) (*models.EnrolledBundlePagination, error) {
	// Add authorization check - users who can view the student's course data
	canView, err := c.authSvc.CanViewStudentCourseData(ctx, studentId)
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view this student's course data")
	}

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

	// Base pipeline: match student bundle enrollments
	basePipeline := []bson.M{
		{"$match": bson.M{"studentId": _studentId, "trash": false}},
	}

	// Lookup bundle details
	bundleLookup := []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academyBundles",
				"localField":   "bundleId",
				"foreignField": "_id",
				"as":           "bundle",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$bundle",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$match": bson.M{"bundle.trash": false},
		},
	}

	pipeline := append(basePipeline, bundleLookup...)

	// Apply user-provided filters after bundle lookup
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}

	// Count total enrollments
	count, err := c.studentBundleRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	// Add pagination
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	// Project initial structure
	projectStage := bson.M{
		"$project": bson.M{
			"enrollment": bson.M{
				"_id":       "$_id",
				"studentId": "$studentId",
				"bundleId":  "$bundleId",
				"createdAt": "$createdAt",
				"updatedAt": "$updatedAt",
				"createdBy": "$createdBy",
				"updatedBy": "$updatedBy",
				"trash":     "$trash",
			},
			"bundle": bson.M{
				"_id":         "$bundle._id",
				"name":        "$bundle.name",
				"status":      "$bundle.status",
				"thumbnail":   "$bundle.thumbnail",
				"description": "$bundle.description",
				"content":     "$bundle.content",
				"createdAt":   "$bundle.createdAt",
				"updatedAt":   "$bundle.updatedAt",
			},
		},
	}

	pipeline = append(pipeline, projectStage)

	var result []struct {
		Enrollment models.StudentBundle  `bson:"enrollment" json:"enrollment"`
		Bundle     models.BundleEnriched `bson:"bundle" json:"bundle"`
	}

	if err := c.studentBundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Calculate progress for each bundle
	enrolledBundles := make([]models.EnrolledBundleResponse, 0, len(result))
	for _, item := range result {
		// Calculate progress for this bundle
		progress, contentInfo, err := c.calculateBundleProgress(ctx, _studentId, item.Bundle.Id, item.Bundle.Content)
		if err != nil {
			return nil, err
		}

		enrolledBundles = append(enrolledBundles, models.EnrolledBundleResponse{
			Bundle:      item.Bundle,
			Enrollment:  item.Enrollment,
			Progress:    progress,
			ContentInfo: contentInfo,
		})
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.EnrolledBundlePagination{
		Bundles:    enrolledBundles,
		Pagination: pagination,
	}, nil
}

func (c *coursesvcs) MarkLessonAsCompleted(ctx context.Context, courseId, lessonId string) (*models.EnrolledCourseResponse, error) {
	_courseId, err := primitive.ObjectIDFromHex(courseId)
	if err != nil {
		return nil, errors.New("invalid course id")
	}

	_lessonId, err := primitive.ObjectIDFromHex(lessonId)
	if err != nil {
		return nil, errors.New("invalid lesson id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	enrollment, err := c.studentCourseRepo.GetByFilter(ctx, bson.M{
		"courseId":  _courseId,
		"studentId": cfg.User.Id,
		"trash":     false,
	})
	if err != nil {
		return nil, errors.New("enrollment not found")
	}

	update := bson.M{
		"$set": bson.M{"updatedAt": time.Now()},
	}

	if enrollment.CompletedLessons == nil {
		update["$set"].(bson.M)["completedLessons"] = []primitive.ObjectID{_lessonId}
	} else {
		update["$addToSet"] = bson.M{"completedLessons": _lessonId}
	}

	updatedEnrollment, err := c.studentCourseRepo.Patch(ctx, bson.M{"_id": enrollment.Id}, update)
	if err != nil {
		return nil, err
	}

	course, err := c.repo.GetByFilter(ctx, bson.M{"_id": _courseId, "trash": false})
	if err != nil {
		return nil, errors.New("course not found")
	}

	totalLessons := c.countLessons(course.Content)
	completedLessonsCount := len(updatedEnrollment.CompletedLessons)
	if totalLessons > 0 {
		progress := (completedLessonsCount * 100) / totalLessons
		if progress > 100 {
			progress = 100
		}

		progressUpdate := bson.M{
			"$set": bson.M{
				"progress":  progress,
				"updatedAt": time.Now(),
			},
		}

		if progress == 100 {
			progressUpdate["$set"].(bson.M)["status"] = enums.EnrollmentStatusCompleted
		}

		updatedEnrollment, err = c.studentCourseRepo.Patch(ctx, bson.M{"_id": enrollment.Id}, progressUpdate)
		if err != nil {
			return nil, err
		}
	}

	studentStatus := string(enrollment.Status)
	if updatedEnrollment.Status != "" {
		studentStatus = string(updatedEnrollment.Status)
	}

	duration := course.BasicInfo.Duration

	return &models.EnrolledCourseResponse{
		Course: models.CourseEnriched{
			Id:          course.Id,
			BasicInfo:   course.BasicInfo,
			Content:     course.Content,
			Thumbnail:   course.BasicInfo.Thumbnail,
			Description: course.BasicInfo.Description,
			Status:      course.BasicInfo.Status,
			Type:        course.BasicInfo.Type,
			Duration:    duration,
			Rating:      course.Rating,
			CreatedAt:   course.CreatedAt,
			UpdatedAt:   course.UpdatedAt,
		},
		Enrollment:                 *updatedEnrollment,
		StudentStatus:              studentStatus,
		Teachers:                   []models.TeacherBasicInfo{},
		Supervisors:                []models.SupervisorBasicInfo{},
		TotalLessons:               totalLessons,
		CompletedLessons:           completedLessonsCount,
		CompletedSessionsBySection: c.calculateCompletedBySection(course.Content, updatedEnrollment.CompletedLessons),
		UpcomingLiveSession:        nil,
	}, nil
}

// GetStudentCourseStats returns per-course and aggregated lesson stats for a student
func (c *coursesvcs) GetStudentCourseStats(ctx context.Context, studentId string) (*models.StudentCourseStatsResponse, error) {
	_studentId, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	// Fetch all enrollments for the student
	var enrollments []models.StudentCourse
	err = c.studentCourseRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"studentId": _studentId,
			"trash":     false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	})
	if err != nil {
		return nil, err
	}

	if len(enrollments) == 0 {
		return &models.StudentCourseStatsResponse{
			Courses:          []models.StudentCourseStatsItem{},
			TotalLessons:     0,
			CompletedLessons: 0,
			OverallProgress:  0,
		}, nil
	}

	courseIds := make([]primitive.ObjectID, 0, len(enrollments))
	for _, e := range enrollments {
		courseIds = append(courseIds, e.CourseId)
	}

	// Fetch all related courses in bulk
	var courses []models.Course
	err = c.repo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{
			"_id":   bson.M{"$in": courseIds},
			"trash": false,
		}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	})
	if err != nil {
		return nil, err
	}

	courseMap := make(map[primitive.ObjectID]models.Course, len(courses))
	for _, course := range courses {
		courseMap[course.Id] = course
	}

	items := make([]models.StudentCourseStatsItem, 0, len(enrollments))
	totalLessonsAll := 0
	completedLessonsAll := 0

	for _, enrollment := range enrollments {
		course, ok := courseMap[enrollment.CourseId]
		if !ok {
			continue
		}

		totalLessons := c.countLessons(course.Content)
		completedLessons := len(enrollment.CompletedLessons)

		totalLessonsAll += totalLessons
		completedLessonsAll += completedLessons

		progress := 0
		if totalLessons > 0 {
			progress = (completedLessons * 100) / totalLessons
		}

		items = append(items, models.StudentCourseStatsItem{
			CourseId:         course.Id,
			Name:             course.BasicInfo.Name,
			Thumbnail:        course.BasicInfo.Thumbnail,
			TotalLessons:     totalLessons,
			CompletedLessons: completedLessons,
			Progress:         progress,
		})
	}

	overallProgress := 0.0
	if totalLessonsAll > 0 {
		overallProgress = (float64(completedLessonsAll) / float64(totalLessonsAll)) * 100.0
	}

	return &models.StudentCourseStatsResponse{
		Courses:          items,
		TotalLessons:     totalLessonsAll,
		CompletedLessons: completedLessonsAll,
		OverallProgress:  overallProgress,
	}, nil
}

func (c *coursesvcs) calculateCompletedBySection(sections []models.Section, completedLessons []primitive.ObjectID) map[string]int {
	result := make(map[string]int)
	completedSet := make(map[primitive.ObjectID]bool)
	for _, lid := range completedLessons {
		completedSet[lid] = true
	}

	for _, section := range sections {
		count := 0
		for _, lesson := range section.Lessons {
			if completedSet[lesson.Id] {
				count++
			}
		}
		for _, subSection := range section.SubSections {
			for _, lesson := range subSection.Lessons {
				if completedSet[lesson.Id] {
					count++
				}
			}
		}
		if count > 0 {
			result[section.Id.Hex()] = count
		}
	}
	return result
}

// calculateBundleProgress calculates the overall progress of a bundle based on its content
func (c *coursesvcs) calculateBundleProgress(ctx context.Context, studentId primitive.ObjectID, bundleId primitive.ObjectID, content []models.BundleContent) (int, []models.ContentInfo, error) {
	if len(content) == 0 {
		return 0, []models.ContentInfo{}, nil
	}

	var totalProgress int
	contentInfo := make([]models.ContentInfo, 0, len(content))

	// Collect course and coaching IDs
	courseIds := make([]primitive.ObjectID, 0)
	coachingIds := make([]primitive.ObjectID, 0)

	for _, item := range content {
		if item.Type == "course" {
			courseIds = append(courseIds, item.RefId)
		} else if item.Type == "coaching" {
			coachingIds = append(coachingIds, item.RefId)
		}
	}

	// Get course progress
	courseProgressMap := make(map[primitive.ObjectID]int)
	courseNameMap := make(map[primitive.ObjectID]transl.Localizable[string])
	if len(courseIds) > 0 {
		coursePipeline := []bson.M{
			{"$match": bson.M{
				"studentId": studentId,
				"bundleId":  bundleId,
				"courseId":  bson.M{"$in": courseIds},
				"trash":     false,
			}},
			{"$project": bson.M{
				"courseId": 1,
				"progress": 1,
			}},
		}

		var courseEnrollments []struct {
			CourseId primitive.ObjectID `bson:"courseId"`
			Progress int                `bson:"progress"`
		}

		if err := c.studentCourseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courseEnrollments)
		}); err != nil {
			return 0, nil, err
		}

		for _, enrollment := range courseEnrollments {
			courseProgressMap[enrollment.CourseId] = enrollment.Progress
		}

		// Fetch course names to enrich content info
		courseNamePipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": courseIds},
				"trash": false,
			}},
			{"$project": bson.M{
				"_id": 1,
				"name": bson.M{
					"$ifNull": bson.A{"$basicInfo.name", bson.M{}},
				},
			}},
		}

		var courseNameDocs []struct {
			Id   primitive.ObjectID         `bson:"_id"`
			Name transl.Localizable[string] `bson:"name"`
		}

		if err := c.repo.Aggregate(ctx, courseNamePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courseNameDocs)
		}); err != nil {
			return 0, nil, err
		}

		for _, course := range courseNameDocs {
			courseNameMap[course.Id] = course.Name
		}
	}

	// Get coaching progress
	coachingProgressMap := make(map[primitive.ObjectID]int)
	coachingNameMap := make(map[primitive.ObjectID]transl.Localizable[string])
	if len(coachingIds) > 0 {
		// Group sessions by coaching program to calculate progress
		coachingPipeline := []bson.M{
			{"$match": bson.M{
				"studentId":         studentId,
				"bundleId":          bundleId,
				"coachingProgramId": bson.M{"$in": coachingIds},
				"trash":             false,
			}},
			{"$group": bson.M{
				"_id":           "$coachingProgramId",
				"totalSessions": bson.M{"$sum": 1},
				"completedSessions": bson.M{
					"$sum": bson.M{
						"$cond": bson.A{
							bson.M{"$eq": bson.A{"$status", enums.StudentSessionStatusCompleted}},
							1,
							0,
						},
					},
				},
			}},
			{"$project": bson.M{
				"coachingProgramId": "$_id",
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

		var coachingProgress []struct {
			CoachingProgramId primitive.ObjectID `bson:"coachingProgramId"`
			Progress          int                `bson:"progress"`
		}

		if err := c.studentSessionRepo.Aggregate(ctx, coachingPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachingProgress)
		}); err != nil {
			return 0, nil, err
		}

		for _, prog := range coachingProgress {
			coachingProgressMap[prog.CoachingProgramId] = prog.Progress
		}

		// Fetch coaching names to enrich content info
		coachingNamePipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": coachingIds},
				"trash": false,
			}},
			{"$project": bson.M{
				"_id":   1,
				"title": 1,
			}},
		}

		var coachingNameDocs []struct {
			Id    primitive.ObjectID         `bson:"_id"`
			Title transl.Localizable[string] `bson:"title"`
		}

		if err := c.coachingRepo.Aggregate(ctx, coachingNamePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &coachingNameDocs)
		}); err != nil {
			return 0, nil, err
		}

		for _, coaching := range coachingNameDocs {
			coachingNameMap[coaching.Id] = coaching.Title
		}
	}

	// Calculate progress for each content item and overall average
	for _, item := range content {
		var progress int
		if item.Type == "course" {
			progress = courseProgressMap[item.RefId]
			contentInfo = append(contentInfo, models.ContentInfo{
				Type:     item.Type,
				RefId:    item.RefId,
				Progress: progress,
				Name:     courseNameMap[item.RefId],
			})
		} else if item.Type == "coaching" {
			progress = coachingProgressMap[item.RefId]
			contentInfo = append(contentInfo, models.ContentInfo{
				Type:     item.Type,
				RefId:    item.RefId,
				Progress: progress,
				Name:     coachingNameMap[item.RefId],
			})
			totalProgress += progress
			continue
		}

		totalProgress += progress
	}

	// Calculate average progress
	avgProgress := 0
	if len(content) > 0 {
		avgProgress = totalProgress / len(content)
	}

	return avgProgress, contentInfo, nil
}

func (c *coursesvcs) CanViewStudentCourseData(ctx context.Context, studentId string) (bool, error) {
	return c.authSvc.CanViewStudentCourseData(ctx, studentId)
}
