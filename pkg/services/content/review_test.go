package content

import (
	"context"
	"testing"
	"time"

	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"

	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

// MockMainRepo is a simple mock implementation of the MainRepo interface
type MockMainRepo struct {
	PatchFunc func(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*models.Review, error)
}

// setupTestContext creates a context with proper AppCfg for testing
func setupTestContext() context.Context {
	user := &types.User{
		Id: primitive.NewObjectID(),
	}
	appCfg := &types.AppCfg{
		User: user,
	}
	return context.WithValue(context.Background(), util.ReqAppCfg, appCfg)
}

func (m *MockMainRepo) GetByFilter(ctx context.Context, filter bson.M, opts ...*options.FindOneOptions) (*models.Review, error) {
	return nil, nil
}

func (m *MockMainRepo) Aggregate(ctx context.Context, pipeline any, callback func(cur *mongo.Cursor) error) error {
	return nil
}

func (m *MockMainRepo) Add(ctx context.Context, data *models.Review, opts ...*options.InsertOneOptions) error {
	return nil
}

func (m *MockMainRepo) AddMany(ctx context.Context, data []any, opts ...*options.InsertManyOptions) error {
	return nil
}

func (m *MockMainRepo) Patch(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*models.Review, error) {
	if m.PatchFunc != nil {
		return m.PatchFunc(ctx, filter, update, ops...)
	}
	return nil, nil
}

func (m *MockMainRepo) BulkWrite(ctx context.Context, writeOps []mongo.WriteModel) (*mongo.BulkWriteResult, error) {
	return nil, nil
}

func (m *MockMainRepo) Count(ctx context.Context, filter any, opts ...*options.CountOptions) (int64, error) {
	return 0, nil
}

func (m *MockMainRepo) DeleteMany(ctx context.Context, filter bson.M) error {
	return nil
}
func (m *MockMainRepo) DeleteMain(ctx context.Context, filter bson.M) error {
	return nil
}

func TestReviewSvcs_UpdateApproval_ApproveReview_Success(t *testing.T) {
	mockRepo := &MockMainRepo{}
	service := &reviewsvcs{repo: mockRepo}

	ctx := setupTestContext()
	reviewId := primitive.NewObjectID().Hex()

	// Create expected review
	approvedValue := "approved"
	expectedReview := &models.Review{
		Id:           primitive.NewObjectID(),
		UserId:       primitive.NewObjectID(),
		Date:         time.Now(),
		ReviewStatus: approvedValue, // approved = "approved"
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Setup mock to verify the update and return expected review
	mockRepo.PatchFunc = func(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*models.Review, error) {
		// Verify the update contains the approved field set to true
		setFields, ok := update["$set"].(bson.M)
		assert.True(t, ok, "update should contain $set")
		status, ok := setFields["reviewStatus"]
		assert.True(t, ok, "update should contain reviewStatus field")
		assert.Equal(t, "approved", status, "reviewStatus should be \"approved\"")
		return expectedReview, nil
	}

	// Test approval
	approveValue := "approved"
	dto := &models.ReviewApprovalDto{
		ReviewStatus: approveValue, // approve
	}

	result, err := service.UpdateApproval(ctx, reviewId, dto)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedReview.Id, result.Id)
}

func TestReviewSvcs_UpdateApproval_RejectReview_Success(t *testing.T) {
	mockRepo := &MockMainRepo{}
	service := &reviewsvcs{repo: mockRepo}

	ctx := setupTestContext()
	reviewId := primitive.NewObjectID().Hex()

	// Create expected review
	rejectedValue := "rejected"
	expectedReview := &models.Review{
		Id:           primitive.NewObjectID(),
		ReviewStatus: rejectedValue, // rejected = "rejected"
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Setup mock to verify the update and return expected review
	mockRepo.PatchFunc = func(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*models.Review, error) {
		// Verify the update contains the approved field set to false
		setFields, ok := update["$set"].(bson.M)
		assert.True(t, ok, "update should contain $set")
		status, ok := setFields["reviewStatus"]
		assert.True(t, ok, "update should contain reviewStatus field")
		assert.Equal(t, "rejected", status, "reviewStatus should be \"rejected\"")
		return expectedReview, nil
	}

	// Test rejection
	rejectValue := "rejected"
	dto := &models.ReviewApprovalDto{
		ReviewStatus: rejectValue, // reject
	}

	result, err := service.UpdateApproval(ctx, reviewId, dto)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedReview.Id, result.Id)
}

func TestReviewSvcs_UpdateApproval_ResetToPending_Success(t *testing.T) {
	mockRepo := &MockMainRepo{}
	service := &reviewsvcs{repo: mockRepo}

	ctx := setupTestContext()
	reviewId := primitive.NewObjectID().Hex()

	// Create expected review (approved should become pending)
	expectedReview := &models.Review{
		Id:           primitive.NewObjectID(),
		ReviewStatus: "pending",
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Setup mock to verify the update and return expected review
	mockRepo.PatchFunc = func(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*models.Review, error) {
		// Verify the update contains $set for approved == "pending"
		setFields, ok := update["$set"].(bson.M)
		assert.True(t, ok, "update should contain $set")
		status, ok := setFields["reviewStatus"]
		assert.True(t, ok, "update should contain reviewStatus field")
		assert.Equal(t, "pending", status, "reviewStatus should be \"pending\"")
		return expectedReview, nil
	}

	// Test reset to pending
	dto := &models.ReviewApprovalDto{
		ReviewStatus: "pending", // reset to pending
	}

	result, err := service.UpdateApproval(ctx, reviewId, dto)

	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, expectedReview.Id, result.Id)
}

func TestReviewSvcs_UpdateApproval_InvalidReviewId_Error(t *testing.T) {
	mockRepo := &MockMainRepo{}
	service := &reviewsvcs{repo: mockRepo}

	ctx := setupTestContext()
	invalidReviewId := "invalid-id"

	approveValue := "approved"
	dto := &models.ReviewApprovalDto{
		ReviewStatus: approveValue,
	}

	result, err := service.UpdateApproval(ctx, invalidReviewId, dto)

	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Contains(t, err.Error(), "invalid review id")
}

func TestReviewSvcs_DeleteReview_Success(t *testing.T) {
	mockRepo := &MockMainRepo{}
	service := &reviewsvcs{repo: mockRepo}

	ctx := setupTestContext()
	reviewId := primitive.NewObjectID().Hex()

	// Setup mock to return success
	mockRepo.PatchFunc = func(ctx context.Context, filter, update bson.M, ops ...*options.FindOneAndUpdateOptions) (*models.Review, error) {
		// Verify the update sets trash to true
		setFields, ok := update["$set"].(bson.M)
		assert.True(t, ok, "update should contain $set")
		trash, ok := setFields["trash"]
		assert.True(t, ok, "update should contain trash field")
		assert.Equal(t, true, trash, "trash should be set to true")
		return &models.Review{}, nil
	}

	err := service.DeleteReview(ctx, reviewId)

	assert.NoError(t, err)
}

func TestReviewSvcs_DeleteReview_InvalidReviewId_Error(t *testing.T) {
	mockRepo := &MockMainRepo{}
	service := &reviewsvcs{repo: mockRepo}

	ctx := setupTestContext()
	invalidReviewId := "invalid-id"

	err := service.DeleteReview(ctx, invalidReviewId)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid review id")
}
