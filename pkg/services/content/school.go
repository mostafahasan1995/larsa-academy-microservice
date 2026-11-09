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

type SchoolSvcs interface {
	GetById(ctx context.Context, id string) (*models.School, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SchoolPagination, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]models.School, error)
	Add(ctx context.Context, data *models.SchoolDto) (*models.School, error)
	Update(ctx context.Context, id string, data *models.SchoolDto) (*models.School, error)
	Delete(ctx context.Context, id string) error
}

type schoolsvcs struct {
	repo db.MainRepo[models.School]
}

func NewSchoolSvcs(i *do.Injector) (SchoolSvcs, error) {
	return &schoolsvcs{
		repo: db.NewRepo[models.School](i),
	}, nil
}

func (s *schoolsvcs) GetById(ctx context.Context, id string) (*models.School, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid school id")
	}

	return s.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
}

func (s *schoolsvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SchoolPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := s.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var result []models.School
	err = s.repo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.SchoolPagination{
		Schools:    result,
		Pagination: pagination,
	}, nil
}

func (s *schoolsvcs) GetAll(ctx context.Context, query *query.Conditions) ([]models.School, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("invalid query")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}
	if len(filter) > 0 {
		pipeline = append(pipeline, bson.M{"$match": filter})
	}
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	var result []models.School
	err = s.repo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &result)
	})
	if err != nil {
		return nil, errors.New("failed to fetch schools: " + err.Error())
	}

	return result, nil
}

func (s *schoolsvcs) Add(ctx context.Context, data *models.SchoolDto) (*models.School, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	school := &models.School{
		Id:        primitive.NewObjectID(),
		Name:      data.Name,
		Country:   data.Country,
		CreatedBy: cfg.User.Id,
		UpdatedBy: cfg.User.Id,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if err := s.repo.Add(ctx, school); err != nil {
		return nil, err
	}
	return school, nil
}

func (s *schoolsvcs) Update(ctx context.Context, id string, data *models.SchoolDto) (*models.School, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid school id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	school := &models.School{
		Id:        _id,
		Name:      data.Name,
		Country:   data.Country,
		UpdatedBy: cfg.User.Id,
		UpdatedAt: time.Now(),
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": school}

	updatedSchool, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	return updatedSchool, nil
}

func (s *schoolsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid school id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedBy": cfg.User.Id, "updatedAt": time.Now()}}

	_, err = s.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}
