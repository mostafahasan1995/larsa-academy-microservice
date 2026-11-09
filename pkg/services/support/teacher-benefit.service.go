package support

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contactModels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/support/enums"
	"larsa-academy-microservice/pkg/services/support/models"
	"larsa-academy-microservice/pkg/services/support/repo"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type TeacherBenefitSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherBenefitPagination, error)
	GetGrouped(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherBenefitGroupedPagination, error)
	GetOne(ctx context.Context, id string) (*models.TeacherBenefit, error)
	GetByTeacher(ctx context.Context, teacherId string, skip, limit int64) (*models.TeacherBenefitPagination, error)
	Add(ctx context.Context, data *models.TeacherBenefitDto) (*models.TeacherBenefit, error)
	BulkAdd(ctx context.Context, data *models.BulkTeacherBenefitRequest) (*models.BulkTeacherBenefitResponse, error)
	Update(ctx context.Context, id string, data *models.TeacherBenefitDto) (*models.TeacherBenefit, error)
	BulkUpdate(ctx context.Context, data *models.BulkTeacherBenefitUpdateRequest) (*models.BulkTeacherBenefitResponse, error)
	Delete(ctx context.Context, id string) error
	GetStats(ctx context.Context) (*models.BenefitStats, error)
}

type teacherBenefitSvcs struct {
	repo            repo.TeacherBenefitRepo
	benefitRepo     repo.BenefitRepo
	benefitTypeRepo repo.BenefitTypeRepo
	teacherRepo     db.MainRepo[contactModels.Teacher]
	withtxn         *db.WithTxn
}

func NewTeacherBenefitSvcs(i *do.Injector) (TeacherBenefitSvcs, error) {
	return &teacherBenefitSvcs{
		repo:            do.MustInvoke[repo.TeacherBenefitRepo](i),
		benefitRepo:     do.MustInvoke[repo.BenefitRepo](i),
		benefitTypeRepo: do.MustInvoke[repo.BenefitTypeRepo](i),
		teacherRepo:     db.NewRepo[contactModels.Teacher](i),
		withtxn:         do.MustInvoke[*db.WithTxn](i),
	}, nil
}

func (s *teacherBenefitSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherBenefitPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	// Apply visibility filtering
	// Show only public benefits or benefits assigned to the requesting user
	// Note: In a production environment, you would check user roles here
	// For now, we only filter by visibility and teacherId
	pipeline = append(pipeline, bson.M{
		"$match": bson.M{
			"$or": []bson.M{
				{"visibility": enums.VisibilityPublic},
				{"teacherId": cfg.User.Id},
			},
		},
	})

	count, err := s.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.TeacherBenefit
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	// Populate related objects
	for i := range result {
		s.populateTeacherBenefit(ctx, &result[i])
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.TeacherBenefitPagination{
		TeacherBenefits: result,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *teacherBenefitSvcs) GetGrouped(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.TeacherBenefitGroupedPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Build match pipeline with visibility filtering
	matchStage := bson.M{"trash": false}
	for k, v := range filter {
		matchStage[k] = v
	}

	// Apply visibility filtering
	visibilityMatch := bson.M{
		"$or": []bson.M{
			{"visibility": enums.VisibilityPublic},
			{"teacherId": cfg.User.Id},
		},
	}

	// Aggregation pipeline to group by teacher
	pipeline := []bson.M{
		{"$match": matchStage},
		{"$match": visibilityMatch},
		{
			"$group": bson.M{
				"_id":        "$teacherId",
				"benefits":   bson.M{"$push": "$$ROOT"},
				"totalCount": bson.M{"$sum": 1},
			},
		},
		{
			"$project": bson.M{
				"teacherId":  "$_id",
				"benefits":   1,
				"totalCount": 1,
			},
		},
		{"$sort": bson.M{"totalCount": -1}},
	}

	// Count total groups
	countPipeline := append([]bson.M{}, pipeline...)
	countPipeline = append(countPipeline, bson.M{"$count": "total"})
	var countResult []struct {
		Total int64 `bson:"total"`
	}
	err = s.repo.Aggregate(ctx, countPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &countResult)
	})
	if err != nil {
		return nil, err
	}
	totalGroups := int64(0)
	if len(countResult) > 0 {
		totalGroups = countResult[0].Total
	}

	// Add pagination to main pipeline
	pipeline = append(pipeline,
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var grouped []models.TeacherBenefitGrouped
	err = s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &grouped)
	})
	if err != nil {
		return nil, err
	}

	// Populate teacher objects and benefit details
	for i := range grouped {
		group := &grouped[i]

		// Populate teacher object
		if !group.TeacherId.IsZero() {
			teacher, err := s.teacherRepo.GetByFilter(ctx, bson.M{"_id": group.TeacherId, "trash": false})
			if err == nil {
				group.TeacherObj = teacher
			}
		}

		// Populate benefit objects for each benefit in the array
		for j := range group.Benefits {
			s.populateTeacherBenefit(ctx, &group.Benefits[j])
		}
	}

	totalPages := math.Ceil(float64(totalGroups) / float64(limit))

	return &models.TeacherBenefitGroupedPagination{
		Grouped: grouped,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: totalGroups,
		},
	}, nil
}

