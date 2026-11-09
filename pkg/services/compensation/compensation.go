package compensation

import (
	"context"
	"errors"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/compensation/constant"
	"larsa-academy-microservice/pkg/services/compensation/models"
	contact_models "larsa-academy-microservice/pkg/services/contact/models"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/exchange"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type CompensationSvcs interface {
	AddTeacherPricing(ctx context.Context, teacherId string, dto *models.TeacherPricingCreateDto) (*models.TeacherPricing, error)
	UpdateTeacherPricing(ctx context.Context, teacherId, pricingId string, dto *models.TeacherPricingUpdateDto) (*models.TeacherPricing, error)
	DeleteTeacherPricing(ctx context.Context, teacherId, pricingId string) error
	GetTeacherPricings(ctx context.Context, teacherId string, skip, limit int64, cond *query.Conditions, isActive *bool) (*models.TeacherPricingPagination, error)
	Quote(ctx context.Context, teacherId, ref, refId string, durationMinutes float64) (*QuoteResponse, error)

	AddTeacherHours(ctx context.Context, teacherId string, dto *models.TeacherHoursCreateDto) (*models.TeacherHours, error)
	UpdateTeacherHours(ctx context.Context, teacherId, hoursId string, dto *models.TeacherHoursUpdateDto) (*models.TeacherHours, error)
	RejectTeacherHours(ctx context.Context, teacherId, hoursId, reason string) (*models.TeacherHours, error)
	GetTeacherHours(ctx context.Context, teacherId string, skip, limit int64, cond *query.Conditions, start, end *time.Time, status string) (*models.TeacherHoursPagination, error)

	GetCompensationBreakdown(ctx context.Context, cond *query.Conditions, start, end *time.Time, includeRejected bool, hideZeroRecorded bool) ([]models.TeacherCompensationSummary, error)
	GetCompensationSummary(ctx context.Context, cond *query.Conditions, start, end *time.Time, includeRejected bool) (*models.CompensationSummary, error)

	// Session integration methods
	RecordHoursFromSession(ctx context.Context, sessionId primitive.ObjectID, teacherId primitive.ObjectID, coachingProgramId primitive.ObjectID, occurredAt time.Time, durationMinutes float64) error
	RejectHoursBySessionId(ctx context.Context, sessionId primitive.ObjectID, reason string) error
}

type compensationSvcs struct {
	pricingRepo  db.MainRepo[models.TeacherPricing]
	hoursRepo    db.MainRepo[models.TeacherHours]
	teacherRepo  db.MainRepo[contact_models.Teacher]
	subjectRepo  db.MainRepo[content_models.Subject]
	courseRepo   db.MainRepo[content_models.Course]
	coachingRepo db.MainRepo[content_models.Coaching]
	withTxn      *db.WithTxn
	exchangeSvc  exchange.ExchangeService
}

type QuoteResponse struct {
	PricingId       primitive.ObjectID `json:"pricingId"`
	HourlyRate      float64            `json:"hourlyRate"`
	Currency        string             `json:"currency"`
	DurationMinutes float64            `json:"durationMinutes"`
	Amount          float64            `json:"amount"`
}

func NewCompensationSvcs(i *do.Injector) (CompensationSvcs, error) {
	return &compensationSvcs{
		pricingRepo:  db.NewRepo[models.TeacherPricing](i),
		hoursRepo:    db.NewRepo[models.TeacherHours](i),
		teacherRepo:  db.NewRepo[contact_models.Teacher](i),
		subjectRepo:  db.NewRepo[content_models.Subject](i),
		courseRepo:   db.NewRepo[content_models.Course](i),
		coachingRepo: db.NewRepo[content_models.Coaching](i),
		withTxn:      do.MustInvoke[*db.WithTxn](i),
		exchangeSvc:  do.MustInvoke[exchange.ExchangeService](i),
	}, nil
}

func (s *compensationSvcs) AddTeacherPricing(ctx context.Context, teacherId string, dto *models.TeacherPricingCreateDto) (*models.TeacherPricing, error) {
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, constant.ErrInvalidTeacherId
	}

	subjectOID, err := primitive.ObjectIDFromHex(dto.Criteria.SubjectId)
	if err != nil {
		return nil, constant.ErrInvalidRefId
	}

	if err := s.validateCurrency(ctx, dto.Currency); err != nil {
		return nil, err
	}

	now := time.Now().UTC()
	performedBy := getPerformedBy(ctx)
	audit := appendAudit(nil, "create", performedBy, "", nil, map[string]any{
		"hourlyRate": dto.HourlyRate,
		"currency":   dto.Currency,
		"criteria": map[string]any{
			"subjectId":  subjectOID,
			"schoolType": dto.Criteria.SchoolType,
			"country":    dto.Criteria.Country,
			"level":      dto.Criteria.Level,
		},
		"isActive": dto.IsActive,
	})

	pricing := &models.TeacherPricing{
		TeacherId: teacherOID,
		Criteria: models.PricingCriteria{
			SubjectId:  subjectOID,
			SchoolType: dto.Criteria.SchoolType,
			Country:    dto.Criteria.Country,
			Level:      dto.Criteria.Level,
		},
		HourlyRate: dto.HourlyRate,
		Currency:   dto.Currency,
		IsActive:   dto.IsActive,
		Notes:      dto.Notes,
		AuditLog:   audit,
		CreatedAt:  now,
		UpdatedAt:  now,
		CreatedBy:  performedBy,
		UpdatedBy:  performedBy,
		Trash:      false,
	}

	// Execute deactivation and creation in a transaction
	result, err := s.withTxn.Exec(ctx, func(sessionCtx mongo.SessionContext) (any, error) {
		if dto.IsActive {
			if err := s.deactivateExistingByCriteria(sessionCtx, teacherOID, pricing.Criteria); err != nil {
				return nil, err
			}
		}

		if err := s.pricingRepo.Add(sessionCtx, pricing); err != nil {
			if mongo.IsDuplicateKeyError(err) {
				return nil, constant.ErrActivePricingExists
			}
			return nil, err
		}

		return pricing, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.TeacherPricing), nil
}

