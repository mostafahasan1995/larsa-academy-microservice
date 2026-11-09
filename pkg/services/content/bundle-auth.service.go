package content

import (
	"context"

	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// BundleAuthService defines the interface for bundle authorization operations
type BundleAuthService interface {
	CanViewBundle(ctx context.Context, bundleId string) (bool, error)
	CanManageBundle(ctx context.Context, bundleId string) (bool, error)
	CanManageBundlePricing(ctx context.Context, bundleId string) (bool, error)
	CanViewBundleMembers(ctx context.Context, bundleId string) (bool, error)
	FilterBundlesByPermissions(ctx context.Context, bundles []models.Bundle) ([]models.Bundle, error)
	CheckBundlesPermissions(ctx context.Context, bundleIds []primitive.ObjectID) (map[primitive.ObjectID]bool, error)
}

type bundleAuthService struct {
	i                 *do.Injector
	bundleRepo        db.MainRepo[models.Bundle]
	studentBundleRepo db.MainRepo[models.StudentBundle]
}

func NewBundleAuthService(i *do.Injector) (BundleAuthService, error) {
	return &bundleAuthService{
		i:                 i,
		bundleRepo:        db.NewRepo[models.Bundle](i),
		studentBundleRepo: db.NewRepo[models.StudentBundle](i),
	}, nil
}

func (s *bundleAuthService) CanViewBundle(ctx context.Context, bundleId string) (bool, error) {
	canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllBundles)
	if canViewAll {
		return true, nil
	}

	canViewPublic, _ := util.HasCapability(ctx, CapabilityViewPublicBundles)
	isPublic, _ := s.isBundlePublic(ctx, bundleId)
	if isPublic && canViewPublic {
		return true, nil
	}

	if isEnrolled, _ := s.isStudentEnrolledInBundle(ctx, bundleId); isEnrolled {
		return true, nil
	}

	return false, helpers.Forbidden("unauthorized to view bundle")
}

func (s *bundleAuthService) CanManageBundle(ctx context.Context, bundleId string) (bool, error) {
	return util.HasCapability(ctx, CapabilityManageAllBundles)
}

func (s *bundleAuthService) CanManageBundlePricing(ctx context.Context, bundleId string) (bool, error) {
	return util.HasCapability(ctx, CapabilityManageBundlePricing)
}

func (s *bundleAuthService) CanViewBundleMembers(ctx context.Context, bundleId string) (bool, error) {
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllBundles); canViewAll {
		return true, nil
	}

	return util.HasCapability(ctx, CapabilityViewBundleMembers)
}

func (s *bundleAuthService) FilterBundlesByPermissions(ctx context.Context, bundles []models.Bundle) ([]models.Bundle, error) {
	if len(bundles) == 0 {
		return bundles, nil
	}

	// If admin, they can view everything
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllBundles); canViewAll {
		return bundles, nil
	}

	bundleIds := make([]primitive.ObjectID, len(bundles))
	for i, b := range bundles {
		bundleIds[i] = b.Id
	}

	perms, err := s.CheckBundlesPermissions(ctx, bundleIds)
	if err != nil {
		return nil, err
	}

	var filtered []models.Bundle
	for _, b := range bundles {
		if perms[b.Id] {
			filtered = append(filtered, b)
		}
	}
	return filtered, nil
}

func (s *bundleAuthService) CheckBundlesPermissions(ctx context.Context, bundleIds []primitive.ObjectID) (map[primitive.ObjectID]bool, error) {
	res := make(map[primitive.ObjectID]bool)
	if len(bundleIds) == 0 {
		return res, nil
	}

	// Admin check
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllBundles); canViewAll {
		for _, id := range bundleIds {
			res[id] = true
		}
		return res, nil
	}

	canViewPublic, _ := util.HasCapability(ctx, CapabilityViewPublicBundles)
	cfg, _ := util.GetReqAppCfg(ctx)

	// Fetch bundles to check public status
	var bundles []models.Bundle
	if err := s.bundleRepo.Aggregate(ctx, []bson.M{
		{"$match": bson.M{"_id": bson.M{"$in": bundleIds}, "trash": false}},
	}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &bundles)
	}); err != nil {
		return nil, err
	}

	bundleMap := make(map[primitive.ObjectID]models.Bundle)
	for _, b := range bundles {
		bundleMap[b.Id] = b
	}

	// Fetch student enrollments for these bundles
	enrolledMap := make(map[primitive.ObjectID]bool)
	if cfg != nil {
		var enrollments []models.StudentBundle
		if err := s.studentBundleRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"studentId": cfg.User.Id,
				"bundleId":  bson.M{"$in": bundleIds},
				"trash":     false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		}); err == nil {
			for _, e := range enrollments {
				enrolledMap[e.BundleId] = true
			}
		}
	}

	for _, id := range bundleIds {
		// Enrolled students can always view
		if enrolledMap[id] {
			res[id] = true
			continue
		}

		// Public check
		if canViewPublic {
			if b, ok := bundleMap[id]; ok && b.Status == "active" {
				res[id] = true
				continue
			}
		}

		res[id] = false
	}

	return res, nil
}

func (s *bundleAuthService) isBundlePublic(ctx context.Context, bundleId string) (bool, error) {
	_id, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return false, helpers.BadRequest("invalid bundle id")
	}

	bundle, err := s.bundleRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return false, err
	}

	return bundle.Status == "active", nil
}

func (s *bundleAuthService) isStudentEnrolledInBundle(ctx context.Context, bundleId string) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return false, err
	}

	if cfg.User == nil {
		return false, nil
	}

	_id, err := primitive.ObjectIDFromHex(bundleId)
	if err != nil {
		return false, helpers.BadRequest("invalid bundle id")
	}

	count, err := s.studentBundleRepo.Count(ctx, bson.M{
		"studentId": cfg.User.Id,
		"bundleId":  _id,
		"trash":     false,
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
