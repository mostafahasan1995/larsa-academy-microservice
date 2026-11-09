package repo

import (
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/support/models"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/mongo"
)

type FeedbackRepo interface {
	db.MainRepo[models.Feedback]
}

type feedbackrepo struct {
	db.MainRepoImpl[models.Feedback]
}

func NewFeedbackRepo(i *do.Injector) (FeedbackRepo, error) {
	return &feedbackrepo{
		MainRepoImpl: db.MainRepoImpl[models.Feedback]{
			Db:       do.MustInvoke[*mongo.Client](i),
			CollName: "academyFeedbacks",
		},
	}, nil
}
