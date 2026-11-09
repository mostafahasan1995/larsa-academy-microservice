package support

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"strconv"
	"time"

	"larsa-academy-microservice/pkg/services/messaging"
	messagingmodels "larsa-academy-microservice/pkg/services/messaging/models"
	"larsa-academy-microservice/pkg/shared"
	contact_models "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/sorting"
	"larsa-academy-microservice/pkg/services/support/enums"
	"larsa-academy-microservice/pkg/services/support/models"
	"larsa-academy-microservice/pkg/services/support/repo"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type ComplaintSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ComplaintPagination, error)
	GetOne(ctx context.Context, id string) (*models.ComplaintWithDetails, error)
	GetStats(ctx context.Context, period string, fromStr, toStr string, statusFilter string) (*models.ComplaintStats, error)
	Add(ctx context.Context, data *models.ComplaintDto) (*models.ComplaintWithDetails, error)
	Update(ctx context.Context, id string, data *models.ComplaintDto) (*models.ComplaintWithDetails, error)
	Delete(ctx context.Context, id string) error
	ChangeStatus(ctx context.Context, id string, data *models.ComplaintStatusDto) (*models.ComplaintWithDetails, error)
	AssignTo(ctx context.Context, id string, assignToId string, data *models.AssignToDto) (*models.ComplaintWithDetails, error)
	Reply(ctx context.Context, id string, data *models.ReplyDto) (*models.ComplaintWithDetails, error)
	AddLogHistory(ctx context.Context, id string, action enums.ChangeActionType, from any, to any) (*models.ComplaintWithDetails, error)
	GetAssignedComplaints(ctx context.Context, supervisorId string, skip, limit int64, query *query.Conditions) (*models.ComplaintPagination, error)
	GetParentComplaints(ctx context.Context, parentUserId primitive.ObjectID, skip, limit int64, query *query.Conditions) (*models.ComplaintPagination, error)
}

type complaintSvcs struct {
	repo           repo.ComplaintRepo
	supervisorRepo db.MainRepo[contact_models.Supervisor]
	withtxn        *db.WithTxn
	sortingsvcs    sorting.SortingSvcs
	parentChecker  shared.ParentChecker
	messageSvcs    messaging.MessageSvcs
}

func NewComplaintSvcs(i *do.Injector) (ComplaintSvcs, error) {
	return &complaintSvcs{
		repo:           do.MustInvoke[repo.ComplaintRepo](i),
		supervisorRepo: db.NewRepo[contact_models.Supervisor](i),
		withtxn:        do.MustInvoke[*db.WithTxn](i),
		sortingsvcs:    do.MustInvoke[sorting.SortingSvcs](i),
		parentChecker:  do.MustInvoke[shared.ParentChecker](i),
		messageSvcs:    do.MustInvoke[messaging.MessageSvcs](i),
	}, nil
}

var createdByLookup = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "users",
			"localField":   "createdBy",
			"foreignField": "_id",
			"as":           "createdByUser",
		},
	},
	{
		"$addFields": bson.M{
			"createdByName": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"user": bson.M{"$arrayElemAt": []interface{}{"$createdByUser", 0}},
					},
					"in": bson.M{
						"$ifNull": []interface{}{
							bson.M{"$concat": []interface{}{
								bson.M{"$ifNull": []interface{}{"$$user.firstName", ""}},
								" ",
								bson.M{"$ifNull": []interface{}{"$$user.lastName", ""}},
							}},
							"",
						},
					},
				},
			},
			"createdByPicture": bson.M{
				"$let": bson.M{
					"vars": bson.M{
						"user": bson.M{"$arrayElemAt": []interface{}{"$createdByUser", 0}},
					},
					"in": bson.M{"$ifNull": []interface{}{"$$user.picture", ""}},
				},
			},
		},
	},
	{
		"$project": bson.M{
			"createdByUser": 0,
		},
	},
}