func (s *teacherBenefitSvcs) GetOne(ctx context.Context, id string) (*models.TeacherBenefit, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	teacherBenefit, err := s.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("teacher benefit not found")
		}
		return nil, err
	}

	// Check visibility - if benefit is private and not assigned to the requesting user, deny access
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	if teacherBenefit.Visibility == enums.VisibilityPrivate && teacherBenefit.TeacherId != cfg.User.Id {
		return nil, errors.New("access denied: private benefit")
	}

	// Populate related objects
	s.populateTeacherBenefit(ctx, teacherBenefit)

	return teacherBenefit, nil
}

func (s *teacherBenefitSvcs) GetByTeacher(ctx context.Context, teacherId string, skip, limit int64) (*models.TeacherBenefitPagination, error) {
	teacherObjId, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	filter := bson.M{
		"trash":     false,
		"teacherId": teacherObjId,
	}

	// If user is not the same teacher, only show public benefits
	if cfg.User.Id != teacherObjId {
		filter["visibility"] = enums.VisibilityPublic
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := s.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.TeacherBenefit
	errAg := s.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	// Populate related objects
	for i := range result {
		s.populateTeacherBenefit(ctx, &result[i])
	}

	totalPages := math.Ceil(float64(count) / float64(limit))

	return &models.TeacherBenefitPagination{
		TeacherBenefits: result,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *teacherBenefitSvcs) Add(ctx context.Context, data *models.TeacherBenefitDto) (*models.TeacherBenefit, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate visibility enum
	if !data.Visibility.IsValid() {
		return nil, errors.New("invalid visibility value")
	}

	// Validate date range
	if data.FromDate.After(data.ToDate) || data.FromDate.Equal(data.ToDate) {
		return nil, errors.New("fromDate must be before toDate")
	}

	// Validate that the benefit exists
	if !data.BenefitId.IsZero() {
		_, err := s.benefitRepo.GetByFilter(ctx, bson.M{"_id": data.BenefitId, "trash": false})
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("benefit not found")
			}
			return nil, err
		}
	}

	// Validate that the teacher exists
	if !data.TeacherId.IsZero() {
		_, err := s.teacherRepo.GetByFilter(ctx, bson.M{"_id": data.TeacherId, "trash": false})
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("teacher not found")
			}
			return nil, err
		}
	}

	teacherBenefit := &models.TeacherBenefit{
		Id:                primitive.NewObjectID(),
		TeacherBenefitDto: *data,
		CreatedAt:         time.Now(),
		CreatedBy:         cfg.User.Id,
		UpdatedAt:         time.Now(),
		UpdatedBy:         cfg.User.Id,
	}

	if err := s.repo.Add(ctx, teacherBenefit); err != nil {
		return nil, err
	}

	return teacherBenefit, nil
}

