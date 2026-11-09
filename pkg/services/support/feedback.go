package support

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"larsa-academy-microservice/pkg/services/sorting"
	"larsa-academy-microservice/pkg/services/support/models"
	"larsa-academy-microservice/pkg/services/support/repo"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type FeedbackSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.FeedbackPagination, error)
	GetOne(ctx context.Context, id string) (*models.Feedback, error)
	Add(ctx context.Context, data *models.FeedbackDto) (*models.Feedback, error)
	Update(ctx context.Context, id string, data *models.FeedbackDto) (*models.Feedback, error)
	Delete(ctx context.Context, id string) error
	GetStats(ctx context.Context) (*models.FeedbackStats, error)
}

type feedbackSvcs struct {
	repo        repo.FeedbackRepo
	withtxn     *db.WithTxn
	sortingsvcs sorting.SortingSvcs
	studentRepo db.MainRepo[contactModels.Student]
	teacherRepo db.MainRepo[contactModels.Teacher]
}

func NewFeedbackSvcs(i *do.Injector) (FeedbackSvcs, error) {
	return &feedbackSvcs{
		repo:        do.MustInvoke[repo.FeedbackRepo](i),
		withtxn:     do.MustInvoke[*db.WithTxn](i),
		sortingsvcs: do.MustInvoke[sorting.SortingSvcs](i),
		studentRepo: db.NewRepo[contactModels.Student](i),
		teacherRepo: db.NewRepo[contactModels.Teacher](i),
	}, nil
}

func (r *feedbackSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.FeedbackPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.Feedback
	errAg := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	// Populate StudentObj and TeacherObj for each feedback
	for i := range result {
		feedback := &result[i]

		// Populate StudentObj
		if !feedback.StudentId.IsZero() {
			student, err := r.studentRepo.GetByFilter(ctx, bson.M{"_id": feedback.StudentId, "trash": false})
			if err == nil {
				feedback.StudentObj = student
			}
		}

		// Populate TeacherObj
		if !feedback.TeacherId.IsZero() {
			teacher, err := r.teacherRepo.GetByFilter(ctx, bson.M{"_id": feedback.TeacherId, "trash": false})
			if err == nil {
				feedback.TeacherObj = teacher
			}
		}
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.FeedbackPagination{
		Feedbacks:  result,
		Pagination: pagination,
	}, nil
}
func (r *feedbackSvcs) GetOne(ctx context.Context, id string) (*models.Feedback, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	relation, err := r.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("feedback not found")
		}
		return nil, err
	}

	// Populate StudentObj
	if !relation.StudentId.IsZero() {
		student, err := r.studentRepo.GetByFilter(ctx, bson.M{"_id": relation.StudentId, "trash": false})
		if err == nil {
			relation.StudentObj = student
		}
	}

	// Populate TeacherObj
	if !relation.TeacherId.IsZero() {
		teacher, err := r.teacherRepo.GetByFilter(ctx, bson.M{"_id": relation.TeacherId, "trash": false})
		if err == nil {
			relation.TeacherObj = teacher
		}
	}

	return relation, nil
}
func (r *feedbackSvcs) Add(ctx context.Context, data *models.FeedbackDto) (*models.Feedback, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	relation := &models.Feedback{
		Id:          primitive.NewObjectID(),
		FeedbackDto: *data,

		CreatedAt: time.Now(),
		CreatedBy: cfg.User.Id,
		UpdatedAt: time.Now(),
		UpdatedBy: cfg.User.Id,
	}
	if err := r.repo.Add(ctx, relation); err != nil {
		return nil, err
	}

	return relation, nil
}

func (r *feedbackSvcs) Update(ctx context.Context, id string, data *models.FeedbackDto) (*models.Feedback, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the existing feedback
	oldFeedback, err := r.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create a copy of the old feedback and update only the DTO fields
	updatedFeedback := *oldFeedback
	updatedFeedback.FeedbackDto = *data
	updatedFeedback.UpdatedAt = time.Now()
	updatedFeedback.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedFeedback}

	updatedFeedbackResult, err := r.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("feedback not found")
		}
		return nil, err
	}
	return updatedFeedbackResult, nil
}
func (s *feedbackSvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{
		"$set": bson.M{
			"trash":     true,
			"updatedAt": time.Now(),
			"updatedBy": cfg.User.Id,
		},
	}

	// Use the repo to update the document
	_, err = s.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (s *feedbackSvcs) GetStats(ctx context.Context) (*models.FeedbackStats, error) {
	totalPipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$count": "total"},
	}
	var totalResult []struct {
		Total int64 `bson:"total"`
	}
	err := s.repo.Aggregate(ctx, totalPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &totalResult)
	})
	if err != nil {
		return nil, err
	}
	total := int64(0)
	if len(totalResult) > 0 {
		total = totalResult[0].Total
	}
	approvedCount := s.countByStatus(ctx, "approved")
	rejectedCount := s.countByStatus(ctx, "rejected")
	pendingCount := s.countByStatus(ctx, "pending")
	syncedPipeline := []bson.M{
		{"$match": bson.M{"trash": false, "isShowInApp": true}},
		{"$count": "total"},
	}
	var syncedResult []struct {
		Total int64 `bson:"total"`
	}
	err = s.repo.Aggregate(ctx, syncedPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &syncedResult)
	})
	if err != nil {
		return nil, err
	}
	totalSynced := int64(0)
	if len(syncedResult) > 0 {
		totalSynced = syncedResult[0].Total
	}
	return &models.FeedbackStats{
		Total:                       total,
		TotalApproved:               approvedCount,
		TotalRejected:               rejectedCount,
		TotalPending:                pendingCount,
		TotalSynchronizedWithGoogle: totalSynced,
	}, nil
}
func (s *feedbackSvcs) countByStatus(ctx context.Context, status string) int64 {
	pipeline := []bson.M{
		{"$match": bson.M{"trash": false, "status": status}},
		{"$count": "total"},
	}
	var result []struct {
		Total int64 `bson:"total"`
	}
	err := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil || len(result) == 0 {
		return 0
	}
	return result[0].Total
}
