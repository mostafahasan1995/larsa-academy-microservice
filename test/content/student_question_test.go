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

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// createTeacherAssignmentDirect creates a teacher assignment directly in the database
func (f *TestFixture) createTeacherAssignmentDirect(t *testing.T, courseID primitive.ObjectID, teacherIDs []primitive.ObjectID) {
	assignment := contentmodels.TeacherAssignment{
		Id:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Trash:     false,
		TeacherAssignmentDto: contentmodels.TeacherAssignmentDto{
			Type:       "course",
			TargetId:   courseID,
			TeacherIds: teacherIDs,
			Status:     "assigned",
		},
	}

	_, err := f.DB.Collection("academyTeacherAssignments").InsertOne(f.Ctx, assignment)
	require.NoError(t, err, "Failed to create teacher assignment")
}

// createSupervisor creates a supervisor with random values
func (f *TestFixture) createSupervisor(t *testing.T, dto *models.SupervisorDTO) models.Supervisor {
	if dto == nil {
		dto = &models.SupervisorDTO{}
	}

	if len(dto.FirstName) == 0 {
		dto.FirstName = transl.Localizable[string]{"en": "Supervisor" + helpers.RandomString(4)}
	}
	if len(dto.LastName) == 0 {
		dto.LastName = transl.Localizable[string]{"en": "Test" + helpers.RandomString(4)}
	}
	if dto.NationalID == "" {
		dto.NationalID = helpers.RandomNationalID()
	}
	if dto.Gender == "" {
		dto.Gender = "male"
	}
	if dto.Nationality == "" {
		dto.Nationality = "US"
	}
	if dto.DateOfBirth.IsZero() {
		dto.DateOfBirth = time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC)
	}
	if dto.MobileNumber == (models.PhoneNumber{}) {
		phone := helpers.RandomPhone()
		dto.MobileNumber = models.PhoneNumber{
			Prefix:  "+1",
			Content: phone[2:], // Remove +1 prefix
		}
	}

	if dto.AccountSettings.Email == "" {
		dto.AccountSettings.Email = helpers.RandomEmail()
	}
	if dto.AccountSettings.InitialPass == "" {
		dto.AccountSettings.InitialPass = "password123"
	}
	if dto.AccountSettings.ConfirmPass == "" {
		dto.AccountSettings.ConfirmPass = dto.AccountSettings.InitialPass
	}
	if dto.AccountSettings.JoinStatus == "" {
		dto.AccountSettings.JoinStatus = "active"
	}
	if !dto.AccountSettings.IsActive {
		dto.AccountSettings.IsActive = true
	}

	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/supervisors", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusCreated {
		t.Fatalf("Failed to create supervisor: status %d, body: %s", w.Code, w.Body.String())
	}

	var supervisor models.Supervisor
	if err := json.Unmarshal(w.Body.Bytes(), &supervisor); err != nil {
		t.Fatalf("Failed to unmarshal supervisor: %v", err)
	}
	return supervisor
}

