package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/support/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type TeacherBenefitRepo interface {
	db.MainRepo[models.TeacherBenefit]
}

type teacherBenefitRepo struct {
	db.MainRepoImpl[models.TeacherBenefit]
}

func NewTeacherBenefitRepo(i *do.Injector) (TeacherBenefitRepo, error) {
	return &teacherBenefitRepo{
		MainRepoImpl: db.MainRepoImpl[models.TeacherBenefit]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "teacherBenefits",
		},
	}, nil
}
