package contact

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// createBundle creates a bundle directly in the database (helper method)
func createBundleForFavoritesTest(t *testing.T, fixture *helpers.TestFixture, dto *contentmodels.BundleDto) contentmodels.Bundle {
	if dto == nil {
		dto = &contentmodels.BundleDto{}
	}

	// Set defaults if not provided
	if len(dto.Name) == 0 {
		dto.Name = transl.Localizable[string]{
			"en": "Test Bundle",
			"ar": "باقة اختبار",
		}
	}
	if dto.Status == "" {
		dto.Status = "active"
	}
	if len(dto.Content) == 0 {
		dto.Content = []contentmodels.BundleContent{}
	}

	bundle := contentmodels.Bundle{
		Id:        primitive.NewObjectID(),
		BundleDto: *dto,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
	}

	_, err := fixture.DB.Collection("academyBundles").InsertOne(fixture.Ctx, bundle)
	require.NoError(t, err, "Failed to create bundle")
	return bundle
}

// callUpdateFavorites makes the API call to PATCH /api/students/me/favorites
func callUpdateFavorites(t *testing.T, fixture *helpers.TestFixture, favorites []models.StudentFavorite) *httptest.ResponseRecorder {
	body, _ := json.Marshal(favorites)
	req := httptest.NewRequest("PATCH", "/api/students/me/favorites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

func callAddFavorite(t *testing.T, fixture *helpers.TestFixture, favorite models.StudentFavorite) *httptest.ResponseRecorder {
	body, _ := json.Marshal(favorite)
	req := httptest.NewRequest("POST", "/api/students/me/favorites", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

func callRemoveFavorite(t *testing.T, fixture *helpers.TestFixture, favoriteType string, refId primitive.ObjectID) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/api/students/me/favorites/%s/%s", favoriteType, refId.Hex())
	req := httptest.NewRequest("DELETE", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

// callListFavorites makes the API call to POST /api/students/me/favorites/list
func callListFavorites(t *testing.T, fixture *helpers.TestFixture, query map[string]interface{}, page, perPage int) *httptest.ResponseRecorder {
	queryBody, _ := json.Marshal(query)
	url := fmt.Sprintf("/api/students/me/favorites/list?page=%d&perPage=%d", page, perPage)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

// TestUpdateFavorites_BasicSuccess tests adding favorites successfully
func TestUpdateFavorites_BasicSuccess(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	coaching := fixture.CreateCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	bundle := createBundleForFavoritesTest(t, fixture, &contentmodels.BundleDto{})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Update favorites
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: course.Id, Meta: map[string]any{"note": "Great course"}},
		{Type: "coaching", RefId: coaching.Id},
		{Type: "bundle", RefId: bundle.Id},
	}
	w := callUpdateFavorites(t, fixture, favorites)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())

	var studentResponse models.Student
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &studentResponse))

	assert.Len(t, studentResponse.Favorites, 3)
	assert.Equal(t, "course", studentResponse.Favorites[0].Type)
	assert.Equal(t, course.Id, studentResponse.Favorites[0].RefId)
	assert.Equal(t, "Great course", studentResponse.Favorites[0].Meta["note"])
	assert.Equal(t, "coaching", studentResponse.Favorites[1].Type)
	assert.Equal(t, coaching.Id, studentResponse.Favorites[1].RefId)
	assert.Equal(t, "bundle", studentResponse.Favorites[2].Type)
	assert.Equal(t, bundle.Id, studentResponse.Favorites[2].RefId)
}

// TestUpdateFavorites_ReplaceExisting tests that updating favorites replaces the entire array
func TestUpdateFavorites_ReplaceExisting(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)
	course1 := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	course2 := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	course3 := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// First update - add 3 favorites
	favorites1 := []models.StudentFavorite{
		{Type: "course", RefId: course1.Id},
		{Type: "course", RefId: course2.Id},
		{Type: "course", RefId: course3.Id},
	}
	w1 := callUpdateFavorites(t, fixture, favorites1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Second update - replace with only 1 favorite
	favorites2 := []models.StudentFavorite{
		{Type: "course", RefId: course1.Id},
	}
	w2 := callUpdateFavorites(t, fixture, favorites2)
	require.Equal(t, http.StatusOK, w2.Code)

	var studentResponse models.Student
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &studentResponse))

	// Should only have 1 favorite now (replaced, not appended)
	assert.Len(t, studentResponse.Favorites, 1)
	assert.Equal(t, course1.Id, studentResponse.Favorites[0].RefId)
}