// callCreateStudentQuestion makes the API call to create a student question
func (f *TestFixture) callCreateStudentQuestion(t *testing.T, dto *contentmodels.StudentQuestionDto) *httptest.ResponseRecorder {
	body, _ := json.Marshal(dto)
	req := httptest.NewRequest("POST", "/student-questions", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

// callGetStudentQuestions makes the API call to get student questions by ref
func (f *TestFixture) callGetStudentQuestions(t *testing.T, refType, refID string, query map[string]interface{}, skip, limit int) *httptest.ResponseRecorder {
	queryBody, _ := json.Marshal(query)
	url := fmt.Sprintf("/student-questions/ref/%s/%s?skip=%d&limit=%d", refType, refID, skip, limit)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(queryBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

// callGetStudentQuestion makes the API call to get a single student question
func (f *TestFixture) callGetStudentQuestion(t *testing.T, questionID string) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/student-questions/%s", questionID)
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

// callAddReply makes the API call to add a reply to a question
func (f *TestFixture) callAddReply(t *testing.T, questionID string, dto *contentmodels.StudentQuestionReplyDto) *httptest.ResponseRecorder {
	body, _ := json.Marshal(dto)
	url := fmt.Sprintf("/student-questions/%s/replies", questionID)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func TestCreateStudentQuestion_Success(t *testing.T) {
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

	// Create teacher assignment
	fixture.createTeacherAssignmentDirect(t, course.Id, []primitive.ObjectID{teacher.Id})

	// Switch to student context
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Create question
	questionDto := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "How do I access the course materials?",
		Description: "I'm having trouble finding the course materials.",
	}

	w := fixture.callCreateStudentQuestion(t, questionDto)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code, "Response body: %s", w.Body.String())

	var question contentmodels.StudentQuestion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &question))
	assert.Equal(t, course.Id, question.RefId)
	assert.Equal(t, "course", question.RefType)
	assert.Equal(t, student.Id, question.StudentId)
	assert.Equal(t, questionDto.Title, question.Title)
	assert.Equal(t, questionDto.Description, question.Description)
}

func TestGetStudentQuestionsByRef_Success(t *testing.T) {
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

	// Switch to student context and create questions
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	question1 := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Question 1",
		Description: "Description 1",
	}
	w1 := fixture.callCreateStudentQuestion(t, question1)
	require.Equal(t, http.StatusOK, w1.Code)

	question2 := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Question 2",
		Description: "Description 2",
	}
	w2 := fixture.callCreateStudentQuestion(t, question2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Get questions
	query := map[string]interface{}{
		"columns": []map[string]interface{}{},
	}
	w := fixture.callGetStudentQuestions(t, "course", course.Id.Hex(), query, 0, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.StudentQuestionPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))
	assert.Equal(t, int64(2), response.Pagination.TotalCount)
	assert.Len(t, response.Questions, 2)
}

func TestAddReply_TeacherAssigned_Success(t *testing.T) {
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

	// Create teacher assignment
	fixture.createTeacherAssignmentDirect(t, course.Id, []primitive.ObjectID{teacher.Id})

	// Student creates question
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})
	questionDto := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Test Question",
		Description: "Test Description",
	}
	w := fixture.callCreateStudentQuestion(t, questionDto)
	require.Equal(t, http.StatusOK, w.Code)

	var question contentmodels.StudentQuestion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &question))

	// Teacher replies
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})
	replyDto := &contentmodels.StudentQuestionReplyDto{
		Content: "This is a reply from the teacher.",
	}
	wReply := fixture.callAddReply(t, question.Id.Hex(), replyDto)

	// Assertions
	require.Equal(t, http.StatusOK, wReply.Code, "Response body: %s", wReply.Body.String())

	var reply contentmodels.StudentQuestionReply
	require.NoError(t, json.Unmarshal(wReply.Body.Bytes(), &reply))
	// QuestionId is no longer in embedded replies
	assert.Equal(t, teacher.Id, reply.UserId)
	assert.Equal(t, replyDto.Content, reply.Content)
}

func TestAddReply_TeacherNotAssigned_Forbidden(t *testing.T) {
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
	teacher1 := fixture.createTeacher(t, nil)
	teacher2 := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher1.Id},
		},
	})

	// Create teacher assignment only for teacher1
	fixture.createTeacherAssignmentDirect(t, course.Id, []primitive.ObjectID{teacher1.Id})

	// Student creates question
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})
	questionDto := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Test Question",
		Description: "Test Description",
	}
	w := fixture.callCreateStudentQuestion(t, questionDto)
	require.Equal(t, http.StatusOK, w.Code)

	var question contentmodels.StudentQuestion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &question))

	// Teacher2 (not assigned) tries to reply
	fixture.switchAuthContext(teacher2.Id.Hex(), []string{"authenticate"})
	replyDto := &contentmodels.StudentQuestionReplyDto{
		Content: "This should fail.",
	}
	wReply := fixture.callAddReply(t, question.Id.Hex(), replyDto)

	// Assertions
	assert.Equal(t, http.StatusBadRequest, wReply.Code)
}

