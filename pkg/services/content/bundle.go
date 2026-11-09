package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/helpers"
	querypkg "larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"strconv"
	"sync"
	"time"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"golang.org/x/sync/errgroup"
)

type BundleSvcs interface {
	GetById(ctx context.Context, id string) (*models.EnrichedBundle, error)
	Get(ctx context.Context, skip, limit int64, query *querypkg.Conditions, enrichPrice bool) (*models.BundlePagination, error)
	GetAll(ctx context.Context, query *querypkg.Conditions) ([]models.EnrichedBundle, error)
	Add(ctx context.Context, data *models.BundleDto) (*models.Bundle, error)
	Update(ctx context.Context, bundleId string, data *models.BundleDto) (*models.Bundle, error)
	Delete(ctx context.Context, bundleId string) error
	HardDeleteAll(ctx context.Context) error // Hard delete all bundles - for staging data clearing only

	// Bundle pricing operations
	GetBundlePricing(ctx context.Context, bundleId string, skip, limit int64) (*models.BundlePricingPagination, error)
	AddBundlePricing(ctx context.Context, bundleId string, data *models.BundlePricingDto) (*models.BundlePricing, error)
	UpdateBundlePricing(ctx context.Context, bundleId, pricingId string, data *models.BundlePricingDto) (*models.BundlePricing, error)
	DeleteBundlePricing(ctx context.Context, bundleId, pricingId string) error

	// Student facing list operations
	GetStudentList(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions, enrichPrice bool) (*models.StudentBundlePagination, error)

	GetBundleMembers(ctx context.Context, bundleId string, skip, limit int64, query *querypkg.Conditions) (*models.MemberBundlePagination, error)
}

type bundlesvcs struct {
	injector          *do.Injector
	bundleRepo        db.MainRepo[models.Bundle]
	courseRepo        db.MainRepo[models.Course]
	coachingRepo      db.MainRepo[models.Coaching]
	bundlePricingRepo db.MainRepo[models.BundlePricing]
	studentBundleRepo db.MainRepo[models.StudentBundle]
	db                *mongo.Client
	authSvc           BundleAuthService
	pricingSvcs       sales.PricingSvcs
	parentSvcs        contact.ParentSvcs
}

func NewBundleSvcs(i *do.Injector) (BundleSvcs, error) {
	pricingSvcs, err := do.Invoke[sales.PricingSvcs](i)
	if err != nil {
		return nil, err
	}

	return &bundlesvcs{
		injector:          i,
		bundleRepo:        db.NewRepo[models.Bundle](i),
		courseRepo:        db.NewRepo[models.Course](i),
		coachingRepo:      db.NewRepo[models.Coaching](i),
		bundlePricingRepo: db.NewRepo[models.BundlePricing](i),
		studentBundleRepo: db.NewRepo[models.StudentBundle](i),
		db:                do.MustInvoke[*mongo.Client](i),
		authSvc:           do.MustInvoke[BundleAuthService](i),
		pricingSvcs:       pricingSvcs,
		parentSvcs:        do.MustInvoke[contact.ParentSvcs](i),
	}, nil
}

func (b *bundlesvcs) GetById(ctx context.Context, id string) (*models.EnrichedBundle, error) {
	if canView, err := b.authSvc.CanViewBundle(ctx, id); err != nil {
		return nil, err
	} else if !canView {
		return nil, helpers.Forbidden("unauthorized to view bundle")
	}

	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, errors.New("invalid bundle id")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"_id": _id, "trash": false}},
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "schoolId",
				"foreignField": "_id",
				"as":           "schoolData",
			},
		},
		{
			"$addFields": bson.M{
				"schoolObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
						nil,
					},
				},
			},
		},
		{
			"$project": bson.M{
				"schoolData": 0,
			},
		},
	}

	var results []models.EnrichedBundle
	if err := b.bundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	if len(results) == 0 {
		return nil, errors.New("bundle not found")
	}

	populated, err := b.populateEnrichedBundlesContent(ctx, results)
	if err != nil {
		return nil, err
	}

	return &populated[0], nil
}

