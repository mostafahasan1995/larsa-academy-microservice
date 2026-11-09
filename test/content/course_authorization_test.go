package content

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCourseAuthorization_AdminCanViewAllCourses(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses"} // Add course management capability
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

	// Create test data (this uses the default auth which should work)
	teacher := fixture.createTeacher(t, nil)
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Status:   "draft", // Not public
		},
	})

	// Switch to admin context with ViewAllCourses capability
	adminUserId := primitive.NewObjectID().Hex()
	fixture.switchAuthContext(adminUserId, []string{"authenticate", "ViewAllCourses"})
	w := callGetCourseById(t, fixture, course.Id.Hex())

	require.Equal(t, http.StatusOK, w.Code, "Admin should be able to view draft course: %s", w.Body.String())
	var response contentmodels.Course
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, course.Id, response.Id)
}

func TestCourseAuthorization_TeacherCanViewAssignedCourses(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses"} // For creating data
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

	assignedCourse := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name: transl.Localizable[string]{
				"en": "Assigned Course",
				"ar": "دورة معينة",
			},
		},
	})
	unassignedCourse := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{otherTeacher.Id},
			Name: transl.Localizable[string]{
				"en": "Unassigned Course",
				"ar": "دورة غير معينة",
			},
		},
	})

	// Test teacher can view assigned course
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewAssignedCourses"})
	w := callGetCourseById(t, fixture, assignedCourse.Id.Hex())

	require.Equal(t, http.StatusOK, w.Code, "Teacher should be able to view assigned course: %s", w.Body.String())

	// Test teacher cannot view unassigned course
	w2 := callGetCourseById(t, fixture, unassignedCourse.Id.Hex())
	require.Equal(t, http.StatusForbidden, w2.Code, "Teacher should not be able to view unassigned course")
}

func TestCourseAuthorization_StudentCanViewPublicCourses(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses"} // For creating data
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

	publicCourse := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name: transl.Localizable[string]{
				"en": "Public Course",
				"ar": "دورة عامة",
			},
			Status: "active", // Public course
		},
	})
	draftCourse := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name: transl.Localizable[string]{
				"en": "Draft Course",
				"ar": "دورة مسودة",
			},
			Status: "draft", // Not public
		},
	})

	// Test student can view public course
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", "ViewPublicCourses"})
	w := callGetCourseById(t, fixture, publicCourse.Id.Hex())

	require.Equal(t, http.StatusOK, w.Code, "Student should be able to view public course: %s", w.Body.String())

	// Test student cannot view draft course
	w2 := callGetCourseById(t, fixture, draftCourse.Id.Hex())
	require.Equal(t, http.StatusForbidden, w2.Code, "Student should not be able to view draft course")
}

func TestCourseAuthorization_StudentCanViewEnrolledCourses(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses"} // For creating data
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

	draftCourse := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name: transl.Localizable[string]{
				"en": "Draft Course",
				"ar": "دورة مسودة",
			},
			Status: "draft", // Not public
		},
	})

	// Enroll the student in the draft course
	fixture.enrollStudentInCourse(t, student.Id, draftCourse.Id, "active", 0)

	// Test student can view enrolled draft course
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", "ViewPublicCourses"})
	w := callGetCourseById(t, fixture, draftCourse.Id.Hex())

	require.Equal(t, http.StatusOK, w.Code, "Student should be able to view enrolled draft course: %s", w.Body.String())
	var response contentmodels.Course
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, draftCourse.Id, response.Id)
}

func TestCourseAuthorization_TeacherCanCreateCourseResources(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses"} // For creating data
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
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name: transl.Localizable[string]{
				"en": "Teacher's Course",
				"ar": "دورة المعلم",
			},
		},
	})

	// Test teacher can create course test (resource)
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "EditCourseResources"})

	testDto := &contentmodels.CourseTestDto{
		Title: transl.Localizable[string]{
			"en": "Sample Test",
			"ar": "اختبار تجريبي",
		},
		Questions: []contentmodels.Question{},
		Settings: contentmodels.TestSettings{
			RecordGrads:     true,
			InstantFeedback: true,
		},
	}

	url := fmt.Sprintf("/courses/%s/tests", course.Id.Hex())
	body, _ := json.Marshal(testDto)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Teacher should be able to create course test: %s", w.Body.String())
}

func TestCourseAuthorization_NonTeacherCannotCreateCourseResources(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate", "ManageAllCourses"} // For creating data
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
			Name: transl.Localizable[string]{
				"en": "Public Course",
				"ar": "دورة عامة",
			},
			Status: "active", // Public course
		},
	})

	// Test student cannot create course test (resource)
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", "ViewPublicCourses"}) // No edit permissions

	testDto := &contentmodels.CourseTestDto{
		Title: transl.Localizable[string]{
			"en": "Unauthorized Test",
			"ar": "اختبار غير مصرح",
		},
		Questions: []contentmodels.Question{},
		Settings: contentmodels.TestSettings{
			RecordGrads:     true,
			InstantFeedback: true,
		},
	}

	url := fmt.Sprintf("/courses/%s/tests", course.Id.Hex())
	body, _ := json.Marshal(testDto)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusForbidden, w.Code, "Student should not be able to create course test")
}

func TestCourseAuthorization_CourseCreationAuthorization(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{"authenticate"} // No course management capabilities initially
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

	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
			Name: transl.Localizable[string]{
				"en": "Test Course",
				"ar": "دورة تجریبية",
			},
			Type:       "curriculum",
			SchoolType: "public",
			Level:      "beginner",
			Status:     "draft",
		},
	}

	// Test 1: User without course management capabilities cannot create courses
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate"})
	w := callCreateCourse(t, fixture, courseDto)
	require.Equal(t, http.StatusForbidden, w.Code, "User without course management capabilities should not be able to create courses")

	// Test 2: User with ManageAllCourses capability can create courses
	adminUserId := primitive.NewObjectID().Hex()
	fixture.switchAuthContext(adminUserId, []string{"authenticate", "ManageAllCourses"})
	w = callCreateCourse(t, fixture, courseDto)
	require.Equal(t, http.StatusOK, w.Code, "User with ManageAllCourses capability should be able to create courses")

	// Test 3: User with ManageAssignedCourses capability can create courses
	teacherUserId := primitive.NewObjectID().Hex()
	fixture.switchAuthContext(teacherUserId, []string{"authenticate", "ManageAssignedCourses"})
	w = callCreateCourse(t, fixture, courseDto)
	require.Equal(t, http.StatusOK, w.Code, "User with ManageAssignedCourses capability should be able to create courses")
}

// Helper function to call GET /courses/{id}
func callGetCourseById(t *testing.T, fixture *TestFixture, courseId string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/courses/%s", courseId)
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

// Helper function to call POST /courses (create course)
func callCreateCourse(t *testing.T, fixture *TestFixture, courseDto *contentmodels.CourseDto) *httptest.ResponseRecorder {
	body, _ := json.Marshal(courseDto)
	req := httptest.NewRequest("POST", "/courses", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}

// Helper function to call PATCH /api/courses/{id} (update course)
func callUpdateCourse(t *testing.T, fixture *TestFixture, courseId string, courseDto *contentmodels.CourseDto) *httptest.ResponseRecorder {
	body, _ := json.Marshal(courseDto)
	url := fmt.Sprintf("/api/courses/%s", courseId)
	req := httptest.NewRequest("PATCH", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)

	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	return w
}
