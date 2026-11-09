package content

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RenewalSvcs interface {
	GetAll(ctx context.Context, query *query.Conditions) ([]models.Renewal, error)
	Add(ctx context.Context, data *models.RenewalDto) (*models.Renewal, error)
	UpdateStatus(ctx context.Context, id string, status enums.RenewalStatus) (*models.Renewal, error)
	Delete(ctx context.Context, id string) error
}

type renewalsvcs struct {
	repo db.MainRepo[models.Renewal]
}

func NewRenewalSvcs(i *do.Injector) (RenewalSvcs, error) {
	return &renewalsvcs{
		repo: db.NewRepo[models.Renewal](i),
	}, nil
}

func (r *renewalsvcs) GetAll(ctx context.Context, query *query.Conditions) ([]models.Renewal, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	var result []models.Renewal
	err = r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil

}

func (r *renewalsvcs) Add(ctx context.Context, data *models.RenewalDto) (*models.Renewal, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	renewal := &models.Renewal{
		RenewalDto: *data,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		CreatedBy:  cfg.User.Id,
		UpdatedBy:  cfg.User.Id,
		Trash:      false,
	}

	if err := r.repo.Add(ctx, renewal); err != nil {
		return nil, err
	}

	return renewal, nil
}

func (r *renewalsvcs) UpdateStatus(ctx context.Context, id string, status enums.RenewalStatus) (*models.Renewal, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid renewal id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	renewal, err := r.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return nil, errors.New("renewal not found")
	}

	renewal.Status = status
	renewal.UpdatedAt = time.Now()
	renewal.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": renewal}
	return r.repo.Patch(ctx, filter, update)
}

func (r *renewalsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid renewal id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = r.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}