func (s *compensationSvcs) UpdateTeacherPricing(ctx context.Context, teacherId, pricingId string, dto *models.TeacherPricingUpdateDto) (*models.TeacherPricing, error) {
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, constant.ErrInvalidTeacherId
	}
	pricingOID, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return nil, constant.ErrInvalidPricingId
	}

	existing, err := s.pricingRepo.GetByFilter(ctx, bson.M{
		"_id":       pricingOID,
		"teacherId": teacherOID,
		"trash":     false,
	})
	if err != nil {
		return nil, err
	}

	updates := bson.M{}
	old := map[string]any{}
	targetCriteria := existing.Criteria

	if dto.Criteria != nil {
		subjectOID, err := primitive.ObjectIDFromHex(dto.Criteria.SubjectId)
		if err != nil {
			return nil, constant.ErrInvalidRefId
		}
		newCriteria := models.PricingCriteria{
			SubjectId:  subjectOID,
			SchoolType: dto.Criteria.SchoolType,
			Country:    dto.Criteria.Country,
			Level:      dto.Criteria.Level,
		}
		if newCriteria != existing.Criteria {
			updates["criteria"] = newCriteria
			old["criteria"] = existing.Criteria
			targetCriteria = newCriteria
		}
	}
	if dto.HourlyRate != 0 && dto.HourlyRate != existing.HourlyRate {
		updates["hourlyRate"] = dto.HourlyRate
		old["hourlyRate"] = existing.HourlyRate
	}
	if dto.Currency != "" && dto.Currency != existing.Currency {
		if err := s.validateCurrency(ctx, dto.Currency); err != nil {
			return nil, err
		}
		updates["currency"] = dto.Currency
		old["currency"] = existing.Currency
	}
	if dto.IsActive != nil && *dto.IsActive != existing.IsActive {
		updates["isActive"] = *dto.IsActive
		old["isActive"] = existing.IsActive
	}
	if dto.Notes != "" && dto.Notes != existing.Notes {
		updates["notes"] = dto.Notes
		old["notes"] = existing.Notes
	}

	if len(updates) == 0 {
		return existing, nil
	}

	now := time.Now().UTC()
	performedBy := getPerformedBy(ctx)
	updates["updatedAt"] = now
	updates["updatedBy"] = performedBy

	newAudit := appendAudit(existing.AuditLog, "update", performedBy, "", old, updates)

	update := bson.M{
		"$set": updates,
		"$setOnInsert": bson.M{
			"createdAt": existing.CreatedAt,
		},
		"$push": bson.M{
			"auditLog": bson.M{"$each": []models.AuditEntry{newAudit[len(newAudit)-1]}},
		},
	}

	// Execute deactivation (if activating) and update in a transaction
	result, err := s.withTxn.Exec(ctx, func(sessionCtx mongo.SessionContext) (any, error) {
		if dto.IsActive != nil && *dto.IsActive && !existing.IsActive {
			if err := s.deactivateExistingByCriteria(sessionCtx, teacherOID, targetCriteria); err != nil {
				return nil, err
			}
		}

		updated, err := s.pricingRepo.Patch(sessionCtx, bson.M{"_id": pricingOID}, update)
		if err != nil {
			return nil, err
		}
		updated.AuditLog = newAudit
		return updated, nil
	})

	if err != nil {
		return nil, err
	}

	return result.(*models.TeacherPricing), nil
}

func (s *compensationSvcs) DeleteTeacherPricing(ctx context.Context, teacherId, pricingId string) error {
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return constant.ErrInvalidTeacherId
	}
	pricingOID, err := primitive.ObjectIDFromHex(pricingId)
	if err != nil {
		return constant.ErrInvalidPricingId
	}

	existing, err := s.pricingRepo.GetByFilter(ctx, bson.M{
		"_id":       pricingOID,
		"teacherId": teacherOID,
		"trash":     false,
	})
	if err != nil {
		return err
	}

	// Prevent deletion when any hours reference this pricing (historical or current).
	referenceCount, err := s.hoursRepo.Count(ctx, []bson.M{
		{"$match": bson.M{
			"pricingId": pricingOID,
			"trash":     false,
		}},
		{"$limit": 1},
	})
	if err != nil {
		return err
	}
	if referenceCount > 0 {
		return constant.ErrPricingInUse
	}

	now := time.Now().UTC()
	performedBy := getPerformedBy(ctx)
	audit := appendAudit(existing.AuditLog, "delete", performedBy, "", map[string]any{"trash": existing.Trash}, map[string]any{"trash": true})

	update := bson.M{
		"$set": bson.M{
			"trash":     true,
			"updatedAt": now,
			"updatedBy": performedBy,
		},
		"$push": bson.M{
			"auditLog": bson.M{"$each": []models.AuditEntry{audit[len(audit)-1]}},
		},
	}

	_, err = s.pricingRepo.Patch(ctx, bson.M{"_id": pricingOID}, update)
	return err
}

