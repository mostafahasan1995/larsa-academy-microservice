package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestContentList_PaginationFavorites(t *testing.T) {
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

	// 1. Create 45 coachings
	teacher := fixture.createTeacher(t, nil)
	coachings := make([]contentmodels.Coaching, 45)
	for i := 0; i < 45; i++ {
		c := contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Coachs: []primitive.ObjectID{teacher.Id},
				Title:  map[string]string{"en": fmt.Sprintf("Coaching %02d", i)},
			},
			Trash:     false,
			CreatedAt: time.Now().Add(time.Duration(i) * time.Minute),
			UpdatedAt: time.Now().Add(time.Duration(i) * time.Minute),
		}
		coachings[i] = c
	}

	// With 45 items and _id: -1 sort, index 30 will be on Page 2
	favoriteId := coachings[30].Id

	// 2. Create a student with favorites
	studentId := primitive.NewObjectID()
	student := models.Student{
		Id: studentId,
		StudentDto: models.StudentDto{
			Favorites: []models.StudentFavorite{
				{Type: "coaching", RefId: favoriteId},
			},
		},
		Trash: false,
	}
	_, err := database.Collection("academyStudents").InsertOne(fixture.Ctx, student)
	require.NoError(t, err)

	// Verify student was inserted correctly
	var checkStudent models.Student
	err = database.Collection("academyStudents").FindOne(fixture.Ctx, bson.M{"_id": studentId}).Decode(&checkStudent)
	require.NoError(t, err)
	t.Logf("Student inserted with ID: %s, favorites count: %d", checkStudent.Id.Hex(), len(checkStudent.Favorites))

	// Update mock auth to return this student ID
	authConfig.UserID = studentId.Hex()
	authConfig.Capabilities = []string{"authenticate", "ViewPublicCoaching"}
	fixture.AuthServer.UpdateConfig(authConfig)

	// Insert coachings
	for _, c := range coachings {
		_, err := database.Collection("academyCoachings").InsertOne(fixture.Ctx, c)
		require.NoError(t, err)
	}

	// 3. Test Page 2 (items 11-20) - Should contain the favorite item
	query := map[string]interface{}{"columns": []interface{}{}}
	body, _ := json.Marshal(query)
	// Page 2: skip=10, limit=10
	url := "/api/coaching/student/list?skip=10&limit=10"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var resp contentmodels.StudentCoachingPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))

	require.Len(t, resp.Coachings, 10)

	// Page 2: items 11-20 (indices 34-25 in descending order)
	// favoriteId is coachings[30].Id, which is the 15th item in descending order (index 14)
	// So it MUST be in Page 2.

	foundFavorite := false
	for i, c := range resp.Coachings {
		if c.Id == favoriteId {
			foundFavorite = true
			require.True(t, c.IsFavorite, fmt.Sprintf("Item %d (ID %s) should be favorite", i, c.Id.Hex()))
		} else {
			require.False(t, c.IsFavorite, fmt.Sprintf("Item %d (ID %s) should NOT be favorite", i, c.Id.Hex()))
		}
	}
	require.True(t, foundFavorite, "Favorite item (index 30) should be in Page 2 (11-20)")

	// 4. Test Page 1 (items 1-10) - Should NOT contain the favorite item
	url = "/api/coaching/student/list?skip=0&limit=10"
	req = httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	for _, c := range resp.Coachings {
		require.False(t, c.IsFavorite, "Page 1 should not have any favorites in this test setup")
	}
}
