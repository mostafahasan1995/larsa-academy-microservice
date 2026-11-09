package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// TestFixture embeds the shared TestFixture and adds content-specific methods
// Fields are exposed directly for backward compatibility with existing tests
type TestFixture struct {
	*helpers.TestFixture
	Router     *chi.Mux
	DBName     string
	Ctx        context.Context
	AuthServer *setup.MockAuthServer
	DB         *mongo.Database
}

// initSharedFixture initializes the embedded shared TestFixture
func (f *TestFixture) initSharedFixture() {
	if f.TestFixture == nil {
		f.TestFixture = &helpers.TestFixture{
			Router:     f.Router,
			DBName:     f.DBName,
			Ctx:        f.Ctx,
			AuthServer: f.AuthServer,
			DB:         f.DB,
		}
	}
}

// createTeacher delegates to shared helper
func (f *TestFixture) createTeacher(t *testing.T, dto *models.TeacherDto) models.Teacher {
	f.initSharedFixture()
	return f.TestFixture.CreateTeacher(t, dto)
}

// createStudent delegates to shared helper
func (f *TestFixture) createStudent(t *testing.T, dto *models.StudentDto) models.Student {
	f.initSharedFixture()
	return f.TestFixture.CreateStudent(t, dto)
}

// createCourse delegates to shared helper
func (f *TestFixture) createCourse(t *testing.T, dto *contentmodels.CourseDto) contentmodels.Course {
	f.initSharedFixture()
	return f.TestFixture.CreateCourse(t, dto)
}

// createCoaching delegates to shared helper
func (f *TestFixture) createCoaching(t *testing.T, dto *contentmodels.CoachingDto) contentmodels.Coaching {
	f.initSharedFixture()
	return f.TestFixture.CreateCoaching(t, dto)
}

// createBundle creates a bundle
func (f *TestFixture) createBundle(t *testing.T, dto *contentmodels.BundleDto) contentmodels.Bundle {
	// Switch to admin context with bundle management capabilities
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.UserID = originalConfig.UserID
	adminConfig.Capabilities = []string{"authenticate", "ManageAllBundles"}
	adminConfig.ShouldAuthenticate = true
	adminConfig.ShouldAuthorize = true
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig) // Restore original config

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
	if dto.SchoolType == "" {
		dto.SchoolType = "public"
	}
	if len(dto.Content) == 0 {
		// Create a default bundle with empty content
		dto.Content = []contentmodels.BundleContent{}
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/bundles", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to create bundle: body: %s", w.Body.String())

	var bundle contentmodels.Bundle
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &bundle), "Failed to unmarshal bundle")
	return bundle
}

