package points

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/types"
	"math"
	"time"

	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RewardSvcs interface {
	GetById(ctx context.Context, id string) (*models.Reward, error)
	Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.RewardPagination, error)
	GetAll(ctx context.Context, filter bson.M) ([]models.Reward, error)
	Add(ctx context.Context, data *models.Reward) (*models.Reward, error)
	Update(ctx context.Context, id string, data *models.Reward) (*models.Reward, error)
	Delete(ctx context.Context, id string) error
}

type rewardsvcs struct {
	rewardRepo db.MainRepo[models.Reward]
}

func NewRewardSvcs(i *do.Injector) (RewardSvcs, error) {
	return &rewardsvcs{
		rewardRepo: db.NewRepo[models.Reward](i),
	}, nil
}

func (s *rewardsvcs) GetById(ctx context.Context, id string) (*models.Reward, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid reward id")
	}
	return s.rewardRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
}

func (s *rewardsvcs) Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.RewardPagination, error) {
	filter := bson.M{"trash": false}

	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}
		queryFilter, err := conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		for k, v := range queryFilter {
			filter[k] = v
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"_id": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.Reward `bson:"data"`
	}

	err := s.rewardRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	rewards := []models.Reward{}
	if len(result) > 0 {
		rewards = result[0].Data
	}
	pages := 0.0
	if total > 0 && limit > 0 {
		pages = math.Ceil(float64(total) / float64(limit))
	}
	return &models.RewardPagination{
		Rewards: rewards,
		Pagination: types.Pagination{
			TotalPages: pages,
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *rewardsvcs) GetAll(ctx context.Context, filter bson.M) ([]models.Reward, error) {
	if filter == nil {
		filter = bson.M{}
	}
	filter["trash"] = false

	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
	}

	var result []models.Reward
	err := s.rewardRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *rewardsvcs) Add(ctx context.Context, data *models.Reward) (*models.Reward, error) {
	data.ID = primitive.NewObjectID()
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	data.CreatedBy = ctx.Value(util.ReqUser).(*types.User).Id
	data.UpdatedBy = ctx.Value(util.ReqUser).(*types.User).Id
	if err := s.rewardRepo.Add(ctx, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *rewardsvcs) Update(ctx context.Context, id string, data *models.Reward) (*models.Reward, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid reward id")
	}
	data.UpdatedAt = time.Now()
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": data}
	return s.rewardRepo.Patch(ctx, filter, update)
}

func (s *rewardsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid reward id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.rewardRepo.Patch(ctx, filter, update)
	return err
}
