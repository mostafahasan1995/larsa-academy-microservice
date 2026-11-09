package content

import (
	"context"
	"encoding/json"
	"fmt"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestGetEnrolledBundles_BasicSuccess(t *testing.T) {
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
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Create bundle with course
	bundle := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Math Bundle",
			"ar": "باقة الرياضيات",
		},
		Status: "active",
		Content: []contentmodels.BundleContent{
			{
				Type:  "course",
				RefId: course.Id,
				Meta:  map[string]any{},
			},
		},
	})

	// Enroll student in bundle
	fixture.enrollStudentInBundle(t, student.Id, bundle.Id)

	// Enroll student in course (part of bundle) - must set bundleId
	fixture.enrollStudentInCourseWithBundle(t, student.Id, course.Id, "active", 50, bundle.Id)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Make API call
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.Len(t, response.Bundles, 1, "Expected 1 bundle, got %d", len(response.Bundles))
	assert.Equal(t, bundle.Id, response.Bundles[0].Bundle.Id)
	assert.Equal(t, int64(1), response.Pagination.TotalCount)

	// Check enrollment details
	assert.False(t, response.Bundles[0].Enrollment.Id.IsZero(), "Enrollment ID should not be zero")
	assert.Equal(t, student.Id, response.Bundles[0].Enrollment.StudentId, "Student ID mismatch")
	assert.Equal(t, bundle.Id, response.Bundles[0].Enrollment.BundleId, "Bundle ID mismatch")
	assert.False(t, response.Bundles[0].Enrollment.CreatedAt.IsZero(), "CreatedAt should not be zero")

	// Check progress calculation
	assert.Equal(t, 50, response.Bundles[0].Progress) // Course progress is 50%
	assert.Len(t, response.Bundles[0].ContentInfo, 1)
	assert.Equal(t, "course", response.Bundles[0].ContentInfo[0].Type)
	assert.Equal(t, course.Id, response.Bundles[0].ContentInfo[0].RefId)
	assert.Equal(t, 50, response.Bundles[0].ContentInfo[0].Progress)
	assert.Equal(t, course.BasicInfo.Name, response.Bundles[0].ContentInfo[0].Name)
}

func TestGetEnrolledBundles_ProgressWithCoursesAndCoaching(t *testing.T) {
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
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create bundle with both course and coaching
	bundle := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Complete Learning Bundle",
			"ar": "باقة التعلم الكاملة",
		},
		Status: "active",
		Content: []contentmodels.BundleContent{
			{
				Type:  "course",
				RefId: course.Id,
				Meta:  map[string]any{},
			},
			{
				Type:  "coaching",
				RefId: coaching.Id,
				Meta:  map[string]any{"numOfSessions": 10},
			},
		},
	})

	// Enroll student in bundle
	fixture.enrollStudentInBundle(t, student.Id, bundle.Id)

	// Enroll student in course (80% progress) - must set bundleId
	fixture.enrollStudentInCourseWithBundle(t, student.Id, course.Id, "active", 80, bundle.Id)

	// Create coaching sessions (6 out of 10 completed = 60% progress) - must set bundleId
	now := time.Now()
	for i := 0; i < 6; i++ {
		fixture.createCoachingSessionWithBundle(t, student.Id, coaching.Id, teacher.Id, now.Add(time.Duration(i)*24*time.Hour), contentenums.StudentSessionStatusCompleted, bundle.Id)
	}
	for i := 6; i < 10; i++ {
		fixture.createCoachingSessionWithBundle(t, student.Id, coaching.Id, teacher.Id, now.Add(time.Duration(i)*24*time.Hour), contentenums.StudentSessionStatusScheduled, bundle.Id)
	}

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Make API call
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.Len(t, response.Bundles, 1)

	// Overall progress should be average: (80 + 60) / 2 = 70
	assert.Equal(t, 70, response.Bundles[0].Progress)

	// Check content info
	assert.Len(t, response.Bundles[0].ContentInfo, 2)

	// Find course and coaching in content info
	var courseInfo, coachingInfo *contentmodels.ContentInfo
	for i := range response.Bundles[0].ContentInfo {
		if response.Bundles[0].ContentInfo[i].Type == "course" {
			courseInfo = &response.Bundles[0].ContentInfo[i]
		} else if response.Bundles[0].ContentInfo[i].Type == "coaching" {
			coachingInfo = &response.Bundles[0].ContentInfo[i]
		}
	}

	require.NotNil(t, courseInfo)
	require.NotNil(t, coachingInfo)
	assert.Equal(t, 80, courseInfo.Progress)
	assert.Equal(t, course.BasicInfo.Name, courseInfo.Name)
	assert.Equal(t, 60, coachingInfo.Progress)
}

func TestGetEnrolledBundles_MultipleBundles(t *testing.T) {
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
	student := fixture.createStudent(t, nil)

	// Create 3 bundles
	bundle1 := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Bundle 1",
			"ar": "باقة 1",
		},
		Status: "active",
	})
	bundle2 := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Bundle 2",
			"ar": "باقة 2",
		},
		Status: "active",
	})
	bundle3 := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Bundle 3",
			"ar": "باقة 3",
		},
		Status: "active",
	})

	// Enroll student in all bundles
	fixture.enrollStudentInBundle(t, student.Id, bundle1.Id)
	fixture.enrollStudentInBundle(t, student.Id, bundle2.Id)
	fixture.enrollStudentInBundle(t, student.Id, bundle3.Id)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Make API call
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, int64(3), response.Pagination.TotalCount)
	assert.Len(t, response.Bundles, 3)
}