var assignToLookup = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "users",
			"localField":   "assignTo",
			"foreignField": "_id",
			"as":           "assignToUser",
		},
	},
	{
		"$addFields": bson.M{
			"assignToObj": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$eq": []interface{}{"$assignTo", primitive.NilObjectID}},
					"then": nil,
					"else": bson.M{
						"$let": bson.M{
							"vars": bson.M{
								"user": bson.M{"$arrayElemAt": []interface{}{"$assignToUser", 0}},
							},
							"in": bson.M{
								"_id":       "$$user._id",
								"firstName": bson.M{"$ifNull": []interface{}{"$$user.firstName", ""}},
								"lastName":  bson.M{"$ifNull": []interface{}{"$$user.lastName", ""}},
							},
						},
					},
				},
			},
		},
	},
	{
		"$project": bson.M{
			"assignToUser": 0,
		},
	},
}

var repliesLookup = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "users",
			"localField":   "replies.createdBy",
			"foreignField": "_id",
			"as":           "repliesUsers",
		},
	},
	{
		"$addFields": bson.M{
			"replies": bson.M{
				"$map": bson.M{
					"input": "$replies",
					"as":    "reply",
					"in": bson.M{
						"$mergeObjects": []interface{}{
							"$$reply",
							bson.M{
								"$let": bson.M{
									"vars": bson.M{
										"user": bson.M{
											"$arrayElemAt": []interface{}{
												"$repliesUsers",
												bson.M{
													"$indexOfArray": []interface{}{
														"$repliesUsers._id",
														"$$reply.createdBy",
													},
												},
											},
										},
									},
									"in": bson.M{
										"createdByName": bson.M{
											"$ifNull": []interface{}{
												bson.M{"$concat": []interface{}{
													bson.M{"$ifNull": []interface{}{"$$user.firstName", ""}},
													" ",
													bson.M{"$ifNull": []interface{}{"$$user.lastName", ""}},
												}},
												"",
											},
										},
										"createdByPicture": bson.M{"$ifNull": []interface{}{"$$user.picture", ""}},
									},
								},
							},
						},
					},
				},
			},
		},
	},
}

var complaintTypeLookup = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "complaintTypes",
			"localField":   "complaintType",
			"foreignField": "_id",
			"as":           "complaintTypeObj",
			"pipeline": func() []bson.M {
				p := []bson.M{
					{"$match": bson.M{"trash": false}}, // Filter out deleted complaint types
				}
				// Enrich with ownerObj using the shared ownerLookup pipeline
				p = append(p, ownerLookup...)
				return p
			}(),
		},
	},
	{
		"$unwind": bson.M{
			"path":                       "$complaintTypeObj",
			"preserveNullAndEmptyArrays": true,
		},
	},
}

var relatedStudentLookup = []bson.M{
	{
		"$lookup": bson.M{
			"from":         "users",
			"localField":   "relatedStudent",
			"foreignField": "_id",
			"as":           "relatedStudentUser",
		},
	},
	{
		"$addFields": bson.M{
			"studentObj": bson.M{
				"$cond": bson.M{
					"if":   bson.M{"$eq": []interface{}{"$relatedStudent", primitive.NilObjectID}},
					"then": nil,
						"else": bson.M{
						"$let": bson.M{
							"vars": bson.M{
								"user": bson.M{"$arrayElemAt": []interface{}{"$relatedStudentUser", 0}},
							},
							"in": bson.M{
								"_id":       "$$user._id",
								"firstName": bson.M{"$ifNull": []interface{}{"$$user.firstName", ""}},
								"lastName":  bson.M{"$ifNull": []interface{}{"$$user.lastName", ""}},
							},
						},
					},
				},
			},
		},
	},
	{
		"$project": bson.M{
			"relatedStudentUser": 0,
		},
	},
}

