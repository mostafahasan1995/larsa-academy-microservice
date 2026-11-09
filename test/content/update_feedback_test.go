package content

import (
	"bytes"
	"context"
	"encoding/json"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestUpdateStudentSessionFeedback_TeacherUpdatesTeacherFields(t *testing.T) {
	// Setup
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create session
	sessionId := primitive.NewObjectID()
	session := contentmodels.StudentSession{
		Id:                sessionId,
		StudentId:         student.Id,
		CoachingProgramId: coaching.Id,
		TeacherId:         teacher.Id,
		SessionNum:        0,
		Date:              time.Now(),
		From:              "10:00",
		To:                "11:00",
		Status:            contentenums.StudentSessionStatusCompleted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}
	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
	require.NoError(t, err)

	// Switch to teacher context
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

	// Prepare update data - use map to only include fields we want
	feedbackDate := time.Now()
	updateData := map[string]interface{}{
		"teacherFeedback":     "Great progress today!",
		"teacherRating":       5,
		"teacherFeedbackDate": feedbackDate,
		"studentEngagement":   85,
	}

	// Make API call
	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response contentmodels.StudentSession
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.NotNil(t, response.Feedback)
	assert.Equal(t, "Great progress today!", *response.Feedback.TeacherFeedback.Value)
	assert.Equal(t, 5, *response.Feedback.TeacherRating.Value)
	assert.Equal(t, 85, *response.Feedback.StudentEngagement.Value)
	assert.NotNil(t, response.Feedback.TeacherFeedbackDate)
}

func TestUpdateStudentSessionFeedback_StudentUpdatesStudentFields(t *testing.T) {
	// Setup
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create session
	sessionId := primitive.NewObjectID()
	session := contentmodels.StudentSession{
		Id:                sessionId,
		StudentId:         student.Id,
		CoachingProgramId: coaching.Id,
		TeacherId:         teacher.Id,
		SessionNum:        0,
		Date:              time.Now(),
		From:              "10:00",
		To:                "11:00",
		Status:            contentenums.StudentSessionStatusCompleted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}
	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
	require.NoError(t, err)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Prepare update data - use map to only include fields we want
	feedbackDate := time.Now()
	updateData := map[string]interface{}{
		"studentFeedback":     "I enjoyed the session",
		"studentRating":       4,
		"studentFeedbackDate": feedbackDate,
		"teacherFeedback":     "Student can provide teacher feedback",
	}

	// Make API call
	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response contentmodels.StudentSession
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.NotNil(t, response.Feedback)
	assert.Equal(t, "I enjoyed the session", *response.Feedback.StudentFeedback.Value)
	assert.Equal(t, 4, *response.Feedback.StudentRating.Value)
	assert.Equal(t, "Student can provide teacher feedback", *response.Feedback.TeacherFeedback.Value)
	assert.NotNil(t, response.Feedback.StudentFeedbackDate)
}

func TestUpdateStudentSessionFeedback_AdminUpdatesAllFields(t *testing.T) {
	// Setup
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create session
	sessionId := primitive.NewObjectID()
	session := contentmodels.StudentSession{
		Id:                sessionId,
		StudentId:         student.Id,
		CoachingProgramId: coaching.Id,
		TeacherId:         teacher.Id,
		SessionNum:        0,
		Date:              time.Now(),
		From:              "10:00",
		To:                "11:00",
		Status:            contentenums.StudentSessionStatusCompleted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}
	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
	require.NoError(t, err)

	// Switch to admin context
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ManageSessionDetails"})

	// Prepare update data
	teacherFeedbackDate := time.Now()
	studentFeedbackDate := time.Now().Add(1 * time.Hour)
	updateData := contentmodels.UpdateSessionFeedbackDto{
		TeacherFeedback:     types.Nullable[string]{Value: stringPtr("Admin teacher feedback")},
		TeacherRating:       types.Nullable[int]{Value: intPtr(5)},
		TeacherFeedbackDate: &teacherFeedbackDate,
		StudentEngagement:   types.Nullable[int]{Value: intPtr(90)},
		StudentFeedback:     types.Nullable[string]{Value: stringPtr("Admin student feedback")},
		StudentRating:       types.Nullable[int]{Value: intPtr(5)},
		StudentFeedbackDate: &studentFeedbackDate,
	}

	// Make API call
	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

	var response contentmodels.StudentSession
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.NotNil(t, response.Feedback)
	assert.Equal(t, "Admin teacher feedback", *response.Feedback.TeacherFeedback.Value)
	assert.Equal(t, 5, *response.Feedback.TeacherRating.Value)
	assert.Equal(t, 90, *response.Feedback.StudentEngagement.Value)
	assert.Equal(t, "Admin student feedback", *response.Feedback.StudentFeedback.Value)
	assert.Equal(t, 5, *response.Feedback.StudentRating.Value)
}

func TestUpdateStudentSessionFeedback_TeacherCannotUpdateStudentFields(t *testing.T) {
	// Setup
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create session
	sessionId := primitive.NewObjectID()
	session := contentmodels.StudentSession{
		Id:                sessionId,
		StudentId:         student.Id,
		CoachingProgramId: coaching.Id,
		TeacherId:         teacher.Id,
		SessionNum:        0,
		Date:              time.Now(),
		From:              "10:00",
		To:                "11:00",
		Status:            contentenums.StudentSessionStatusCompleted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}
	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
	require.NoError(t, err)

	// Switch to teacher context
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

	// Try to update student feedback (should fail)
	updateData := contentmodels.UpdateSessionFeedbackDto{
		StudentFeedback: types.Nullable[string]{Value: stringPtr("Should not work")},
	}

	// Make API call
	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "teachers cannot update studentFeedback")
}

