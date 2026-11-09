package seed

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"math"
	"time"

	"larsa-academy-microservice/pkg/services/seed/models"
	"larsa-academy-microservice/pkg/services/seed/repo"
	"larsa-academy-microservice/pkg/services/sorting"

	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type SeedSvcs interface {
	Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SeedPagination, error)
	GetOne(ctx context.Context, id string) (*models.Seed, error)
	Add(ctx context.Context, data *models.SeedDto) (*models.Seed, error)
	Update(ctx context.Context, id string, data *models.SeedDto) (*models.Seed, error)
	Delete(ctx context.Context, id string) error
	SeedRolesAndCapabilities(ctx context.Context) error
	CreateRoles(ctx context.Context) error
	CreateCapabilities(ctx context.Context) error
}

type seedSvcs struct {
	repo        repo.SeedRepo
	withtxn     *db.WithTxn
	sortingsvcs sorting.SortingSvcs
}

func NewSeedSvcs(i *do.Injector) (SeedSvcs, error) {
	return &seedSvcs{
		repo:        do.MustInvoke[repo.SeedRepo](i),
		withtxn:     do.MustInvoke[*db.WithTxn](i),
		sortingsvcs: do.MustInvoke[sorting.SortingSvcs](i),
	}, nil
}

func (r *seedSvcs) Get(ctx context.Context, skip, limit int64, query *query.Conditions) (*models.SeedPagination, error) {
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

	var result []models.Seed
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

	return &models.SeedPagination{
		Seeds:      result,
		Pagination: pagination,
	}, nil
}
func (r *seedSvcs) GetOne(ctx context.Context, id string) (*models.Seed, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}

	relation, err := r.repo.GetByFilter(ctx, bson.M{"_id": _id, "trash": false})
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("seed not found")
		}
		return nil, err
	}
	return relation, nil
}
func (r *seedSvcs) Add(ctx context.Context, data *models.SeedDto) (*models.Seed, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	relation := &models.Seed{
		Id:      primitive.NewObjectID(),
		SeedDto: *data,

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

func (r *seedSvcs) Update(ctx context.Context, id string, data *models.SeedDto) (*models.Seed, error) {
	_id, err := primitive.ObjectIDFromHex(id)
	if err != nil {
		return nil, err
	}
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}

	// Get the existing seed
	oldSeed, err := r.GetOne(ctx, id)
	if err != nil {
		return nil, err
	}

	// Create a copy of the old seed and update only the DTO fields
	updatedSeed := *oldSeed
	updatedSeed.SeedDto = *data
	updatedSeed.UpdatedAt = time.Now()
	updatedSeed.UpdatedBy = cfg.User.Id

	filter := bson.M{"_id": _id}
	update := bson.M{"$set": updatedSeed}

	updatedSeedResult, err := r.repo.Patch(ctx, filter, update)
	if err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errors.New("seed not found")
		}
		return nil, err
	}
	return updatedSeedResult, nil
}
func (s *seedSvcs) Delete(ctx context.Context, id string) error {
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
func (s *seedSvcs) SeedRolesAndCapabilities(ctx context.Context) error {
	// Check if we already have a seed record
	existingSeeds, err := s.Get(ctx, 0, 1, &query.Conditions{})
	if err != nil {
		return err
	}

	// If we have existing seeds and both flags are true, no need to do anything
	if len(existingSeeds.Seeds) > 0 {
		latestSeed := existingSeeds.Seeds[0]
		if latestSeed.IsCreateRoles && latestSeed.IsCreateCapabilities {
			return nil // Already seeded, nothing to do
		}
	}
	// Create roles and capabilities

	if !existingSeeds.Seeds[0].IsCreateRoles {
		if err := s.CreateRoles(ctx); err != nil {
			return err
		}

	}
	if !existingSeeds.Seeds[0].IsCreateCapabilities {
		if err := s.CreateCapabilities(ctx); err != nil {
			return err
		}
	}

	// Create or update seed record
	seedDto := &models.SeedDto{
		IsCreateRoles:        true,
		IsCreateCapabilities: true,
	}

	if len(existingSeeds.Seeds) > 0 {
		// Update existing seed record
		latestSeed := existingSeeds.Seeds[0]
		_, err = s.Update(ctx, latestSeed.Id.Hex(), seedDto)
		return err
	}

	// Create new seed record
	_, err = s.Add(ctx, seedDto)
	return err
}

// createRoles creates initial roles in the system
func (s *seedSvcs) CreateRoles(ctx context.Context) error {
	// TODO: Implement actual role creation logic
	// This is where you'll create roles like "admin", "user", "moderator", etc.
	// Example:
	// - Connect to your role service/repository
	// - Check if roles already exist
	// - Create default roles if they don't exist

	// Placeholder implementation
	roles := []string{"admin", "user", "moderator", "guest"}

	for _, role := range roles {
		fmt.Println(role)
		// Check if role exists, if not create it
		// roleService.CreateIfNotExists(ctx, role)
	}

	return nil
}

// createCapabilities creates initial capabilities/permissions in the system
func (s *seedSvcs) CreateCapabilities(ctx context.Context) error {
	// TODO: Implement actual capabilities creation logic
	// This is where you'll create permissions like "read:users", "write:users", etc.
	// Example:
	// - Connect to your capabilities service/repository
	// - Check if capabilities already exist
	// - Create default capabilities if they don't exist

	// Placeholder implementation
	capabilities := []string{
		"read:users",
		"write:users",
		"delete:users",
		"read:posts",
		"write:posts",
		"delete:posts",
	}

	for _, capability := range capabilities {
		fmt.Println(capability)

		// Check if capability exists, if not create it
		// capabilityService.CreateIfNotExists(ctx, capability)
	}

	return nil
}

// Optional: Function to reset seeding (for testing/development)