func TestGetEnrolledBundles_EmptyBundle(t *testing.T) {
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
	student := fixture.createStudent(t, nil)

	// Create bundle with no content
	bundle := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Empty Bundle",
			"ar": "باقة فارغة",
		},
		Status:  "active",
		Content: []contentmodels.BundleContent{},
	})

	// Enroll student in bundle
	fixture.enrollStudentInBundle(t, student.Id, bundle.Id)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Make API call
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.Len(t, response.Bundles, 1)
	assert.Equal(t, 0, response.Bundles[0].Progress) // No content = 0% progress
	assert.Len(t, response.Bundles[0].ContentInfo, 0)
}

func TestGetEnrolledBundles_NoEnrollments(t *testing.T) {
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

	// Create student with no bundle enrollments
	student := fixture.createStudent(t, nil)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Make API call
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	assert.Equal(t, int64(0), response.Pagination.TotalCount)
	assert.Len(t, response.Bundles, 0)
}

func TestGetEnrolledBundles_Pagination(t *testing.T) {
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
	student := fixture.createStudent(t, nil)

	// Create 5 bundles
	for i := 0; i < 5; i++ {
		bundle := fixture.createBundle(t, &contentmodels.BundleDto{
			Name: transl.Localizable[string]{
				"en": fmt.Sprintf("Bundle %d", i+1),
				"ar": fmt.Sprintf("باقة %d", i+1),
			},
			Status: "active",
		})
		fixture.enrollStudentInBundle(t, student.Id, bundle.Id)
	}

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// First page
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 3)

	require.Equal(t, http.StatusOK, w.Code)
	var response1 contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response1))

	assert.Equal(t, int64(5), response1.Pagination.TotalCount)
	assert.GreaterOrEqual(t, len(response1.Bundles), 1)
	assert.LessOrEqual(t, len(response1.Bundles), 5)

	// Second page
	w2 := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 3, 3)

	require.Equal(t, http.StatusOK, w2.Code)
	var response2 contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &response2))

	assert.Equal(t, int64(5), response2.Pagination.TotalCount)
	assert.GreaterOrEqual(t, len(response2.Bundles), 0)
	assert.LessOrEqual(t, len(response2.Bundles), 5)
}

func TestGetEnrolledBundles_FilterByStatus(t *testing.T) {
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
	student := fixture.createStudent(t, nil)

	// Create bundles with different statuses
	bundle1 := fixture.createBundle(t, &contentmodels.BundleDto{
		Name:   transl.Localizable[string]{"en": "Active Bundle", "ar": "باقة نشطة"},
		Status: "active",
	})
	bundle2 := fixture.createBundle(t, &contentmodels.BundleDto{
		Name:   transl.Localizable[string]{"en": "Inactive Bundle", "ar": "باقة غير نشطة"},
		Status: "inactive",
	})

	// Enroll student in both
	fixture.enrollStudentInBundle(t, student.Id, bundle1.Id)
	fixture.enrollStudentInBundle(t, student.Id, bundle2.Id)

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Filter by active status
	query := map[string]interface{}{
		"columns": []map[string]interface{}{
			{
				"name":  "bundle.status",
				"exp":   "eq",
				"value": "active",
			},
		},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	// Should only return active bundle
	assert.Equal(t, int64(1), response.Pagination.TotalCount)
	assert.Len(t, response.Bundles, 1)
	if len(response.Bundles) > 0 {
		assert.Equal(t, "active", response.Bundles[0].Bundle.Status)
	}
}

func TestGetEnrolledBundles_ProgressWithOnlyCoaching(t *testing.T) {
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

	// Create bundle with only coaching
	bundle := fixture.createBundle(t, &contentmodels.BundleDto{
		Name: transl.Localizable[string]{
			"en": "Coaching Only Bundle",
			"ar": "باقة التدريب فقط",
		},
		Status: "active",
		Content: []contentmodels.BundleContent{
			{
				Type:  "coaching",
				RefId: coaching.Id,
				Meta:  map[string]any{"numOfSessions": 5},
			},
		},
	})

	// Enroll student in bundle
	fixture.enrollStudentInBundle(t, student.Id, bundle.Id)

	// Create coaching sessions (3 out of 5 completed = 60% progress) - must set bundleId
	now := time.Now()
	for i := 0; i < 3; i++ {
		fixture.createCoachingSessionWithBundle(t, student.Id, coaching.Id, teacher.Id, now.Add(time.Duration(i)*24*time.Hour), contentenums.StudentSessionStatusCompleted, bundle.Id)
	}
	for i := 3; i < 5; i++ {
		fixture.createCoachingSessionWithBundle(t, student.Id, coaching.Id, teacher.Id, now.Add(time.Duration(i)*24*time.Hour), contentenums.StudentSessionStatusScheduled, bundle.Id)
	}

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Make API call
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetEnrolledBundles(t, student.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.EnrolledBundlePagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.Len(t, response.Bundles, 1)
	assert.Equal(t, 60, response.Bundles[0].Progress) // 3/5 = 60%
	assert.Len(t, response.Bundles[0].ContentInfo, 1)
	assert.Equal(t, "coaching", response.Bundles[0].ContentInfo[0].Type)
	assert.Equal(t, 60, response.Bundles[0].ContentInfo[0].Progress)
}