func TestUpdateStudentSessionFeedback_StudentCannotUpdateTeacherOnlyFields(t *testing.T) {
	// Setup
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create session
	sessionId := primitive.NewObjectID()
	session := contentmodels.StudentSession{
		Id:                sessionId,
		StudentId:         student.Id,
		CoachingProgramId: coaching.Id,
		TeacherId:         teacher.Id,
		SessionNum:        0,
		Date:              time.Now(),
		From:              "10:00",
		To:                "11:00",
		Status:            contentenums.StudentSessionStatusCompleted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}
	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
	require.NoError(t, err)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Try to update teacher rating (should fail)
	updateData := contentmodels.UpdateSessionFeedbackDto{
		TeacherRating: types.Nullable[int]{Value: intPtr(5)},
	}

	// Make API call
	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "students cannot update teacherRating")
}

func TestUpdateStudentSessionFeedback_UnauthorizedUser(t *testing.T) {
	// Setup
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

	// Create test data
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	otherStudent := fixture.createStudent(t, nil)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create session
	sessionId := primitive.NewObjectID()
	session := contentmodels.StudentSession{
		Id:                sessionId,
		StudentId:         student.Id,
		CoachingProgramId: coaching.Id,
		TeacherId:         teacher.Id,
		SessionNum:        0,
		Date:              time.Now(),
		From:              "10:00",
		To:                "11:00",
		Status:            contentenums.StudentSessionStatusCompleted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}
	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
	require.NoError(t, err)

	// Switch to unauthorized user context (other student)
	fixture.switchAuthContext(otherStudent.Id.Hex(), []string{"authenticate"})

	// Try to update feedback
	updateData := contentmodels.UpdateSessionFeedbackDto{
		StudentFeedback: types.Nullable[string]{Value: stringPtr("Should not work")},
	}

	// Make API call
	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "you are not authorized to update feedback")
}

// func TestUpdateStudentSessionFeedback_DeleteFieldsWithNull(t *testing.T) {
// 	// Setup
// 	dbConfig := setup.SetupTestDB(t)
// 	defer dbConfig.Cleanup()

// 	authConfig := setup.DefaultMockAuthConfig()
// 	authServer := setup.NewMockAuthServer(authConfig)
// 	defer authServer.Close()

// 	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

// 	fixture := &TestFixture{
// 		Router:     router,
// 		DBName:     dbConfig.Database,
// 		Ctx:        context.Background(),
// 		AuthServer: authServer,
// 		DB:         database,
// 	}

// 	// Create test data
// 	teacher := fixture.createTeacher(t, nil)
// 	student := fixture.createStudent(t, nil)
// 	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
// 		Coachs: []primitive.ObjectID{teacher.Id},
// 	})

// 	// Create session with existing feedback
// 	session := contentmodels.StudentSession{
// 		Id:                primitive.NewObjectID(),
// 		StudentId:         student.Id,
// 		CoachingProgramId: coaching.Id,
// 		TeacherId:         teacher.Id,
// 		SessionNum:        0,
// 		Date:              time.Now(),
// 		From:              "10:00",
// 		To:                "11:00",
// 		Status:            contentenums.StudentSessionStatusCompleted,
// 		Feedback: &contentmodels.SessionFeedback{
// 			TeacherFeedback: types.Nullable[string]{Value: stringPtr("Existing feedback"), IsExplicitNull: false},
// 			TeacherRating:   types.Nullable[int]{Value: intPtr(4), IsExplicitNull: false},
// 			StudentFeedback: types.Nullable[string]{Value: stringPtr("Existing student feedback"), IsExplicitNull: false},
// 			StudentRating:   types.Nullable[int]{Value: intPtr(5), IsExplicitNull: false},
// 		},
// 		CreatedAt: time.Now(),
// 		UpdatedAt: time.Now(),
// 		Trash:     false,
// 	}

// 	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
// 	require.NoError(t, err)

// 	// Switch to teacher context
// 	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

// 	// Delete teacher feedback by setting to null
// 	// When JSON has explicit null, Nullable will set IsExplicitNull = true
// 	updateDataJSON := `{"teacherFeedback": null, "teacherRating": null}`
// 	body := []byte(updateDataJSON)
// 	url := "/api/coaching/sessions/" + session.Id.Hex() + "/update-feedback"
// 	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("x-access-token", "test-token")
// 	req.Header.Set("x-client", dbConfig.Database)
// 	req = req.WithContext(fixture.Ctx)

