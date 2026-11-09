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

type BenefitSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.BenefitPagination, error)
	GetAll(ctx context.Context) ([]models.Benefit, error)
	GetOne(ctx context.Context, id string) (*models.Benefit, error)
	Add(ctx context.Context, data *models.BenefitDto) (*models.Benefit, error)
	Update(ctx context.Context, id string, data *models.BenefitDto) (*models.Benefit, error)
	Delete(ctx context.Context, id string) error
}

type benefitSvcs struct {
	repo            repo.BenefitRepo
	benefitTypeRepo repo.BenefitTypeRepo
	withtxn         *db.WithTxn
}

func NewBenefitSvcs(i *do.Injector) (BenefitSvcs, error) {
	return &benefitSvcs{
		repo:            do.MustInvoke[repo.BenefitRepo](i),
		benefitTypeRepo: do.MustInvoke[repo.BenefitTypeRepo](i),
		withtxn:         do.MustInvoke[*db.WithTxn](i),
	}, nil
}

func (s *benefitSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.BenefitPagination, error) {
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

	var result []models.Benefit
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	// Populate BenefitTypeObj for each benefit
	for i := range result {
		benefit := &result[i]
		if !benefit.BenefitTypeId.IsZero() {
			benefitType, err := s.benefitTypeRepo.GetByFilter(ctx, bson.M{"_id": benefit.BenefitTypeId, "trash": false})
			if err == nil {
				benefit.BenefitTypeObj = benefitType
			}
		}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.BenefitPagination{
		Benefits: result,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *benefitSvcs) GetAll(ctx context.Context) ([]models.Benefit, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$sort": bson.M{"_id": -1}},
	}

	var result []models.Benefit
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	// Populate BenefitTypeObj for each benefit
	for i := range result {
		benefit := &result[i]
		if !benefit.BenefitTypeId.IsZero() {
			benefitType, err := s.benefitTypeRepo.GetByFilter(ctx, bson.M{"_id": benefit.BenefitTypeId, "trash": false})
			if err == nil {
				benefit.BenefitTypeObj = benefitType
			}
		}
	}

	return result, nil
}

func (s *benefitSvcs) GetOne(ctx context.Context, id string) (*models.Benefit, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	benefit, err := s.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("benefit not found")
		}
		return nil, err
	}

	// Populate BenefitTypeObj
	if !benefit.BenefitTypeId.IsZero() {
		benefitType, err := s.benefitTypeRepo.GetByFilter(ctx, bson.M{"_id": benefit.BenefitTypeId, "trash": false})
		if err == nil {
			benefit.BenefitTypeObj = benefitType
		}
	}

	return benefit, nil
}

func (s *benefitSvcs) Add(ctx context.Context, data *models.BenefitDto) (*models.Benefit, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate that the benefit type exists
	if !data.BenefitTypeId.IsZero() {
		_, err := s.benefitTypeRepo.GetByFilter(ctx, bson.M{"_id": data.BenefitTypeId, "trash": false})
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("benefit type not found")
			}
			return nil, err
		}
	}

	benefit := &models.Benefit{
		Id:         primitive.NewObjectID(),
		BenefitDto: *data,
		CreatedAt:  time.Now(),
		CreatedBy:  cfg.User.Id,
		UpdatedAt:  time.Now(),
		UpdatedBy:  cfg.User.Id,
	}

	if err := s.repo.Add(ctx, benefit); err != nil {
		return nil, err
	}

	return benefit, nil
}

func (s *benefitSvcs) Update(ctx context.Context, id string, data *models.BenefitDto) (*models.Benefit, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate that the benefit type exists
	if !data.BenefitTypeId.IsZero() {
		_, err := s.benefitTypeRepo.GetByFilter(ctx, bson.M{"_id": data.BenefitTypeId, "trash": false})
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("benefit type not found")
			}
			return nil, err
		}
	}

	// Get the existing benefit
	oldBenefit, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create a copy and update fields
	updatedBenefit := *oldBenefit
	updatedBenefit.BenefitDto = *data
	updatedBenefit.UpdatedAt = time.Now()
	updatedBenefit.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedBenefit}

	result, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("benefit not found")
		}
		return nil, err
	}

	return result, nil
}

func (s *benefitSvcs) Delete(ctx context.Context, id string) error {
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
