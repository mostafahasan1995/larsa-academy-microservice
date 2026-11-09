package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type StudentContentProgressSvcs interface {
	UpsertProgress(ctx context.Context, dto *models.StudentContentProgress) (*models.StudentContentProgress, error)
	GetProgress(ctx context.Context, studentId, contentId, courseId, lessonId primitive.ObjectID) (*models.StudentContentProgress, error)
	List(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentContentProgressPagination, error)
	ValidateContent(ctx context.Context, contentId primitive.ObjectID) (contentType string, err error)
	FindContentInCourse(ctx context.Context, contentId primitive.ObjectID) (courseId, lessonId primitive.ObjectID, contentType string, err error)
}

type studentContentProgressSvcs struct {
	repo       db.MainRepo[models.StudentContentProgress]
	courseRepo db.MainRepo[models.Course]
	i          *do.Injector
}

func NewStudentContentProgressSvcs(i *do.Injector) (StudentContentProgressSvcs, error) {
	return &studentContentProgressSvcs{
		repo:       db.NewRepo[models.StudentContentProgress](i),
		courseRepo: db.NewRepo[models.Course](i),
		i:          i,
	}, nil
}

func (s *studentContentProgressSvcs) FindContentInCourse(ctx context.Context, contentId primitive.ObjectID) (courseId, lessonId primitive.ObjectID, contentType string, err error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$unwind": "$content"},
		{"$unwind": "$content.lessons"},
		{"$unwind": "$content.lessons.content"},
		{"$match": bson.M{"content.lessons.content._id": contentId}},
		{"$project": bson.M{
			"_id":         1,
			"lessonId":    "$content.lessons._id",
			"contentType": "$content.lessons.content.type",
		}},
		{"$limit": 1},
	}

	var results []bson.M
	err = s.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	})
	if err != nil {
		return primitive.NilObjectID, primitive.NilObjectID, "", err
	}

	if len(results) == 0 {
		pipeline = []bson.M{
			{"$match": bson.M{"trash": false}},
			{"$unwind": "$content"},
			{"$unwind": "$content.subSections"},
			{"$unwind": "$content.subSections.lessons"},
			{"$unwind": "$content.subSections.lessons.content"},
			{"$match": bson.M{"content.subSections.lessons.content._id": contentId}},
			{"$project": bson.M{
				"_id":         1,
				"lessonId":    "$content.subSections.lessons._id",
				"contentType": "$content.subSections.lessons.content.type",
			}},
			{"$limit": 1},
		}
		err = s.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &results)
		})
		if err != nil {
			return primitive.NilObjectID, primitive.NilObjectID, "", err
		}
	}

	// If still not found, try deeper nesting (subSections within subSections)
	if len(results) == 0 {
		pipeline = []bson.M{
			{"$match": bson.M{"trash": false}},
			{"$unwind": "$content"},
			{"$unwind": "$content.subSections"},
			{"$unwind": "$content.subSections.subSections"},
			{"$unwind": "$content.subSections.subSections.lessons"},
			{"$unwind": "$content.subSections.subSections.lessons.content"},
			{"$match": bson.M{"content.subSections.subSections.lessons.content._id": contentId}},
			{"$project": bson.M{
				"_id":         1,
				"lessonId":    "$content.subSections.subSections.lessons._id",
				"contentType": "$content.subSections.subSections.lessons.content.type",
			}},
			{"$limit": 1},
		}
		err = s.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &results)
		})
		if err != nil {
			return primitive.NilObjectID, primitive.NilObjectID, "", err
		}
	}

	if len(results) == 0 {
		return primitive.NilObjectID, primitive.NilObjectID, "", fmt.Errorf("content not found in any course")
	}

	courseId = results[0]["_id"].(primitive.ObjectID)
	lessonId = results[0]["lessonId"].(primitive.ObjectID)
	contentType = results[0]["contentType"].(string)
	return courseId, lessonId, contentType, nil
}

