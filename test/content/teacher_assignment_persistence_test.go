package content

import (
	"bytes"
	"context"
	"encoding/json"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestCourseTeachersFieldIsPersisted verifies that the Teachers field is stored
// directly in the course document, and is managed through TeacherAssignment collection
func TestCourseTeachersFieldIsPersisted(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add required capabilities for course operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a teacher first
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "John"},
		LastName:  transl.Localizable[string]{"en": "Doe"},
	})

	// Create a course with the teacher assigned
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name: transl.Localizable[string]{
				"en": "Test Course",
			},
			Type:     "curriculum",
			Status:   "active",
			Teachers: []primitive.ObjectID{teacher.Id}, // Assign teacher
			Duration: 60,
		},
		Content: []contentmodels.Section{},
	}
	course := fixture.createCourse(t, courseDto)

	// Verify course was created successfully
	require.NotEqual(t, primitive.NilObjectID, course.Id)

	// Query the database directly to check what's actually persisted
	var rawCourse bson.M
	err := fixture.DB.Collection("academyCourses").FindOne(
		context.Background(),
		bson.M{"_id": course.Id},
	).Decode(&rawCourse)
	require.NoError(t, err, "Failed to query course from database")

	// Check that basicInfo exists
	basicInfo, ok := rawCourse["basicInfo"].(bson.M)
	require.True(t, ok, "basicInfo should exist in course document")

	// Verify that the teachers field IS in the database document
	teachersRaw, teachersExists := basicInfo["teachers"]
	assert.True(t, teachersExists, "Teachers field SHOULD be persisted in the course document")

	if teachersExists {
		teachersArray, ok := teachersRaw.(primitive.A)
		require.True(t, ok, "Teachers should be an array")
		require.Len(t, teachersArray, 1, "Should have one teacher persisted")
		assert.Equal(t, teacher.Id, teachersArray[0], "Persisted teacher ID should match")
	}

	// Verify that TeacherAssignment was created separately
	var assignment contentmodels.TeacherAssignment
	err = fixture.DB.Collection("academyTeacherAssignments").FindOne(
		context.Background(),
		bson.M{
			"type":     "course",
			"targetId": course.Id,
			"trash":    false,
		},
	).Decode(&assignment)
	require.NoError(t, err, "TeacherAssignment should exist")
	assert.Equal(t, []primitive.ObjectID{teacher.Id}, assignment.TeacherIds, "Teacher should be in assignment")
}

// TestCourseTeachersFieldEnrichedOnRetrieval verifies that Teachers is persisted and
// populated when retrieving a course via the API
func TestCourseTeachersFieldEnrichedOnRetrieval(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add required capabilities for course operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Jane"},
		LastName:  transl.Localizable[string]{"en": "Smith"},
	})

	// Create a course with the teacher
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name: transl.Localizable[string]{
				"en": "Test Course with Teacher",
			},
			Type:     "curriculum",
			Status:   "active",
			Teachers: []primitive.ObjectID{teacher.Id},
			Duration: 60,
		},
		Content: []contentmodels.Section{},
	}
	course := fixture.createCourse(t, courseDto)

	// Get the course via API
	req := httptest.NewRequest("GET", "/courses/"+course.Id.Hex(), nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "Should retrieve course successfully")

	// Parse response
	var retrievedCourse contentmodels.Course
	err := json.NewDecoder(w.Body).Decode(&retrievedCourse)
	require.NoError(t, err)

	// Verify Teachers field is populated (enriched from TeacherAssignment)
	require.NotNil(t, retrievedCourse.BasicInfo.Teachers, "Teachers should be enriched")
	require.Len(t, retrievedCourse.BasicInfo.Teachers, 1, "Should have one teacher")
	assert.Equal(t, teacher.Id, retrievedCourse.BasicInfo.Teachers[0], "Teacher ID should match")
}