func (r *complaintSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.ComplaintPagination, error) {
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

	pipeline = append(pipeline, assignToLookup...)
	pipeline = append(pipeline, createdByLookup...)
	pipeline = append(pipeline, repliesLookup...)
	pipeline = append(pipeline, complaintTypeLookup...)
	pipeline = append(pipeline, relatedStudentLookup...)
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.ComplaintWithDetails
	errAg := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ComplaintPagination{
		Complaints: result,
		Pagination: pagination,
	}, nil
}
func (r *complaintSvcs) GetOne(ctx context.Context, id string) (*models.ComplaintWithDetails, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
	}

	pipeline = append(pipeline, assignToLookup...)
	pipeline = append(pipeline, createdByLookup...)
	pipeline = append(pipeline, repliesLookup...)
	pipeline = append(pipeline, complaintTypeLookup...)
	pipeline = append(pipeline, relatedStudentLookup...)

	var result []models.ComplaintWithDetails
	err = r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if err != nil {
		return nil, err
	}

	if len(result) == 0 {
		return nil, errors.New("complaint not found")
	}

	return &result[0], nil
}
func (r *complaintSvcs) Add(ctx context.Context, data *models.ComplaintDto) (*models.ComplaintWithDetails, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get creator name from context
	creatorName := cfg.User.UserData.FirstName + " " + cfg.User.UserData.LastName

	relation := &models.Complaint{
		Id:           primitive.NewObjectID(),
		ComplaintDto: *data,
		ComplaintStatusDto: models.ComplaintStatusDto{
			Status: "open",
		},
		LogHistory:    []models.LogHistory{},
		Replies:       []models.Reply{},
		CreatedAt:     time.Now(),
		CreatedBy:     cfg.User.Id,
		CreatedByName: creatorName,
		UpdatedAt:     time.Now(),
		UpdatedBy:     cfg.User.Id,
	}
	if err := r.repo.Add(ctx, relation); err != nil {
		return nil, err
	}

	// Fetch the complete record with populated details
	return r.GetOne(ctx, relation.Id.Hex())
}
func (r *complaintSvcs) Update(ctx context.Context, id string, data *models.ComplaintDto) (*models.ComplaintWithDetails, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the existing complaint
	oldComplaint, err := r.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create a copy of the old complaint and update only the DTO fields
	updatedComplaint := *oldComplaint
	updatedComplaint.ComplaintDto = *data
	updatedComplaint.UpdatedAt = time.Now()
	updatedComplaint.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedComplaint}

	_, err = r.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("complaint not found")
		}
		return nil, err
	}

	// Fetch the complete record with populated details
	return r.GetOne(ctx, id)
}
func (s *complaintSvcs) Delete(ctx context.Context, id string) error {
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
func (r *complaintSvcs) ChangeStatus(ctx context.Context, id string, data *models.ComplaintStatusDto) (*models.ComplaintWithDetails, error) {

	result, err := r.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		_id, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return nil, err
		}

		oldComplaint, err := r.GetOne(ctx, id)
		if err != nil {
			return nil, err
		}

		newRelation := *oldComplaint
		newRelation.ComplaintStatusDto = *data
		newRelation.ComplaintStatusDto.Status = data.Status
		newRelation.ComplaintStatusDto.Reason = data.Reason
		newRelation.UpdatedAt = time.Now()
		newRelation.UpdatedBy = cfg.User.Id

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": newRelation}

		_, err = r.repo.Patch(ctx, filter, update)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("complaint not found")
			}
			return nil, err
		}
		_, err = r.AddLogHistory(ctx, id, enums.ChangeActionStatus, oldComplaint.Status, data.Status)
		if err != nil {
			return nil, err
		}

		// Fetch the complete record with populated details
		return r.GetOne(ctx, id)
	})
	if err != nil {
		return nil, err
	}
	complaint := result.(*models.ComplaintWithDetails)
	_ = r.messageSvcs.SendNotification(ctx, &messagingmodels.Notification{
		ServiceName: "academy",
		Title:       "Complaint status updated",
		Content:     "Your complaint status has been updated.",
		Recipient:   complaint.CreatedBy.Hex(),
	})
	return complaint, nil
}
func (r *complaintSvcs) AssignTo(ctx context.Context, id string, assignToId string, data *models.AssignToDto) (*models.ComplaintWithDetails, error) {

	result, err := r.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {

		_id, err := primitive.ObjectIDFromHex(id)
		if err != nil {
			return nil, err
		}
		cfg, err := util.GetReqAppCfg(ctx)
		if err != nil {
			return nil, err
		}
		assignToObjId, err := primitive.ObjectIDFromHex(assignToId)
		if err != nil {
			return nil, err
		}

		// Validate that the user is a supervisor
		_, err = r.supervisorRepo.GetByFilter(ctx, bson.M{"_id": assignToObjId, "trash": false})
		if err != nil {
			return nil, errors.New("assigned user is not a supervisor")
		}

		oldComplaint, err := r.GetOne(ctx, id)
		if err != nil {
			return nil, err
		}

		newRelation := *oldComplaint
		newRelation.AssignTo = assignToObjId
		newRelation.SupervisorNote = data.Note
		newRelation.UpdatedAt = time.Now()
		newRelation.UpdatedBy = cfg.User.Id

		filter := bson.M{"_id": _id}
		update := bson.M{"$set": newRelation}

		_, err = r.repo.Patch(ctx, filter, update)
		if err != nil {
			if errors.Is(err, mongo.ErrNoDocuments) {
				return nil, errors.New("complaint not found")
			}
			return nil, err
		}
		_, err = r.AddLogHistory(ctx, id, enums.ChangeActionAssignTo, oldComplaint.AssignTo, assignToObjId)
		if err != nil {
			return nil, err
		}

		// Fetch the complete record with populated details
		return r.GetOne(ctx, id)

	})
	if err != nil {

		return nil, err
	}

	return result.(*models.ComplaintWithDetails), nil
}
func (r *complaintSvcs) Reply(ctx context.Context, id string, data *models.ReplyDto) (*models.ComplaintWithDetails, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the existing complaint to check permissions
	oldComplaint, err := r.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Permission check: Creator or user with CapabilityReplyToComplaints
	hasCap, _ := util.HasCapability(ctx, CapabilityReplyToComplaints)
	if oldComplaint.CreatedBy != cfg.User.Id && !hasCap {
		return nil, errors.New("you are not authorized to reply to this complaint")
	}

	// Get replier name from context
	creatorName := cfg.User.UserData.FirstName + " " + cfg.User.UserData.LastName

	reply := models.Reply{
		Id:            primitive.NewObjectID(),
		Message:       data.Message,
		CreatedAt:     time.Now(),
		CreatedBy:     cfg.User.Id,
		CreatedByName: creatorName,
	}

	result, err := r.withtxn.Exec(ctx, func(ctx mongo.SessionContext) (any, error) {
		filter := bson.M{"_id": _id}
		update := bson.M{
			"$push": bson.M{"replies": reply},
			"$set": bson.M{
				"updatedAt": time.Now(),
				"updatedBy": cfg.User.Id,
			},
		}

		_, err = r.repo.Patch(ctx, filter, update)
		if err != nil {
			return nil, err
		}

		_, err = r.AddLogHistory(ctx, id, enums.ChangeActionReply, nil, reply.Id)
		if err != nil {
			return nil, err
		}

		return r.GetOne(ctx, id)
	})

	if err != nil {
		return nil, err
	}

	complaint := result.(*models.ComplaintWithDetails)
	// Notify complaint creator that there is a new reply (only if replier is not the creator)
	if complaint.CreatedBy != cfg.User.Id {
		_ = r.messageSvcs.SendNotification(ctx, &messagingmodels.Notification{
			ServiceName: "academy",
			Title:       "New reply on your complaint",
			Content:     "Your complaint has received a new reply.",
			Recipient:   complaint.CreatedBy.Hex(),
		})
	}
	return complaint, nil
}
func (r *complaintSvcs) AddLogHistory(ctx context.Context, id string, action enums.ChangeActionType, from any, to any) (*models.ComplaintWithDetails, error) {

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	oldComplaint, err := r.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	newLogHistory := oldComplaint.LogHistory

	newLog := models.LogHistory{
		ChangeActionType: action,
		From:             from,
		To:               to,
	}

	newLogHistory = append(newLogHistory, newLog)

	newRelation := *oldComplaint
	newRelation.LogHistory = newLogHistory

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": newRelation}

	_, err = r.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("Complaint not found")
		}
		return nil, err
	}

	// Fetch the complete record with populated details
	return r.GetOne(ctx, id)
}

