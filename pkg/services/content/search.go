package content

import (
	"context"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"sync"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
)

type GlobalSearchSvcs interface {
	Search(ctx context.Context, req *models.GlobalSearchRequest) (*models.GlobalSearchResponse, error)
}

type GlobalSearchSvcsImpl struct {
	injector     *do.Injector
	courseRepo   db.MainRepo[models.Course]
	bundleRepo   db.MainRepo[models.Bundle]
	coachingRepo db.MainRepo[models.Coaching]
}

func NewGlobalSearchSvcs(i *do.Injector) (GlobalSearchSvcs, error) {
	return &GlobalSearchSvcsImpl{
		injector:     i,
		courseRepo:   db.NewRepo[models.Course](i),
		bundleRepo:   db.NewRepo[models.Bundle](i),
		coachingRepo: db.NewRepo[models.Coaching](i),
	}, nil
}

func (s *GlobalSearchSvcsImpl) Search(ctx context.Context, req *models.GlobalSearchRequest) (*models.GlobalSearchResponse, error) {
	if req.Page <= 0 {
		req.Page = 1
	}
	if req.Limit <= 0 {
		req.Limit = 10
	}

	skip := int64((req.Page - 1) * req.Limit)
	limit := int64(req.Limit)

	// If no entities specified, search all
	if len(req.Entities) == 0 {
		req.Entities = []string{"courses", "bundles", "coachings"}
	}

	results := make([]models.GlobalSearchResult, 0)
	var totalCount int64 = 0
	var mu sync.Mutex

	for _, entity := range req.Entities {
		var entityFilter bson.M
		var err error
		var collName string

		// Map generic "name" to entity specific field
		mappedQuery := s.MapQueryForEntity(ctx, req.Query, entity)

		if mappedQuery != nil {
			entityFilter, err = mappedQuery.ConvertToMongo()
			if err != nil {
				return nil, err
			}
			fmt.Printf("Entity: %s, Filter: %+v\n", entity, entityFilter)
		} else {
			entityFilter = bson.M{}
		}

		// Ensure trash is false
		// entityFilter["trash"] = false
		fmt.Printf("Entity: %s, Final Filter: %+v\n", entity, entityFilter)

		var pipeline []bson.M
		pipeline = append(pipeline, bson.M{"$match": entityFilter})

		// Add school lookup
		schoolLocalField := "schoolId"
		if entity == "courses" {
			schoolLocalField = "basicInfo.schoolId"
		}

		pipeline = append(pipeline, bson.M{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   schoolLocalField,
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

		switch entity {
		case "courses":
			collName = "academyCourses"
			pipeline = append(pipeline, bson.M{"$project": bson.M{
				"id":        "$_id",
				"name":      "$basicInfo.name",
				"image":     "$basicInfo.thumbnail",
				"type":      bson.M{"$literal": "course"},
				"trash":     "$trash",
				"schoolObj": "$schoolObj",
			}})
		case "bundles":
			collName = "academyBundles"
			pipeline = append(pipeline, bson.M{"$project": bson.M{
				"id":        "$_id",
				"name":      "$name",
				"image":     "$thumbnail",
				"type":      bson.M{"$literal": "bundle"},
				"trash":     "$trash",
				"schoolObj": "$schoolObj",
			}})
		case "coachings":
			collName = "academyCoachings"
			pipeline = append(pipeline, bson.M{"$project": bson.M{
				"id":        "$_id",
				"name":      "$title",
				"image":     "$thumbnail",
				"type":      bson.M{"$literal": "coaching"},
				"trash":     "$trash",
				"schoolObj": "$schoolObj",
			}})
		}

		if collName == "" {
			continue
		}

		// Get results for this entity
		var entityResults []models.GlobalSearchResult
		var errAgg error
		switch entity {
		case "courses":
			errAgg = s.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &entityResults)
			})
		case "bundles":
			errAgg = s.bundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &entityResults)
			})
		case "coachings":
			errAgg = s.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &entityResults)
			})
		}

		if errAgg == nil {
			mu.Lock()
			results = append(results, entityResults...)
			totalCount += int64(len(entityResults))
			mu.Unlock()
		}
	}

	if len(results) == 0 {
		return &models.GlobalSearchResponse{
			Results: []models.GlobalSearchResult{},
			Pagination: types.Pagination{
				TotalCount: 0,
				TotalPages: 0,
				PerPage:    limit,
			},
		}, nil
	}

	// Manual pagination for now as we merged in-memory
	start := skip
	if start > int64(len(results)) {
		start = int64(len(results))
	}
	end := skip + limit
	if end > int64(len(results)) {
		end = int64(len(results))
	}

	paginatedResults := results[start:end]

	return &models.GlobalSearchResponse{
		Results: paginatedResults,
		Pagination: types.Pagination{
			TotalCount: totalCount,
			TotalPages: math.Ceil(float64(totalCount) / float64(limit)),
			PerPage:    limit,
		},
	}, nil
}

func (s *GlobalSearchSvcsImpl) MapQueryForEntity(ctx context.Context, q *query.Conditions, entity string) *query.Conditions {
	if q == nil {
		return nil
	}

	lang := "en"
	if cfg, err := util.GetReqAppCfg(ctx); err == nil && cfg.Lang != "" {
		lang = cfg.Lang
	}
	fmt.Printf("Search Lang: %s\n", lang)

	newCols := make([]query.Column, len(q.Columns))
	for i, col := range q.Columns {
		newCol := col
		if col.Name == "name" {
			switch entity {
			case "courses":
				newCol.Name = "basicInfo.name." + lang
			case "bundles":
				newCol.Name = "name." + lang
			case "coachings":
				newCol.Name = "title." + lang
			}
		}
		newCols[i] = newCol
	}

	return &query.Conditions{Columns: newCols}
}