// TestCoachingCoachsFieldIsPersisted verifies that the Coachs field is stored
// directly in the coaching document, and is managed through TeacherAssignment collection
func TestCoachingCoachsFieldIsPersisted(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add required capabilities for course operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a teacher/coach
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Coach"},
		LastName:  transl.Localizable[string]{"en": "Smith"},
	})

	// Create a subject first (required for coaching)
	subjectId := primitive.NewObjectID()
	_, err := fixture.DB.Collection("academySubjects").InsertOne(
		context.Background(),
		bson.M{
			"_id": subjectId,
			"name": bson.M{
				"en": "Mathematics",
				"ar": "رياضيات",
			},
			"trash": false,
		},
	)
	require.NoError(t, err, "Failed to create subject")

	// Create a coaching program with the coach assigned
	coachingDto := &contentmodels.CoachingDto{
		Title: transl.Localizable[string]{
			"en": "Test Coaching Program",
		},
		ProgramType:     "foundational",
		Class:           []string{"Grade 1"},
		Subject:         subjectId,
		Coachs:          []primitive.ObjectID{teacher.Id}, // Assign coach
		MinSessionNum:   4,
		MaxSessionNum:   8,
		SessionDuration: 60,
		Sessions: []contentmodels.CoachingSession{
			{SessionNum: 0, SessionName: transl.Localizable[string]{"en": "Session 1"}},
			{SessionNum: 1, SessionName: transl.Localizable[string]{"en": "Session 2"}},
		},
	}
	coaching := fixture.createCoaching(t, coachingDto)

	// Verify coaching was created successfully
	require.NotEqual(t, primitive.NilObjectID, coaching.Id)

	// Query the database directly to check what's actually persisted
	var rawCoaching bson.M
	err = fixture.DB.Collection("academyCoachings").FindOne(
		context.Background(),
		bson.M{"_id": coaching.Id},
	).Decode(&rawCoaching)
	require.NoError(t, err, "Failed to query coaching from database")

	// Verify that the coachs field IS in the database document
	coachsRaw, coachsExists := rawCoaching["coachs"]
	assert.True(t, coachsExists, "Coachs field SHOULD be persisted in the coaching document")

	if coachsExists {
		coachsArray, ok := coachsRaw.(primitive.A)
		require.True(t, ok, "Coachs should be an array")
		require.Len(t, coachsArray, 1, "Should have one coach persisted")
		assert.Equal(t, teacher.Id, coachsArray[0], "Persisted coach ID should match")
	}

	// Verify that TeacherAssignment was created separately
	var assignment contentmodels.TeacherAssignment
	err = fixture.DB.Collection("academyTeacherAssignments").FindOne(
		context.Background(),
		bson.M{
			"type":     "coaching",
			"targetId": coaching.Id,
			"trash":    false,
		},
	).Decode(&assignment)
	require.NoError(t, err, "TeacherAssignment should exist")
	assert.Equal(t, []primitive.ObjectID{teacher.Id}, assignment.TeacherIds, "Coach should be in assignment")
}

// TestCoachingCoachsFieldEnrichedOnRetrieval verifies that Coachs is persisted and
// populated when retrieving a coaching via the API
func TestCoachingCoachsFieldEnrichedOnRetrieval(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add required capabilities for course operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a coach
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Expert"},
		LastName:  transl.Localizable[string]{"en": "Coach"},
	})

	// Create a subject
	subjectId := primitive.NewObjectID()
	_, err := fixture.DB.Collection("academySubjects").InsertOne(
		context.Background(),
		bson.M{
			"_id": subjectId,
			"name": bson.M{
				"en": "Science",
				"ar": "علوم",
			},
			"trash": false,
		},
	)
	require.NoError(t, err, "Failed to create subject")

	// Create coaching with the coach
	coachingDto := &contentmodels.CoachingDto{
		Title: transl.Localizable[string]{
			"en": "Test Coaching with Coach",
		},
		ProgramType:     "foundational",
		Class:           []string{"Grade 1"},
		Subject:         subjectId,
		Coachs:          []primitive.ObjectID{teacher.Id},
		MinSessionNum:   4,
		MaxSessionNum:   8,
		SessionDuration: 60,
		Sessions: []contentmodels.CoachingSession{
			{SessionNum: 0, SessionName: transl.Localizable[string]{"en": "Session 1"}},
		},
	}
	coaching := fixture.createCoaching(t, coachingDto)

	// Get the coaching via API
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ViewAllCoachingPrograms"})
	req := httptest.NewRequest("GET", "/coaching/"+coaching.Id.Hex(), nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "Should retrieve coaching successfully")

	// Parse response
	var retrievedCoaching contentmodels.Coaching
	err = json.NewDecoder(w.Body).Decode(&retrievedCoaching)
	require.NoError(t, err)

	// Verify Coachs field is populated (enriched from TeacherAssignment)
	require.NotNil(t, retrievedCoaching.Coachs, "Coachs should be enriched")
	require.Len(t, retrievedCoaching.Coachs, 1, "Should have one coach")
	assert.Equal(t, teacher.Id, retrievedCoaching.Coachs[0], "Coach ID should match")
}

