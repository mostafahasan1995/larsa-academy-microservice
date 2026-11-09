package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/support/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type BenefitTypeRepo interface {
	db.MainRepo[models.BenefitType]
}

type benefitTypeRepo struct {
	db.MainRepoImpl[models.BenefitType]
}

func NewBenefitTypeRepo(i *do.Injector) (BenefitTypeRepo, error) {
	return &benefitTypeRepo{
		MainRepoImpl: db.MainRepoImpl[models.BenefitType]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "benefitTypes",
		},
	}, nil
}