// 	w := httptest.NewRecorder()
// 	fixture.Router.ServeHTTP(w, req)

// 	// Assertions
// 	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

// 	var response contentmodels.StudentSession
// 	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

// 	assert.NotNil(t, response.Feedback)
// 	// Fields should be deleted (null)
// 	assert.True(t, response.Feedback.TeacherFeedback.IsExplicitlyNull() || response.Feedback.TeacherFeedback.Value == nil)
// 	assert.True(t, response.Feedback.TeacherRating.IsExplicitlyNull() || response.Feedback.TeacherRating.Value == nil)
// 	// Student fields should remain unchanged
// 	assert.Equal(t, "Existing student feedback", *response.Feedback.StudentFeedback.Value)
// 	assert.Equal(t, 5, *response.Feedback.StudentRating.Value)
// }

func TestUpdateStudentSessionFeedback_InvalidSessionId(t *testing.T) {
	// Setup
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
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

	// Try with invalid session ID
	updateData := contentmodels.UpdateSessionFeedbackDto{
		TeacherFeedback: types.Nullable[string]{Value: stringPtr("Test")},
	}

	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/invalid-id/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusBadRequest, w.Code)
}

func TestUpdateStudentSessionFeedback_SessionNotFound(t *testing.T) {
	// Setup
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
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

	// Try with non-existent session ID
	nonExistentId := primitive.NewObjectID()
	updateData := contentmodels.UpdateSessionFeedbackDto{
		TeacherFeedback: types.Nullable[string]{Value: stringPtr("Test")},
	}

	body, _ := json.Marshal(updateData)
	url := "/api/coaching/sessions/" + nonExistentId.Hex() + "/update-feedback"
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Assertions
	require.Equal(t, http.StatusBadRequest, w.Code)
	assert.Contains(t, w.Body.String(), "error get student session")
}

// func TestUpdateStudentSessionFeedback_UpdateFeedbackDates(t *testing.T) {
// 	// Setup
// 	dbConfig := setup.SetupTestDB(t)
// 	defer dbConfig.Cleanup()

// 	authConfig := setup.DefaultMockAuthConfig()
// 	authServer := setup.NewMockAuthServer(authConfig)
// 	defer authServer.Close()

// 	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

// 	fixture := &TestFixture{
// 		Router:     router,
// 		DBName:     dbConfig.Database,
// 		Ctx:        context.Background(),
// 		AuthServer: authServer,
// 		DB:         database,
// 	}

// 	// Create test data
// 	teacher := fixture.createTeacher(t, nil)
// 	student := fixture.createStudent(t, nil)
// 	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
// 		Coachs: []primitive.ObjectID{teacher.Id},
// 	})

// 	// Create session
// 	sessionId := primitive.NewObjectID()
// 	session := contentmodels.StudentSession{
// 		Id:                sessionId,
// 		StudentId:         student.Id,
// 		CoachingProgramId: coaching.Id,
// 		TeacherId:         teacher.Id,
// 		SessionNum:        0,
// 		Date:              time.Now(),
// 		From:              "10:00",
// 		To:                "11:00",
// 		Status:            contentenums.StudentSessionStatusCompleted,
// 		CreatedAt:         time.Now(),
// 		UpdatedAt:         time.Now(),
// 		Trash:             false,
// 	}
// 	_, err := database.Collection("academyCoachingStudentSessions").InsertOne(fixture.Ctx, session)
// 	require.NoError(t, err)

// 	// Switch to teacher context
// 	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

// 	// Update with feedback dates - use map to only include fields we want
// 	teacherFeedbackDate := time.Now().Add(-2 * time.Hour)
// 	updateData := map[string]interface{}{
// 		"teacherFeedback":     "Feedback with date",
// 		"teacherFeedbackDate": teacherFeedbackDate,
// 	}

// 	body, _ := json.Marshal(updateData)
// 	url := "/api/coaching/sessions/" + sessionId.Hex() + "/update-feedback"
// 	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("x-access-token", "test-token")
// 	req.Header.Set("x-client", dbConfig.Database)
// 	req = req.WithContext(fixture.Ctx)

// 	w := httptest.NewRecorder()
// 	fixture.Router.ServeHTTP(w, req)

// 	// Assertions
// 	require.Equal(t, http.StatusOK, w.Code, w.Body.String())

// 	var response contentmodels.StudentSession
// 	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

// 	assert.NotNil(t, response.Feedback)
// 	assert.NotNil(t, response.Feedback.TeacherFeedbackDate)
// 	// Check that the date is approximately correct (within 1 second)
// 	assert.WithinDuration(t, teacherFeedbackDate, *response.Feedback.TeacherFeedbackDate, 1*time.Second)
// }

// Helper functions
func stringPtr(s string) *string {
	return &s
}

func intPtr(i int) *int {
	return &i
}
