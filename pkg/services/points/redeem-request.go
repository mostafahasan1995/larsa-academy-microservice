package points

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	contact "larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/messaging"
	messagingmodels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/services/points/enums"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type RedeemRequestSvcs interface {
	GetById(ctx context.Context, id string) (*models.RedeemRequest, error)
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.RedeemRequestPagination, error)
	GetAll(ctx context.Context, query *query.Conditions) ([]models.RedeemRequest, error)
	Create(ctx context.Context, data *models.RedeemRequest) (*models.RedeemRequest, error)
	Approve(ctx context.Context, id string) (*models.RedeemRequest, error)
	Reject(ctx context.Context, id string, reason string) (*models.RedeemRequest, error)
	Delete(ctx context.Context, id string) error
}

type redeemRequestsvcs struct {
	redeemRequestRepo  db.MainRepo[models.RedeemRequest]
	studentPointRepo   db.MainRepo[models.StudentPoint]
	rewardRepo         db.MainRepo[models.Reward]
	transactionRepo    db.MainRepo[models.PointTransaction]
	contactStudentSvcs contact.StudentSvcs
	withTxn            *db.WithTxn
	messageSvcs        messaging.MessageSvcs
}

func NewRedeemRequestSvcs(i *do.Injector) (RedeemRequestSvcs, error) {
	return &redeemRequestsvcs{
		redeemRequestRepo:  db.NewRepo[models.RedeemRequest](i),
		studentPointRepo:   db.NewRepo[models.StudentPoint](i),
		rewardRepo:         db.NewRepo[models.Reward](i),
		transactionRepo:    db.NewRepo[models.PointTransaction](i),
		contactStudentSvcs: do.MustInvoke[contact.StudentSvcs](i),
		withTxn:            do.MustInvoke[*db.WithTxn](i),
		messageSvcs:        do.MustInvoke[messaging.MessageSvcs](i),
	}, nil
}

func (s *redeemRequestsvcs) GetById(ctx context.Context, id string) (*models.RedeemRequest, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid redeem request id")
	}

	// Use aggregation to populate StudentObj and RewardObj
	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "studentObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$studentObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyRewards",
				"localField":   "rewardId",
				"foreignField": "_id",
				"as":           "rewardObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$rewardObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
	}

	var result []models.RedeemRequest
	err = s.redeemRequestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("redeem request not found")
	}

	return &result[0], nil
}

func (s *redeemRequestsvcs) Get(ctx context.Context, skip, limit int64, conditions *query.Conditions) (*models.RedeemRequestPagination, error) {
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
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "studentObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$studentObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyRewards",
				"localField":   "rewardId",
				"foreignField": "_id",
				"as":           "rewardObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$rewardObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{"$facet": bson.M{
			"metadata": []bson.M{{"$count": "total"}},
			"data":     []bson.M{{"$sort": bson.M{"_id": -1}}, {"$skip": skip}, {"$limit": limit}},
		}},
	}

	var result []struct {
		Metadata []struct {
			Total int64 `bson:"total"`
		} `bson:"metadata"`
		Data []models.RedeemRequest `bson:"data"`
	}

	err := s.redeemRequestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	total := int64(0)
	if len(result) > 0 && len(result[0].Metadata) > 0 {
		total = result[0].Metadata[0].Total
	}
	requests := []models.RedeemRequest{}
	if len(result) > 0 {
		requests = result[0].Data
	}

	pages := 0.0
	if total > 0 && limit > 0 {
		pages = math.Ceil(float64(total) / float64(limit))
	}
	return &models.RedeemRequestPagination{
		Requests: requests,
		Pagination: types.Pagination{
			TotalPages: pages,
			PerPage:    limit,
			TotalCount: total,
		},
	}, nil
}

