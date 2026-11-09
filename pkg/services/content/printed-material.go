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

type PrintedMaterialSvcs interface {
	GetById(ctx context.Context, printedMaterialId string) (*models.EnrichedPrintedMaterial, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.PrintedMaterialPagination, error)
	Add(ctx context.Context, data *models.PrintedMaterialDto) (*models.PrintedMaterial, error)
	Update(ctx context.Context, printedMaterialId string, data *models.PrintedMaterialDto) (*models.PrintedMaterial, error)
	Delete(ctx context.Context, printedMaterialId string) error
}

type printedMaterialsvcs struct {
	printedMaterialrepo db.MainRepo[models.PrintedMaterial]
	subjectrepo         db.MainRepo[models.Subject]
}

func NewPrintedMaterialSvcs(i *do.Injector) (PrintedMaterialSvcs, error) {
	return &printedMaterialsvcs{
		printedMaterialrepo: db.NewRepo[models.PrintedMaterial](i),
		subjectrepo:         db.NewRepo[models.Subject](i),
	}, nil
}

func (s *printedMaterialsvcs) GetById(ctx context.Context, printedMaterialId string) (*models.EnrichedPrintedMaterial, error) {
	_id, err := primitive.ObjectIDFromHex(printedMaterialId)
	if err != nil {
		return nil, errors.New("invalid printed material id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{
			"$lookup": bson.M{
				"from":         "academySubjects",
				"localField":   "subject",
				"foreignField": "_id",
				"as":           "subjectData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "schoolId",
				"foreignField": "_id",
				"as":           "schoolData",
			},
		},
		{
			"$addFields": bson.M{
				"subjectObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$subjectData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$subjectData", 0}},
						nil,
					},
				},
				"schoolObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
						nil,
					},
				},
			},
		},
		{
			"$project": bson.M{
				"subjectData": 0,
				"schoolData":  0,
			},
		},
	}

	var results []models.EnrichedPrintedMaterial
	err = s.printedMaterialrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
		return cursor.All(ctx, &results)
	})
	if err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, errors.New("printed material not found")
	}

	return &results[0], nil
}

func (s *printedMaterialsvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.PrintedMaterialPagination, error) {
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

	count, err := s.printedMaterialrepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	pipeline = append(pipeline, []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academySubjects",
				"localField":   "subject",
				"foreignField": "_id",
				"as":           "subjectData",
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "schoolId",
				"foreignField": "_id",
				"as":           "schoolData",
			},
		},
		{
			"$addFields": bson.M{
				"subjectObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$subjectData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$subjectData", 0}},
						nil,
					},
				},
				"schoolObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
						nil,
					},
				},
			},
		},
		{
			"$project": bson.M{
				"subjectData": 0,
				"schoolData":  0,
			},
		},
	}...)

	var result []models.EnrichedPrintedMaterial
	err = s.printedMaterialrepo.Aggregate(ctx, pipeline, func(cursor *mongo.Cursor) error {
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

	return &models.PrintedMaterialPagination{
		PrintedMaterials: result,
		Pagination:       pagination,
	}, nil
}

func (s *printedMaterialsvcs) Add(ctx context.Context, data *models.PrintedMaterialDto) (*models.PrintedMaterial, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate subject reference
	if !data.Subject.IsZero() {
		_, err := s.subjectrepo.GetByFilter(ctx, bson.M{"_id": data.Subject, "trash": false})
		if err != nil {
			return nil, errors.New("invalid subject reference")
		}
	}

	printedMaterial := &models.PrintedMaterial{
		Id:                 primitive.NewObjectID(),
		PrintedMaterialDto: *data,
		CreatedBy:          cfg.User.Id,
		UpdatedBy:          cfg.User.Id,
		CreatedAt:          time.Now(),
		UpdatedAt:          time.Now(),
		Trash:              false,
	}

	if err := s.printedMaterialrepo.Add(ctx, printedMaterial); err != nil {
		return nil, err
	}

	// Populate subject object
	if !printedMaterial.Subject.IsZero() {
		subject, err := s.subjectrepo.GetByFilter(ctx, bson.M{"_id": printedMaterial.Subject, "trash": false})
		if err == nil {
			printedMaterial.SubjectObj = subject
		}
	}

	return printedMaterial, nil
}

func (s *printedMaterialsvcs) Update(ctx context.Context, printedMaterialId string, data *models.PrintedMaterialDto) (*models.PrintedMaterial, error) {
	_id, err := primitive.ObjectIDFromHex(printedMaterialId)
	if err != nil {
		return nil, errors.New("invalid printed material id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate subject reference
	if !data.Subject.IsZero() {
		_, err := s.subjectrepo.GetByFilter(ctx, bson.M{"_id": data.Subject, "trash": false})
		if err != nil {
			return nil, errors.New("invalid subject reference")
		}
	}

	printedMaterial := &models.PrintedMaterial{
		Id:                 _id,
		PrintedMaterialDto: *data,
		UpdatedBy:          cfg.User.Id,
		UpdatedAt:          time.Now(),
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": printedMaterial}

	updatedPrintedMaterial, err := s.printedMaterialrepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	// Populate subject object
	if !updatedPrintedMaterial.Subject.IsZero() {
		subject, err := s.subjectrepo.GetByFilter(ctx, bson.M{"_id": updatedPrintedMaterial.Subject, "trash": false})
		if err == nil {
			updatedPrintedMaterial.SubjectObj = subject
		}
	}

	return updatedPrintedMaterial, nil
}

func (s *printedMaterialsvcs) Delete(ctx context.Context, printedMaterialId string) error {
	_id, err := primitive.ObjectIDFromHex(printedMaterialId)
	if err != nil {
		return errors.New("invalid printed material id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{
		"trash":     true,
		"updatedBy": cfg.User.Id,
		"updatedAt": time.Now(),
	}}

	_, err = s.printedMaterialrepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}
