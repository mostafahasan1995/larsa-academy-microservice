package search_test

import (
	"bytes"
	"context"
	"encoding/json"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGlobalSearchAPI(t *testing.T) {
	// 1. Setup Environment
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, db := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	fixture := &helpers.TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        ctx,
		AuthServer: authServer,
		DB:         db,
	}

	// 2. Seed Data
	student := fixture.CreateStudent(t, nil)
	teacher := fixture.CreateTeacher(t, nil)
	fixture.SwitchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Create a Course
	fixture.CreateCourse(t, &models.CourseDto{
		BasicInfo: models.CourseBasicInfo{
			Name:     transl.Localizable[string]{"en": "Math for Beginners"},
			Status:   "active",
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Create a Coaching
	fixture.CreateCoaching(t, &models.CoachingDto{
		Title:       transl.Localizable[string]{"en": "Personal Fitness Coaching"},
		ProgramType: "foundational",
		Coachs:      []primitive.ObjectID{teacher.Id},
		Class:       []string{"Grade 1"},
	})

	// 3. Test Search API
	t.Run("Search Math", func(t *testing.T) {
		// Correctly setup query.Conditions
		reqJSON := `{
			"query": {
				"columns": [
					{"name": "name", "exp": "like", "value": "Math"}
				]
			}
		}`

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/search", bytes.NewBuffer([]byte(reqJSON)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-client", fixture.DBName)
		fixture.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res models.GlobalSearchResponse
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)

		assert.NotEmpty(t, res.Results)
		assert.Contains(t, res.Results[0].Name["en"], "Math")
		assert.Equal(t, "course", res.Results[0].Type)
	})

	t.Run("Search Coaching", func(t *testing.T) {
		reqJSON := `{
			"query": {
				"columns": [
					{"name": "name", "exp": "like", "value": "Fitness"}
				]
			}
		}`

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/search", bytes.NewBuffer([]byte(reqJSON)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-client", fixture.DBName)
		fixture.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res models.GlobalSearchResponse
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)

		assert.NotEmpty(t, res.Results)
		assert.Contains(t, res.Results[0].Name["en"], "Fitness")
		assert.Equal(t, "coaching", res.Results[0].Type)
	})

	t.Run("Search All", func(t *testing.T) {
		reqJSON := `{
			"query": {
				"columns": []
			},
			"limit": 10
		}`

		w := httptest.NewRecorder()
		req, _ := http.NewRequest("POST", "/search", bytes.NewBuffer([]byte(reqJSON)))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-client", fixture.DBName)
		fixture.Router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code)
		var res models.GlobalSearchResponse
		err := json.Unmarshal(w.Body.Bytes(), &res)
		require.NoError(t, err)

		// Should find both the course and the coaching
		assert.GreaterOrEqual(t, len(res.Results), 2)
	})
}
