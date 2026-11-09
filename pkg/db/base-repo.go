package db

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/util"
	"log"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type CollectionNamer interface {
	CollName() string
}
type DbNamer interface {
	DbName() string
}
type Indexer interface {
	Index()
}
type PreSaver interface {
	PreSave()
}

type MainRepo[T any] interface {
	GetByFilter(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) (*T, error)
	Aggregate(ctx context.Context, pipeline any, callback func(cur *mongo.Cursor) error) error
	Add(ctx context.Context, data *T, opts ...*options.InsertOneOptions) error
	AddMany(ctx context.Context, data []any, opts ...*options.InsertManyOptions) error
	Patch(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*T, error)
	BulkWrite(ctx context.Context, writeOps []mongo.WriteModel) (*mongo.BulkWriteResult, error)
	Count(ctx context.Context, filter any, opts ...*options.CountOptions) (int64, error)
	DeleteMain(ctx context.Context, filter bson.M) error
	DeleteMany(ctx context.Context, filter bson.M) error
	//

}

type MainRepoImpl[T any] struct {
	Db       *mongo.Client
	CollName string
	DbName   string
}

func (m *MainRepoImpl[T]) getDbName(ctx context.Context) (string, error) {
	var dbName string
	if m.DbName != "" {
		dbName = m.DbName
	} else {
		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return "", err
		}
		dbName = cfg.Db
	}
	return dbName, nil
}

func (m *MainRepoImpl[T]) DeleteMain(ctx context.Context, filter bson.M) error {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return err
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	_, err = coll.DeleteOne(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (m *MainRepoImpl[T]) DeleteMany(ctx context.Context, filter bson.M) error {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return err
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	_, err = coll.DeleteMany(ctx, filter)
	if err != nil {
		return err
	}
	return nil
}

func (m *MainRepoImpl[T]) Add(ctx context.Context, data *T, opts ...*options.InsertOneOptions) error {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return err
	}

	// Automatically call PreSave if the model implements PreSaver interface
	if preSaver, ok := any(data).(PreSaver); ok {
		preSaver.PreSave()
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	if _, err := coll.InsertOne(ctx, data, opts...); err != nil {
		return err
	}
	return nil
}

func (m *MainRepoImpl[T]) AddMany(ctx context.Context, data []any, opts ...*options.InsertManyOptions) error {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return err
	}
	coll := m.Db.Database(dbName).Collection(m.CollName)

	if _, err := coll.InsertMany(ctx, data, opts...); err != nil {
		return err
	}
	return nil
}

func (m *MainRepoImpl[T]) GetByFilter(ctx context.Context, filter bson.M, ops ...*options.FindOneOptions) (*T, error) {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return nil, errors.New("error getting db name")
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	var result T
	if err := coll.FindOne(ctx, filter, ops...).Decode(&result); err != nil {
		return nil, err
	}

	return &result, nil
}

func (m *MainRepoImpl[T]) Patch(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*T, error) {

	dbName, err := m.getDbName(ctx)
	if err != nil {
		return nil, errors.New("error getting db name")
	}

	// Automatically call PreSave if there's a $set operation with the model
	if setData, ok := update["$set"]; ok {
		if preSaver, ok := setData.(PreSaver); ok {
			preSaver.PreSave()
		}
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	upsert := false
	after := options.After

	opts := []*options.FindOneAndUpdateOptions{
		{
			Upsert:         &upsert,
			ReturnDocument: &after,
		},
	}

	opts = append(opts, ops...)

	res := coll.FindOneAndUpdate(ctx, filter, update, opts...)

	var result T
	if err := res.Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (m *MainRepoImpl[T]) Aggregate(ctx context.Context, pipeline any, callback func(cur *mongo.Cursor) error) error {

	dbName, err := m.getDbName(ctx)
	if err != nil {
		return errors.New("error getting db name")
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	cur, err := coll.Aggregate(ctx, pipeline)
	if err != nil {
		return err
	}

	if err := callback(cur); err != nil {
		return err
	}

	return nil
}

func (m *MainRepoImpl[T]) BulkWrite(ctx context.Context, writeOps []mongo.WriteModel) (*mongo.BulkWriteResult, error) {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return nil, errors.New("error getting db name")
	}
	coll := m.Db.Database(dbName).Collection(m.CollName)

	opts := options.BulkWrite().SetOrdered(true)
	result, errIns := coll.BulkWrite(ctx, writeOps, opts)

	if errIns != nil {
		return nil, errIns
	}

	return result, nil
}

func (m *MainRepoImpl[T]) Count(ctx context.Context, filter any, opts ...*options.CountOptions) (int64, error) {
	dbName, err := m.getDbName(ctx)
	if err != nil {
		return 0, errors.New("error getting db name")
	}

	coll := m.Db.Database(dbName).Collection(m.CollName)

	switch f := filter.(type) {
	case bson.M:
		count, err := coll.CountDocuments(ctx, f, opts...)
		if err != nil {
			return 0, err
		}
		return count, nil
	case []bson.M:
		pipeline := append(f, bson.M{"$count": "count"})
		var result []struct {
			Count int64 `bson:"count"`
		}
		err := m.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &result)
		})
		if err != nil {
			return 0, err
		}
		if len(result) == 0 {
			return 0, nil
		}
		return result[0].Count, nil
	default:
		return 0, fmt.Errorf("unsupported filter type: %T", filter)
	}
}

// NewRepo is a function that creates a new repository for a given model, it will check if the model implements the CollectionNamer and DbNamer interfaces
func NewRepo[T any](i *do.Injector) *MainRepoImpl[T] {
	var model T
	var collName, dbName string

	if namer, ok := any(&model).(CollectionNamer); ok {
		collName = namer.CollName()
	} else {
		log.Fatalf("model %T does not implement CollectionNamer, missing CollName method", model)
	}

	if dbNamer, ok := any(&model).(DbNamer); ok {
		dbName = dbNamer.DbName()
	}

	return &MainRepoImpl[T]{
		Db:       do.MustInvoke[*mongo.Client](i),
		CollName: collName,
		DbName:   dbName,
	}
}