func (s *compensationSvcs) GetTeacherPricings(ctx context.Context, teacherId string, skip, limit int64, cond *query.Conditions, isActive *bool) (*models.TeacherPricingPagination, error) {
	filter := bson.M{"trash": false}

	if cond != nil {
		if err := cond.CheckValid(); err != nil {
			return nil, constant.ErrQueryNotValid
		}
		whitelist := map[string]bool{
			"teacherId":           true,
			"criteria.subjectId":  true,
			"criteria.schoolType": true,
			"criteria.country":    true,
			"criteria.level":      true,
			"isActive":            true,
			"currency":            true,
		}
		converted, err := cond.ConvertToMongo(query.WithWhitelistNames(whitelist))
		if err != nil {
			return nil, err
		}
		if len(converted) > 0 {
			filter = bson.M{"$and": []bson.M{filter, converted}}
		}
	}
	if isActive != nil {
		filter = bson.M{"$and": []bson.M{filter, {"isActive": *isActive}}}
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	count, err := s.pricingRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline,
		bson.M{"$lookup": bson.M{
			"from": "academyTeachers",
			"let":  bson.M{"teacherId": "$teacherId"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{"$and": []bson.M{
						{"$eq": []any{"$_id", "$$teacherId"}},
						{"$eq": []any{"$trash", false}},
					}},
				}},
				{"$project": bson.M{
					"_id":       1,
					"firstName": 1,
					"lastName":  1,
					"email":     "$accountSettings.email",
				}},
			},
			"as": "teacher",
		}},
		bson.M{"$unwind": bson.M{
			"path":                       "$teacher",
			"preserveNullAndEmptyArrays": true,
		}},
		bson.M{"$lookup": bson.M{
			"from": "academySubjects",
			"let":  bson.M{"subjectId": "$criteria.subjectId"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{"$and": []bson.M{
						{"$eq": []any{"$_id", "$$subjectId"}},
						{"$eq": []any{"$trash", false}},
					}},
				}},
				{"$project": bson.M{
					"_id":  1,
					"name": 1,
					"code": 1,
				}},
			},
			"as": "subject",
		}},
		bson.M{"$unwind": bson.M{
			"path":                       "$subject",
			"preserveNullAndEmptyArrays": true,
		}},
	)

	var pricings []models.TeacherPricingDetails
	if err := s.pricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &pricings)
	}); err != nil {
		return nil, err
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	return &models.TeacherPricingPagination{
		TeacherPricings: pricings,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *compensationSvcs) Quote(ctx context.Context, teacherId, ref, refId string, durationMinutes float64) (*QuoteResponse, error) {
	if durationMinutes <= 0 {
		return nil, constant.ErrInvalidDuration
	}
	teacherOID, refOID, err := parseTeacherAndRef(teacherId, refId)
	if err != nil {
		return nil, err
	}
	pricing, err := s.resolvePricing(ctx, teacherOID, ref, refOID)
	if err != nil {
		return nil, err
	}

	// Convert to default currency if needed (for old data)
	defaultCurrency, err := s.getDefaultCurrency(ctx)
	if err != nil {
		return nil, err
	}

	hourlyRate := pricing.HourlyRate
	if pricing.Currency != defaultCurrency {
		convertedRate, err := s.exchangeSvc.Convert(ctx, hourlyRate, pricing.Currency, defaultCurrency)
		if err != nil {
			return nil, err
		}
		hourlyRate = roundMoney(convertedRate)
	}

	amount := roundMoney(hourlyRate * (durationMinutes / 60))
	return &QuoteResponse{
		PricingId:       pricing.Id,
		HourlyRate:      hourlyRate,
		Currency:        defaultCurrency,
		DurationMinutes: durationMinutes,
		Amount:          amount,
	}, nil
}

func (s *compensationSvcs) AddTeacherHours(ctx context.Context, teacherId string, dto *models.TeacherHoursCreateDto) (*models.TeacherHours, error) {
	teacherOID, refOID, err := parseTeacherAndRef(teacherId, dto.RefId)
	if err != nil {
		return nil, err
	}
	var sessionOID *primitive.ObjectID
	if dto.SessionId != "" {
		oid, err := primitive.ObjectIDFromHex(dto.SessionId)
		if err != nil {
			return nil, constant.ErrInvalidSessionId
		}
		sessionOID = &oid
	}

	pricing, err := s.resolvePricing(ctx, teacherOID, dto.Ref, refOID)
	if err != nil {
		return nil, err
	}

	amount := roundMoney(pricing.HourlyRate * (dto.DurationMinutes / 60))
	now := time.Now().UTC()
	performedBy := getPerformedBy(ctx)
	audit := appendAudit(nil, "create", performedBy, "", nil, map[string]any{
		"durationMinutes": dto.DurationMinutes,
		"amount":          amount,
		"pricingId":       pricing.Id,
	})

	record := &models.TeacherHours{
		TeacherId:       teacherOID,
		Ref:             dto.Ref,
		RefId:           refOID,
		SessionId:       sessionOID,
		OccurredAt:      dto.OccurredAt,
		DurationMinutes: dto.DurationMinutes,
		HourlyRate:      pricing.HourlyRate,
		Currency:        pricing.Currency,
		PricingId:       pricing.Id,
		Amount:          amount,
		Status:          models.HoursStatusRecorded,
		RecordedBy:      performedBy,
		Notes:           dto.Notes,
		AuditLog:        audit,
		CreatedAt:       now,
		UpdatedAt:       now,
		Trash:           false,
	}

	if err := s.hoursRepo.Add(ctx, record); err != nil {
		return nil, err
	}
	return record, nil
}

func (s *compensationSvcs) UpdateTeacherHours(ctx context.Context, teacherId, hoursId string, dto *models.TeacherHoursUpdateDto) (*models.TeacherHours, error) {
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, constant.ErrInvalidTeacherId
	}
	hoursOID, err := primitive.ObjectIDFromHex(hoursId)
	if err != nil {
		return nil, constant.ErrInvalidHoursId
	}

	existing, err := s.hoursRepo.GetByFilter(ctx, bson.M{
		"_id":       hoursOID,
		"teacherId": teacherOID,
		"trash":     false,
	})
	if err != nil {
		return nil, err
	}
	if existing.Status == models.HoursStatusRejected {
		return nil, constant.ErrCannotEditRejectedHours
	}

	updates := bson.M{}
	old := map[string]any{}
	if dto.OccurredAt != nil && !dto.OccurredAt.Equal(existing.OccurredAt) {
		updates["occurredAt"] = *dto.OccurredAt
		old["occurredAt"] = existing.OccurredAt
	}
	if dto.DurationMinutes != nil && *dto.DurationMinutes != existing.DurationMinutes {
		updates["durationMinutes"] = *dto.DurationMinutes
		old["durationMinutes"] = existing.DurationMinutes
		newAmount := roundMoney(existing.HourlyRate * (*dto.DurationMinutes / 60))
		updates["amount"] = newAmount
		old["amount"] = existing.Amount
	}
	if dto.Notes != nil && *dto.Notes != existing.Notes {
		updates["notes"] = *dto.Notes
		old["notes"] = existing.Notes
	}

	if len(updates) == 0 {
		return existing, nil
	}

	now := time.Now().UTC()
	performedBy := getPerformedBy(ctx)
	updates["updatedAt"] = now
	audit := appendAudit(existing.AuditLog, "edit", performedBy, "", old, updates)

	update := bson.M{
		"$set": updates,
		"$push": bson.M{
			"auditLog": bson.M{"$each": []models.AuditEntry{audit[len(audit)-1]}},
		},
	}

	updated, err := s.hoursRepo.Patch(ctx, bson.M{"_id": hoursOID}, update)
	if err != nil {
		return nil, err
	}
	updated.AuditLog = audit
	return updated, nil
}