// TestCourseUpdateTeachersIsPersisted verifies that updating teachers persists them
func TestCourseUpdateTeachersIsPersisted(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add required capabilities for course operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create two teachers
	teacher1 := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Teacher"},
		LastName:  transl.Localizable[string]{"en": "One"},
	})
	teacher2 := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Teacher"},
		LastName:  transl.Localizable[string]{"en": "Two"},
	})

	// Create course with teacher1
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:     transl.Localizable[string]{"en": "Updatable Course"},
			Type:     "curriculum",
			Status:   "active",
			Teachers: []primitive.ObjectID{teacher1.Id},
			Duration: 60,
		},
		Content: []contentmodels.Section{},
	}
	course := fixture.createCourse(t, courseDto)

	// Update course with teacher2
	updateDto := courseDto
	updateDto.BasicInfo.Teachers = []primitive.ObjectID{teacher2.Id}

	body, _ := json.Marshal(updateDto)
	req := httptest.NewRequest("PUT", "/courses/"+course.Id.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "Should update course successfully")

	// Check database - teachers field should be persisted
	var rawCourse bson.M
	err := fixture.DB.Collection("academyCourses").FindOne(
		context.Background(),
		bson.M{"_id": course.Id},
	).Decode(&rawCourse)
	require.NoError(t, err)

	basicInfo, ok := rawCourse["basicInfo"].(bson.M)
	require.True(t, ok)
	teachersRaw, teachersExists := basicInfo["teachers"]
	assert.True(t, teachersExists, "Teachers field SHOULD be persisted even after update")

	if teachersExists {
		teachersArray, ok := teachersRaw.(primitive.A)
		require.True(t, ok, "Teachers should be an array")
		require.Len(t, teachersArray, 1, "Should have one teacher persisted")
		assert.Equal(t, teacher2.Id, teachersArray[0], "Persisted teacher ID should match updated teacher")
	}

	// Verify TeacherAssignment was updated
	var assignment contentmodels.TeacherAssignment
	err = fixture.DB.Collection("academyTeacherAssignments").FindOne(
		context.Background(),
		bson.M{
			"type":     "course",
			"targetId": course.Id,
			"trash":    false,
		},
	).Decode(&assignment)
	require.NoError(t, err)
	assert.Equal(t, []primitive.ObjectID{teacher2.Id}, assignment.TeacherIds, "Assignment should be updated")
}

// TestCourseUpdateTeachersToEmptyArray verifies that updating teachers to empty list persists as empty array, not null
func TestCourseUpdateTeachersToEmptyArray(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Teacher"},
		LastName:  transl.Localizable[string]{"en": "One"},
	})

	// Create course with teacher
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:     transl.Localizable[string]{"en": "Empty Array Course"},
			Type:     "curriculum",
			Status:   "active",
			Teachers: []primitive.ObjectID{teacher.Id},
			Duration: 60,
		},
		Content: []contentmodels.Section{},
	}
	course := fixture.createCourse(t, courseDto)

	// Update course with empty teachers list
	updateDto := courseDto
	updateDto.BasicInfo.Teachers = []primitive.ObjectID{} // Explicitly empty

	body, _ := json.Marshal(updateDto)
	req := httptest.NewRequest("PUT", "/courses/"+course.Id.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "Should update course successfully")

	// Check database
	var rawCourse bson.M
	err := fixture.DB.Collection("academyCourses").FindOne(
		context.Background(),
		bson.M{"_id": course.Id},
	).Decode(&rawCourse)
	require.NoError(t, err)

	basicInfo, ok := rawCourse["basicInfo"].(bson.M)
	require.True(t, ok)
	teachersRaw, teachersExists := basicInfo["teachers"]
	assert.True(t, teachersExists, "Teachers field SHOULD be persisted")

	if teachersExists {
		// Check if it's nil
		assert.NotNil(t, teachersRaw, "Teachers field should NOT be nil/null")

		teachersArray, ok := teachersRaw.(primitive.A)
		require.True(t, ok, "Teachers should be an array")
		assert.Len(t, teachersArray, 0, "Should be an empty array")
	}
}

