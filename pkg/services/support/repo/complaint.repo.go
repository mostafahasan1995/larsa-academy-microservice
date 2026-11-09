package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/support/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type ComplaintRepo interface {
	db.MainRepo[models.Complaint]
}

type complaintrepo struct {
	db.MainRepoImpl[models.Complaint]
}

func NewComplaintRepo(i *do.Injector) (ComplaintRepo, error) {
	return &complaintrepo{
		MainRepoImpl: db.MainRepoImpl[models.Complaint]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "academyComplaints",
		},
	}, nil
}