func (s *studentContentProgressSvcs) UpsertProgress(ctx context.Context, dto *models.StudentContentProgress) (*models.StudentContentProgress, error) {
	// Auto-populate courseId and lessonId if not provided
	if dto.CourseId.IsZero() || dto.LessonId.IsZero() {
		courseId, lessonId, _, err := s.FindContentInCourse(ctx, dto.ContentId)
		if err != nil {
			return nil, fmt.Errorf("failed to find content in course: %v", err)
		}
		dto.CourseId = courseId
		dto.LessonId = lessonId
	}

	// Use composite key: studentId + contentId + courseId + lessonId for perfect uniqueness
	filter := bson.M{
		"studentId": dto.StudentId,
		"contentId": dto.ContentId,
		"courseId":  dto.CourseId,
		"lessonId":  dto.LessonId,
		"trash":     false,
	}

	existing, err := s.repo.GetByFilter(ctx, filter)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}

	now := time.Now()
	if existing != nil {
		update := bson.M{
			"$set": bson.M{
				"currentTimeSeconds": dto.CurrentTimeSeconds,
				"courseId":           dto.CourseId,
				"lessonId":           dto.LessonId,
				"updatedAt":          now,
			},
		}
		updated, err := s.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}
		return updated, nil
	}

	dto.Id = primitive.NewObjectID()
	dto.UpdatedAt = now
	dto.Trash = false
	if err := s.repo.Add(ctx, dto); err != nil {
		return nil, err
	}

	return dto, nil
}

func (s *studentContentProgressSvcs) GetProgress(ctx context.Context, studentId, contentId, courseId, lessonId primitive.ObjectID) (*models.StudentContentProgress, error) {
	// If courseId or lessonId not provided, lookup from contentId
	if courseId.IsZero() || lessonId.IsZero() {
		lookupCourseId, lookupLessonId, _, err := s.FindContentInCourse(ctx, contentId)
		if err != nil {
			// If content not found, return nil (not error)
			return nil, nil
		}
		courseId = lookupCourseId
		lessonId = lookupLessonId
	}

	// Use composite key: studentId + contentId + courseId + lessonId for perfect uniqueness
	filter := bson.M{
		"studentId": studentId,
		"contentId": contentId,
		"courseId":  courseId,
		"lessonId":  lessonId,
		"trash":     false,
	}

	progress, err := s.repo.GetByFilter(ctx, filter)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, nil
		}
		return nil, err
	}

	return progress, nil
}

func (s *studentContentProgressSvcs) List(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.StudentContentProgressPagination, error) {
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
	}

	filter := bson.M{"trash": false}
	if query != nil {
		queryFilter, err := query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		for k, v := range queryFilter {
			filter[k] = v
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{
			"$facet": bson.M{
				"metadata": []bson.M{
					{"$count": "total"},
				},
				"data": []bson.M{
					{"$sort": bson.M{"_id": -1}},
					{"$skip": skip},
					{"$limit": limit},
				},
			},
		},
	}

	type facetResult struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.StudentContentProgress `bson:"data"`
	}

	var results []facetResult
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &models.StudentContentProgressPagination{
			Progress: []models.StudentContentProgress{},
			Pagination: types.Pagination{
				TotalPages: 0,
				PerPage:    limit,
				TotalCount: 0,
			},
		}, nil
	}

	result := results[0]
	count := int64(0)
	if len(result.Metadata) > 0 {
		count = result.Metadata[0].Total
	}

	progressList := result.Data
	if progressList == nil {
		progressList = []models.StudentContentProgress{}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.StudentContentProgressPagination{
		Progress: progressList,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *studentContentProgressSvcs) ValidateContent(ctx context.Context, contentId primitive.ObjectID) (contentType string, err error) {
	_, _, contentType, err = s.FindContentInCourse(ctx, contentId)
	return contentType, err
}
