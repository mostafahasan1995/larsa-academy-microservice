package contact

import (
	"context"
	"errors"
	"math"

	// "fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"time"

	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/sorting"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ParentStudentRelationSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ParentStudentRelationsPagination, error)
	GetOne(ctx context.Context, id string) (*models.ParentStudentRelation, error)
	Delete(ctx context.Context, id string) error
	Add(ctx context.Context, data *models.ParentStudentRelationDto) (*models.ParentStudentRelation, error)
	Update(ctx context.Context, id string, data *models.ParentStudentRelationDto) (*models.ParentStudentRelation, error)
}

type parentStudentRelationSvcs struct {
	repo        db.MainRepo[models.ParentStudentRelation]
	withtxn     *db.WithTxn
	credSvcs    CredSvcs
	sortingsvcs sorting.SortingSvcs
}

func NewParentStudentRelationSvcs(i *do.Injector) (ParentStudentRelationSvcs, error) {
	return &parentStudentRelationSvcs{
		repo:        db.NewRepo[models.ParentStudentRelation](i),
		withtxn:     do.MustInvoke[*db.WithTxn](i),
		credSvcs:    do.MustInvoke[CredSvcs](i),
		sortingsvcs: do.MustInvoke[sorting.SortingSvcs](i),
	}, nil
}

// var studentLookup = []bson.M{

// 	bson.M{
// 		"$lookup": bson.M{
// 			"from":         "academyStudents",
// 			"localField":   "studentId",
// 			"foreignField": "_id",
// 			"as":           "studentObj",
// 		},
// 	},
// 	{"$unwind": "$studentObj"},
// }

// var parentLookup = []bson.M{

// 	bson.M{
// 		"$lookup": bson.M{
// 			"from":         "academyParents",
// 			"localField":   "parentId",
// 			"foreignField": "_id",
// 			"as":           "parentObj",
// 		},
// 	},
// 	{"$unwind": "$parentObj"},
// }

func (s *parentStudentRelationSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ParentStudentRelationsPagination, error) {
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

	var result []models.ParentStudentRelation
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ParentStudentRelationsPagination{
		ParentStudentRelations: result,
		Pagination:             pagination,
	}, nil
}

func (r *parentStudentRelationSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid relation ID")
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

	_, err = r.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return errors.New("parent-student relation not found")
		}
		return err
	}

	return nil
}

func (r *parentStudentRelationSvcs) Add(ctx context.Context, data *models.ParentStudentRelationDto) (*models.ParentStudentRelation, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate that only parent and student types are allowed
	if data.FirstType != enums.PersonTypeParent && data.FirstType != enums.PersonTypeStudent {
		return nil, errors.New("firstType must be either 'parent' or 'student'")
	}
	if data.SecondType != enums.PersonTypeParent && data.SecondType != enums.PersonTypeStudent {
		return nil, errors.New("secondType must be either 'parent' or 'student'")
	}

	// Ensure one is parent and one is student
	if (data.FirstType == enums.PersonTypeParent && data.SecondType != enums.PersonTypeStudent) ||
		(data.FirstType == enums.PersonTypeStudent && data.SecondType != enums.PersonTypeParent) {
		return nil, errors.New("relation must be between one parent and one student")
	}

	// Check if the relation already exists

	relation := &models.ParentStudentRelation{
		Id:                       primitive.NewObjectID(),
		ParentStudentRelationDto: *data,
		CreatedAt:                time.Now(),
		CreatedBy:                cfg.User.Id,
		UpdatedAt:                time.Now(),
		UpdatedBy:                cfg.User.Id,
	}
	if err := r.repo.Add(ctx, relation); err != nil {
		return nil, err
	}

	return relation, nil
}
func (r *parentStudentRelationSvcs) GetOne(ctx context.Context, id string) (*models.ParentStudentRelation, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	relation, err := r.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("parent-student relation not found")
		}
		return nil, err
	}
	return relation, nil
}

func (r *parentStudentRelationSvcs) Update(ctx context.Context, id string, data *models.ParentStudentRelationDto) (*models.ParentStudentRelation, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate that only parent and student types are allowed
	if data.FirstType != enums.PersonTypeParent && data.FirstType != enums.PersonTypeStudent {
		return nil, errors.New("firstType must be either 'parent' or 'student'")
	}
	if data.SecondType != enums.PersonTypeParent && data.SecondType != enums.PersonTypeStudent {
		return nil, errors.New("secondType must be either 'parent' or 'student'")
	}

	// Ensure one is parent and one is student
	if (data.FirstType == enums.PersonTypeParent && data.SecondType != enums.PersonTypeStudent) ||
		(data.FirstType == enums.PersonTypeStudent && data.SecondType != enums.PersonTypeParent) {
		return nil, errors.New("relation must be between one parent and one student")
	}

	relation := &models.ParentStudentRelation{
		ParentStudentRelationDto: *data,
		UpdatedAt:                time.Now(),
		UpdatedBy:                cfg.User.Id,
	}
	relation.Id = _id
	filter := bson.M{"_id": _id}
	update := bson.M{"$set": relation}

	updatedRelation, err := r.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("parent-student relation not found")
		}
		return nil, err
	}
	return updatedRelation, nil
}