func (b *bundlesvcs) Get(ctx context.Context, skip, limit int64, query *querypkg.Conditions, enrichPrice bool) (*models.BundlePagination, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
	}

	count, err := b.bundleRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})
	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})

	pipeline = append(pipeline, []bson.M{
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "schoolId",
				"foreignField": "_id",
				"as":           "schoolData",
			},
		},
		{
			"$addFields": bson.M{
				"schoolObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
						nil,
					},
				},
			},
		},
		{
			"$project": bson.M{
				"schoolData": 0,
			},
		},
	}...)

	if enrichPrice {
		stages, err := b.pricingSvcs.GetPriceEnrichmentStages(ctx, enums.CartTypeBundle, "$_id")
		if err == nil {
			pipeline = append(pipeline, stages...)
		}
		pipeline = append(pipeline, bson.M{"$match": bson.M{"pricing": bson.M{"$ne": nil}}})
	}

	var result []models.EnrichedBundle
	if err := b.bundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	// Filter based on EnrichedBundle instead of Bundle
	result, err = b.filterBundlesByPermissions(ctx, result)
	if err != nil {
		return nil, err
	}

	// Keep count from pipeline (includes isEnrolled filter) - do not overwrite with len(result)
	// which would incorrectly set TotalCount to page size and break pagination

	// Populate content for EnrichedBundle
	result, err = b.populateEnrichedBundlesContent(ctx, result)
	if err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.BundlePagination{
		Bundles:    result,
		Pagination: pagination,
	}, nil
}

func (b *bundlesvcs) GetAll(ctx context.Context, query *querypkg.Conditions) ([]models.EnrichedBundle, error) {
	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, err
	}

	pipeline := []bson.M{
		{"$match": bson.M{"trash": false}},
		{"$match": filter},
		{"$sort": bson.M{"_id": -1}},
		{
			"$lookup": bson.M{
				"from":         "academySchools",
				"localField":   "schoolId",
				"foreignField": "_id",
				"as":           "schoolData",
			},
		},
		{
			"$addFields": bson.M{
				"schoolObj": bson.M{
					"$cond": []interface{}{
						bson.M{"$gt": []interface{}{bson.M{"$size": bson.M{"$ifNull": bson.A{"$schoolData", bson.A{}}}}, 0}},
						bson.M{"$arrayElemAt": []interface{}{"$schoolData", 0}},
						nil,
					},
				},
			},
		},
		{
			"$project": bson.M{
				"schoolData": 0,
			},
		},
	}

	var result []models.EnrichedBundle
	if err := b.bundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	result, err = b.filterBundlesByPermissions(ctx, result)
	if err != nil {
		return nil, err
	}

	result, err = b.populateEnrichedBundlesContent(ctx, result)
	if err != nil {
		return nil, err
	}

	return result, nil

}