func (r *complaintSvcs) GetStats(ctx context.Context, period string, fromStr, toStr string, statusFilter string) (*models.ComplaintStats, error) {
	// Calculate date range based on explicit from/to dates (if provided) or period
	now := time.Now()
	var startDate time.Time
	var endDate time.Time

	// Build optional status filter for match stages
	statusMatch := bson.M{}
	if statusFilter != "" {
		statusMatch["status"] = enums.ComplaintStatus(statusFilter)
	}

	// 1) If from/to are provided, they take precedence
	if fromStr != "" || toStr != "" {
		if fromStr != "" {
			if t, err := time.Parse("2006-01-02", fromStr); err == nil {
				// Parse returns UTC midnight (start of day)
				startDate = t
			}
		}
		if toStr != "" {
			if t, err := time.Parse("2006-01-02", toStr); err == nil {
				// End of day (23:59:59.999) in UTC to include the entire 'to' day
				endDate = time.Date(t.Year(), t.Month(), t.Day(), 23, 59, 59, 999999999, time.UTC)
			}
		}
	} else if period != "" {
		// 2) Otherwise, fall back to period logic (week/month/year) relative to 'now'
		switch period {
		case "week":
			startDate = now.AddDate(0, 0, -7) // Last 7 days
		case "month":
			startDate = now.AddDate(0, -1, 0) // Last month
		case "year":
			startDate = now.AddDate(-1, 0, 0) // Last year
		}
	}

	// Build base match filter
	dateFilter := bson.M{}
	if !startDate.IsZero() || !endDate.IsZero() {
		rangeCond := bson.M{}
		if !startDate.IsZero() {
			rangeCond["$gte"] = startDate
		}
		if !endDate.IsZero() {
			rangeCond["$lte"] = endDate
		}
		dateFilter = bson.M{"createdAt": rangeCond}
	}

	// Get total open tickets
	openMatch := bson.M{"trash": false, "status": enums.ComplaintStatusOpen}
	if len(statusMatch) > 0 {
		openMatch = bson.M{"trash": false, "status": statusMatch["status"]}
	}
	totalOpenPipeline := []bson.M{{"$match": openMatch}}
	if len(dateFilter) > 0 {
		totalOpenPipeline = append(totalOpenPipeline, bson.M{"$match": dateFilter})
	}
	totalOpenPipeline = append(totalOpenPipeline, bson.M{"$count": "count"})

	var openResult []bson.M
	err := r.repo.Aggregate(ctx, totalOpenPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &openResult)
	})
	if err != nil {
		return nil, err
	}

	totalOpenTickets := int64(0)
	if len(openResult) > 0 {
		if count, ok := openResult[0]["count"].(int32); ok {
			totalOpenTickets = int64(count)
		}
	}

	// Get total closed tickets
	closedMatch := bson.M{"trash": false, "status": enums.ComplaintStatusResolved}
	if len(statusMatch) > 0 {
		closedMatch = bson.M{"trash": false, "status": statusMatch["status"]}
	}
	totalClosedPipeline := []bson.M{{"$match": closedMatch}}
	if len(dateFilter) > 0 {
		totalClosedPipeline = append(totalClosedPipeline, bson.M{"$match": dateFilter})
	}
	totalClosedPipeline = append(totalClosedPipeline, bson.M{"$count": "count"})

	var closedResult []bson.M
	err = r.repo.Aggregate(ctx, totalClosedPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &closedResult)
	})
	if err != nil {
		return nil, err
	}

	totalClosedTickets := int64(0)
	if len(closedResult) > 0 {
		if count, ok := closedResult[0]["count"].(int32); ok {
			totalClosedTickets = int64(count)
		}
	}

	// Get deleted complaints
	deletedMatch := bson.M{"trash": true}
	if len(statusMatch) > 0 {
		deletedMatch["status"] = statusMatch["status"]
	}
	deletedPipeline := []bson.M{{"$match": deletedMatch}}
	if len(dateFilter) > 0 {
		deletedPipeline = append(deletedPipeline, bson.M{"$match": dateFilter})
	}
	deletedPipeline = append(deletedPipeline, bson.M{"$count": "count"})

	var deletedResult []bson.M
	err = r.repo.Aggregate(ctx, deletedPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &deletedResult)
	})
	if err != nil {
		return nil, err
	}

	deletedComplaints := int64(0)
	if len(deletedResult) > 0 {
		if count, ok := deletedResult[0]["count"].(int32); ok {
			deletedComplaints = int64(count)
		}
	}

	// Calculate average and max resolution time for resolved complaints
	resolutionMatch := bson.M{"trash": false, "status": enums.ComplaintStatusResolved}
	if len(statusMatch) > 0 {
		resolutionMatch["status"] = statusMatch["status"]
	}
	resolutionTimePipeline := []bson.M{{"$match": resolutionMatch}}
	if len(dateFilter) > 0 {
		resolutionTimePipeline = append(resolutionTimePipeline, bson.M{"$match": dateFilter})
	}
	resolutionTimePipeline = append(resolutionTimePipeline,
		bson.M{"$addFields": bson.M{
			"resolutionTimeHours": bson.M{
				"$divide": []interface{}{
					bson.M{"$subtract": []interface{}{"$updatedAt", "$createdAt"}},
					3600000, // milliseconds to hours
				},
			},
		}},
		bson.M{"$group": bson.M{
			"_id":     nil,
			"avgTime": bson.M{"$avg": "$resolutionTimeHours"},
			"maxTime": bson.M{"$max": "$resolutionTimeHours"},
		}},
	)

	var timeResult []bson.M
	err = r.repo.Aggregate(ctx, resolutionTimePipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &timeResult)
	})
	if err != nil {
		return nil, err
	}

	avgTime := "0h"
	maxTime := "0h"

	if len(timeResult) > 0 {
		if avg, ok := timeResult[0]["avgTime"].(float64); ok {
			avgTime = formatDuration(avg)
		}
		if max, ok := timeResult[0]["maxTime"].(float64); ok {
			maxTime = formatDuration(max)
		}
	}

	return &models.ComplaintStats{
		TicketsSummary: models.TicketsSummary{
			TotalOpenTickets:      totalOpenTickets,
			NumberOfClosedTickets: totalClosedTickets,
			DeletedComplaints:     deletedComplaints,
		},
		TicketsResolutionTime: models.TicketsResolutionTime{
			AverageTime: avgTime,
			MaxTime:     maxTime,
		},
	}, nil
}