func (s *compensationSvcs) RejectTeacherHours(ctx context.Context, teacherId, hoursId, reason string) (*models.TeacherHours, error) {
	if reason == "" {
		return nil, constant.ErrReasonRequired
	}
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, constant.ErrInvalidTeacherId
	}
	hoursOID, err := primitive.ObjectIDFromHex(hoursId)
	if err != nil {
		return nil, constant.ErrInvalidHoursId
	}

	existing, err := s.hoursRepo.GetByFilter(ctx, bson.M{
		"_id":       hoursOID,
		"teacherId": teacherOID,
		"trash":     false,
	})
	if err != nil {
		return nil, err
	}

	if existing.Status == models.HoursStatusRejected {
		return existing, nil
	}

	performedBy := getPerformedBy(ctx)
	now := time.Now().UTC()

	audit := appendAudit(existing.AuditLog, "reject", performedBy, reason, map[string]any{"status": existing.Status}, map[string]any{"status": models.HoursStatusRejected})

	update := bson.M{
		"$set": bson.M{
			"status":    models.HoursStatusRejected,
			"updatedAt": now,
		},
		"$push": bson.M{
			"auditLog": bson.M{"$each": []models.AuditEntry{audit[len(audit)-1]}},
		},
	}

	updated, err := s.hoursRepo.Patch(ctx, bson.M{"_id": hoursOID}, update)
	if err != nil {
		return nil, err
	}
	updated.AuditLog = audit
	return updated, nil
}

func (s *compensationSvcs) GetTeacherHours(ctx context.Context, teacherId string, skip, limit int64, cond *query.Conditions, start, end *time.Time, status string) (*models.TeacherHoursPagination, error) {
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, constant.ErrInvalidTeacherId
	}
	filter := bson.M{
		"teacherId": teacherOID,
		"trash":     false,
	}
	if cond != nil {
		if err := cond.CheckValid(); err != nil {
			return nil, constant.ErrQueryNotValid
		}
		whitelist := map[string]bool{
			"ref":       true,
			"refId":     true,
			"status":    true,
			"currency":  true,
			"pricingId": true,
			"sessionId": true,
		}
		converted, err := cond.ConvertToMongo(query.WithWhitelistNames(whitelist))
		if err != nil {
			return nil, err
		}
		filter = bson.M{"$and": []bson.M{filter, converted}}
	}
	if start != nil || end != nil {
		rangeFilter := bson.M{}
		if start != nil {
			rangeFilter["$gte"] = *start
		}
		if end != nil {
			rangeFilter["$lte"] = *end
		}
		filter = bson.M{"$and": []bson.M{filter, {"occurredAt": rangeFilter}}}
	}
	if status != "" {
		filter = bson.M{"$and": []bson.M{filter, {"status": status}}}
	}

	pipeline := []bson.M{{"$match": filter}}
	count, err := s.hoursRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"occurredAt": -1, "_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	var hours []models.TeacherHours
	if err := s.hoursRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &hours)
	}); err != nil {
		return nil, err
	}

	// Convert old data to default currency
	defaultCurrency, err := s.getDefaultCurrency(ctx)
	if err != nil {
		return nil, err
	}
	for i := range hours {
		if hours[i].Currency != defaultCurrency {
			convertedAmount, err := s.exchangeSvc.Convert(ctx, hours[i].Amount, hours[i].Currency, defaultCurrency)
			if err != nil {
				return nil, err
			}
			convertedRate, err := s.exchangeSvc.Convert(ctx, hours[i].HourlyRate, hours[i].Currency, defaultCurrency)
			if err != nil {
				return nil, err
			}
			hours[i].Amount = roundMoney(convertedAmount)
			hours[i].HourlyRate = roundMoney(convertedRate)
			hours[i].Currency = defaultCurrency
		}
	}

	totalPages := math.Ceil(float64(count) / float64(limit))
	return &models.TeacherHoursPagination{
		TeacherHours: hours,
		Pagination: types.Pagination{
			TotalPages: totalPages,
			PerPage:    limit,
			TotalCount: count,
		},
	}, nil
}

