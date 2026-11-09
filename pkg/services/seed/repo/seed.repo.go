package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/seed/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type SeedRepo interface {
	db.MainRepo[models.Seed]
}

type seedrepo struct {
	db.MainRepoImpl[models.Seed]
}

func NewSeedRepo(i *do.Injector) (SeedRepo, error) {
	return &seedrepo{
		MainRepoImpl: db.MainRepoImpl[models.Seed]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "academySeeds",
		},
	}, nil
}