func (b *bundlesvcs) Add(ctx context.Context, data *models.BundleDto) (*models.Bundle, error) {
	if canManage, err := b.authSvc.CanManageBundle(ctx, ""); err != nil || !canManage {
		return nil, errors.New("unauthorized to add bundle")
	}

	if err := models.ValidateBundleContentTypes(data.Content); err != nil {
		return nil, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	bundle := &models.Bundle{
		Id:        primitive.NewObjectID(),
		BundleDto: *data,
		CreatedAt: time.Now(),
		CreatedBy: cfg.User.Id,
		UpdatedAt: time.Now(),
		UpdatedBy: cfg.User.Id,
	}

	if err := b.bundleRepo.Add(ctx, bundle); err != nil {
		return nil, err
	}

	return bundle, nil
}

func (b *bundlesvcs) Update(ctx context.Context, bundleId string, data *models.BundleDto) (*models.Bundle, error) {
	if canManage, err := b.authSvc.CanManageBundle(ctx, bundleId); err != nil || !canManage {
		return nil, errors.New("unauthorized to update bundle")
	}

	if err := models.ValidateBundleContentTypes(data.Content); err != nil {
		return nil, err
	}

	_id, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return nil, errors.New("invalid bundle id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	bundle := &models.Bundle{
		Id:        _id,
		BundleDto: *data,
		UpdatedAt: time.Now(),
		UpdatedBy: cfg.User.Id,
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bundle}

	updatedBundle, err := b.bundleRepo.Patch(ctx, filter, update)
	if err != nil {
		return nil, err
	}

	return updatedBundle, nil
}

func (b *bundlesvcs) Delete(ctx context.Context, bundleId string) error {
	if canManage, err := b.authSvc.CanManageBundle(ctx, bundleId); err != nil || !canManage {
		return errors.New("unauthorized to delete bundle")
	}

	_id, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return errors.New("invalid bundle id")
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	filter := bson.M{"_id": _id, "trash": false}
	update := bson.M{"$set": bson.M{"trash": true, "updatedAt": time.Now(), "updatedBy": cfg.User.Id}}

	_, err = b.bundleRepo.Patch(ctx, filter, update)
	if err != nil {
		return err
	}

	return nil
}

func (b *bundlesvcs) filterBundlesByPermissions(ctx context.Context, bundles []models.EnrichedBundle) ([]models.EnrichedBundle, error) {
	if len(bundles) == 0 {
		return bundles, nil
	}

	// We need to convert back and forth to use the existing auth service method
	// Or we can just check permissions for each bundle manually.
	// Looking at existing code, b.authSvc.FilterBundlesByPermissions(ctx, result) was used.

	// Convert EnrichedBundle to Bundle for auth check
	baseBundles := make([]models.Bundle, len(bundles))
	for i, eb := range bundles {
		baseBundles[i] = eb.Bundle
	}

	filteredBase, err := b.authSvc.FilterBundlesByPermissions(ctx, baseBundles)
	if err != nil {
		return nil, err
	}

	// Map filtered base bundles back to enriched bundles
	filteredMap := make(map[primitive.ObjectID]bool)
	for _, fb := range filteredBase {
		filteredMap[fb.Id] = true
	}

	var result []models.EnrichedBundle
	for _, eb := range bundles {
		if filteredMap[eb.Id] {
			result = append(result, eb)
		}
	}

	return result, nil
}

func (b *bundlesvcs) populateEnrichedBundlesContent(ctx context.Context, bundles []models.EnrichedBundle) ([]models.EnrichedBundle, error) {
	if len(bundles) == 0 {
		return bundles, nil
	}

	// Convert to []models.Bundle to reuse populateBundlesContent
	baseBundles := make([]models.Bundle, len(bundles))
	for i, eb := range bundles {
		baseBundles[i] = eb.Bundle
	}

	populatedBase, err := b.populateBundlesContent(ctx, baseBundles)
	if err != nil {
		return nil, err
	}

	// Put back the populated content
	for i := range bundles {
		bundles[i].Bundle = populatedBase[i]
	}

	return bundles, nil
}

func (b *bundlesvcs) populateBundlesContent(ctx context.Context, bundles []models.Bundle) ([]models.Bundle, error) {
	if len(bundles) == 0 {
		return bundles, nil
	}

	// Use maps for deduplication
	courseIdMap := make(map[primitive.ObjectID]struct{})
	coachingIdMap := make(map[primitive.ObjectID]struct{})

	for _, bundle := range bundles {
		for _, item := range bundle.Content {
			if item.Type == "course" {
				courseIdMap[item.RefId] = struct{}{}
			} else if item.Type == "coaching" {
				coachingIdMap[item.RefId] = struct{}{}
			}
		}
	}

	var courseIds, coachingIds []primitive.ObjectID
	for id := range courseIdMap {
		courseIds = append(courseIds, id)
	}
	for id := range coachingIdMap {
		coachingIds = append(coachingIds, id)
	}

	// Shared error and synchronization
	g, gCtx := errgroup.WithContext(ctx)

	// Thread-safe maps for results
	var coursesMap sync.Map
	var coachingsMap sync.Map

	// Fetch Courses in parallel
	if len(courseIds) > 0 {
		g.Go(func() error {
			var courses []models.Course
			if err := b.courseRepo.Aggregate(gCtx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": courseIds}}}}, func(cur *mongo.Cursor) error {
				return cur.All(gCtx, &courses)
			}); err != nil {
				return err
			}
			for _, c := range courses {
				coursesMap.Store(c.Id, c)
			}
			return nil
		})
	}

	// Fetch Coachings in parallel
	if len(coachingIds) > 0 {
		g.Go(func() error {
			var coachings []models.Coaching
			if err := b.coachingRepo.Aggregate(gCtx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": coachingIds}}}}, func(cur *mongo.Cursor) error {
				return cur.All(gCtx, &coachings)
			}); err != nil {
				return err
			}
			for _, c := range coachings {
				coachingsMap.Store(c.Id, c)
			}
			return nil
		})
	}

	// Wait for both to finish
	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Assign data back to bundles
	for i, bundle := range bundles {
		for j, item := range bundle.Content {
			if item.Type == "course" {
				if val, ok := coursesMap.Load(item.RefId); ok {
					if course, ok := val.(models.Course); ok {
						bundles[i].Content[j].Data = map[string]any{
							"title": course.BasicInfo.Name,
						}
					}
				}
			} else if item.Type == "coaching" {
				if val, ok := coachingsMap.Load(item.RefId); ok {
					if coaching, ok := val.(models.Coaching); ok {
						bundles[i].Content[j].Data = map[string]any{
							"title": coaching.Title,
						}
					}
				}
			}
		}
	}

	return bundles, nil
}

