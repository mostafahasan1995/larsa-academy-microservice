package scheduler

import (
	"context"
	"larsa-academy-microservice/pkg/db"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type Lock struct {
	Id        primitive.ObjectID `json:"id,omitempty" bson:"_id,omitempty"`
	Key       string             `json:"key" bson:"key"`
	IsLocked  bool               `json:"isLocked" bson:"isLocked"`
	ExpiresAt time.Time          `json:"expiresAt,omitempty" bson:"expiresAt,omitempty"`
}

type Locker struct {
	db.MainRepoImpl[Lock]
}

func NewLocker(i *do.Injector) *Locker {
	return &Locker{
		db.MainRepoImpl[Lock]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "academyLocker",
			DbName:   "academyScheduler",
		},
	}
}

func (l *Locker) EnsureIndexes(ctx context.Context) error {

	coll := l.Db.Database(l.DbName).Collection(l.CollName)

	indexModel := mongo.IndexModel{
		Keys:    bson.D{{Key: "key", Value: 1}},
		Options: options.Index().SetUnique(true).SetName("key_unique"),
	}

	_, err := coll.Indexes().CreateOne(ctx, indexModel)
	if err != nil {
		if !mongo.IsDuplicateKeyError(err) {
			return err
		}
	}

	return nil
}

func (l *Locker) Lock(ctx context.Context, key string) (*Lock, error) {
	filter := bson.M{
		"key": key,
		"$or": []bson.M{
			{"isLocked": false},
			{"expiresAt": bson.M{"$lt": time.Now()}},
		},
	}

	expiresAt := time.Now().Add(10 * time.Minute)
	update := bson.M{
		"$set": bson.M{
			"isLocked":  true,
			"expiresAt": expiresAt,
		},
	}

	upsert := true
	after := options.After
	opts := []*options.FindOneAndUpdateOptions{
		{
			Upsert:         &upsert,
			ReturnDocument: &after,
		},
	}

	doc, err := l.Patch(ctx, filter, update, opts...)
	if err != nil {
		// if mongo.IsDuplicateKeyError(err) {
		// 	return nil, nil
		// }
		log.Info().Msg("faild to acquire the lock" + err.Error())
		return nil, err
	}

	log.Info().Msg("lock acquired successfully for" + key)

	return doc, nil
}

func (l *Locker) Unlock(ctx context.Context, id primitive.ObjectID) error {
	filter := bson.M{"_id": id}
	update := bson.M{"$set": bson.M{"isLocked": false}}

	if _, err := l.Patch(ctx, filter, update); err != nil {
		return err
	}

	return nil
}