// createTeacherAssignment creates a teacher assignment directly in the database
func (f *TestFixture) createTeacherAssignment(t *testing.T, teacherID, targetID primitive.ObjectID) {
	assignment := contentmodels.TeacherAssignment{
		Id: primitive.NewObjectID(),
		TeacherAssignmentDto: contentmodels.TeacherAssignmentDto{
			Type:       "coaching",
			TeacherIds: []primitive.ObjectID{teacherID},
			TargetId:   targetID,
			Status:     "assigned",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
	}

	_, err := f.DB.Collection("academyTeacherAssignments").InsertOne(f.Ctx, assignment)
	require.NoError(t, err, "Failed to create teacher assignment")
}

// switchAuthContext delegates to shared helper
func (f *TestFixture) switchAuthContext(userID string, capabilities []string) {
	f.initSharedFixture()
	f.TestFixture.SwitchAuthContext(userID, capabilities)
}

// enrollStudentInCoaching creates a CoachingStudent enrollment directly in the database
func (f *TestFixture) enrollStudentInCoaching(t *testing.T, studentID primitive.ObjectID, coachingID primitive.ObjectID) {
	enrollment := contentmodels.CoachingStudent{
		Id:         primitive.NewObjectID(),
		StudentId:  studentID,
		CoachingId: coachingID,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
		Trash:      false,
	}

	_, err := f.DB.Collection("academyCoachingStudents").InsertOne(f.Ctx, enrollment)
	require.NoError(t, err, "Failed to create coaching student enrollment")
}

// callGetCoachingById makes the API call to GetCoachingById
func (f *TestFixture) callGetCoachingById(t *testing.T, coachingID string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/coaching/%s", coachingID)
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}
func (f *TestFixture) enrollStudentInCourse(t *testing.T, studentID primitive.ObjectID, courseID primitive.ObjectID, status string, progress int) {
	f.enrollStudentInCourseWithBundle(t, studentID, courseID, status, progress, primitive.NilObjectID)
}

// enrollStudentInCourseWithBundle creates a StudentCourse enrollment with optional bundleId
func (f *TestFixture) enrollStudentInCourseWithBundle(t *testing.T, studentID primitive.ObjectID, courseID primitive.ObjectID, status string, progress int, bundleID primitive.ObjectID) {
	enrollment := contentmodels.StudentCourse{
		Id:                primitive.NewObjectID(),
		StudentId:         studentID,
		CourseId:          courseID,
		BundleId:          bundleID,
		StartDate:         time.Now(),
		Status:            contentenums.EnrollmentStatus(status),
		CertificateStatus: "pending",
		Progress:          progress,
		Logins:            0,
		LastActivity:      time.Now(),
		CreatedAt:         time.Now(),
		CreatedBy:         studentID,
		UpdatedAt:         time.Now(),
		UpdatedBy:         studentID,
		Trash:             false,
	}

	_, err := f.DB.Collection("academyCourseStudents").InsertOne(f.Ctx, enrollment)
	require.NoError(t, err, "Failed to create student course enrollment")
}

// createCoachingSession creates a coaching session directly in the database
func (f *TestFixture) createCoachingSession(t *testing.T, studentID, coachingID, teacherID primitive.ObjectID, date time.Time, status contentenums.StudentSessionStatus) {
	f.createCoachingSessionWithBundle(t, studentID, coachingID, teacherID, date, status, primitive.NilObjectID)
}

// createCoachingSessionWithBundle creates a coaching session with optional bundleId
func (f *TestFixture) createCoachingSessionWithBundle(t *testing.T, studentID, coachingID, teacherID primitive.ObjectID, date time.Time, status contentenums.StudentSessionStatus, bundleID primitive.ObjectID) {
	session := contentmodels.StudentSession{
		Id:                primitive.NewObjectID(),
		StudentId:         studentID,
		CoachingProgramId: coachingID,
		BundleId:          bundleID,
		TeacherId:         teacherID,
		SessionNum:        0,
		Date:              date,
		From:              "10:00",
		To:                "11:00",
		Status:            status,
		CanReschedule:     true,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
		Trash:             false,
	}

	_, err := f.DB.Collection("academyCoachingStudentSessions").InsertOne(f.Ctx, session)
	require.NoError(t, err, "Failed to create coaching session")
}

// callGetEnrolledCourses makes the API call to GetEnrolledCourses
func (f *TestFixture) callGetEnrolledCourses(t *testing.T, studentID string, query map[string]interface{}, skip, limit int) *httptest.ResponseRecorder {
	queryBody, _ := json.Marshal(query)
	url := fmt.Sprintf("/api/students/%s/courses/enrolled?skip=%d&limit=%d", studentID, skip, limit)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

// callGetEnrolledCoachings makes the API call to GetEnrolledCoachings
func (f *TestFixture) callGetEnrolledCoachings(t *testing.T, studentID string, query map[string]interface{}, skip, limit int) *httptest.ResponseRecorder {
	queryBody, _ := json.Marshal(query)
	url := fmt.Sprintf("/api/students/%s/coachings/enrolled?skip=%d&limit=%d", studentID, skip, limit)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

// enrollStudentInBundle creates a StudentBundle enrollment directly in the database
func (f *TestFixture) enrollStudentInBundle(t *testing.T, studentID primitive.ObjectID, bundleID primitive.ObjectID) {
	enrollment := contentmodels.StudentBundle{
		Id:        primitive.NewObjectID(),
		StudentId: studentID,
		BundleId:  bundleID,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
	}

	_, err := f.DB.Collection("academyBundleStudents").InsertOne(f.Ctx, enrollment)
	require.NoError(t, err, "Failed to create student bundle enrollment")
}

// callGetEnrolledBundles makes the API call to GetEnrolledBundles
func (f *TestFixture) callGetEnrolledBundles(t *testing.T, studentID string, query map[string]interface{}, skip, limit int) *httptest.ResponseRecorder {
	queryBody, _ := json.Marshal(query)
	url := fmt.Sprintf("/api/students/%s/bundles/enrolled?skip=%d&limit=%d", studentID, skip, limit)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}