// TestUpdateFavorites_InvalidType tests validation of invalid favorite type
func TestUpdateFavorites_InvalidType(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Try to add favorite with invalid type
	favorites := []models.StudentFavorite{
		{Type: "invalid_type", RefId: primitive.NewObjectID()},
	}
	w := callUpdateFavorites(t, fixture, favorites)

	// Should fail validation
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// TestUpdateFavorites_NonExistentRef tests validation of non-existent referenced item
func TestUpdateFavorites_NonExistentRef(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Try to add favorite with non-existent course ID
	nonExistentID := primitive.NewObjectID()
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: nonExistentID},
	}
	w := callUpdateFavorites(t, fixture, favorites)

	// Should fail because course doesn't exist
	assert.NotEqual(t, http.StatusOK, w.Code)
}

// TestUpdateFavorites_DuplicatesInArray tests updating favorites with duplicates within the array (should deduplicate)
func TestUpdateFavorites_DuplicatesInArray(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	teacher := fixture.CreateTeacher(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Update with duplicate favorites in the same array - should deduplicate and succeed
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: course.Id},
		{Type: "course", RefId: course.Id}, // Duplicate - should be removed
	}
	w := callUpdateFavorites(t, fixture, favorites)
	require.Equal(t, http.StatusOK, w.Code)

	var response models.Student
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	// Should only have 1 unique favorite (duplicates removed)
	assert.Len(t, response.Favorites, 1)
	assert.Equal(t, "course", response.Favorites[0].Type)
	assert.Equal(t, course.Id, response.Favorites[0].RefId)
}

// TestListFavorites_BasicSuccess tests listing favorites with enrichment
func TestListFavorites_BasicSuccess(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name:     transl.Localizable[string]{"en": "Test Course", "ar": "دورة اختبار"},
		},
	})
	coaching := fixture.CreateCoaching(t, &contentmodels.CoachingDto{
		Coachs:      []primitive.ObjectID{teacher.Id},
		Title:       transl.Localizable[string]{"en": "Test Coaching", "ar": "برنامج اختبار"},
		ProgramType: "foundational",
		Class:       []string{"Grade 1"},
	})
	bundle := createBundleForFavoritesTest(t, fixture, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{"en": "Test Bundle", "ar": "باقة اختبار"},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add favorites
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: course.Id},
		{Type: "coaching", RefId: coaching.Id},
		{Type: "bundle", RefId: bundle.Id},
	}
	wUpdate := callUpdateFavorites(t, fixture, favorites)
	require.Equal(t, http.StatusOK, wUpdate.Code)

	// List favorites
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := callListFavorites(t, fixture, query, 1, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())

	var response models.StudentFavoritePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, int64(3), response.Pagination.TotalCount)
	assert.Len(t, response.Favorites, 3)

	// Check enrichment - each favorite should have Data populated
	for _, fav := range response.Favorites {
		assert.NotNil(t, fav.Data, "Favorite should have enriched data")
		dataMap, ok := fav.Data.(map[string]interface{})
		require.True(t, ok, "Data should be a map")

		switch fav.Type {
		case "course":
			assert.Equal(t, course.Id.Hex(), dataMap["_id"].(string))
			assert.NotNil(t, dataMap["basicInfo"])
		case "coaching":
			assert.Equal(t, coaching.Id.Hex(), dataMap["_id"].(string))
			assert.NotNil(t, dataMap["title"])
		case "bundle":
			assert.Equal(t, bundle.Id.Hex(), dataMap["_id"].(string))
			assert.NotNil(t, dataMap["name"])
		}
	}
}

// TestListFavorites_EmptyList tests listing when student has no favorites
func TestListFavorites_EmptyList(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// List favorites (student has none)
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := callListFavorites(t, fixture, query, 1, 10)

	require.Equal(t, http.StatusOK, w.Code)

	var response models.StudentFavoritePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, int64(0), response.Pagination.TotalCount)
	assert.Len(t, response.Favorites, 0)
}