func (s *redeemRequestsvcs) GetAll(ctx context.Context, conditions *query.Conditions) ([]models.RedeemRequest, error) {
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
		{
			"$lookup": bson.M{
				"from":         "academyStudents",
				"localField":   "studentId",
				"foreignField": "_id",
				"as":           "studentObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$studentObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$lookup": bson.M{
				"from":         "academyRewards",
				"localField":   "rewardId",
				"foreignField": "_id",
				"as":           "rewardObj",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$rewardObj",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{"$sort": bson.M{"_id": -1}},
	}

	var result []models.RedeemRequest
	err := s.redeemRequestRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	return result, nil
}

func (s *redeemRequestsvcs) Create(ctx context.Context, data *models.RedeemRequest) (*models.RedeemRequest, error) {
	// Validate that StudentID and RewardID are provided
	if data.StudentID.IsZero() {
		return nil, errors.New("studentId is required")
	}
	if data.RewardID.IsZero() {
		return nil, errors.New("rewardId is required")
	}

	// Fetch student details from contact service
	student, err := s.contactStudentSvcs.GetOne(ctx, data.StudentID.Hex())
	if err != nil {
		return nil, errors.New("student not found")
	}

	// Fetch reward details
	reward, err := s.rewardRepo.GetByFilter(ctx, bson.M{"_id": data.RewardID, "trash": false})
	if err != nil {
		return nil, errors.New("reward not found")
	}

	// Validate reward is active
	if reward.Status != enums.RewardStatusActive {
		return nil, errors.New("reward is not active")
	}

	// Validate reward is in stock (if quantity tracking is enabled)
	if reward.QuantityAvailable > 0 && reward.QuantityAvailable < 1 {
		return nil, errors.New("reward is out of stock")
	}

	// Auto-populate fields from student
	firstName := student.GeneralInfo.FirstName["en"]
	lastName := student.GeneralInfo.LastName["en"]
	data.StudentName = fmt.Sprintf("%s %s", firstName, lastName)
	data.Grade = student.AcademicInfo.Stage
	data.Class = student.AcademicInfo.Class

	// Auto-populate fields from reward
	data.RewardTitle = reward.Title
	data.PointsRequired = reward.PointsRequired

	// Set default fields
	data.ID = primitive.NewObjectID()
	data.Status = enums.RedeemStatusPending
	data.RequestDate = time.Now()
	data.CreatedAt = time.Now()
	data.UpdatedAt = time.Now()
	data.CreatedBy = ctx.Value(util.ReqUser).(*types.User).Id
	data.UpdatedBy = ctx.Value(util.ReqUser).(*types.User).Id

	if err := s.redeemRequestRepo.Add(ctx, data); err != nil {
		return nil, err
	}

	// Populate objects for response
	data.StudentObj = &student.Student
	data.RewardObj = reward

	return data, nil
}

func (s *redeemRequestsvcs) Approve(ctx context.Context, id string) (*models.RedeemRequest, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	req, err := s.GetById(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != enums.RedeemStatusPending {
		return nil, errors.New("request is not pending")
	}

	// Get student point record and validate they have enough points
	sp, err := s.studentPointRepo.GetByFilter(ctx, bson.M{"studentId": req.StudentID, "trash": false})
	if err != nil {
		return nil, errors.New("student point record not found")
	}

	// Validate student has enough remaining points
	if sp.RemainingPoints < req.PointsRequired {
		return nil, errors.New("insufficient points for redemption")
	}

	result, err := s.withTxn.Exec(ctx, func(sessCtx mongo.SessionContext) (any, error) {
		// Update redeem request
		now := time.Now()
		req.Status = enums.RedeemStatusApproved
		req.ApprovedBy = cfg.User.Id
		req.ApprovedAt = &now
		req.UpdatedAt = now

		filter := bson.M{"_id": req.ID, "trash": false}
		update := bson.M{"$set": req}
		updatedReq, err := s.redeemRequestRepo.Patch(sessCtx, filter, update)
		if err != nil {
			return nil, err
		}

		// Deduct points from student by increasing redeemed points
		sp.RedeemedPoints += req.PointsRequired
		sp.RemainingPoints = sp.TotalPoints - sp.RedeemedPoints
		sp.UpdatedAt = now

		studentPointFilter := bson.M{"studentId": req.StudentID, "trash": false}
		studentPointUpdate := bson.M{"$set": bson.M{
			"redeemedPoints":  sp.RedeemedPoints,
			"remainingPoints": sp.RemainingPoints,
			"updatedAt":       sp.UpdatedAt,
		}}
		_, err = s.studentPointRepo.Patch(sessCtx, studentPointFilter, studentPointUpdate)
		if err != nil {
			return nil, fmt.Errorf("failed to update student points: %w", err)
		}

		// Create transaction record
		transaction := &models.PointTransaction{
			ID:              primitive.NewObjectID(),
			StudentID:       req.StudentID,
			StudentName:     req.StudentName,
			Points:          -req.PointsRequired,
			TransactionType: enums.TransactionTypeRedemption,
			Source:          enums.TransactionSourceSystem,
			ReferenceID:     req.ID,
			ReferenceType:   "RedeemRequest",
			CreatedAt:       time.Now(),
		}
		if err := s.transactionRepo.Add(sessCtx, transaction); err != nil {
			return nil, fmt.Errorf("failed to create transaction record: %w", err)
		}

		return updatedReq, nil
	})
	if err != nil {
		return nil, err
	}

	updatedReq := result.(*models.RedeemRequest)
	_ = s.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{updatedReq.StudentID}, &messagingmodels.Notification{
		ServiceName: "academy",
		Title:       "Redeem request approved",
		Content:     "Your redeem request has been approved.",
		Recipient:   updatedReq.StudentID.Hex(),
	})
	return updatedReq, nil
}

func (s *redeemRequestsvcs) Reject(ctx context.Context, id string, reason string) (*models.RedeemRequest, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	req, err := s.GetById(ctx, id)
	if err != nil {
		return nil, err
	}
	if req.Status != enums.RedeemStatusPending {
		return nil, errors.New("request is not pending")
	}

	req.Status = enums.RedeemStatusRejected
	req.RejectedReason = reason
	req.UpdatedAt = time.Now()
	req.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": req.ID, "trash": false}
	update := bson.M{"$set": req}
	updated, err := s.redeemRequestRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}
	_ = s.messageSvcs.SendUserNotification(ctx, []primitive.ObjectID{req.StudentID}, &messagingmodels.Notification{
		ServiceName: "academy",
		Title:       "Redeem request rejected",
		Content:     "Your redeem request has been rejected.",
		Recipient:   req.StudentID.Hex(),
	})
	return updated, nil
}

func (s *redeemRequestsvcs) Delete(ctx context.Context, id string) error {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return errors.New("invalid redeem request id")
	}
	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now()}}
	_, err = s.redeemRequestRepo.Patch(ctx, filter, update)
	return err
}
