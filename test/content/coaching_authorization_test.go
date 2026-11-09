package content

import (
	"bytes"
	"context"
	"encoding/json"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCoachingAuthorization_AdminCanViewAllCoachings(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCoachingPrograms"}
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	coaching := fixture.createCoaching(t, &models.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Switch to admin context with ViewAllCoachingPrograms capability
	adminUserId := primitive.NewObjectID().Hex()
	fixture.switchAuthContext(adminUserId, []string{"authenticate", "ViewAllCoachingPrograms"})
	w := fixture.callGetCoachingById(t, coaching.Id.Hex())

	require.Equal(t, http.StatusOK, w.Code, "Admin should be able to view coaching: %s", w.Body.String())
	var response models.EnrichedCoaching
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, coaching.Id, response.Id)
}

func TestCoachingAuthorization_TeacherCanViewAssignedCoachings(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCoachingPrograms"}
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	otherTeacher := fixture.createTeacher(t, nil)

	assignedCoaching := fixture.createCoaching(t, &models.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	unassignedCoaching := fixture.createCoaching(t, &models.CoachingDto{
		Coachs: []primitive.ObjectID{otherTeacher.Id},
	})

	// Test teacher can view assigned coaching
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewAssignedCoaching"})
	w := fixture.callGetCoachingById(t, assignedCoaching.Id.Hex())
	if w.Code != http.StatusOK {
		t.Logf("Response Body: %s", w.Body.String())
	}
	require.Equal(t, http.StatusOK, w.Code, "Teacher should be able to view assigned coaching")

	// Test teacher cannot view unassigned coaching
	w2 := fixture.callGetCoachingById(t, unassignedCoaching.Id.Hex())
	require.Equal(t, http.StatusForbidden, w2.Code, "Teacher should not be able to view unassigned coaching")
}

func TestCoachingAuthorization_StudentCanViewEnrolledCoachings(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCoachingPrograms"}
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &models.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Test without permissions
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})
	w := fixture.callGetCoachingById(t, coaching.Id.Hex())
	// Student with only authenticate capability should receive 403 Forbidden
	assert.Equal(t, http.StatusForbidden, w.Code, "Student without permissions should not view coaching: %s", w.Body.String())

	// Enroll and test with ViewPublicCoaching
	fixture.enrollStudentInCoaching(t, student.Id, coaching.Id)
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", "ViewPublicCoaching"})
	w = fixture.callGetCoachingById(t, coaching.Id.Hex())
	require.Equal(t, http.StatusOK, w.Code, "Enrolled student should be able to view coaching")
}

func TestCoachingAuthorization_ManageCoachingPricing(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCoachingPrograms"}
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
	coaching := fixture.createCoaching(t, &models.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	pricingDto := &models.CoachingPricingDto{
		Name:             transl.Localizable[string]{"en": "Standard"},
		SubscriptionType: "paid",
		PricePerSession:  100,
		Currency:         "USD",
		Active:           true,
	}

	// User without ManageCoachingPricing
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	w := callAddCoachingPricing(t, fixture, coaching.Id.Hex(), pricingDto)
	require.Equal(t, http.StatusForbidden, w.Code)

	// User with ManageCoachingPricing
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ManageCoachingPricing"})
	w = callAddCoachingPricing(t, fixture, coaching.Id.Hex(), pricingDto)
	assert.Contains(t, []int{http.StatusOK, http.StatusCreated}, w.Code, "Expected 200 OK or 201 Created")
}

func callAddCoachingPricing(t *testing.T, fixture *TestFixture, coachingId string, dto *models.CoachingPricingDto) *httptest.ResponseRecorder {
	body, _ := json.Marshal(dto)
	url := "/coaching/" + coachingId + "/pricing"
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}