// TestListFavorites_Pagination tests pagination of favorites
func TestListFavorites_Pagination(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)

	// Create 5 courses
	var courseIds []primitive.ObjectID
	for i := 0; i < 5; i++ {
		course := fixture.CreateCourse(t, &contentmodels.CourseDto{
			BasicInfo: contentmodels.CourseBasicInfo{
				Teachers: []primitive.ObjectID{teacher.Id},
			},
		})
		courseIds = append(courseIds, course.Id)
	}

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add all 5 favorites
	favorites := make([]models.StudentFavorite, 5)
	for i, courseId := range courseIds {
		favorites[i] = models.StudentFavorite{Type: "course", RefId: courseId}
	}
	wUpdate := callUpdateFavorites(t, fixture, favorites)
	require.Equal(t, http.StatusOK, wUpdate.Code)

	// First page - perPage 2
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w1 := callListFavorites(t, fixture, query, 1, 2)
	require.Equal(t, http.StatusOK, w1.Code)

	var response1 models.StudentFavoritePagination
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &response1))

	assert.Equal(t, int64(5), response1.Pagination.TotalCount)
	assert.Len(t, response1.Favorites, 2)

	// Second page - page 2, perPage 2
	w2 := callListFavorites(t, fixture, query, 2, 2)
	require.Equal(t, http.StatusOK, w2.Code)

	var response2 models.StudentFavoritePagination
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &response2))

	assert.Equal(t, int64(5), response2.Pagination.TotalCount)
	assert.Len(t, response2.Favorites, 2)

	// Third page - page 3, perPage 2
	w3 := callListFavorites(t, fixture, query, 3, 2)
	require.Equal(t, http.StatusOK, w3.Code)

	var response3 models.StudentFavoritePagination
	require.NoError(t, json.Unmarshal(w3.Body.Bytes(), &response3))

	assert.Equal(t, int64(5), response3.Pagination.TotalCount)
	assert.Len(t, response3.Favorites, 1)
}

// TestListFavorites_FilterByType tests filtering favorites by type
func TestListFavorites_FilterByType(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	coaching := fixture.CreateCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	bundle := createBundleForFavoritesTest(t, fixture, &contentmodels.BundleDto{})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add favorites of different types
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: course.Id},
		{Type: "coaching", RefId: coaching.Id},
		{Type: "bundle", RefId: bundle.Id},
	}
	wUpdate := callUpdateFavorites(t, fixture, favorites)
	require.Equal(t, http.StatusOK, wUpdate.Code)

	// Filter by type "course"
	query := map[string]interface{}{
		"columns": []map[string]interface{}{
			{
				"name":  "type",
				"exp":   "eq",
				"value": "course",
			},
		},
	}
	w := callListFavorites(t, fixture, query, 1, 10)

	require.Equal(t, http.StatusOK, w.Code)

	var response models.StudentFavoritePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	// Should only return course favorites
	assert.Equal(t, int64(1), response.Pagination.TotalCount)
	assert.Len(t, response.Favorites, 1)
	assert.Equal(t, "course", response.Favorites[0].Type)
	assert.Equal(t, course.Id, response.Favorites[0].RefId)
}

// TestGetMe_IncludesFavorites tests that GET /me includes favorites in response
func TestGetMe_IncludesFavorites(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add favorite
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: course.Id},
	}
	wUpdate := callUpdateFavorites(t, fixture, favorites)
	require.Equal(t, http.StatusOK, wUpdate.Code)

	// Get profile via GET /me
	req := httptest.NewRequest("GET", "/api/students/me", nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var profile models.StudentProfileResponseDto
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &profile))

	// Profile should include favorites
	assert.Len(t, profile.Favorites, 1)
	assert.Equal(t, "course", profile.Favorites[0].Type)
	assert.Equal(t, course.Id, profile.Favorites[0].RefId)
}