func (s *teacherBenefitSvcs) BulkAdd(ctx context.Context, data *models.BulkTeacherBenefitRequest) (*models.BulkTeacherBenefitResponse, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Pre-validate all items before starting transaction
	for index, item := range data.TeacherBenefits {
		// Validate visibility enum
		if !item.Visibility.IsValid() {
			return nil, errors.New("invalid visibility value at index " + string(rune(index)))
		}

		// Validate date range
		if item.FromDate.After(item.ToDate) || item.FromDate.Equal(item.ToDate) {
			return nil, errors.New("fromDate must be before toDate at index " + string(rune(index)))
		}

		// Validate that the benefit exists
		if !item.BenefitId.IsZero() {
			_, err := s.benefitRepo.GetByFilter(ctx, bson.M{"_id": item.BenefitId, "trash": false})
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, errors.New("benefit not found at index " + string(rune(index)))
				}
				return nil, err
			}
		}

		// Validate that the teacher exists
		if !item.TeacherId.IsZero() {
			_, err := s.teacherRepo.GetByFilter(ctx, bson.M{"_id": item.TeacherId, "trash": false})
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, errors.New("teacher not found at index " + string(rune(index)))
				}
				return nil, err
			}
		}
	}

	// All validations passed, execute in transaction
	result, err := s.withtxn.Exec(ctx, func(sc mongo.SessionContext) (any, error) {
		var createdBenefits []models.TeacherBenefit
		for _, item := range data.TeacherBenefits {
			teacherBenefit := &models.TeacherBenefit{
				Id:                primitive.NewObjectID(),
				TeacherBenefitDto: item,
				CreatedAt:         time.Now(),
				CreatedBy:         cfg.User.Id,
				UpdatedAt:         time.Now(),
				UpdatedBy:         cfg.User.Id,
			}

			if err := s.repo.Add(sc, teacherBenefit); err != nil {
				return nil, err
			}
			createdBenefits = append(createdBenefits, *teacherBenefit)
		}
		return createdBenefits, nil
	})

	if err != nil {
		return nil, err
	}

	createdBenefits := result.([]models.TeacherBenefit)
	return &models.BulkTeacherBenefitResponse{
		SuccessCount: len(createdBenefits),
		FailureCount: 0,
		Successes:    createdBenefits,
		Failures:     []models.BulkErrorItem{},
	}, nil
}

func (s *teacherBenefitSvcs) Update(ctx context.Context, id string, data *models.TeacherBenefitDto) (*models.TeacherBenefit, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Validate visibility enum
	if !data.Visibility.IsValid() {
		return nil, errors.New("invalid visibility value")
	}

	// Validate date range
	if data.FromDate.After(data.ToDate) || data.FromDate.Equal(data.ToDate) {
		return nil, errors.New("fromDate must be before toDate")
	}

	// Validate that the benefit exists
	if !data.BenefitId.IsZero() {
		_, err := s.benefitRepo.GetByFilter(ctx, bson.M{"_id": data.BenefitId, "trash": false})
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("benefit not found")
			}
			return nil, err
		}
	}

	// Validate that the teacher exists
	if !data.TeacherId.IsZero() {
		_, err := s.teacherRepo.GetByFilter(ctx, bson.M{"_id": data.TeacherId, "trash": false})
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("teacher not found")
			}
			return nil, err
		}
	}

	// Get the existing teacher benefit
	oldTeacherBenefit, err := s.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create a copy and update fields
	updatedTeacherBenefit := *oldTeacherBenefit
	updatedTeacherBenefit.TeacherBenefitDto = *data
	updatedTeacherBenefit.UpdatedAt = time.Now()
	updatedTeacherBenefit.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedTeacherBenefit}

	result, err := s.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("teacher benefit not found")
		}
		return nil, err
	}

	return result, nil
}

func (s *teacherBenefitSvcs) BulkUpdate(ctx context.Context, data *models.BulkTeacherBenefitUpdateRequest) (*models.BulkTeacherBenefitResponse, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Pre-validate all items before starting transaction
	for index, item := range data.TeacherBenefits {
		// Validate visibility enum
		if !item.Visibility.IsValid() {
			return nil, errors.New("invalid visibility value at index " + string(rune(index)))
		}

		// Validate date range
		if item.FromDate.After(item.ToDate) || item.FromDate.Equal(item.ToDate) {
			return nil, errors.New("fromDate must be before toDate at index " + string(rune(index)))
		}

		// Validate that the benefit exists
		if !item.BenefitId.IsZero() {
			_, err := s.benefitRepo.GetByFilter(ctx, bson.M{"_id": item.BenefitId, "trash": false})
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, errors.New("benefit not found at index " + string(rune(index)))
				}
				return nil, err
			}
		}

		// Validate that the teacher exists
		if !item.TeacherId.IsZero() {
			_, err := s.teacherRepo.GetByFilter(ctx, bson.M{"_id": item.TeacherId, "trash": false})
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, errors.New("teacher not found at index " + string(rune(index)))
				}
				return nil, err
			}
		}
	}

	// All validations passed, execute in transaction
	result, err := s.withtxn.Exec(ctx, func(sc mongo.SessionContext) (any, error) {
		var updatedBenefits []models.TeacherBenefit
		for _, item := range data.TeacherBenefits {
			_id, err := primitive.ObjectIDFromHex(item.Id)
			if err != nil {
				return nil, err
			}

			// Get the existing teacher benefit
			oldTeacherBenefit, err := s.repo.GetByFilter(sc, bson.M{"_id": _id, "trash": false})
			if err != nil {
				if errors.Is(err, mongo.ErrNoDocuments) {
					return nil, errors.New("teacher benefit not found: " + item.Id)
				}
				return nil, err
			}

			// Create a copy and update fields
			updatedTeacherBenefit := *oldTeacherBenefit
			updatedTeacherBenefit.TeacherId = item.TeacherId
			updatedTeacherBenefit.BenefitId = item.BenefitId
			updatedTeacherBenefit.FromDate = item.FromDate
			updatedTeacherBenefit.ToDate = item.ToDate
			updatedTeacherBenefit.Visibility = item.Visibility
			updatedTeacherBenefit.UpdatedAt = time.Now()
			updatedTeacherBenefit.UpdatedBy = cfg.User.Id

			filter := bson.M{"_id": _id}
			update := bson.M{"$set": updatedTeacherBenefit}

			patchedBenefit, err := s.repo.Patch(sc, filter, update)
			if err != nil {
				return nil, err
			}
			updatedBenefits = append(updatedBenefits, *patchedBenefit)
		}
		return updatedBenefits, nil
	})

	if err != nil {
		return nil, err
	}

	updatedBenefits := result.([]models.TeacherBenefit)
	return &models.BulkTeacherBenefitResponse{
		SuccessCount: len(updatedBenefits),
		FailureCount: 0,
		Successes:    updatedBenefits,
		Failures:     []models.BulkErrorItem{},
	}, nil
}

