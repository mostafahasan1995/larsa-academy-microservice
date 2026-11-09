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

type ComplaintTypeSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ComplaintTypePagination, error)
	GetAll(ctx context.Context) ([]models.ComplaintTypeWithOwner, error)
	GetOne(ctx context.Context, id string) (*models.ComplaintTypeWithOwner, error)
	Add(ctx context.Context, data *models.ComplaintTypeDto) (*models.ComplaintTypeWithOwner, error)
	Update(ctx context.Context, id string, data *models.ComplaintTypeDto) (*models.ComplaintTypeWithOwner, error)
	Delete(ctx context.Context, id string) error
}

type complaintTypeSvcs struct {
	repo    repo.ComplaintTypeRepo
	withtxn *db.WithTxn
}

func NewComplaintTypeSvcs(i *do.Injector) (ComplaintTypeSvcs, error) {
	return &complaintTypeSvcs{
		repo:    do.MustInvoke[repo.ComplaintTypeRepo](i),
		withtxn: do.MustInvoke[*db.WithTxn](i),
	}, nil
}

var ownerLookup = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "users",
			"localField":   "owner",
			"foreignField": "_id",
			"as":           "ownerUser",
		},
	},
	{
		"$addFields": bson.M{
			"ownerObj": bson.M{
				"$cond": bson.M{
					"if": bson.M{"$eq": []interface{}{"$owner", primitive.NilObjectID}},
					"then": nil,
					"else": bson.M{
						"$let": bson.M{
							"vars": bson.M{
								"user": bson.M{"$arrayElemAt": []interface{}{"$ownerUser", 0}},
							},
							"in": bson.M{
								"_id":       "$$user._id",
								"firstName": bson.M{"$ifNull": []interface{}{"$$user.firstName", ""}},
								"lastName":  bson.M{"$ifNull": []interface{}{"$$user.lastName", ""}},
							},
						},
					},
				},
			},
		},
	},
	{
		"$project": bson.M{
			"ownerUser": 0,
		},
	},
}

func (s *complaintTypeSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ComplaintTypePagination, error) {
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

	pipeline = append(pipeline, ownerLookup...)
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.ComplaintTypeWithOwner
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.ComplaintTypePagination{
		ComplaintTypes: result,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *complaintTypeSvcs) GetAll(ctx context.Context) ([]models.ComplaintTypeWithOwner, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
	}

	pipeline = append(pipeline, ownerLookup...)
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	var result []models.ComplaintTypeWithOwner
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *complaintTypeSvcs) GetOne(ctx context.Context, id string) (*models.ComplaintTypeWithOwner, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}

	pipeline = append(pipeline, ownerLookup...)

	var result []models.ComplaintTypeWithOwner
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("complaint type not found")
	}

	return &result[0], nil
}

func (s *complaintTypeSvcs) Add(ctx context.Context, data *models.ComplaintTypeDto) (*models.ComplaintTypeWithOwner, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Check if code already exists
	existing, _ := s.repo.GetByFilter(ctx, bson.M{"code": data.Code, "trash": false})
	if existing != nil {
		return nil, errors.New("complaint type with this code already exists")
	}

	complaintType := &models.ComplaintType{
		Id:               primitive.NewObjectID(),
		ComplaintTypeDto: *data,
		CreatedAt:        time.Now(),
		CreatedBy:        cfg.User.Id,
		UpdatedAt:        time.Now(),
		UpdatedBy:        cfg.User.Id,
	}

	if err := s.repo.Add(ctx, complaintType); err != nil {
		return nil, err
	}

	// Fetch the complete record with owner populated
	return s.GetOne(ctx, complaintType.Id.Hex())
}

func (s *complaintTypeSvcs) Update(ctx context.Context, id string, data *models.ComplaintTypeDto) (*models.ComplaintTypeWithOwner, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the existing complaint type
	oldComplaintType, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Check if code already exists in another record
	if oldComplaintType.Code != data.Code {
		existing, _ := s.repo.GetByFilter(ctx, bson.M{"code": data.Code, "trash": false})
		if existing != nil {
			return nil, errors.New("complaint type with this code already exists")
		}
	}

	// Create a copy and update fields
	updatedComplaintType := *oldComplaintType
	updatedComplaintType.ComplaintTypeDto = *data
	updatedComplaintType.UpdatedAt = time.Now()
	updatedComplaintType.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedComplaintType}

	_, err = s.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("complaint type not found")
		}
		return nil, err
	}

	// Fetch the complete record with owner populated
	return s.GetOne(ctx, id)
}

func (s *complaintTypeSvcs) Delete(ctx context.Context, id string) error {
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