func (s *compensationSvcs) GetCompensationBreakdown(ctx context.Context, cond *query.Conditions, start, end *time.Time, includeRejected bool, hideZeroRecorded bool) ([]models.TeacherCompensationSummary, error) {
	// Build filter for pricing collection
	pricingFilter := bson.M{
		"trash": false,
	}

	// Build hours filter for the lookup
	hoursFilter := bson.M{
		"trash": false,
	}

	// Apply conditions - split between pricing and hours filters
	if cond != nil {
		if err := cond.CheckValid(); err != nil {
			return nil, constant.ErrQueryNotValid
		}
		// Conditions that apply to pricing
		pricingWhitelist := map[string]bool{
			"teacherId":           true,
			"criteria.subjectId":  true,
			"criteria.schoolType": true,
			"criteria.country":    true,
			"criteria.level":      true,
			"currency":            true,
		}
		// Conditions that apply to hours
		hoursWhitelist := map[string]bool{
			"status":    true,
			"teacherId": true,
		}

		// Apply pricing conditions
		pricingConverted, err := cond.ConvertToMongo(query.WithWhitelistNames(pricingWhitelist))
		if err != nil {
			return nil, err
		}
		if len(pricingConverted) > 0 {
			pricingFilter = bson.M{"$and": []bson.M{pricingFilter, pricingConverted}}
		}

		// Apply hours conditions
		hoursConverted, err := cond.ConvertToMongo(query.WithWhitelistNames(hoursWhitelist))
		if err != nil {
			return nil, err
		}
		if len(hoursConverted) > 0 {
			hoursFilter = bson.M{"$and": []bson.M{hoursFilter, hoursConverted}}
		}
	}

	// Apply status filter to hours
	if !includeRejected {
		hoursFilter["status"] = models.HoursStatusRecorded
	} else {
		hoursFilter["status"] = bson.M{"$in": []string{models.HoursStatusRecorded, models.HoursStatusRejected}}
	}

	// Apply date range filter to hours
	if start != nil || end != nil {
		rangeFilter := bson.M{}
		if start != nil {
			rangeFilter["$gte"] = *start
		}
		if end != nil {
			rangeFilter["$lte"] = *end
		}
		hoursFilter["occurredAt"] = rangeFilter
	}

	// Start from TeacherPricing and lookup TeacherHours
	pipeline := []bson.M{
		{"$match": pricingFilter},
		{"$lookup": bson.M{
			"from": "academyTeacherHours",
			"let":  bson.M{"pricingId": "$_id", "teacherId": "$teacherId"},
			"pipeline": []bson.M{
				{"$match": bson.M{
					"$expr": bson.M{
						"$and": []bson.M{
							{"$eq": []any{"$pricingId", "$$pricingId"}},
							{"$eq": []any{"$teacherId", "$$teacherId"}},
						},
					},
				}},
				{"$match": hoursFilter},
			},
			"as": "hours",
		}},
		{"$unwind": bson.M{
			"path":                       "$hours",
			"preserveNullAndEmptyArrays": true,
		}},
		{"$group": bson.M{
			"_id": bson.M{
				"teacherId":  "$teacherId",
				"pricingId":  "$_id",
				"criteria":   "$criteria",
				"currency":   "$currency",
				"hourlyRate": "$hourlyRate",
				"isActive":   "$isActive",
			},
			"recordedMinutes": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{
							"$and": []bson.M{
								{"$ne": []any{"$hours", nil}},
								{"$eq": []any{"$hours.status", models.HoursStatusRecorded}},
							},
						},
						"$hours.durationMinutes",
						0,
					},
				},
			},
			"recordedAmount": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{
							"$and": []bson.M{
								{"$ne": []any{"$hours", nil}},
								{"$eq": []any{"$hours.status", models.HoursStatusRecorded}},
							},
						},
						"$hours.amount",
						0,
					},
				},
			},
			"rejectedMinutes": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{
							"$and": []bson.M{
								{"$ne": []any{"$hours", nil}},
								{"$eq": []any{"$hours.status", models.HoursStatusRejected}},
							},
						},
						"$hours.durationMinutes",
						0,
					},
				},
			},
			"rejectedAmount": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{
							"$and": []bson.M{
								{"$ne": []any{"$hours", nil}},
								{"$eq": []any{"$hours.status", models.HoursStatusRejected}},
							},
						},
						"$hours.amount",
						0,
					},
				},
			},
			"recordCount": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{
							"$and": []bson.M{
								{"$ne": []any{"$hours", nil}},
								{"$eq": []any{"$hours.status", models.HoursStatusRecorded}},
							},
						},
						1,
						0,
					},
				},
			},
			"rejectedCount": bson.M{
				"$sum": bson.M{
					"$cond": []any{
						bson.M{
							"$and": []bson.M{
								{"$ne": []any{"$hours", nil}},
								{"$eq": []any{"$hours.status", models.HoursStatusRejected}},
							},
						},
						1,
						0,
					},
				},
			},
		}},
	}

	if hideZeroRecorded {
		pipeline = append(pipeline, bson.M{"$match": bson.M{"recordCount": bson.M{"$gt": 0}}})
	}

	pipeline = append(pipeline, bson.M{
		"$project": bson.M{
			"teacherId":       "$_id.teacherId",
			"pricingId":       "$_id.pricingId",
			"criteria":        "$_id.criteria",
			"currency":        "$_id.currency",
			"hourlyRate":      "$_id.hourlyRate",
			"isActive":        "$_id.isActive",
			"recordedMinutes": 1,
			"recordedAmount":  1,
			"rejectedMinutes": 1,
			"rejectedAmount":  1,
			"recordCount":     1,
			"rejectedCount":   1,
		},
	})

	type aggRow struct {
		TeacherId       primitive.ObjectID     `bson:"teacherId"`
		PricingId       primitive.ObjectID     `bson:"pricingId"`
		Criteria        models.PricingCriteria `bson:"criteria"`
		Currency        string                 `bson:"currency"`
		HourlyRate      float64                `bson:"hourlyRate"`
		IsActive        bool                   `bson:"isActive"`
		RecordedMinutes float64                `bson:"recordedMinutes"`
		RecordedAmount  float64                `bson:"recordedAmount"`
		RejectedMinutes float64                `bson:"rejectedMinutes"`
		RejectedAmount  float64                `bson:"rejectedAmount"`
		RecordCount     int                    `bson:"recordCount"`
		RejectedCount   int                    `bson:"rejectedCount"`
	}

	var rows []aggRow
	if err := s.pricingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &rows)
	}); err != nil {
		return nil, err
	}

	// Get default currency for conversion
	defaultCurrency, err := s.getDefaultCurrency(ctx)
	if err != nil {
		return nil, err
	}

	teacherMap := map[primitive.ObjectID]*models.TeacherCompensationSummary{}
	subjectIds := []primitive.ObjectID{}

	for _, row := range rows {
		teacherSummary, ok := teacherMap[row.TeacherId]
		if !ok {
			teacherSummary = &models.TeacherCompensationSummary{
				TeacherId:             row.TeacherId,
				TotalAmountByCurrency: map[string]float64{},
				PricingSummaries:      []models.PricingSummary{},
				StatusBreakdown:       map[string]int{},
			}
			teacherMap[row.TeacherId] = teacherSummary
		}

		// Convert amounts to default currency if needed (for old data)
		recordedAmount := row.RecordedAmount
		rejectedAmount := row.RejectedAmount
		hourlyRate := row.HourlyRate
		currency := row.Currency

		if row.Currency != defaultCurrency {
			convertedRecorded, err := s.exchangeSvc.Convert(ctx, row.RecordedAmount, row.Currency, defaultCurrency)
			if err != nil {
				return nil, err
			}
			recordedAmount = roundMoney(convertedRecorded)

			convertedRejected, err := s.exchangeSvc.Convert(ctx, row.RejectedAmount, row.Currency, defaultCurrency)
			if err != nil {
				return nil, err
			}
			rejectedAmount = roundMoney(convertedRejected)

			convertedRate, err := s.exchangeSvc.Convert(ctx, row.HourlyRate, row.Currency, defaultCurrency)
			if err != nil {
				return nil, err
			}
			hourlyRate = roundMoney(convertedRate)
			currency = defaultCurrency
		}

		teacherSummary.TotalHours += row.RecordedMinutes / 60
		teacherSummary.TotalAmountByCurrency[defaultCurrency] += recordedAmount
		if includeRejected {
			teacherSummary.StatusBreakdown[models.HoursStatusRejected] += row.RejectedCount
		}
		teacherSummary.StatusBreakdown[models.HoursStatusRecorded] += row.RecordCount

		ps := models.PricingSummary{
			PricingId:      row.PricingId,
			Criteria:       row.Criteria,
			HourlyRate:     hourlyRate,
			Currency:       currency,
			IsActive:       row.IsActive,
			HoursRecorded:  row.RecordedMinutes / 60,
			TotalAmount:    recordedAmount,
			RecordCount:    row.RecordCount,
			RejectedHours:  row.RejectedMinutes / 60,
			RejectedAmount: rejectedAmount,
			RejectedCount:  row.RejectedCount,
		}
		teacherSummary.PricingSummaries = append(teacherSummary.PricingSummaries, ps)

		subjectIds = append(subjectIds, row.Criteria.SubjectId)
	}

	// enrich teacher basic info
	if len(teacherMap) > 0 {
		var teacherIds []primitive.ObjectID
		for id := range teacherMap {
			teacherIds = append(teacherIds, id)
		}
		teacherPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": teacherIds},
				"trash": false,
			}},
		}
		var teachers []contact_models.Teacher
		if err := s.teacherRepo.Aggregate(ctx, teacherPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &teachers)
		}); err == nil {
			for _, t := range teachers {
				if ts, ok := teacherMap[t.Id]; ok {
					ts.Teacher = models.TeacherBasicInfo{
						Id:        t.Id,
						FirstName: t.FirstName,
						LastName:  t.LastName,
						Email:     t.AccountSettings.Email,
					}
				}
			}
		}
	}

	// enrich subject info for each pricing summary
	if len(subjectIds) > 0 {
		subjectPipeline := []bson.M{
			{"$match": bson.M{
				"_id":   bson.M{"$in": subjectIds},
				"trash": false,
			}},
		}
		var subjects []content_models.Subject
		if err := s.subjectRepo.Aggregate(ctx, subjectPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &subjects)
		}); err == nil {
			subjectMap := map[primitive.ObjectID]content_models.Subject{}
			for _, sub := range subjects {
				subjectMap[sub.Id] = sub
			}
			for _, ts := range teacherMap {
				for idx, ps := range ts.PricingSummaries {
					if sub, ok := subjectMap[ps.Criteria.SubjectId]; ok {
						ps.Subject = models.SubjectInfo{
							Id:   sub.Id,
							Name: sub.Name,
							Code: sub.Code,
						}
					}
					ts.PricingSummaries[idx] = ps
				}
			}
		}
	}

	// flatten to slice
	result := []models.TeacherCompensationSummary{}
	for _, v := range teacherMap {
		result = append(result, *v)
	}
	return result, nil
}

