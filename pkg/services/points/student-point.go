package points

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/types"
	"math"
	"time"

	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type StudentPointSvcs interface {
	GetById(ctx context.Context, id string) (*models.StudentPoint, error)
	GetByStudentId(ctx context.Context, studentId string) (*models.StudentPoint, error)
	Get(ctx context.Context, skip, limit int64, filter *query.Conditions) (*models.StudentPointPagination, error)
	GetAll(ctx context.Context, conditions *query.Conditions) ([]models.StudentPoint, error)
	GetStatistics(ctx context.Context, topN int) (*models.PointsStatistics, error)
	CreateOrUpdate(ctx context.Context, data *models.StudentPoint) (*models.StudentPoint, error)
	UpdateBalance(ctx context.Context, studentId string, pointsDelta int) (*models.StudentPoint, error)
	Delete(ctx context.Context, id string) error
}

type studentPointsvcs struct {
	studentPointRepo db.MainRepo[models.StudentPoint]
	transactionRepo  db.MainRepo[models.PointTransaction]
	rewardRepo       db.MainRepo[models.Reward]
}

func NewStudentPointSvcs(i *do.Injector) (StudentPointSvcs, error) {
	return &studentPointsvcs{
		studentPointRepo: db.NewRepo[models.StudentPoint](i),
		transactionRepo:  db.NewRepo[models.PointTransaction](i),
		rewardRepo:       db.NewRepo[models.Reward](i),
	}, nil
}

func (s *studentPointsvcs) GetById(ctx context.Context, id string) (*models.StudentPoint, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid student point id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "studentObj",
		}},
		{"$unwind": bson.M{
			"path":                       "$studentObj",
			"preserveNullAndEmptyArrays": true,
		}},
	}

	var result []models.StudentPoint
	err = s.studentPointRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("student point not found")
	}

	return &result[0], nil
}

func (s *studentPointsvcs) GetByStudentId(ctx context.Context, studentId string) (*models.StudentPoint, error) {
	_id, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"studentId": _id, "trash": false}},
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "studentObj",
		}},
		{"$unwind": bson.M{
			"path":                       "$studentObj",
			"preserveNullAndEmptyArrays": true,
		}},
	}

	var result []models.StudentPoint
	err = s.studentPointRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("student point not found")
	}

	return &result[0], nil
}