func TestGetStudentQuestion_WithReplies(t *testing.T) {
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

	// Create teacher assignment
	fixture.createTeacherAssignmentDirect(t, course.Id, []primitive.ObjectID{teacher.Id})

	// Student creates question
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})
	questionDto := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Test Question",
		Description: "Test Description",
	}
	w := fixture.callCreateStudentQuestion(t, questionDto)
	require.Equal(t, http.StatusOK, w.Code)

	var question contentmodels.StudentQuestion
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &question))

	// Teacher adds a reply
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})
	reply1 := &contentmodels.StudentQuestionReplyDto{Content: "First reply"}
	w1 := fixture.callAddReply(t, question.Id.Hex(), reply1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Teacher updates the reply (replaces it)
	reply2 := &contentmodels.StudentQuestionReplyDto{Content: "Updated reply"}
	w2 := fixture.callAddReply(t, question.Id.Hex(), reply2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Get question with reply
	wGet := fixture.callGetStudentQuestion(t, question.Id.Hex())

	// Assertions
	require.Equal(t, http.StatusOK, wGet.Code)

	var questionRes contentmodels.StudentQuestionRes
	require.NoError(t, json.Unmarshal(wGet.Body.Bytes(), &questionRes))
	assert.Equal(t, question.Id, questionRes.Id)
	assert.NotNil(t, questionRes.Reply)
	assert.Equal(t, "Updated reply", questionRes.Reply.Content)
}

// callGetTeacherStats makes the API call to get teacher statistics
func (f *TestFixture) callGetTeacherStats(t *testing.T, refType, refId string) *httptest.ResponseRecorder {
	url := "/student-questions/teacher/stats"
	if refType != "" || refId != "" {
		url += "?"
		if refType != "" {
			url += fmt.Sprintf("refType=%s", refType)
		}
		if refId != "" {
			if refType != "" {
				url += "&"
			}
			url += fmt.Sprintf("refId=%s", refId)
		}
	}
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

// callGetSupervisorStats makes the API call to get supervisor statistics
func (f *TestFixture) callGetSupervisorStats(t *testing.T, refType, refId string, page, perPage int) *httptest.ResponseRecorder {
	url := fmt.Sprintf("/student-questions/supervisor/stats?page=%d&perPage=%d", page, perPage)
	if refType != "" {
		url += fmt.Sprintf("&refType=%s", refType)
	}
	if refId != "" {
		url += fmt.Sprintf("&refId=%s", refId)
	}
	req := httptest.NewRequest("GET", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)
	return w
}

func TestGetTeacherStats_NoQuestions(t *testing.T) {
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
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Create teacher assignment
	fixture.createTeacherAssignmentDirect(t, course.Id, []primitive.ObjectID{teacher.Id})

	// Switch to teacher context
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})

	// Get stats
	w := fixture.callGetTeacherStats(t, "", "")

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionTeacherStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(0), stats.TotalQuestions)
	assert.Equal(t, int64(0), stats.TotalReplied)
}

func TestGetTeacherStats_WithQuestions(t *testing.T) {
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

	// Create teacher assignment
	fixture.createTeacherAssignmentDirect(t, course.Id, []primitive.ObjectID{teacher.Id})

	// Student creates questions
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	question1 := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Question 1",
		Description: "Description 1",
	}
	w1 := fixture.callCreateStudentQuestion(t, question1)
	require.Equal(t, http.StatusOK, w1.Code)

	var q1 contentmodels.StudentQuestion
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &q1))

	question2 := &contentmodels.StudentQuestionDto{
		RefId:       course.Id,
		Title:       "Question 2",
		Description: "Description 2",
	}
	w2 := fixture.callCreateStudentQuestion(t, question2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Teacher replies to one question
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})
	replyDto := &contentmodels.StudentQuestionReplyDto{
		Content: "Reply to question 1",
	}
	wReply := fixture.callAddReply(t, q1.Id.Hex(), replyDto)
	require.Equal(t, http.StatusOK, wReply.Code)

	// Get stats
	w := fixture.callGetTeacherStats(t, "", "")

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionTeacherStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(2), stats.TotalQuestions)
	assert.Equal(t, int64(1), stats.TotalReplied)
}

func TestGetTeacherStats_WithRefId(t *testing.T) {
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
	course1 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	course2 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})

	// Create teacher assignments
	fixture.createTeacherAssignmentDirect(t, course1.Id, []primitive.ObjectID{teacher.Id})
	fixture.createTeacherAssignmentDirect(t, course2.Id, []primitive.ObjectID{teacher.Id})

	// Student creates questions in both courses
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	question1 := &contentmodels.StudentQuestionDto{
		RefId:       course1.Id,
		Title:       "Question in Course 1",
		Description: "Description",
	}
	w1 := fixture.callCreateStudentQuestion(t, question1)
	require.Equal(t, http.StatusOK, w1.Code)

	question2 := &contentmodels.StudentQuestionDto{
		RefId:       course2.Id,
		Title:       "Question in Course 2",
		Description: "Description",
	}
	w2 := fixture.callCreateStudentQuestion(t, question2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Switch to teacher context and get stats for course1 only
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate"})
	w := fixture.callGetTeacherStats(t, "course", course1.Id.Hex())

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionTeacherStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(1), stats.TotalQuestions)
	assert.Equal(t, int64(0), stats.TotalReplied)
}

