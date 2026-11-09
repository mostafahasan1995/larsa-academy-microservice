package sales

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/types"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CouponSvcs interface {
	GetById(ctx context.Context, id string) (*models.Coupon, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.CouponPagination, error)
	GetAll(ctx context.Context) ([]models.Coupon, error)
	Add(ctx context.Context, data *models.Coupon) (*models.Coupon, error)
	Update(ctx context.Context, id string, data *models.Coupon) (*models.Coupon, error)
	Delete(ctx context.Context, id string) error
}

type couponSvcs struct {
	couponRepo db.MainRepo[models.Coupon]
}

func NewCouponSvcs(i *do.Injector) (CouponSvcs, error) {
	return &couponSvcs{
		couponRepo: db.NewRepo[models.Coupon](i),
	}, nil
}

func (s *couponSvcs) GetById(ctx context.Context, id string) (*models.Coupon, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid coupon id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{
			"$lookup": bson.M{
				"from":         "academyCourses",
				"localField":   "coursesIds",
				"foreignField": "_id",
				"as":           "coursesData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "coachingProgramsIds",
				"foreignField": "_id",
				"as":           "coachingProgramsData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyBundles",
				"localField":   "bundlesIds",
				"foreignField": "_id",
				"as":           "bundlesData",
			},
		},
	}

	var result []models.Coupon
	err = s.couponRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("coupon not found")
	}

	return &result[0], nil
}

func (s *couponSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.CouponPagination, error) {
	filter := bson.M{}
	if query != nil {
		if err := query.CheckValid(); err != nil {
			return nil, err
		}
		var err error
		filter, err = query.ConvertToMongo()
		if err != nil {
			return nil, err
		}
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
		{
			"$lookup": bson.M{
				"from":         "academyCourses",
				"localField":   "coursesIds",
				"foreignField": "_id",
				"as":           "coursesData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "coachingProgramsIds",
				"foreignField": "_id",
				"as":           "coachingProgramsData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyBundles",
				"localField":   "bundlesIds",
				"foreignField": "_id",
				"as":           "bundlesData",
			},
		},
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"_id": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.Coupon `bson:"data"`
	}

	err := s.couponRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	coupons := []models.Coupon{}
	if len(result) > 0 {
		coupons = result[0].Data
	}
	return &models.CouponPagination{
		Coupons: coupons,
		Pagination: types.Pagination{
			TotalPages: math.Ceil(float64(total) / float64(limit)),
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *couponSvcs) GetAll(ctx context.Context) ([]models.Coupon, error) {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{
			"$lookup": bson.M{
				"from":         "academyCourses",
				"localField":   "coursesIds",
				"foreignField": "_id",
				"as":           "coursesData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyCoachings",
				"localField":   "coachingProgramsIds",
				"foreignField": "_id",
				"as":           "coachingProgramsData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyBundles",
				"localField":   "bundlesIds",
				"foreignField": "_id",
				"as":           "bundlesData",
			},
		},
	}
	var result []models.Coupon
	err := s.couponRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	return result, err
}
func (s *couponSvcs) Add(ctx context.Context, data *models.Coupon) (*models.Coupon, error) {
	// Populate Products with all IDs
	data.Products = []primitive.ObjectID{}
	data.Products = append(data.Products, data.CoursesIds...)
	data.Products = append(data.Products, data.CoachingProgramsIds...)
	data.Products = append(data.Products, data.BundlesIds...)

	data.ID = primitive.NewObjectID()
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	if err := s.couponRepo.Add(ctx, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *couponSvcs) Update(ctx context.Context, id string, data *models.Coupon) (*models.Coupon, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid coupon id")
	}

	// Populate Products with all IDs
	data.Products = []primitive.ObjectID{}
	data.Products = append(data.Products, data.CoursesIds...)
	data.Products = append(data.Products, data.CoachingProgramsIds...)
	data.Products = append(data.Products, data.BundlesIds...)

	data.UpdatedAt = time.Now()
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": data}
	return s.couponRepo.Patch(ctx, filter, update)
}

func (s *couponSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid coupon id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.couponRepo.Patch(ctx, filter, update)
	return err
}