// TestCourseUpdateTeachersWithNull verifies that sending null for teachers persists as empty array
func TestCourseUpdateTeachersWithNull(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Teacher"},
		LastName:  transl.Localizable[string]{"en": "One"},
	})

	// Create course with teacher
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:     transl.Localizable[string]{"en": "Null Teachers Course"},
			Type:     "curriculum",
			Status:   "active",
			Teachers: []primitive.ObjectID{teacher.Id},
			Duration: 60,
		},
		Content: []contentmodels.Section{},
	}
	course := fixture.createCourse(t, courseDto)

	// Custom struct to force null value in JSON
	type CourseBasicInfoWithNullTeachers struct {
		Name           transl.Localizable[string] `json:"name"`
		Type           string                     `json:"type"`
		Status         string                     `json:"status"`
		Teachers       *[]primitive.ObjectID      `json:"teachers"` // Pointer allows null
		SchoolType     string                     `json:"schoolType"`
		Country        string                     `json:"country"`
		Level          string                     `json:"level"`
		LinkedSubjects []primitive.ObjectID       `json:"linkedSubjects"`
		Duration       int                        `json:"duration"`
	}

	type CourseDtoWithNullTeachers struct {
		BasicInfo CourseBasicInfoWithNullTeachers `json:"basicInfo"`
		Content   []contentmodels.Section         `json:"content"`
	}

	// Prepare update with null teachers
	updateData := CourseDtoWithNullTeachers{
		BasicInfo: CourseBasicInfoWithNullTeachers{
			Name:     courseDto.BasicInfo.Name,
			Type:     courseDto.BasicInfo.Type,
			Status:   courseDto.BasicInfo.Status,
			Teachers: nil, // Send null explicitly
			Duration: courseDto.BasicInfo.Duration,
		},
		Content: []contentmodels.Section{},
	}

	body, _ := json.Marshal(updateData)
	req := httptest.NewRequest("PUT", "/courses/"+course.Id.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "Should update course successfully")

	// Check database
	var rawCourse bson.M
	err := fixture.DB.Collection("academyCourses").FindOne(
		context.Background(),
		bson.M{"_id": course.Id},
	).Decode(&rawCourse)
	require.NoError(t, err)

	basicInfo, ok := rawCourse["basicInfo"].(bson.M)
	require.True(t, ok)
	teachersRaw, teachersExists := basicInfo["teachers"]
	assert.True(t, teachersExists, "Teachers field SHOULD be persisted")

	if teachersExists {
		// Check if it's nil
		assert.NotNil(t, teachersRaw, "Teachers field should NOT be nil/null in DB")

		teachersArray, ok := teachersRaw.(primitive.A)
		require.True(t, ok, "Teachers should be an array")
		assert.Len(t, teachersArray, 0, "Should be an empty array")
	}
}

// TestCoachingUpdateCoachsWithNull verifies that sending null for coachs persists as empty array
func TestCoachingUpdateCoachsWithNull(t *testing.T) {
	// Setup app + fixtures
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
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

	// Create a coach
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Coach"},
		LastName:  transl.Localizable[string]{"en": "One"},
	})

	// Create subject
	subjectId := primitive.NewObjectID()
	_, err := fixture.DB.Collection("academySubjects").InsertOne(
		context.Background(),
		bson.M{"_id": subjectId, "name": bson.M{"en": "Math"}, "trash": false},
	)
	require.NoError(t, err)

	// Create coaching with coach
	coachingDto := &contentmodels.CoachingDto{
		Title:           transl.Localizable[string]{"en": "Null Coachs Coaching"},
		ProgramType:     "foundational",
		Class:           []string{"Grade 1"},
		Subject:         subjectId,
		Coachs:          []primitive.ObjectID{teacher.Id},
		MinSessionNum:   4,
		MaxSessionNum:   8,
		SessionDuration: 60,
		Sessions:        []contentmodels.CoachingSession{},
	}
	coaching := fixture.createCoaching(t, coachingDto)

	// Custom struct to force null value in JSON
	type CoachingDtoWithNullCoachs struct {
		Title           transl.Localizable[string]      `json:"title"`
		ProgramType     string                          `json:"programType"`
		Subject         primitive.ObjectID              `json:"subject"`
		Coachs          *[]primitive.ObjectID           `json:"coachs"` // Pointer allows null
		MinSessionNum   int                             `json:"minSessionNum"`
		MaxSessionNum   int                             `json:"maxSessionNum"`
		SessionDuration int                             `json:"sessionDuration"`
		Sessions        []contentmodels.CoachingSession `json:"sessions"`
	}

	// Prepare update with null coachs
	fixture.switchAuthContext(primitive.NewObjectID().Hex(), []string{"authenticate", "ManageAllCoachingPrograms"})
	updateData := CoachingDtoWithNullCoachs{
		Title:           coachingDto.Title,
		ProgramType:     coachingDto.ProgramType,
		Subject:         coachingDto.Subject,
		Coachs:          nil, // Send null explicitly
		MinSessionNum:   coachingDto.MinSessionNum,
		MaxSessionNum:   coachingDto.MaxSessionNum,
		SessionDuration: coachingDto.SessionDuration,
		Sessions:        coachingDto.Sessions,
	}

	body, _ := json.Marshal(updateData)
	req := httptest.NewRequest("PUT", "/coaching/"+coaching.Id.Hex(), bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)
	require.Equal(t, 200, w.Code, "Should update coaching successfully")

	// Check database
	var rawCoaching bson.M
	err = fixture.DB.Collection("academyCoachings").FindOne(
		context.Background(),
		bson.M{"_id": coaching.Id},
	).Decode(&rawCoaching)
	require.NoError(t, err)

	coachsRaw, coachsExists := rawCoaching["coachs"]
	assert.True(t, coachsExists, "Coachs field SHOULD be persisted")

	if coachsExists {
		// Check if it's nil
		assert.NotNil(t, coachsRaw, "Coachs field should NOT be nil/null in DB")

		coachsArray, ok := coachsRaw.(primitive.A)
		require.True(t, ok, "Coachs should be an array")
		assert.Len(t, coachsArray, 0, "Should be an empty array")
	}
}
