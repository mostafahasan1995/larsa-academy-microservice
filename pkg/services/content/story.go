package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
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

type StorySvcs interface {
	GetById(ctx context.Context, id string) (*models.Story, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions, requesterStudentId *primitive.ObjectID) (*models.StoryPagination, error)
	Add(ctx context.Context, data *models.StoryDto) (*models.Story, error)
	Update(ctx context.Context, id string, data *models.StoryDto) (*models.Story, error)
	Delete(ctx context.Context, id string) error
}

type storysvcs struct {
	storyRepo         db.MainRepo[models.Story]
	courseRepo        db.MainRepo[models.Course]
	studentCourseRepo db.MainRepo[models.StudentCourse]
}

func NewStorySvcs(i *do.Injector) (StorySvcs, error) {
	return &storysvcs{
		storyRepo:         db.NewRepo[models.Story](i),
		courseRepo:        db.NewRepo[models.Course](i),
		studentCourseRepo: db.NewRepo[models.StudentCourse](i),
	}, nil
}

func (s *storysvcs) GetById(ctx context.Context, id string) (*models.Story, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid story id")
	}

	story, err := s.storyRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, err
	}

	// Populate course object
	if !story.Course.IsZero() {
		course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": story.Course, "trash": false})
		if err == nil {
			story.CourseObj = course
		}
	}

	return story, nil
}

func (s *storysvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions, requesterStudentId *primitive.ObjectID) (*models.StoryPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	// Build aggregation pipeline with course lookup
	pipeline := []bson.M{
		// Match non-deleted stories first
		{"$match": bson.M{"trash": false}},

		// Lookup course information
		{"$lookup": bson.M{
			"from":         "academyCourses",
			"localField":   "course",
			"foreignField": "_id",
			"as":           "courseObj",
		}},
		{"$unwind": bson.M{
			"path":                       "$courseObj",
			"preserveNullAndEmptyArrays": true,
		}},
	}

	// When requester is a student (has enrollments), restrict visibility based on scope and enrolled courses
	if requesterStudentId != nil && !requesterStudentId.IsZero() {
		var enrollments []models.StudentCourse
		if err := s.studentCourseRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"studentId": *requesterStudentId,
				"trash":     false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		}); err == nil && len(enrollments) > 0 {
			enrolledCourseIds := make([]primitive.ObjectID, 0, len(enrollments))
			for _, e := range enrollments {
				if !e.CourseId.IsZero() {
					enrolledCourseIds = append(enrolledCourseIds, e.CourseId)
				}
			}

			// scopeFilter := bson.M{
			// 	"$or": []bson.M{
			// 		{"scope": bson.M{"$exists": false}},
			// 		{"scope": "global"},
			// 	},
			// }

			// if len(enrolledCourseIds) > 0 {
			// 	scopeFilter["$or"] = append(scopeFilter["$or"].([]bson.M), bson.M{
			// 		"$and": []bson.M{
			// 			{"scope": "course"},
			// 			{"course": bson.M{"$in": enrolledCourseIds}},
			// 		},
			// 	})
			// }

			// pipeline = append(pipeline, bson.M{"$match": scopeFilter})

		}
	}

	// Apply user filters
	pipeline = append(pipeline,
		bson.M{"$match": filter},

		// Facet for pagination
		bson.M{"$facet": bson.M{
			"metadata": []bson.M{
				{"$count": "total"},
			},
			"data": []bson.M{
				{"$sort": bson.M{"_id": -1}},
				{"$skip": skip},
				{"$limit": limit},
			},
		}},
	)

	type facetResult struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.Story `bson:"data"`
	}

	var results []facetResult
	if err := s.storyRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return &models.StoryPagination{
			Stories: []models.Story{},
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

	stories := result.Data

	totalPages := math.Ceil(float64(count) / float64(limit))
	return &models.StoryPagination{
		Stories: stories,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *storysvcs) Add(ctx context.Context, data *models.StoryDto) (*models.Story, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get creator name from context
	creatorName := ""
	if cfg.User.UserData.FirstName != "" || cfg.User.UserData.LastName != "" {
		creatorName = cfg.User.UserData.FirstName + " " + cfg.User.UserData.LastName
	}

	story := &models.Story{
		Id:            primitive.NewObjectID(),
		StoryDto:      *data,
		CreatedAt:     time.Now(),
		CreatedBy:     cfg.User.Id,
		CreatedByName: creatorName,
		UpdatedAt:     time.Now(),
		UpdatedBy:     cfg.User.Id,
	}

	if err := s.storyRepo.Add(ctx, story); err != nil {
		return nil, err
	}

	return story, nil
}

func (s *storysvcs) Update(ctx context.Context, storyId string, data *models.StoryDto) (*models.Story, error) {
	_id, err := primitive.ObjectIDFromHex(storyId)
	if err != nil {
		return nil, errors.New("invalid story id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	story := &models.Story{
		Id:        _id,
		StoryDto:  *data,
		UpdatedAt: time.Now(),
		UpdatedBy: cfg.User.Id,
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": story}

	updatedStory, err := s.storyRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedStory, nil
}

func (s *storysvcs) Delete(ctx context.Context, storyId string) error {
	_id, err := primitive.ObjectIDFromHex(storyId)
	if err != nil {
		return errors.New("invalid story id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = s.storyRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}