func (s *teacherBenefitSvcs) Delete(ctx context.Context, id string) error {
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

	_, err = s.repo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (s *teacherBenefitSvcs) GetStats(ctx context.Context) (*models.BenefitStats, error) {
	// Total benefits given
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

	// Teachers with benefits (distinct count)
	teachersPipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$group": bson.M{"_id": "$teacherId"}},
		{"$count": "total"},
	}
	var teachersResult []struct {
		Total int64 `bson:"total"`
	}
	err = s.repo.Aggregate(ctx, teachersPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teachersResult)
	})
	if err != nil {
		return nil, err
	}
	teachersWithBenefits := int64(0)
	if len(teachersResult) > 0 {
		teachersWithBenefits = teachersResult[0].Total
	}

	// Calculate average
	avgBenefitsPerTeacher := float64(0)
	if teachersWithBenefits > 0 {
		avgBenefitsPerTeacher = float64(total) / float64(teachersWithBenefits)
	}

	// Type breakdown - aggregate by benefit type
	typeBreakdownPipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$lookup": bson.M{
			"from":         "benefits",
			"localField":   "benefitId",
			"foreignField": "_id",
			"as":           "benefit",
		}},
		{"$unwind": "$benefit"},
		{"$lookup": bson.M{
			"from":         "benefitTypes",
			"localField":   "benefit.benefitTypeId",
			"foreignField": "_id",
			"as":           "benefitType",
		}},
		{"$unwind": "$benefitType"},
		{"$group": bson.M{
			"_id":      "$benefitType._id",
			"typeName": bson.M{"$first": "$benefitType.name"},
			"count":    bson.M{"$sum": 1},
		}},
		{"$sort": bson.M{"count": -1}},
	}

	var typeBreakdown []models.TypeBreakdown
	err = s.repo.Aggregate(ctx, typeBreakdownPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &typeBreakdown)
	})
	if err != nil {
		return nil, err
	}

	return &models.BenefitStats{
		TotalBenefitsGiven:    total,
		TeachersWithBenefits:  teachersWithBenefits,
		AvgBenefitsPerTeacher: avgBenefitsPerTeacher,
		TypeBreakdown:         typeBreakdown,
	}, nil
}

// Helper function to populate related objects
func (s *teacherBenefitSvcs) populateTeacherBenefit(ctx context.Context, tb *models.TeacherBenefit) {
	// Populate BenefitObj
	if !tb.BenefitId.IsZero() {
		benefit, err := s.benefitRepo.GetByFilter(ctx, bson.M{"_id": tb.BenefitId, "trash": false})
		if err == nil {
			tb.BenefitObj = benefit

			// Also populate BenefitTypeObj within BenefitObj
			if !benefit.BenefitTypeId.IsZero() {
				benefitType, err := s.benefitTypeRepo.GetByFilter(ctx, bson.M{"_id": benefit.BenefitTypeId, "trash": false})
				if err == nil {
					tb.BenefitObj.BenefitTypeObj = benefitType
				}
			}
		}
	}

	// Populate TeacherObj
	if !tb.TeacherId.IsZero() {
		teacher, err := s.teacherRepo.GetByFilter(ctx, bson.M{"_id": tb.TeacherId, "trash": false})
		if err == nil {
			tb.TeacherObj = teacher
		}
	}
}