func (s *compensationSvcs) GetCompensationSummary(ctx context.Context, cond *query.Conditions, start, end *time.Time, includeRejected bool) (*models.CompensationSummary, error) {
	filter := bson.M{
		"trash": false,
	}

	if cond != nil {
		if err := cond.CheckValid(); err != nil {
			return nil, constant.ErrQueryNotValid
		}
		whitelist := map[string]bool{
			"teacherId":           true,
			"criteria.subjectId":  true,
			"criteria.schoolType": true,
			"criteria.country":    true,
			"criteria.level":      true,
			"status":              true,
			"currency":            true,
		}
		converted, err := cond.ConvertToMongo(query.WithWhitelistNames(whitelist))
		if err != nil {
			return nil, err
		}
		filter = bson.M{"$and": []bson.M{filter, converted}}
	}

	if !includeRejected {
		filter = bson.M{"$and": []bson.M{filter, {"status": models.HoursStatusRecorded}}}
	} else {
		filter = bson.M{"$and": []bson.M{filter, {"status": bson.M{"$in": []string{models.HoursStatusRecorded, models.HoursStatusRejected}}}}}
	}

	if start != nil || end != nil {
		rangeFilter := bson.M{}
		if start != nil {
			rangeFilter["$gte"] = *start
		}
		if end != nil {
			rangeFilter["$lte"] = *end
		}
		filter = bson.M{"$and": []bson.M{filter, {"occurredAt": rangeFilter}}}
	}

	// Aggregate pipeline to get total stats
	pipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": bson.M{
				"teacherId": "$teacherId",
				"currency":  "$currency",
			},
			"totalMinutes": bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []any{"$status", models.HoursStatusRecorded}}, "$durationMinutes", 0}}},
			"totalAmount":  bson.M{"$sum": bson.M{"$cond": []any{bson.M{"$eq": []any{"$status", models.HoursStatusRecorded}}, "$amount", 0}}},
		}},
		{"$group": bson.M{
			"_id":            "$_id.currency",
			"totalMinutes":   bson.M{"$sum": "$totalMinutes"},
			"totalAmount":    bson.M{"$sum": "$totalAmount"},
			"uniqueTeachers": bson.M{"$addToSet": "$_id.teacherId"},
		}},
		{"$project": bson.M{
			"currency":       "$_id",
			"totalHours":     bson.M{"$divide": []any{"$totalMinutes", 60}},
			"totalAmount":    1,
			"uniqueTeachers": bson.M{"$size": "$uniqueTeachers"},
		}},
	}

	type aggRow struct {
		Currency       string  `bson:"currency"`
		TotalHours     float64 `bson:"totalHours"`
		TotalAmount    float64 `bson:"totalAmount"`
		UniqueTeachers int     `bson:"uniqueTeachers"`
	}

	var rows []aggRow
	if err := s.hoursRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &rows)
	}); err != nil {
		return nil, err
	}

	// Get default currency for conversion
	defaultCurrency, err := s.getDefaultCurrency(ctx)
	if err != nil {
		return nil, err
	}

	summary := &models.CompensationSummary{
		TotalAmountByCurrency: make(map[string]float64),
		AverageRatePerHour:    make(map[string]float64),
	}

	// Convert all amounts to default currency
	totalAmountInDefault := 0.0
	totalHours := 0.0

	for _, row := range rows {
		amount := row.TotalAmount
		if row.Currency != defaultCurrency {
			converted, err := s.exchangeSvc.Convert(ctx, row.TotalAmount, row.Currency, defaultCurrency)
			if err != nil {
				return nil, err
			}
			amount = roundMoney(converted)
		}
		totalAmountInDefault += amount
		totalHours += row.TotalHours
	}

	// All amounts are now in default currency
	if totalHours > 0 {
		summary.TotalAmountByCurrency[defaultCurrency] = roundMoney(totalAmountInDefault)
		summary.AverageRatePerHour[defaultCurrency] = roundMoney(totalAmountInDefault / totalHours)
	} else {
		summary.TotalAmountByCurrency[defaultCurrency] = 0
		summary.AverageRatePerHour[defaultCurrency] = 0
	}

	// Get total unique teachers across all currencies
	teacherCountPipeline := []bson.M{
		{"$match": filter},
		{"$group": bson.M{
			"_id": "$teacherId",
		}},
		{"$count": "total"},
	}

	type countResult struct {
		Total int `bson:"total"`
	}
	var countRes []countResult
	if err := s.hoursRepo.Aggregate(ctx, teacherCountPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &countRes)
	}); err != nil {
		return nil, err
	}

	if len(countRes) > 0 {
		summary.TotalTeachers = countRes[0].Total
	} else {
		summary.TotalTeachers = 0
	}

	// Total hours already calculated above
	summary.TotalHours = roundMoney(totalHours)

	return summary, nil
}

