package sorting

import (
	"context"
	"larsa-academy-microservice/pkg/services/sorting/repo"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type SortingSvcs interface {
	GetAndUpdateSourceSeq(ctx context.Context, name string) (int64, error)
}

type sortingsvcs struct {
	repo repo.SortingRepo
}

func NewSortingSvcs(i *do.Injector) (SortingSvcs, error) {
	return &sortingsvcs{
		repo: do.MustInvoke[repo.SortingRepo](i),
	}, nil
}

func (s *sortingsvcs) GetAndUpdateSourceSeq(ctx context.Context, name string) (int64, error) {
	filter := bson.M{"name": name}
	update := bson.M{"$inc": bson.M{"seq": 1}}

	upsert := true
	after := options.After
	opts := &options.FindOneAndUpdateOptions{
		ReturnDocument: &after,
		Upsert:         &upsert,
	}

	result, err := s.repo.Patch(ctx, filter, update, opts)
	if err != nil {
		return 0, err
	}

	return result.Seq, nil

}
