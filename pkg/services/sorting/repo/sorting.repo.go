package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/sorting/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type SortingRepo interface {
	db.MainRepo[models.Counter]
}

type sortingrepo struct {
	db.MainRepoImpl[models.Counter]
}

func NewSortingRepo(i *do.Injector) (SortingRepo, error) {
	return &sortingrepo{
		MainRepoImpl: db.MainRepoImpl[models.Counter]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "academySorting",
		},
	}, nil
}