// RecordHoursFromSession records hours automatically when a coaching session is completed.
// This method is idempotent - it checks if hours already exist for this sessionId to avoid duplicates.
// If a rejected hours record exists, it will be re-activated instead of creating a new record.
func (s *compensationSvcs) RecordHoursFromSession(ctx context.Context, sessionId primitive.ObjectID, teacherId primitive.ObjectID, coachingProgramId primitive.ObjectID, occurredAt time.Time, durationMinutes float64) error {
	// First check if hours already recorded with "recorded" status (idempotency)
	existingRecorded, err := s.hoursRepo.GetByFilter(ctx, bson.M{
		"sessionId": sessionId,
		"status":    models.HoursStatusRecorded,
		"trash":     false,
	})
	if err == nil && existingRecorded != nil {
		// Hours already recorded, skip
		return nil
	}

	// Check if there's a rejected hours record that we can re-activate
	existingRejected, err := s.hoursRepo.GetByFilter(ctx, bson.M{
		"sessionId": sessionId,
		"status":    models.HoursStatusRejected,
		"trash":     false,
	})
	if err == nil && existingRejected != nil {
		// Re-activate the rejected hours record
		performedBy := getPerformedBy(ctx)
		now := time.Now().UTC()

		// Update occurredAt and durationMinutes if they've changed
		updates := bson.M{
			"status":    models.HoursStatusRecorded,
			"updatedAt": now,
		}
		old := map[string]any{
			"status": existingRejected.Status,
		}

		if !occurredAt.Equal(existingRejected.OccurredAt) {
			updates["occurredAt"] = occurredAt
			old["occurredAt"] = existingRejected.OccurredAt
		}

		if durationMinutes != existingRejected.DurationMinutes {
			updates["durationMinutes"] = durationMinutes
			old["durationMinutes"] = existingRejected.DurationMinutes
			// Recalculate amount if duration changed
			updates["amount"] = roundMoney(existingRejected.HourlyRate * (durationMinutes / 60))
			old["amount"] = existingRejected.Amount
		}

		audit := appendAudit(existingRejected.AuditLog, "reactivate", performedBy, "session completed again after cancellation/rescheduling", old, map[string]any{
			"status": models.HoursStatusRecorded,
		})

		update := bson.M{
			"$set": updates,
			"$push": bson.M{
				"auditLog": bson.M{"$each": []models.AuditEntry{audit[len(audit)-1]}},
			},
		}

		_, err = s.hoursRepo.Patch(ctx, bson.M{"_id": existingRejected.Id}, update)
		return err
	}

	// No existing hours record found, create a new one
	// Resolve pricing for this teacher/coaching combination
	pricing, err := s.resolvePricing(ctx, teacherId, "coaching", coachingProgramId)
	if err != nil {
		// If no pricing found, don't record hours (as per requirement: no default pricing)
		return nil
	}

	amount := roundMoney(pricing.HourlyRate * (durationMinutes / 60))
	now := time.Now().UTC()
	performedBy := getPerformedBy(ctx)
	audit := appendAudit(nil, "create", performedBy, "auto-recorded from session completion", nil, map[string]any{
		"durationMinutes": durationMinutes,
		"amount":          amount,
		"pricingId":       pricing.Id,
		"sessionId":       sessionId,
	})

	record := &models.TeacherHours{
		TeacherId:       teacherId,
		Ref:             "coaching",
		RefId:           coachingProgramId,
		SessionId:       &sessionId,
		OccurredAt:      occurredAt,
		DurationMinutes: durationMinutes,
		HourlyRate:      pricing.HourlyRate,
		Currency:        pricing.Currency,
		PricingId:       pricing.Id,
		Amount:          amount,
		Status:          models.HoursStatusRecorded,
		RecordedBy:      performedBy,
		Notes:           "Auto-recorded from session completion",
		AuditLog:        audit,
		CreatedAt:       now,
		UpdatedAt:       now,
		Trash:           false,
	}

	if err := s.hoursRepo.Add(ctx, record); err != nil {
		return err
	}
	return nil
}