// TestUpdateMe_PreservesFavorites tests that PATCH /me does not update favorites
func TestUpdateMe_PreservesFavorites(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create test data
	teacher := fixture.CreateTeacher(t, nil)
	student := fixture.CreateStudent(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add favorite first
	favorites := []models.StudentFavorite{
		{Type: "course", RefId: course.Id},
	}
	wUpdate := callUpdateFavorites(t, fixture, favorites)
	require.Equal(t, http.StatusOK, wUpdate.Code)

	// Update profile via PATCH /me (without favorites in body)
	// Use the student we already have - just update the names, preserve everything else
	// Note: ParentPhoneNumber validation requires both prefix and content if present
	// Since ParentPhoneNumber is optional, we need to ensure it has valid values if we include it
	contactInfo := models.ContactInfo{
		MobilePhoneNumber: student.ContactInfo.MobilePhoneNumber,
	}
	// Set ParentPhoneNumber - if student has one, use it; otherwise set a valid dummy value
	// to pass validation (the service will preserve the existing value anyway)
	if student.ContactInfo.ParentPhoneNumber.Prefix != "" && student.ContactInfo.ParentPhoneNumber.Content != "" {
		contactInfo.ParentPhoneNumber = student.ContactInfo.ParentPhoneNumber
	} else {
		// Set a valid dummy value to pass validation
		// The service will preserve the existing (empty) value from the database
		contactInfo.ParentPhoneNumber = models.PhoneNumber{
			Prefix:  "+1",
			Content: "1234567890",
		}
	}

	updateData := models.StudentProfileUpdateDto{
		GeneralInfo: models.GeneralInfoPublic{
			FirstName:   transl.Localizable[string]{"en": "Updated", "ar": "محدث"},
			LastName:    transl.Localizable[string]{"en": "Name", "ar": "اسم"},
			Gender:      student.GeneralInfo.Gender,
			DateOfBirth: student.GeneralInfo.DateOfBirth,
			Nationality: student.GeneralInfo.Nationality,
			Picture:     student.GeneralInfo.Picture,
		},
		ContactInfo:    contactInfo,
		AddressDetails: student.AddressDetails,
	}
	body, _ := json.Marshal(updateData)
	req := httptest.NewRequest("PATCH", "/api/students/me", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Logf("UpdateMe failed with status %d, body: %s", w.Code, w.Body.String())
	}
	require.Equal(t, http.StatusOK, w.Code, "UpdateMe should succeed. Response: %s", w.Body.String())

	var profile models.StudentProfileResponseDto
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &profile))

	// Favorites should still be preserved
	assert.Len(t, profile.Favorites, 1)
	assert.Equal(t, "course", profile.Favorites[0].Type)
	assert.Equal(t, course.Id, profile.Favorites[0].RefId)
}

// TestAddFavorite_BasicSuccess tests adding a single favorite successfully
func TestAddFavorite_BasicSuccess(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	teacher := fixture.CreateTeacher(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add a favorite
	favorite := models.StudentFavorite{
		Type:  "course",
		RefId: course.Id,
	}
	w := callAddFavorite(t, fixture, favorite)
	require.Equal(t, http.StatusOK, w.Code)

	var response models.Student
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	// Should have 1 favorite
	assert.Len(t, response.Favorites, 1)
	assert.Equal(t, "course", response.Favorites[0].Type)
	assert.Equal(t, course.Id, response.Favorites[0].RefId)
}

// TestAddFavorite_Duplicate tests adding a duplicate favorite
func TestAddFavorite_Duplicate(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	teacher := fixture.CreateTeacher(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Add a favorite
	favorite := models.StudentFavorite{
		Type:  "course",
		RefId: course.Id,
	}
	w1 := callAddFavorite(t, fixture, favorite)
	require.Equal(t, http.StatusOK, w1.Code)

	// Try to add the same favorite again
	w2 := callAddFavorite(t, fixture, favorite)
	require.Equal(t, http.StatusBadRequest, w2.Code) // Should fail with duplicate error
}

// TestRemoveFavorite_BasicSuccess tests removing a favorite successfully
func TestRemoveFavorite_BasicSuccess(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	teacher := fixture.CreateTeacher(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// First add a favorite
	favorite := models.StudentFavorite{
		Type:  "course",
		RefId: course.Id,
	}
	wAdd := callAddFavorite(t, fixture, favorite)
	require.Equal(t, http.StatusOK, wAdd.Code)

	// Now remove it
	wRemove := callRemoveFavorite(t, fixture, "course", course.Id)
	require.Equal(t, http.StatusOK, wRemove.Code)

	var response models.Student
	require.NoError(t, json.Unmarshal(wRemove.Body.Bytes(), &response))

	// Should have 0 favorites
	assert.Len(t, response.Favorites, 0)
}

// TestRemoveFavorite_NotFound tests removing a favorite that doesn't exist
func TestRemoveFavorite_NotFound(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	student := fixture.CreateStudent(t, nil)
	teacher := fixture.CreateTeacher(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Switch to student context
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Try to remove a favorite that was never added
	w := callRemoveFavorite(t, fixture, "course", course.Id)
	require.Equal(t, http.StatusBadRequest, w.Code) // Should fail with not found error
}