func TestGetSupervisorStats_NoTeachers(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
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

	// Create supervisor with no teachers
	supervisor := fixture.createSupervisor(t, nil)

	// Switch to supervisor context
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate"})

	// Get stats
	w := fixture.callGetSupervisorStats(t, "", "", 1, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionSupervisorStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(0), stats.TotalQuestions)
	assert.Equal(t, int64(0), stats.TotalReplied)
	assert.Equal(t, float64(0), stats.AvgReplyTime)
	assert.Empty(t, stats.Teachers)
	assert.Equal(t, float64(0), stats.Pagination.TotalPages)
}

func TestGetSupervisorStats_WithTeachersAndQuestions(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
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
	supervisor := fixture.createSupervisor(t, nil)
	teacher1 := fixture.createTeacher(t, nil)
	teacher2 := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)

	// Create supervisor assignment linking supervisor to teachers
	// This is done via supervisor assignment service
	// For now, we'll create a supervisor assignment directly
	supervisorAssignment := contentmodels.SupervisorAssignment{
		Id:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId: supervisor.Id,
			AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
			RefType:      string(contentenums.SupervisorRefTypeTeacher),
			RefIds:       []primitive.ObjectID{teacher1.Id, teacher2.Id},
			Status:       string(contentenums.SupervisorAssignmentStatusAssigned),
		},
	}
	_, err := database.Collection("academySupervisorAssignments").InsertOne(context.Background(), supervisorAssignment)
	require.NoError(t, err)

	// Create courses for teachers
	course1 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher1.Id},
		},
	})
	course2 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher2.Id},
		},
	})

	// Create teacher assignments
	fixture.createTeacherAssignmentDirect(t, course1.Id, []primitive.ObjectID{teacher1.Id})
	fixture.createTeacherAssignmentDirect(t, course2.Id, []primitive.ObjectID{teacher2.Id})

	// Student creates questions
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	// Create questions for teacher1's course
	question1 := &contentmodels.StudentQuestionDto{
		RefId:       course1.Id,
		Title:       "Question 1 for Teacher 1",
		Description: "Description",
	}
	w1 := fixture.callCreateStudentQuestion(t, question1)
	require.Equal(t, http.StatusOK, w1.Code)

	var q1 contentmodels.StudentQuestion
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &q1))

	question2 := &contentmodels.StudentQuestionDto{
		RefId:       course1.Id,
		Title:       "Question 2 for Teacher 1",
		Description: "Description",
	}
	w2 := fixture.callCreateStudentQuestion(t, question2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Create questions for teacher2's course
	question3 := &contentmodels.StudentQuestionDto{
		RefId:       course2.Id,
		Title:       "Question 1 for Teacher 2",
		Description: "Description",
	}
	w3 := fixture.callCreateStudentQuestion(t, question3)
	require.Equal(t, http.StatusOK, w3.Code)

	// Teacher1 replies to question1 (with delay to test reply time)
	time.Sleep(100 * time.Millisecond) // Small delay to ensure different timestamps
	fixture.switchAuthContext(teacher1.Id.Hex(), []string{"authenticate"})
	replyDto := &contentmodels.StudentQuestionReplyDto{
		Content: "Reply from teacher 1",
	}
	wReply := fixture.callAddReply(t, q1.Id.Hex(), replyDto)
	require.Equal(t, http.StatusOK, wReply.Code)

	// Switch to supervisor context and get stats
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate"})
	w := fixture.callGetSupervisorStats(t, "", "", 1, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionSupervisorStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(3), stats.TotalQuestions)
	assert.Equal(t, int64(1), stats.TotalReplied)
	assert.Greater(t, stats.AvgReplyTime, float64(0)) // Should have some reply time
	assert.Len(t, stats.Teachers, 2)
	assert.Equal(t, float64(1), stats.Pagination.TotalPages)
	assert.Equal(t, int64(2), stats.Pagination.TotalCount)

	// Check teacher breakdowns
	teacher1Found := false
	teacher2Found := false
	for _, teacher := range stats.Teachers {
		if teacher.Id == teacher1.Id {
			teacher1Found = true
			assert.Equal(t, int64(2), teacher.TotalQuestions)
			assert.Equal(t, int64(1), teacher.TotalReplied)
			assert.Greater(t, teacher.AvgReplyTime, float64(0))
		}
		if teacher.Id == teacher2.Id {
			teacher2Found = true
			assert.Equal(t, int64(1), teacher.TotalQuestions)
			assert.Equal(t, int64(0), teacher.TotalReplied)
			assert.Equal(t, float64(0), teacher.AvgReplyTime)
		}
	}
	assert.True(t, teacher1Found, "Teacher1 should be in results")
	assert.True(t, teacher2Found, "Teacher2 should be in results")
}