// RejectHoursBySessionId rejects any recorded hours for a given session (e.g., when session is cancelled or rescheduled).
func (s *compensationSvcs) RejectHoursBySessionId(ctx context.Context, sessionId primitive.ObjectID, reason string) error {
	// Find all hours records for this session that are not already rejected
	existing, err := s.hoursRepo.GetByFilter(ctx, bson.M{
		"sessionId": sessionId,
		"status":    models.HoursStatusRecorded,
		"trash":     false,
	})
	if err != nil {
		// No hours found, nothing to reject
		return nil
	}

	if existing.Status == models.HoursStatusRejected {
		// Already rejected
		return nil
	}

	performedBy := getPerformedBy(ctx)
	now := time.Now().UTC()

	audit := appendAudit(existing.AuditLog, "reject", performedBy, reason, map[string]any{"status": existing.Status}, map[string]any{"status": models.HoursStatusRejected})

	update := bson.M{
		"$set": bson.M{
			"status":    models.HoursStatusRejected,
			"updatedAt": now,
		},
		"$push": bson.M{
			"auditLog": bson.M{"$each": []models.AuditEntry{audit[len(audit)-1]}},
		},
	}

	_, err = s.hoursRepo.Patch(ctx, bson.M{"_id": existing.Id}, update)
	return err
}

// Helpers

func (s *compensationSvcs) resolvePricing(ctx context.Context, teacherId primitive.ObjectID, ref string, refId primitive.ObjectID) (*models.TeacherPricing, error) {
	criteria, err := s.getCriteriaFromRef(ctx, ref, refId)
	if err != nil {
		return nil, err
	}

	return s.pricingRepo.GetByFilter(ctx, bson.M{
		"teacherId":           teacherId,
		"criteria.subjectId":  criteria.SubjectId,
		"criteria.schoolType": criteria.SchoolType,
		"criteria.country":    criteria.Country,
		"criteria.level":      criteria.Level,
		"isActive":            true,
		"trash":               false,
	})
}

func (s *compensationSvcs) getCriteriaFromRef(ctx context.Context, ref string, refId primitive.ObjectID) (models.PricingCriteria, error) {
	if ref == "course" {
		course, err := s.courseRepo.GetByFilter(ctx, bson.M{"_id": refId, "trash": false})
		if err != nil {
			return models.PricingCriteria{}, err
		}
		if len(course.BasicInfo.LinkedSubjects) == 0 {
			return models.PricingCriteria{}, errors.New("course has no linked subjects for pricing match")
		}
		return models.PricingCriteria{
			SubjectId:  course.BasicInfo.LinkedSubjects[0],
			SchoolType: course.BasicInfo.SchoolType,
			Country:    course.BasicInfo.Country,
			Level:      course.BasicInfo.Level,
		}, nil
	}

	if ref == "coaching" {
		coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": refId, "trash": false})
		if err != nil {
			return models.PricingCriteria{}, err
		}
		return models.PricingCriteria{
			SubjectId:  coaching.Subject,
			SchoolType: coaching.SchoolType,
			Country:    coaching.Country,
			Level:      coaching.Level,
		}, nil
	}

	return models.PricingCriteria{}, constant.ErrInvalidRef
}

func (s *compensationSvcs) deactivateExistingByCriteria(ctx context.Context, teacherId primitive.ObjectID, criteria models.PricingCriteria) error {
	filter := bson.M{
		"teacherId":           teacherId,
		"criteria.subjectId":  criteria.SubjectId,
		"criteria.schoolType": criteria.SchoolType,
		"criteria.country":    criteria.Country,
		"criteria.level":      criteria.Level,
		"isActive":            true,
		"trash":               false,
	}
	update := bson.M{"$set": bson.M{"isActive": false}}
	updateModel := mongo.NewUpdateManyModel().SetFilter(filter).SetUpdate(update)
	_, err := s.pricingRepo.BulkWrite(ctx, []mongo.WriteModel{updateModel})
	return err
}

// getDefaultCurrency retrieves the default currency from configuration
func (s *compensationSvcs) getDefaultCurrency(ctx context.Context) (string, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return "", err
	}

	// Default to USD if not configured
	defaultCurrency := "USD"
	val, err := common.GetOptionValue("COMPENSATION_DEFAULT_CURRENCY", cfg.Hp)
	if err == nil && val != nil {
		if str, ok := val.(string); ok && str != "" {
			defaultCurrency = str
		}
	}

	return defaultCurrency, nil
}

// validateCurrency ensures the input currency matches the default currency
func (s *compensationSvcs) validateCurrency(ctx context.Context, currency string) error {
	defaultCurrency, err := s.getDefaultCurrency(ctx)
	if err != nil {
		return err
	}

	if currency != defaultCurrency {
		return constant.ErrUnsupportedCurrency
	}

	return nil
}

func parseTeacherAndRef(teacherId, refId string) (primitive.ObjectID, primitive.ObjectID, error) {
	teacherOID, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return primitive.NilObjectID, primitive.NilObjectID, constant.ErrInvalidTeacherId
	}
	refOID, err := primitive.ObjectIDFromHex(refId)
	if err != nil {
		return primitive.NilObjectID, primitive.NilObjectID, constant.ErrInvalidRefId
	}
	return teacherOID, refOID, nil
}

func appendAudit(existing []models.AuditEntry, action string, performedBy primitive.ObjectID, reason string, oldValues, newValues map[string]any) []models.AuditEntry {
	entry := models.AuditEntry{
		Action:      action,
		PerformedBy: performedBy,
		Reason:      reason,
		At:          time.Now().UTC(),
		OldValues:   oldValues,
		NewValues:   newValues,
	}
	return append(existing, entry)
}

func roundMoney(v float64) float64 {
	return math.Round(v*100) / 100
}

func getPerformedBy(ctx context.Context) primitive.ObjectID {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil || cfg.User == nil {
		return primitive.NilObjectID
	}
	return cfg.User.Id
}