func (s *studentPointsvcs) Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.StudentPointPagination, error) {
	filter := bson.M{"trash": false}

	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}
		queryFilter, err := conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		// Merge query filter with trash filter
		for k, v := range queryFilter {
			filter[k] = v
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$lookup": bson.M{
			"from":         "academyStudents",
			"localField":   "studentId",
			"foreignField": "_id",
			"as":           "studentObj",
		}},
		{"$unwind": bson.M{
			"path":                       "$studentObj",
			"preserveNullAndEmptyArrays": true,
		}},
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"_id": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.StudentPoint `bson:"data"`
	}

	err := s.studentPointRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	studentPoints := []models.StudentPoint{}
	if len(result) > 0 {
		studentPoints = result[0].Data
	}

	// Calculate last month's date range
	now := time.Now()
	firstDayOfCurrentMonth := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	lastDayOfLastMonth := firstDayOfCurrentMonth.AddDate(0, 0, -1)
	firstDayOfLastMonth := time.Date(lastDayOfLastMonth.Year(), lastDayOfLastMonth.Month(), 1, 0, 0, 0, 0, now.Location())

	// Enrich student points with "Earned Last Month"
	studentsWithStats := make([]models.StudentPointWithStats, 0, len(studentPoints))
	for _, sp := range studentPoints {
		stats := models.StudentPointWithStats{
			StudentPoint: sp,
		}

		// Calculate points earned last month from PointTransaction collection
		// Query for positive point transactions (earned) in the last month
		lastMonthPipeline := []bson.M{
			{"$match": bson.M{
				"studentId": sp.StudentID,
				"trash":     false,
				"points":    bson.M{"$gt": 0}, // Only positive points (earned)
				"createdAt": bson.M{
					"$gte": firstDayOfLastMonth,
					"$lt":  firstDayOfCurrentMonth,
				},
			}},
			{"$group": bson.M{
				"_id":   nil,
				"total": bson.M{"$sum": "$points"},
			}},
		}
		var lastMonthResult []struct {
			Total int `bson:"total"`
		}
		err = s.transactionRepo.Aggregate(ctx, lastMonthPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &lastMonthResult)
		})
		if err == nil && len(lastMonthResult) > 0 {
			stats.EarnedLastMonth = lastMonthResult[0].Total
		} else {
			stats.EarnedLastMonth = 0
		}

		studentsWithStats = append(studentsWithStats, stats)
	}

	pages := 0.0
	if total > 0 && limit > 0 {
		pages = math.Ceil(float64(total) / float64(limit))
	}
	return &models.StudentPointPagination{
		StudentPoints: studentsWithStats,
		Pagination: types.Pagination{
			TotalPages: pages,
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *studentPointsvcs) GetAll(ctx context.Context, conditions *query.Conditions) ([]models.StudentPoint, error) {
	filter := bson.M{"trash": false}

	if conditions != nil {
		if err := conditions.CheckValid(); err != nil {
			return nil, err
		}
		queryFilter, err := conditions.ConvertToMongo()
		if err != nil {
			return nil, err
		}
		// Merge query filter with trash filter
		for k, v := range queryFilter {
			filter[k] = v
		}
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
	}

	var result []models.StudentPoint
	err := s.studentPointRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *studentPointsvcs) GetStatistics(ctx context.Context, topN int) (*models.PointsStatistics, error) {
	if topN <= 0 {
		topN = 3 // Default to top 3
	}

	stats := &models.PointsStatistics{}

	// 1. Calculate Total Points Distributed (sum of all TotalPoints from student-points collection)
	totalPointsPipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$group": bson.M{
			"_id":   nil,
			"total": bson.M{"$sum": "$totalPoints"},
		}},
	}
	var totalPointsResult []struct {
		Total int `bson:"total"`
	}
	err := s.studentPointRepo.Aggregate(ctx, totalPointsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &totalPointsResult)
	})
	if err != nil {
		return nil, err
	}
	if len(totalPointsResult) > 0 {
		stats.TotalPointsDistributed = totalPointsResult[0].Total
	}

	// 2. Count Active Rewards
	activeRewardsFilter := bson.M{"trash": false, "status": "Active"}
	activeRewardsCount, err := s.rewardRepo.Count(ctx, activeRewardsFilter)
	if err != nil {
		return nil, err
	}
	stats.ActiveRewards = int(activeRewardsCount)

	// 3. Get Top Students (ordered by TotalPoints descending), with fresh student names from students collection
	topStudentsPipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$sort": bson.M{"totalPoints": -1}},
		{"$limit": topN},
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "student",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$student",
				"preserveNullAndEmptyArrays": true,
			},
		},
	}

	type topStudentAgg struct {
		models.StudentPoint `bson:",inline"`
		Student             struct {
			GeneralInfo struct {
				FirstName string `bson:"firstName"`
				LastName  string `bson:"lastName"`
				Class     string `bson:"class"`
			} `bson:"generalInfo"`
		} `bson:"student"`
	}

	var topStudents []topStudentAgg
	err = s.studentPointRepo.Aggregate(ctx, topStudentsPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &topStudents)
	})
	if err != nil {
		return nil, err
	}

	// Convert to TopStudent format with live student name/grade
	stats.TopStudents = make([]models.TopStudent, len(topStudents))
	for i, sp := range topStudents {
		fullName := sp.Student.GeneralInfo.FirstName + " " + sp.Student.GeneralInfo.LastName
		grade := sp.Student.GeneralInfo.Class
		if fullName == " " {
			// Fallback to stored name if lookup missing
			fullName = sp.StudentName
		}
		if grade == "" {
			grade = sp.Grade
		}
		stats.TopStudents[i] = models.TopStudent{
			Rank:        i + 1,
			StudentName: fullName,
			Grade:       grade,
			Points:      sp.TotalPoints,
		}
	}

	return stats, nil
}

func (s *studentPointsvcs) CreateOrUpdate(ctx context.Context, data *models.StudentPoint) (*models.StudentPoint, error) {
	if data.ID.IsZero() {
		data.ID = primitive.NewObjectID()
		data.CreatedAt = time.Now()
	}
	data.UpdatedAt = time.Now()
	data.CreatedBy = ctx.Value(util.ReqUser).(*types.User).Id
	data.UpdatedBy = ctx.Value(util.ReqUser).(*types.User).Id
	// Ensure remaining points is calculated
	if data.RemainingPoints == 0 && data.TotalPoints > 0 {
		data.RemainingPoints = data.TotalPoints - data.RedeemedPoints
	}

	if err := s.studentPointRepo.Add(ctx, data); err != nil {
		return nil, err
	}
	return data, nil
}

func (s *studentPointsvcs) UpdateBalance(ctx context.Context, studentId string, pointsDelta int) (*models.StudentPoint, error) {
	_id, err := primitive.ObjectIDFromHex(studentId)
	if err != nil {
		return nil, errors.New("invalid student id")
	}

	sp, err := s.GetByStudentId(ctx, studentId)
	if err != nil {
		// Create new record if doesn't exist
		sp = &models.StudentPoint{
			ID:              primitive.NewObjectID(),
			StudentID:       _id,
			TotalPoints:     0,
			RedeemedPoints:  0,
			RemainingPoints: 0,
			ThisMonthPoints: 0,
			CreatedAt:       time.Now(),
			UpdatedAt:       time.Now(),
		}
	}

	sp.TotalPoints += pointsDelta
	sp.RemainingPoints = sp.TotalPoints - sp.RedeemedPoints
	if pointsDelta > 0 {
		sp.ThisMonthPoints += pointsDelta
	}
	sp.UpdatedAt = time.Now()

	filter := bson.M{"studentId": _id, "trash": false}
	update := bson.M{"$set": sp}
	return s.studentPointRepo.Patch(ctx, filter, update)
}

func (s *studentPointsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid student point id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.studentPointRepo.Patch(ctx, filter, update)
	return err
}
