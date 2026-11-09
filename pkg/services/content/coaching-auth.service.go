package content

import (
	"context"
	"errors"

	"larsa-academy-microservice/pkg/db"

	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// CoachingAuthService defines the interface for coaching authorization operations
type CoachingAuthService interface {
	CanViewCoaching(ctx context.Context, coachingId string) (bool, error)
	CanManageCoaching(ctx context.Context, coachingId string) (bool, error)
	CanManageCoachingPricing(ctx context.Context, coachingId string) (bool, error)
	CanViewCoachingMembers(ctx context.Context, coachingId string) (bool, error)
	FilterCoachingsByPermissions(ctx context.Context, coachings []models.Coaching) ([]models.Coaching, error)
	CheckCoachingsPermissions(ctx context.Context, coachingIds []primitive.ObjectID) (map[primitive.ObjectID]bool, error)
}

type coachingAuthService struct {
	i                   *do.Injector
	coachingRepo        db.MainRepo[models.Coaching]
	coachingStudentRepo db.MainRepo[models.CoachingStudent]
}

func NewCoachingAuthService(i *do.Injector) (CoachingAuthService, error) {
	return &coachingAuthService{
		i:                   i,
		coachingRepo:        db.NewRepo[models.Coaching](i),
		coachingStudentRepo: db.NewRepo[models.CoachingStudent](i),
	}, nil
}

func (s *coachingAuthService) CanViewCoaching(ctx context.Context, coachingId string) (bool, error) {
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCoachingPrograms); canViewAll {
		return true, nil
	}

	if canViewAssigned, _ := util.HasCapability(ctx, CapabilityViewAssignedCoaching); canViewAssigned {
		return s.isAssignedToCoaching(ctx, coachingId)
	}

	if canViewPublic, _ := util.HasCapability(ctx, CapabilityViewPublicCoaching); canViewPublic {
		if isPublic, _ := s.isCoachingPublic(ctx, coachingId); isPublic {
			return true, nil
		}
		if isEnrolled, _ := s.isStudentEnrolledInCoaching(ctx, coachingId); isEnrolled {
			return true, nil
		}
	}

	return false, nil
}

func (s *coachingAuthService) CanManageCoaching(ctx context.Context, coachingId string) (bool, error) {
	if canManageAll, _ := util.HasCapability(ctx, CapabilityManageAllCoachingPrograms); canManageAll {
		return true, nil
	}

	if canManageAssigned, _ := util.HasCapability(ctx, CapabilityManageAssignedCoaching); canManageAssigned {
		return s.isAssignedToCoaching(ctx, coachingId)
	}

	return false, nil
}

func (s *coachingAuthService) CanManageCoachingPricing(ctx context.Context, coachingId string) (bool, error) {
	return util.HasCapability(ctx, CapabilityManageCoachingPricing)
}

func (s *coachingAuthService) CanViewCoachingMembers(ctx context.Context, coachingId string) (bool, error) {
	if canManageAll, _ := util.HasCapability(ctx, CapabilityManageAllCoachingPrograms); canManageAll {
		return true, nil
	}
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCoachingPrograms); canViewAll {
		return true, nil
	}

	if canViewMembers, _ := util.HasCapability(ctx, CapabilityViewCoachingMembers); canViewMembers {
		return s.isAssignedToCoaching(ctx, coachingId)
	}

	return false, nil
}

func (s *coachingAuthService) FilterCoachingsByPermissions(ctx context.Context, coachings []models.Coaching) ([]models.Coaching, error) {
	if len(coachings) == 0 {
		return coachings, nil
	}

	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCoachingPrograms); canViewAll {
		return coachings, nil
	}

	coachingIds := make([]primitive.ObjectID, len(coachings))
	for i, c := range coachings {
		coachingIds[i] = c.Id
	}

	perms, err := s.CheckCoachingsPermissions(ctx, coachingIds)
	if err != nil {
		return nil, err
	}

	var filtered []models.Coaching
	for _, c := range coachings {
		if perms[c.Id] {
			filtered = append(filtered, c)
		}
	}
	return filtered, nil
}

func (s *coachingAuthService) CheckCoachingsPermissions(ctx context.Context, coachingIds []primitive.ObjectID) (map[primitive.ObjectID]bool, error) {
	res := make(map[primitive.ObjectID]bool)
	if len(coachingIds) == 0 {
		return res, nil
	}

	// Admin check
	if canViewAll, _ := util.HasCapability(ctx, CapabilityViewAllCoachingPrograms); canViewAll {
		for _, id := range coachingIds {
			res[id] = true
		}
		return res, nil
	}

	canViewAssigned, _ := util.HasCapability(ctx, CapabilityViewAssignedCoaching)
	canViewPublic, _ := util.HasCapability(ctx, CapabilityViewPublicCoaching)
	cfg, _ := util.GetReqAppCfg(ctx)

	// Assigned check (Teachers/Supervisors)
	assignedMap := make(map[primitive.ObjectID]bool)
	if canViewAssigned && cfg != nil {
		for _, id := range coachingIds {
			if assigned, _ := s.isAssignedToCoaching(ctx, id.Hex()); assigned {
				assignedMap[id] = true
			}
		}
	}

	// Enrollment check (Students)
	enrolledMap := make(map[primitive.ObjectID]bool)
	if cfg != nil {
		var enrollments []models.CoachingStudent
		if err := s.coachingStudentRepo.Aggregate(ctx, []bson.M{
			{"$match": bson.M{
				"studentId":  cfg.User.Id,
				"coachingId": bson.M{"$in": coachingIds},
				"status":     enums.EnrollmentStatusActive,
				"trash":      false,
			}},
		}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		}); err == nil {
			for _, e := range enrollments {
				enrolledMap[e.CoachingId] = true
			}
		}
	}

	for _, id := range coachingIds {
		if assignedMap[id] || enrolledMap[id] {
			res[id] = true
			continue
		}

		if canViewPublic {
			// For coaching, public means it exists (status check omitted for now as per current logic)
			res[id] = true
			continue
		}

		res[id] = false
	}

	return res, nil
}

func (s *coachingAuthService) isAssignedToCoaching(ctx context.Context, coachingId string) (bool, error) {
	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return false, errors.New("invalid coaching id")
	}
	// 1. Check direct assignments in coaching document
	coaching, err := s.coachingRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return false, err
	}

	cfg, err := util.GetReqAppCfg(ctx)
	if err == nil {
		// Check Coachs
		for _, coachId := range coaching.Coachs {
			if cfg.User != nil && coachId == cfg.User.Id {
				return true, nil
			}
		}
		for _, supervisorId := range coaching.SupervisorIds {
			if cfg.User != nil && supervisorId == cfg.User.Id {
				return true, nil
			}
		}
	}

	return false, nil
}

func (s *coachingAuthService) isCoachingPublic(ctx context.Context, coachingId string) (bool, error) {

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return false, errors.New("invalid coaching id")
	}

	_, err = s.coachingRepo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		return false, err
	}

	return true, nil
}

func (s *coachingAuthService) isStudentEnrolledInCoaching(ctx context.Context, coachingId string) (bool, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil || cfg.User == nil {
		return false, err
	}

	_id, err := primitive.ObjectIDFromHex(coachingId)
	if err != nil {
		return false, errors.New("invalid coaching id")
	}

	count, err := s.coachingStudentRepo.Count(ctx, bson.M{
		"studentId":  cfg.User.Id,
		"coachingId": _id,
		"status":     enums.EnrollmentStatusActive,
		"trash":      false,
	})
	if err != nil {
		return false, err
	}

	return count > 0, nil
}