// HardDeleteAll hard deletes all bundles from the database
// WARNING: This is for staging data clearing only and should be removed later
func (b *bundlesvcs) HardDeleteAll(ctx context.Context) error {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	coll := b.db.Database(cfg.Db).Collection((&models.Bundle{}).CollName())

	// Delete all documents (hard delete)
	_, err = coll.DeleteMany(ctx, bson.M{})
	if err != nil {
		return err
	}

	return nil
}

func (b *bundlesvcs) GetStudentList(ctx context.Context, studentId string, skip, limit int64, query *querypkg.Conditions, enrichPrice bool) (*models.StudentBundlePagination, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	var targetUserId primitive.ObjectID
	var studentIdForFavorites string

	if studentId != "" {
		_studentId, err := primitive.ObjectIDFromHex(studentId)
		if err != nil {
			return nil, errors.New("invalid student id")
		}

		if studentId != cfg.User.Id.Hex() {
			isParent, err := b.parentSvcs.IsUserParentOfStudent(ctx, cfg.User.Id, _studentId)
			if err != nil {
				return nil, fmt.Errorf("error checking parent relationship: %w", err)
			}
			if !isParent {
				return nil, errors.New("unauthorized: you can only view your own or your children's data")
			}
		}
		targetUserId = _studentId
		studentIdForFavorites = studentId
	} else if cfg.User != nil && !cfg.User.Id.IsZero() {
		targetUserId = cfg.User.Id
		studentIdForFavorites = cfg.User.Id.Hex()
	} else {
		res, err := b.Get(ctx, skip, limit, query, enrichPrice)
		if err != nil {
			return nil, err
		}
		return &models.StudentBundlePagination{
			Bundles:    mapToStudentBundleResponse(res.Bundles, nil, nil),
			Pagination: res.Pagination,
		}, nil
	}

	var isEnrolledRequested *bool
	if query != nil {
		newCols := []querypkg.Column{}
		for _, col := range query.Columns {
			if col.Name == "isEnrolled" {
				val, ok := col.Value.(bool)
				if !ok {
					if s, ok := col.Value.(string); ok {
						if bVal, err := strconv.ParseBool(s); err == nil {
							val = bVal
							ok = true
						}
					}
				}
				if ok {
					isEnrolledRequested = &val
				}
				continue
			}
			newCols = append(newCols, col)
		}
		query.Columns = newCols
	}

	enrollmentsMap := make(map[primitive.ObjectID]bool)
	enrollmentIds := []primitive.ObjectID{}
	var enrollments []models.StudentBundle
	err = b.studentBundleRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"studentId": targetUserId, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	})
	if err == nil {
		for _, e := range enrollments {
			enrollmentsMap[e.BundleId] = true
			enrollmentIds = append(enrollmentIds, e.BundleId)
		}
	}

	if isEnrolledRequested != nil {
		if query == nil {
			query = &querypkg.Conditions{}
		}
		if *isEnrolledRequested {
			query.Columns = append(query.Columns, querypkg.Column{
				Name:  "_id",
				Exp:   querypkg.In,
				Value: enrollmentIds,
			})
		} else {
			query.Columns = append(query.Columns, querypkg.Column{
				Name:  "_id",
				Exp:   querypkg.NotIn,
				Value: enrollmentIds,
			})
		}
	}

	res, err := b.Get(ctx, skip, limit, query, enrichPrice)
	if err != nil {
		return nil, err
	}

	studentSvc := do.MustInvoke[contact.StudentSvcs](b.injector)
	student, err := studentSvc.GetOne(ctx, studentIdForFavorites)
	favoritesMap := make(map[primitive.ObjectID]bool)
	if err == nil && student != nil {
		for _, fav := range student.Favorites {
			if fav.Type == "bundle" && !fav.RefId.IsZero() {
				favoritesMap[fav.RefId] = true
			}
		}
	}

	return &models.StudentBundlePagination{
		Bundles:    mapToStudentBundleResponse(res.Bundles, favoritesMap, enrollmentsMap),
		Pagination: res.Pagination,
	}, nil
}