func TestGetSupervisorStats_WithPagination(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
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
	supervisor := fixture.createSupervisor(t, nil)
	teacher1 := fixture.createTeacher(t, nil)
	teacher2 := fixture.createTeacher(t, nil)
	teacher3 := fixture.createTeacher(t, nil)

	// Create supervisor assignment
	supervisorAssignment := contentmodels.SupervisorAssignment{
		Id:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId: supervisor.Id,
			AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
			RefType:      string(contentenums.SupervisorRefTypeTeacher),
			RefIds:       []primitive.ObjectID{teacher1.Id, teacher2.Id, teacher3.Id},
			Status:       string(contentenums.SupervisorAssignmentStatusAssigned),
		},
	}
	_, err := database.Collection("academySupervisorAssignments").InsertOne(context.Background(), supervisorAssignment)
	require.NoError(t, err)

	// Switch to supervisor context and get first page
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate"})
	w := fixture.callGetSupervisorStats(t, "", "", 1, 2)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionSupervisorStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.LessOrEqual(t, len(stats.Teachers), 2)
	assert.Equal(t, float64(2), stats.Pagination.TotalPages) // 3 teachers / 2 per page = 2 pages
	assert.Equal(t, int64(3), stats.Pagination.TotalCount)
}

func TestGetSupervisorStats_WithRefId(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
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
	supervisor := fixture.createSupervisor(t, nil)
	teacher1 := fixture.createTeacher(t, nil)
	teacher2 := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)

	// Create supervisor assignment
	supervisorAssignment := contentmodels.SupervisorAssignment{
		Id:        primitive.NewObjectID(),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId: supervisor.Id,
			AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
			RefType:      string(contentenums.SupervisorRefTypeTeacher),
			RefIds:       []primitive.ObjectID{teacher1.Id, teacher2.Id},
			Status:       string(contentenums.SupervisorAssignmentStatusAssigned),
		},
	}
	_, err := database.Collection("academySupervisorAssignments").InsertOne(context.Background(), supervisorAssignment)
	require.NoError(t, err)

	// Create courses
	course1 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher1.Id},
		},
	})
	course2 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher2.Id},
		},
	})

	// Create teacher assignments
	fixture.createTeacherAssignmentDirect(t, course1.Id, []primitive.ObjectID{teacher1.Id})
	fixture.createTeacherAssignmentDirect(t, course2.Id, []primitive.ObjectID{teacher2.Id})

	// Student creates questions in both courses
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate"})

	question1 := &contentmodels.StudentQuestionDto{
		RefId:       course1.Id,
		Title:       "Question in Course 1",
		Description: "Description",
	}
	w1 := fixture.callCreateStudentQuestion(t, question1)
	require.Equal(t, http.StatusOK, w1.Code)

	question2 := &contentmodels.StudentQuestionDto{
		RefId:       course2.Id,
		Title:       "Question in Course 2",
		Description: "Description",
	}
	w2 := fixture.callCreateStudentQuestion(t, question2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Switch to supervisor context and get stats for course1 only
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate"})
	w := fixture.callGetSupervisorStats(t, "course", course1.Id.Hex(), 1, 10)

	// Assertions
	require.Equal(t, http.StatusOK, w.Code)

	var stats contentmodels.StudentQuestionSupervisorStats
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &stats))
	assert.Equal(t, int64(1), stats.TotalQuestions) // Only questions from course1
	assert.Equal(t, int64(0), stats.TotalReplied)
	// Should only show teacher1 (assigned to course1)
	assert.Len(t, stats.Teachers, 1)
	assert.Equal(t, teacher1.Id, stats.Teachers[0].Id)
}
