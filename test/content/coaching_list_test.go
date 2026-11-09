package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCoachingList_IncludesPlacementTests(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create teacher + coaching
	teacher := fixture.createTeacher(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Insert placement test linked to the coaching
	now := time.Now()
	pt := contentmodels.PlacementTest{
		Id: primitive.NewObjectID(),
		PlacementTestDto: contentmodels.PlacementTestDto{
			Title:  transl.Localizable[string]{"en":  "placement test list"},
			Status:   "active",
			Type:     "electronic",
			RefType:  "coaching",
			RefId:    coaching.Id,
			TestMark: 100,
			Questions: []contentmodels.Question{
				{
					Num: 1,
					Question: map[string]string{
						"en": "q1",
					},
					Answers: []contentmodels.Answer{
						{Num: 1, Answer: map[string]string{"en": "a1"}, IsCorrect: true},
					},
				},
			},
			Settings: contentmodels.PlacementTestSettings{
				AllowRetry:        false,
				NumOfAttempts:     1,
				RetryNeedApproval: false,
				PassScore:         50,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		Trash:     false,
	}

	_, err := database.Collection("academyPlacementTests").InsertOne(fixture.Ctx, pt)
	require.NoError(t, err)

	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	// Call /coaching/list with includePlacementTests
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	body, _ := json.Marshal(query)
	url := "/coaching/list?skip=0&limit=10&includePlacementTests=true"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp contentmodels.CoachingListPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Len(t, resp.Coachings, 1)
	require.Len(t, resp.Coachings[0].PlacementTests, 1)

	gotPT := resp.Coachings[0].PlacementTests[0]
	assert.Equal(t, pt.Id, gotPT.Id)
	assert.Equal(t, pt.TestMark, gotPT.TestMark)
	assert.Equal(t, pt.Status, gotPT.Status)
	assert.Equal(t, 1, gotPT.QuestionsCount)
}

func TestCoachingGetById_EnrichCoachs(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create teacher with localized names
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{
			"en": "John",
			"ar": "جون",
		},
		LastName: transl.Localizable[string]{
			"en": "Doe",
			"ar": "دو",
		},
	})

	// Create coaching with the teacher assigned
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	// Call GET /coaching/{id} with enrichCoachs=true
	url := fmt.Sprintf("/coaching/%s?enrichCoachs=true", coaching.Id.Hex())
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp contentmodels.EnrichedCoaching
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Verify basic coaching data
	assert.Equal(t, coaching.Id, resp.Id)
	assert.Equal(t, []primitive.ObjectID{teacher.Id}, resp.Coachs) // Should still have the original coach IDs

	// Verify enriched coach data
	require.NotNil(t, resp.CoachsObjs)
	require.Len(t, resp.CoachsObjs, 1)

	enrichedCoach := resp.CoachsObjs[0]
	assert.Equal(t, teacher.Id, enrichedCoach.Id)
	assert.Equal(t, transl.Localizable[string]{"en": "John", "ar": "جون"}, enrichedCoach.FirstName)
	assert.Equal(t, transl.Localizable[string]{"en": "Doe", "ar": "دو"}, enrichedCoach.LastName)
	assert.Equal(t, teacher.Picture, enrichedCoach.Picture)
}

func TestCoachingGetById_WithoutEnrichCoachs(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create teacher and coaching
	teacher := fixture.createTeacher(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	// Call GET /coaching/{id} without enrichCoachs parameter
	url := fmt.Sprintf("/coaching/%s", coaching.Id.Hex())
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp contentmodels.Coaching
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	// Verify basic coaching data
	assert.Equal(t, coaching.Id, resp.Id)
	assert.Equal(t, []primitive.ObjectID{teacher.Id}, resp.Coachs) // Should have the coach IDs
}

func TestCoachingList_EnrichCoachs(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create teacher with localized names
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{
			"en": "Jane",
			"ar": "جين",
		},
		LastName: transl.Localizable[string]{
			"en": "Smith",
			"ar": "سميث",
		},
	})

	// Create coaching with the teacher assigned
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	// Call POST /coaching/list with enrichCoachs=true
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	body, _ := json.Marshal(query)
	url := "/coaching/list?skip=0&limit=10&enrichCoachs=true"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp contentmodels.EnrichedCoachingListPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Len(t, resp.Coachings, 1)

	coachingItem := resp.Coachings[0]
	assert.Equal(t, coaching.Id, coachingItem.Id)
	assert.Equal(t, []primitive.ObjectID{teacher.Id}, coachingItem.Coachs) // Should still have the original coach IDs

	// Verify enriched coach data
	require.NotNil(t, coachingItem.CoachsObjs)
	require.Len(t, coachingItem.CoachsObjs, 1)

	enrichedCoach := coachingItem.CoachsObjs[0]
	assert.Equal(t, teacher.Id, enrichedCoach.Id)
	assert.Equal(t, transl.Localizable[string]{"en": "Jane", "ar": "جين"}, enrichedCoach.FirstName)
	assert.Equal(t, transl.Localizable[string]{"en": "Smith", "ar": "سميث"}, enrichedCoach.LastName)
	assert.Equal(t, teacher.Picture, enrichedCoach.Picture)
}