func mapToStudentBundleResponse(bundles []models.EnrichedBundle, favoritesMap map[primitive.ObjectID]bool, enrollmentsMap map[primitive.ObjectID]bool) []models.StudentBundleResponse {
	response := make([]models.StudentBundleResponse, len(bundles))
	for i, bundle := range bundles {
		isFavorite := false
		if favoritesMap != nil {
			isFavorite = favoritesMap[bundle.Id]
		}
		isEnrolled := false
		if enrollmentsMap != nil {
			isEnrolled = enrollmentsMap[bundle.Id]
		}
		response[i] = models.StudentBundleResponse{
			Bundle:     bundle.Bundle,
			Pricing:    bundle.Pricing,
			IsFavorite: isFavorite,
			IsEnrolled: isEnrolled,
		}
	}
	return response
}

func (b *bundlesvcs) GetBundleMembers(ctx context.Context, bundleId string, skip, limit int64, query *querypkg.Conditions) (*models.MemberBundlePagination, error) {
	// Add authorization check - users who can view bundle members
	canView, err := b.authSvc.CanManageBundle(ctx, bundleId) // Using CanManageBundle as a proxy for now, similar to how other handlers check
	if err != nil {
		return nil, err
	}
	if !canView {
		return nil, errors.New("unauthorized to view bundle members")
	}

	_id, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return nil, errors.New("invalid bundle id")
	}

	if err := query.CheckValid(); err != nil {
		return nil, err
	}

	filter, err := query.ConvertToMongo()
	if err != nil {
		return nil, errors.New("invalid convert to mongo filters")
	}

	pipeline := []bson.M{
		{"$match": bson.M{"bundleId": _id, "trash": false}},
		{"$match": filter},
	}

	count, err := b.studentBundleRepo.Count(ctx, pipeline)
	if err != nil {
		return nil, err
	}

	pipeline = append(pipeline, bson.M{"$skip": skip})
	pipeline = append(pipeline, bson.M{"$limit": limit})
	pipeline = append(pipeline, bson.M{"$sort": bson.M{"_id": -1}})

	// Populate student
	pipeline = append(pipeline, []bson.M{
		{
			"$lookup": bson.M{
				"from": "academyStudents",
				"let":  bson.M{"studentId": "$studentId"},
				"pipeline": []bson.M{
					{"$match": bson.M{"$expr": bson.M{"$eq": []interface{}{"$_id", "$$studentId"}}}},
				},
				"as": "student",
			},
		},
		{
			"$unwind": bson.M{
				"path":                       "$student",
				"preserveNullAndEmptyArrays": true,
			},
		},
		{
			"$set": bson.M{
				"firstName": "$student.generalInfo.firstName",
				"lastName":  "$student.generalInfo.lastName",
				"email":     "$student.accountStudentSetting.email",
			},
		},
	}...)

	var result []models.StudentBundleRes
	if err := b.studentBundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &result)
	}); err != nil {
		return nil, err
	}

	var totalPages float64 = math.Ceil(float64(count) / float64(limit))
	pagination := types.Pagination{
		TotalPages: totalPages,
		PerPage:    limit,
		TotalCount: count,
	}

	return &models.MemberBundlePagination{
		StudentBundles: result,
		Pagination:     pagination,
	}, nil
}
