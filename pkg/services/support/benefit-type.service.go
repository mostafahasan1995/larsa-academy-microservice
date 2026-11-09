package support

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/support/models"
	"larsa-academy-microservice/pkg/services/support/repo"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type BenefitTypeSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.BenefitTypePagination, error)
	GetAll(ctx context.Context) ([]models.BenefitType, error)
	GetOne(ctx context.Context, id string) (*models.BenefitType, error)
	Add(ctx context.Context, data *models.BenefitTypeDto) (*models.BenefitType, error)
	Update(ctx context.Context, id string, data *models.BenefitTypeDto) (*models.BenefitType, error)
	Delete(ctx context.Context, id string) error
}

type benefitTypeSvcs struct {
	repo    repo.BenefitTypeRepo
	withtxn *db.WithTxn
}

func NewBenefitTypeSvcs(i *do.Injector) (BenefitTypeSvcs, error) {
	return &benefitTypeSvcs{
		repo:    do.MustInvoke[repo.BenefitTypeRepo](i),
		withtxn: do.MustInvoke[*db.WithTxn](i),
	}, nil
}

func (s *benefitTypeSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.BenefitTypePagination, error) {
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

	count, err := s.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.BenefitType
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.BenefitTypePagination{
		BenefitTypes: result,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *benefitTypeSvcs) GetAll(ctx context.Context) ([]models.BenefitType, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$sort": bson.M{"_id": -1}},
	}

	var result []models.BenefitType
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *benefitTypeSvcs) GetOne(ctx context.Context, id string) (*models.BenefitType, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	benefitType, err := s.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("benefit type not found")
		}
		return nil, err
	}

	return benefitType, nil
}

func (s *benefitTypeSvcs) Add(ctx context.Context, data *models.BenefitTypeDto) (*models.BenefitType, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	benefitType := &models.BenefitType{
		Id:             primitive.NewObjectID(),
		BenefitTypeDto: *data,
		CreatedAt:      time.Now(),
		CreatedBy:      cfg.User.Id,
		UpdatedAt:      time.Now(),
		UpdatedBy:      cfg.User.Id,
	}

	if err := s.repo.Add(ctx, benefitType); err != nil {
		return nil, err
	}

	return benefitType, nil
}

func (s *benefitTypeSvcs) Update(ctx context.Context, id string, data *models.BenefitTypeDto) (*models.BenefitType, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the existing benefit type
	oldBenefitType, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create a copy and update fields
	updatedBenefitType := *oldBenefitType
	updatedBenefitType.BenefitTypeDto = *data
	updatedBenefitType.UpdatedAt = time.Now()
	updatedBenefitType.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedBenefitType}

	result, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("benefit type not found")
		}
		return nil, err
	}

	return result, nil
}

func (s *benefitTypeSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"trash":     true,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	_, err = s.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}
