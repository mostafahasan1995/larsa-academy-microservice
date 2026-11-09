package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/support/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type BenefitRepo interface {
	db.MainRepo[models.Benefit]
}

type benefitRepo struct {
	db.MainRepoImpl[models.Benefit]
}

func NewBenefitRepo(i *do.Injector) (BenefitRepo, error) {
	return &benefitRepo{
		MainRepoImpl: db.MainRepoImpl[models.Benefit]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "benefits",
		},
	}, nil
}
