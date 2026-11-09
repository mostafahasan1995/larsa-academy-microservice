package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/messaging/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type MessageRepo interface {
	db.MainRepo[models.Message]
}

type messagerepo struct {
	db.MainRepoImpl[models.Message]
}

func NewMessageRepo(i *do.Injector) (MessageRepo, error) {
	return &messagerepo{
		MainRepoImpl: db.MainRepoImpl[models.Message]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "tourismMessages",
		},
	}, nil
}