// formatDuration converts hours (float64) to a human-readable format
func formatDuration(hours float64) string {
	if hours < 1 {
		minutes := int(hours * 60)
		return strconv.Itoa(minutes) + "m"
	} else if hours < 24 {
		return strconv.Itoa(int(hours)) + "h"
	} else {
		days := int(hours / 24)
		return strconv.Itoa(days) + "d"
	}
}

func (r *complaintSvcs) GetAssignedComplaints(ctx context.Context, supervisorId string, skip, limit int64, query *query.Conditions) (*models.ComplaintPagination, error) {
	_id, err := primitive.ObjectIDFromHex(supervisorId)
	if err != nil {
		return nil, err
	}

	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false, "assignTo": _id}},
		{"$match": filter},
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, assignToLookup...)
	pipeline = append(pipeline, createdByLookup...)
	pipeline = append(pipeline, repliesLookup...)
	pipeline = append(pipeline, complaintTypeLookup...)
	pipeline = append(pipeline, relatedStudentLookup...)
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.ComplaintWithDetails
	errAg := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ComplaintPagination{
		Complaints: result,
		Pagination: pagination,
	}, nil
}

func (r *complaintSvcs) GetParentComplaints(ctx context.Context, parentUserId primitive.ObjectID, skip, limit int64, query *query.Conditions) (*models.ComplaintPagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, errors.New("query not valid")
	}

	childrenIds, err := r.parentChecker.GetChildrenIds(ctx, parentUserId)
	if err != nil {
		return nil, err
	}

	// createdBy: parent or any of their children (student IDs = user IDs in this system)
	createdByIds := []primitive.ObjectID{parentUserId}
	createdByIds = append(createdByIds, childrenIds...)

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("error converting query to mongo")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false, "createdBy": bson.M{"$in": createdByIds}}},
		{"$match": filter},
	}

	count, err := r.repo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, assignToLookup...)
	pipeline = append(pipeline, createdByLookup...)
	pipeline = append(pipeline, repliesLookup...)
	pipeline = append(pipeline, complaintTypeLookup...)
	pipeline = append(pipeline, relatedStudentLookup...)
	pipeline = append(pipeline,
		bson.M{"$sort": bson.M{"_id": -1}},
		bson.M{"$skip": skip},
		bson.M{"$limit": limit},
	)

	var result []models.ComplaintWithDetails
	errAg := r.repo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	})
	if errAg != nil {
		return nil, errAg
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.ComplaintPagination{
		Complaints: result,
		Pagination: pagination,
	}, nil
}
