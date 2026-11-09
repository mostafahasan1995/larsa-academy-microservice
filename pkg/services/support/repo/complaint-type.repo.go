package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/support/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type ComplaintTypeRepo interface {
	db.MainRepo[models.ComplaintType]
}

type complaintTypeRepo struct {
	db.MainRepoImpl[models.ComplaintType]
}

func NewComplaintTypeRepo(i *do.Injector) (ComplaintTypeRepo, error) {
	return &complaintTypeRepo{
		MainRepoImpl: db.MainRepoImpl[models.ComplaintType]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "complaintTypes",
		},
	}, nil
}