func TestCoachingAll_EnrichCoachs(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create teacher with localized names
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{
			"en": "Bob",
			"ar": "بوب",
		},
		LastName: transl.Localizable[string]{
			"en": "Johnson",
			"ar": "جونسون",
		},
	})

	// Create coaching with the teacher assigned
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	// Call POST /coaching/all with enrichCoachs=true
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	body, _ := json.Marshal(query)
	url := "/coaching/all?enrichCoachs=true"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp []contentmodels.EnrichedCoaching
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Len(t, resp, 1)

	enrichedCoaching := resp[0]
	assert.Equal(t, coaching.Id, enrichedCoaching.Id)
	assert.Equal(t, []primitive.ObjectID{teacher.Id}, enrichedCoaching.Coachs) // Should still have the original coach IDs

	// Verify enriched coach data
	require.NotNil(t, enrichedCoaching.CoachsObjs)
	require.Len(t, enrichedCoaching.CoachsObjs, 1)

	enrichedCoach := enrichedCoaching.CoachsObjs[0]
	assert.Equal(t, teacher.Id, enrichedCoach.Id)
	assert.Equal(t, transl.Localizable[string]{"en": "Bob", "ar": "بوب"}, enrichedCoach.FirstName)
	assert.Equal(t, transl.Localizable[string]{"en": "Johnson", "ar": "جونسون"}, enrichedCoach.LastName)
	assert.Equal(t, teacher.Picture, enrichedCoach.Picture)
}

func TestStudentCoachingList_IncludesPlacementTests(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	teacher := fixture.createTeacher(t, nil)

	now := time.Now()
	coaching := contentmodels.Coaching{
		Id: primitive.NewObjectID(),
		CoachingDto: contentmodels.CoachingDto{
			Coachs: []primitive.ObjectID{teacher.Id},
			Title:  map[string]string{"en": "Test Coaching"},
		},
		Trash:     false,
		CreatedAt: now,
		UpdatedAt: now,
	}

	_, err := database.Collection("academyCoachings").InsertOne(fixture.Ctx, coaching)
	require.NoError(t, err)

	pt := contentmodels.PlacementTest{
		Id: primitive.NewObjectID(),
		PlacementTestDto: contentmodels.PlacementTestDto{
			Title:    transl.Localizable[string]{"en": "student placement test" },
			Status:   "active",
			Type:     "electronic",
			RefType:  "coaching",
			RefId:    coaching.Id,
			TestMark: 100,
			Questions: []contentmodels.Question{
				{
					Num: 1,
					Question: map[string]string{
						"en": "q1",
					},
					Answers: []contentmodels.Answer{
						{Num: 1, Answer: map[string]string{"en": "a1"}, IsCorrect: true},
					},
				},
			},
			Settings: contentmodels.PlacementTestSettings{
				AllowRetry:        false,
				NumOfAttempts:     1,
				RetryNeedApproval: false,
				PassScore:         50,
			},
		},
		CreatedAt: now,
		UpdatedAt: now,
		Trash:     false,
	}

	_, err = database.Collection("academyPlacementTests").InsertOne(fixture.Ctx, pt)
	require.NoError(t, err)

	studentId := primitive.NewObjectID()
	student := models.Student{
		Id: studentId,
		StudentDto: models.StudentDto{
			Favorites: []models.StudentFavorite{},
		},
		Trash: false,
	}
	_, err = database.Collection("academyStudents").InsertOne(fixture.Ctx, student)
	require.NoError(t, err)

	authConfig.UserID = studentId.Hex()
	authConfig.Capabilities = []string{"authenticate", "ViewPublicCoaching"}
	fixture.AuthServer.UpdateConfig(authConfig)

	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	body, _ := json.Marshal(query)
	url := "/api/coaching/student/list?skip=0&limit=10&includePlacementTests=true"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	t.Logf("Response status: %d, body: %s", w.Code, w.Body.String())

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp contentmodels.StudentCoachingPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	t.Logf("Number of coachings returned: %d", len(resp.Coachings))
	for i, c := range resp.Coachings {
		t.Logf("Coaching %d: ID=%s, PlacementTests count=%d", i, c.Id.Hex(), len(c.PlacementTests))
	}

	require.Len(t, resp.Coachings, 1, "Student should see 1 coaching")
	require.Len(t, resp.Coachings[0].PlacementTests, 1, "Placement tests should be included when includePlacementTests=true for student list")

	gotPT := resp.Coachings[0].PlacementTests[0]
	assert.Equal(t, pt.Id, gotPT.Id)
	assert.Equal(t, pt.TestMark, gotPT.TestMark)
	assert.Equal(t, pt.Status, gotPT.Status)
}
