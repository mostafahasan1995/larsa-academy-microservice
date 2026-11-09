package contact

import (
	"context"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestUserLookupSvc_GetUsersBasicInfo(t *testing.T) {
	// Setup test database
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	// Setup mock auth server
	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	// Setup full test app
	injector, _, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	// Get service
	userLookupSvc := do.MustInvoke[contact.UserLookupSvc](injector)

	// Set up context with app config
	ctx := context.Background()
	hp := &common.HeaderParams{
		Client:        dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create test data
	teacher1 := createTestTeacher(t, database, ctx, "John", "Teacher", "john.teacher@test.com")
	teacher2 := createTestTeacher(t, database, ctx, "Jane", "Teacher", "jane.teacher@test.com")
	supervisor1 := createTestSupervisor(t, database, ctx, "Bob", "Supervisor", "bob.supervisor@test.com")
	supervisor2 := createTestSupervisor(t, database, ctx, "Alice", "Supervisor", "alice.supervisor@test.com")
	user1 := createTestUser(t, database, ctx, "Admin", "User", "admin.user@test.com")
	user2 := createTestUser(t, database, ctx, "Regular", "User", "regular.user@test.com")

	t.Run("GetUsersBasicInfo_TeachersOnly_ReturnsCorrectData", func(t *testing.T) {
		userIds := []primitive.ObjectID{teacher1.Id, teacher2.Id}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		require.Len(t, result, 2)

		// Check teacher1
		info1, ok := result[teacher1.Id]
		require.True(t, ok, "Teacher1 should be in result")
		assert.Equal(t, "John Teacher", info1.Name.GetContentByLang("en"))
		assert.Equal(t, "john.teacher@test.com", info1.Email)
		assert.Equal(t, "teacher", info1.Type)
		assert.Equal(t, teacher1.Id, info1.Id)

		// Check teacher2
		info2, ok := result[teacher2.Id]
		require.True(t, ok, "Teacher2 should be in result")
		assert.Equal(t, "Jane Teacher", info2.Name.GetContentByLang("en"))
		assert.Equal(t, "jane.teacher@test.com", info2.Email)
		assert.Equal(t, "teacher", info2.Type)
		assert.Equal(t, teacher2.Id, info2.Id)
	})

	t.Run("GetUsersBasicInfo_SupervisorsOnly_ReturnsCorrectData", func(t *testing.T) {
		userIds := []primitive.ObjectID{supervisor1.Id, supervisor2.Id}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		require.Len(t, result, 2)

		// Check supervisor1
		info1, ok := result[supervisor1.Id]
		require.True(t, ok, "Supervisor1 should be in result")
		assert.Equal(t, "Bob Supervisor", info1.Name.GetContentByLang("en"))
		assert.Equal(t, "bob.supervisor@test.com", info1.Email)
		assert.Equal(t, "supervisor", info1.Type)
		assert.Equal(t, supervisor1.Id, info1.Id)

		// Check supervisor2
		info2, ok := result[supervisor2.Id]
		require.True(t, ok, "Supervisor2 should be in result")
		assert.Equal(t, "Alice Supervisor", info2.Name.GetContentByLang("en"))
		assert.Equal(t, "alice.supervisor@test.com", info2.Email)
		assert.Equal(t, "supervisor", info2.Type)
		assert.Equal(t, supervisor2.Id, info2.Id)
	})

	t.Run("GetUsersBasicInfo_UsersOnly_ReturnsCorrectData", func(t *testing.T) {
		userIds := []primitive.ObjectID{user1["_id"].(primitive.ObjectID), user2["_id"].(primitive.ObjectID)}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		require.Len(t, result, 2)

		user1Id := user1["_id"].(primitive.ObjectID)
		user2Id := user2["_id"].(primitive.ObjectID)

		// Check user1
		info1, ok := result[user1Id]
		require.True(t, ok, "User1 should be in result")
		assert.Equal(t, "Admin User", info1.Name.GetContentByLang("en"))
		assert.Equal(t, "admin.user@test.com", info1.Email)
		assert.Equal(t, "user", info1.Type)
		assert.Equal(t, user1Id, info1.Id)

		// Check user2
		info2, ok := result[user2Id]
		require.True(t, ok, "User2 should be in result")
		assert.Equal(t, "Regular User", info2.Name.GetContentByLang("en"))
		assert.Equal(t, "regular.user@test.com", info2.Email)
		assert.Equal(t, "user", info2.Type)
		assert.Equal(t, user2Id, info2.Id)
	})

	t.Run("GetUsersBasicInfo_MixedTypes_ReturnsCorrectData", func(t *testing.T) {
		userIds := []primitive.ObjectID{
			teacher1.Id,
			supervisor1.Id,
			user1["_id"].(primitive.ObjectID),
			teacher2.Id,
			supervisor2.Id,
			user2["_id"].(primitive.ObjectID),
		}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		require.Len(t, result, 6, "Should return all 6 users")

		// Verify all types are correctly identified
		assert.Equal(t, "teacher", result[teacher1.Id].Type)
		assert.Equal(t, "teacher", result[teacher2.Id].Type)
		assert.Equal(t, "supervisor", result[supervisor1.Id].Type)
		assert.Equal(t, "supervisor", result[supervisor2.Id].Type)
		assert.Equal(t, "user", result[user1["_id"].(primitive.ObjectID)].Type)
		assert.Equal(t, "user", result[user2["_id"].(primitive.ObjectID)].Type)
	})

	t.Run("GetUsersBasicInfo_EmptyList_ReturnsEmptyMap", func(t *testing.T) {
		userIds := []primitive.ObjectID{}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		assert.Empty(t, result)
	})

	t.Run("GetUsersBasicInfo_NonExistentIds_ReturnsEmptyForMissing", func(t *testing.T) {
		nonExistentId := primitive.NewObjectID()
		userIds := []primitive.ObjectID{teacher1.Id, nonExistentId, teacher2.Id}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		// Should only return the 2 existing teachers
		assert.Len(t, result, 2)
		assert.Contains(t, result, teacher1.Id)
		assert.Contains(t, result, teacher2.Id)
		assert.NotContains(t, result, nonExistentId)
	})

	t.Run("GetUsersBasicInfo_TeacherWithoutEmail_HandlesGracefully", func(t *testing.T) {
		teacherNoEmail := createTestTeacherWithoutEmail(t, database, ctx, "NoEmail", "Teacher")
		userIds := []primitive.ObjectID{teacherNoEmail.Id}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		require.Len(t, result, 1)
		info, ok := result[teacherNoEmail.Id]
		require.True(t, ok)
		assert.Equal(t, "NoEmail Teacher", info.Name.GetContentByLang("en"))
		assert.Empty(t, info.Email, "Email should be empty if not set")
		assert.Equal(t, "teacher", info.Type)
	})

	t.Run("GetUsersBasicInfo_FallbackToUsersCollection_Works", func(t *testing.T) {
		// Test that if a user is not found in teachers or supervisors,
		// it falls back to users collection
		user1Id := user1["_id"].(primitive.ObjectID)
		userIds := []primitive.ObjectID{user1Id}
		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		require.Len(t, result, 1)
		info, ok := result[user1Id]
		require.True(t, ok)
		assert.Equal(t, "Admin User", info.Name.GetContentByLang("en"))
		assert.Equal(t, "admin.user@test.com", info.Email)
		assert.Equal(t, "user", info.Type)
	})

	t.Run("GetUsersBasicInfo_LargeBatch_HandlesEfficiently", func(t *testing.T) {
		// Create multiple users of different types
		userIds := make([]primitive.ObjectID, 0)
		expectedCount := 0

		// Add existing users
		userIds = append(userIds, teacher1.Id, teacher2.Id)
		expectedCount += 2
		userIds = append(userIds, supervisor1.Id, supervisor2.Id)
		expectedCount += 2
		userIds = append(userIds, user1["_id"].(primitive.ObjectID), user2["_id"].(primitive.ObjectID))
		expectedCount += 2

		// Create additional users
		for i := 0; i < 5; i++ {
			teacher := createTestTeacher(t, database, ctx, "Batch", "Teacher", helpers.RandomEmail())
			userIds = append(userIds, teacher.Id)
			expectedCount++
		}

		result, err := userLookupSvc.GetUsersBasicInfo(ctx, userIds)

		require.NoError(t, err)
		assert.Equal(t, expectedCount, len(result), "Should return all users")
	})
}

// Helper functions to create test data

func createTestTeacher(t *testing.T, db *mongo.Database, ctx context.Context, firstName, lastName, email string) models.Teacher {
	teacher := models.Teacher{
		Id: primitive.NewObjectID(),
		TeacherDto: models.TeacherDto{
			FirstName: transl.Localizable[string]{"en": firstName},
			LastName:  transl.Localizable[string]{"en": lastName},
			AccountSettings: models.AccountSettings{
				Email:      email,
				InitialPass: "password123",
				ConfirmPass: "password123",
				JoinStatus:  "active",
				IsActive:   true,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
	}

	_, err := db.Collection("academyTeachers").InsertOne(ctx, teacher)
	require.NoError(t, err, "Failed to create test teacher")
	return teacher
}

func createTestTeacherWithoutEmail(t *testing.T, db *mongo.Database, ctx context.Context, firstName, lastName string) models.Teacher {
	teacher := models.Teacher{
		Id: primitive.NewObjectID(),
		TeacherDto: models.TeacherDto{
			FirstName: transl.Localizable[string]{"en": firstName},
			LastName:  transl.Localizable[string]{"en": lastName},
			// No AccountSettings - no email
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
	}

	_, err := db.Collection("academyTeachers").InsertOne(ctx, teacher)
	require.NoError(t, err, "Failed to create test teacher without email")
	return teacher
}

func createTestSupervisor(t *testing.T, db *mongo.Database, ctx context.Context, firstName, lastName, email string) models.Supervisor {
	supervisor := models.Supervisor{
		Id: primitive.NewObjectID(),
		SupervisorDTO: models.SupervisorDTO{
			FirstName: transl.Localizable[string]{"en": firstName},
			LastName:  transl.Localizable[string]{"en": lastName},
			AccountSettings: models.AccountSettings{
				Email:      email,
				InitialPass: "password123",
				ConfirmPass: "password123",
				JoinStatus:  "active",
				IsActive:   true,
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
	}

	_, err := db.Collection("academySupervisors").InsertOne(ctx, supervisor)
	require.NoError(t, err, "Failed to create test supervisor")
	return supervisor
}

func createTestUser(t *testing.T, db *mongo.Database, ctx context.Context, firstName, lastName, email string) bson.M {
	user := bson.M{
		"_id":       primitive.NewObjectID(),
		"firstName": firstName,
		"lastName":  lastName,
		"email":     email,
		"createdAt": time.Now(),
		"updatedAt": time.Now(),
		"trash":     false,
	}

	_, err := db.Collection("users").InsertOne(ctx, user)
	require.NoError(t, err, "Failed to create test user")
	return user
}

