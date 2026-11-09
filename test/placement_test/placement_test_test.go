package placement_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	content "larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/test/setup"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestGetStudentPlacementTests tests the GetStudentPlacementTests endpoint
// It should return all submissions/schedules for a student for a given placement test
func TestGetStudentPlacementTests(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	studentID := primitive.NewObjectID().Hex()
	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = studentID
	authConfig.Capabilities = []string{"authenticate"}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Helper to switch auth context
	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}

	// Create teacher and student
	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)
	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)

	// Create coaching and online placement test
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	onlinePT := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
		Type:    "online",
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	})

	// Create electronic placement test with questions
	electronicPT := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
		Type:    "electronic",
		Questions: []contentmodels.Question{
			{
				Question: transl.Localizable[string]{
					"en": "What is 2+2?",
					"ar": "ما هو 2+2؟",
				},
				Answers: []contentmodels.Answer{
					{
						Answer: transl.Localizable[string]{
							"en": "3",
							"ar": "3",
						},
						IsCorrect: false,
					},
					{
						Answer: transl.Localizable[string]{
							"en": "4",
							"ar": "4",
						},
						IsCorrect: true,
					},
				},
			},
		},
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	})

	// Verify enrichment fields are populated from coaching
	require.Equal(t, coaching.Country, electronicPT.Country, "Country should be enriched from coaching")
	require.Equal(t, coaching.ProgramType, electronicPT.ProgramType, "ProgramType should be enriched from coaching")
	require.Equal(t, coaching.SchoolType, electronicPT.SchoolType, "SchoolType should be enriched from coaching")
	require.Equal(t, coaching.Level, electronicPT.Stage, "Stage should be enriched from coaching Level")
	require.Equal(t, coaching.Class, electronicPT.Class, "Class should be enriched from coaching Class")

	// Switch to student context
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})

	// Create multiple online test schedules
	futureDate1 := time.Now().UTC().AddDate(0, 0, 5)
	futureDate1 = time.Date(futureDate1.Year(), futureDate1.Month(), futureDate1.Day(), 0, 0, 0, 0, time.UTC)

	schedReq1 := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   onlinePT.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate1,
		From:              "10:00",
		To:                "11:00",
	}
	body1, _ := json.Marshal(schedReq1)
	req1 := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body1))
	req1.Header.Set("Content-Type", "application/json")
	req1.Header.Set("x-access-token", "test-token")
	req1.Header.Set("x-client", dbConfig.Database)
	req1 = req1.WithContext(ctx)
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	require.Equal(t, http.StatusOK, w1.Code)

	// Reject first schedule to allow second one
	// Use admin context to reject
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests, content.CapabilityViewAllCoachingPrograms})

	var schedule1 contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w1.Body.Bytes(), &schedule1))

	rejectReason := "Testing multiple schedules"
	rejectDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:          contentenums.StudentPlacementTestStatusRejected,
		RejectionReason: &rejectReason,
	}
	rejectBody, _ := json.Marshal(rejectDto)
	rejectReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule1.Id.Hex()+"/time", bytes.NewBuffer(rejectBody))
	rejectReq.Header.Set("Content-Type", "application/json")
	rejectReq.Header.Set("x-access-token", "test-token")
	rejectReq.Header.Set("x-client", dbConfig.Database)
	rejectReq = rejectReq.WithContext(ctx)
	rejectW := httptest.NewRecorder()
	router.ServeHTTP(rejectW, rejectReq)
	require.Equal(t, http.StatusOK, rejectW.Code)

	// Create second schedule (now that first is rejected)
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	futureDate2 := time.Now().UTC().AddDate(0, 0, 7)
	futureDate2 = time.Date(futureDate2.Year(), futureDate2.Month(), futureDate2.Day(), 0, 0, 0, 0, time.UTC)

	schedReq2 := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   onlinePT.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate2,
		From:              "14:00",
		To:                "15:00",
	}
	body2, _ := json.Marshal(schedReq2)
	req2 := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("x-access-token", "test-token")
	req2.Header.Set("x-client", dbConfig.Database)
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	// Submit electronic test
	switchAuth(adminUserID, []string{"authenticate"})

	submitReq := contentmodels.StudentPlacementTestAnswers{
		PlacementTestId: electronicPT.Id,
		TeacherId:       teacher.Id,
		Answers: []contentmodels.StudentAnswer{
			{
				QuestionNum:  1,
				ChosenAnswer: 2, // Correct answer
			},
		},
		TotalTime: 10,
	}
	submitBody, _ := json.Marshal(submitReq)
	submitHTTPReq := httptest.NewRequest("POST", "/api/placement-tests/student/submit", bytes.NewBuffer(submitBody))
	submitHTTPReq.Header.Set("Content-Type", "application/json")
	submitHTTPReq.Header.Set("x-access-token", "test-token")
	submitHTTPReq.Header.Set("x-client", dbConfig.Database)
	submitHTTPReq = submitHTTPReq.WithContext(ctx)

	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	submitW := httptest.NewRecorder()
	router.ServeHTTP(submitW, submitHTTPReq)
	require.Equal(t, http.StatusOK, submitW.Code)

	// Test GetStudentPlacementTests for online test
	getReq := httptest.NewRequest("GET", "/api/placement-tests/student/"+onlinePT.Id.Hex(), nil)
	getReq.Header.Set("x-access-token", "test-token")
	getReq.Header.Set("x-client", dbConfig.Database)
	getReq = getReq.WithContext(ctx)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code, "get student placement tests failed: %s", getW.Body.String())

	var onlineResults contentmodels.StudentPlacementTestList
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &onlineResults))
	require.Equal(t, onlinePT.Id, onlineResults.PlacementTest.Id)
	require.GreaterOrEqual(t, len(onlineResults.Submissions), 2, "should return at least 2 online test schedules")

	// Verify all results are for the correct placement test and student
	for _, result := range onlineResults.Submissions {
		require.Equal(t, onlinePT.Id, result.PlacementTestId)
		require.Equal(t, student.Id, result.StudentId)
		require.Equal(t, "online", result.Type)
	}

	// Test GetStudentPlacementTests for electronic test
	getReq2 := httptest.NewRequest("GET", "/api/placement-tests/student/"+electronicPT.Id.Hex(), nil)
	getReq2.Header.Set("x-access-token", "test-token")
	getReq2.Header.Set("x-client", dbConfig.Database)
	getReq2 = getReq2.WithContext(ctx)
	getW2 := httptest.NewRecorder()
	router.ServeHTTP(getW2, getReq2)
	require.Equal(t, http.StatusOK, getW2.Code)

	var electronicResults contentmodels.StudentPlacementTestList
	require.NoError(t, json.Unmarshal(getW2.Body.Bytes(), &electronicResults))
	require.Equal(t, electronicPT.Id, electronicResults.PlacementTest.Id)
	require.GreaterOrEqual(t, len(electronicResults.Submissions), 1, "should return at least 1 electronic test submission")

	// Verify electronic test result
	for _, result := range electronicResults.Submissions {
		require.Equal(t, electronicPT.Id, result.PlacementTestId)
		require.Equal(t, student.Id, result.StudentId)
		require.Equal(t, "electronic", result.Type)
		require.NotEmpty(t, result.StudentAnswers, "electronic test should have student answers")
		require.Greater(t, result.Score, 0.0, "electronic test should have a score")
		require.NotEmpty(t, result.Result, "electronic test should have a result")
	}

	// Test that student cannot access another student's tests
	otherStudent := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)
	switchAuth(otherStudent.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})

	getReq3 := httptest.NewRequest("GET", "/api/placement-tests/student/"+onlinePT.Id.Hex(), nil)
	getReq3.Header.Set("x-access-token", "test-token")
	getReq3.Header.Set("x-client", dbConfig.Database)
	getReq3 = getReq3.WithContext(ctx)
	getW3 := httptest.NewRecorder()
	router.ServeHTTP(getW3, getReq3)
	require.Equal(t, http.StatusOK, getW3.Code)

	var otherStudentResults contentmodels.StudentPlacementTestList
	require.NoError(t, json.Unmarshal(getW3.Body.Bytes(), &otherStudentResults))
	require.Equal(t, onlinePT.Id, otherStudentResults.PlacementTest.Id)
	require.Equal(t, 0, len(otherStudentResults.Submissions), "other student should not see first student's tests")
}

// TestSubmitTeacherEvaluationWithResult tests that teacher evaluation calculates result correctly
func TestSubmitTeacherEvaluationWithResult(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllPlacementTests}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}

	// Create teacher and student
	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)
	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)

	// Create coaching and online placement test with PassScore = 70
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	pt := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
		Type:    "online",
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	})

	// Student schedules test
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	futureDate := time.Now().UTC().AddDate(0, 0, 5)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)

	schedReq := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "10:00",
		To:                "11:00",
	}
	body, _ := json.Marshal(schedReq)
	req := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))

	// Admin approves and assigns teacher
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests})
	from := "10:00"
	to := "11:00"
	updateDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &futureDate,
		From:      &from,
		To:        &to,
	}
	updateBody, _ := json.Marshal(updateDto)
	updateReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", dbConfig.Database)
	updateReq = updateReq.WithContext(ctx)
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusOK, updateW.Code)

	// Get student placement test ID
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	getReq := httptest.NewRequest("GET", "/api/placement-tests/student/"+pt.Id.Hex(), nil)
	getReq.Header.Set("x-access-token", "test-token")
	getReq.Header.Set("x-client", dbConfig.Database)
	getReq = getReq.WithContext(ctx)
	getW := httptest.NewRecorder()
	router.ServeHTTP(getW, getReq)
	require.Equal(t, http.StatusOK, getW.Code)

	var studentTests contentmodels.StudentPlacementTestList
	require.NoError(t, json.Unmarshal(getW.Body.Bytes(), &studentTests))
	require.Equal(t, pt.Id, studentTests.PlacementTest.Id)
	require.Greater(t, len(studentTests.Submissions), 0)
	studentPlacementTestId := studentTests.Submissions[0].Id.Hex()

	// Mark as completed (simulate session completion)
	// In real flow, this would be done via a separate endpoint or automatically
	// For testing, we'll directly update the status via admin
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests})
	completeDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status: contentenums.StudentPlacementTestStatusCompleted,
	}
	completeBody, _ := json.Marshal(completeDto)
	completeReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(completeBody))
	completeReq.Header.Set("Content-Type", "application/json")
	completeReq.Header.Set("x-access-token", "test-token")
	completeReq.Header.Set("x-client", dbConfig.Database)
	completeReq = completeReq.WithContext(ctx)
	completeW := httptest.NewRecorder()
	router.ServeHTTP(completeW, completeReq)
	require.Equal(t, http.StatusOK, completeW.Code)

	// Teacher submits evaluation with score above PassScore (should pass)
	switchAuth(teacher.Id.Hex(), []string{"authenticate", content.CapabilityAccessTeacherEndpoints})
	evalDto := contentmodels.TeacherEvaluation{
		Score:           85.5,
		Notes:           "Good performance",
		QuickIndecators: []string{"good-listening"},
	}
	evalBody, _ := json.Marshal(evalDto)
	evalReq := httptest.NewRequest("PATCH", "/api/placement-tests/teacher/"+studentPlacementTestId+"/evaluation", bytes.NewBuffer(evalBody))
	evalReq.Header.Set("Content-Type", "application/json")
	evalReq.Header.Set("x-access-token", "test-token")
	evalReq.Header.Set("x-client", dbConfig.Database)
	evalReq = evalReq.WithContext(ctx)
	evalW := httptest.NewRecorder()
	router.ServeHTTP(evalW, evalReq)
	require.Equal(t, http.StatusOK, evalW.Code)

	var evaluatedTest contentmodels.StudentPlacementTest
	require.NoError(t, json.Unmarshal(evalW.Body.Bytes(), &evaluatedTest))
	require.Equal(t, 85.5, evaluatedTest.Score)
	require.Equal(t, "pass", evaluatedTest.Result, "score 85.5 >= 70 should result in pass")
	require.Equal(t, contentenums.StudentPlacementTestStatusEvaluated, evaluatedTest.Status)
	require.Equal(t, "Good performance", evaluatedTest.Notes)

	// Test with score below PassScore (should fail)
	// For this test, we'll use the same test but need to check if retry is allowed
	// Since the first test is already evaluated, we need to check the test settings
	// For simplicity, let's just verify the first evaluation worked correctly
	// and test the fail case by checking the result calculation logic

	// Actually, let's create a new placement test with different settings to test the fail case
	pt2 := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
		Type:    "online",
		Settings: contentmodels.PlacementTestSettings{
			PassScore:     70,
			AllowRetry:    true,
			NumOfAttempts: 3,
		},
	})

	// Create a new schedule for the second test
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	futureDate2 := time.Now().UTC().AddDate(0, 0, 6)
	futureDate2 = time.Date(futureDate2.Year(), futureDate2.Month(), futureDate2.Day(), 0, 0, 0, 0, time.UTC)

	schedReq2 := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt2.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate2,
		From:              "14:00",
		To:                "15:00",
	}
	body2, _ := json.Marshal(schedReq2)
	req2 := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("x-access-token", "test-token")
	req2.Header.Set("x-client", dbConfig.Database)
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code)

	var schedule2 contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &schedule2))

	// Approve and complete
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests})
	from2 := "14:00"
	to2 := "15:00"
	updateDto2 := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &futureDate2,
		From:      &from2,
		To:        &to2,
	}
	updateBody2, _ := json.Marshal(updateDto2)
	updateReq2 := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule2.Id.Hex()+"/time", bytes.NewBuffer(updateBody2))
	updateReq2.Header.Set("Content-Type", "application/json")
	updateReq2.Header.Set("x-access-token", "test-token")
	updateReq2.Header.Set("x-client", dbConfig.Database)
	updateReq2 = updateReq2.WithContext(ctx)
	updateW2 := httptest.NewRecorder()
	router.ServeHTTP(updateW2, updateReq2)
	require.Equal(t, http.StatusOK, updateW2.Code)

	completeDto2 := contentmodels.UpdatePlacementTestScheduleDto{
		Status: contentenums.StudentPlacementTestStatusCompleted,
	}
	completeBody2, _ := json.Marshal(completeDto2)
	completeReq2 := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule2.Id.Hex()+"/time", bytes.NewBuffer(completeBody2))
	completeReq2.Header.Set("Content-Type", "application/json")
	completeReq2.Header.Set("x-access-token", "test-token")
	completeReq2.Header.Set("x-client", dbConfig.Database)
	completeReq2 = completeReq2.WithContext(ctx)
	completeW2 := httptest.NewRecorder()
	router.ServeHTTP(completeW2, completeReq2)
	require.Equal(t, http.StatusOK, completeW2.Code)

	// Get student placement test ID for second test
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	getReq2 := httptest.NewRequest("GET", "/api/placement-tests/student/"+pt2.Id.Hex(), nil)
	getReq2.Header.Set("x-access-token", "test-token")
	getReq2.Header.Set("x-client", dbConfig.Database)
	getReq2 = getReq2.WithContext(ctx)
	getW2 := httptest.NewRecorder()
	router.ServeHTTP(getW2, getReq2)
	require.Equal(t, http.StatusOK, getW2.Code)

	var studentTests2 contentmodels.StudentPlacementTestList
	require.NoError(t, json.Unmarshal(getW2.Body.Bytes(), &studentTests2))
	require.Equal(t, pt2.Id, studentTests2.PlacementTest.Id)
	require.Greater(t, len(studentTests2.Submissions), 0)

	// Find the student placement test by schedule ID (schedule2 was already declared above)
	var secondTestId string
	for _, test := range studentTests2.Submissions {
		if test.ScheduleId == schedule2.Id {
			secondTestId = test.Id.Hex()
			break
		}
	}
	require.NotEmpty(t, secondTestId)

	// Teacher submits evaluation with score below PassScore (should fail)
	switchAuth(teacher.Id.Hex(), []string{"authenticate", content.CapabilityAccessTeacherEndpoints})
	evalDto2 := contentmodels.TeacherEvaluation{
		Score:           65.0,
		Notes:           "Needs improvement",
		QuickIndecators: []string{"needs-practice"},
	}
	evalBody2, _ := json.Marshal(evalDto2)
	evalReq2 := httptest.NewRequest("PATCH", "/api/placement-tests/teacher/"+secondTestId+"/evaluation", bytes.NewBuffer(evalBody2))
	evalReq2.Header.Set("Content-Type", "application/json")
	evalReq2.Header.Set("x-access-token", "test-token")
	evalReq2.Header.Set("x-client", dbConfig.Database)
	evalReq2 = evalReq2.WithContext(ctx)
	evalW2 := httptest.NewRecorder()
	router.ServeHTTP(evalW2, evalReq2)
	require.Equal(t, http.StatusOK, evalW2.Code)

	var evaluatedTest2 contentmodels.StudentPlacementTest
	require.NoError(t, json.Unmarshal(evalW2.Body.Bytes(), &evaluatedTest2))
	require.Equal(t, 65.0, evaluatedTest2.Score)
	require.Equal(t, "fail", evaluatedTest2.Result, "score 65.0 < 70 should result in fail")
	require.Equal(t, contentenums.StudentPlacementTestStatusEvaluated, evaluatedTest2.Status)
}

// TestOnlinePlacementTestCannotHaveQuestions tests that online placement tests cannot have questions
func TestOnlinePlacementTestCannotHaveQuestions(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllPlacementTests}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Try to create online placement test with questions (should fail)
	ptDto := contentmodels.PlacementTestDto{
		Title:    transl.Localizable[string]{"en": "Online Test with Questions"},
		Status:   "active",
		Type:     "online",
		RefType:  "coaching",
		RefId:    coaching.Id,
		TestMark: 100,
		Questions: []contentmodels.Question{
			{
				Question: transl.Localizable[string]{
					"en": "Test question",
					"ar": "سؤال تجريبي",
				},
				Answers: []contentmodels.Answer{
					{
						Answer: transl.Localizable[string]{
							"en": "Answer",
							"ar": "إجابة",
						},
						IsCorrect: true,
					},
				},
			},
		},
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	}

	body, _ := json.Marshal(ptDto)
	req := httptest.NewRequest("POST", "/placement-tests", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "should reject online test with questions")
	require.Contains(t, w.Body.String(), "online placement tests cannot have questions")

	// Create online placement test without questions (should succeed)
	ptDto2 := contentmodels.PlacementTestDto{
		Title:     transl.Localizable[string]{"en": "Online Test without Questions"},
		Status:    "active",
		Type:      "online",
		RefType:   "coaching",
		RefId:     coaching.Id,
		TestMark:  100,
		Questions: []contentmodels.Question{},
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	}

	body2, _ := json.Marshal(ptDto2)
	req2 := httptest.NewRequest("POST", "/placement-tests", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("x-access-token", "test-token")
	req2.Header.Set("x-client", dbConfig.Database)
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, "should allow online test without questions")

	// Try to update online test to add questions (should fail)
	var createdPT contentmodels.PlacementTest
	require.NoError(t, json.Unmarshal(w2.Body.Bytes(), &createdPT))

	updateDto := contentmodels.PlacementTestDto{
		Title:    transl.Localizable[string]{"en": "Updated Online Test"},
		Status:   "active",
		Type:     "online",
		RefType:  "coaching",
		RefId:    coaching.Id,
		TestMark: 100,
		Questions: []contentmodels.Question{
			{
				Question: transl.Localizable[string]{
					"en": "New question",
					"ar": "سؤال جديد",
				},
				Answers: []contentmodels.Answer{
					{
						Answer: transl.Localizable[string]{
							"en": "Answer",
							"ar": "إجابة",
						},
						IsCorrect: true,
					},
				},
			},
		},
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	}

	updateBody, _ := json.Marshal(updateDto)
	updateReq := httptest.NewRequest("PUT", "/placement-tests/"+createdPT.Id.Hex(), bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", dbConfig.Database)
	updateReq = updateReq.WithContext(ctx)
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusBadRequest, updateW.Code, "should reject updating online test with questions")
	require.Contains(t, updateW.Body.String(), "online placement tests cannot have questions")
}

// TestElectronicPlacementTestMustHaveQuestions tests that electronic placement tests must have questions
func TestElectronicPlacementTestMustHaveQuestions(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllPlacementTests}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Try to create electronic placement test without questions (should fail)
	ptDto := contentmodels.PlacementTestDto{
		Title:     transl.Localizable[string]{"en": "Electronic Test Without Questions"},
		Status:    "active",
		Type:      "electronic",
		RefType:   "coaching",
		RefId:     coaching.Id,
		TestMark:  100,
		Questions: []contentmodels.Question{},
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	}

	body, _ := json.Marshal(ptDto)
	req := httptest.NewRequest("POST", "/placement-tests", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusBadRequest, w.Code, "should reject electronic test without questions")
	require.Contains(t, w.Body.String(), "electronic placement tests must have questions")

	// Create electronic placement test with questions (should succeed)
	ptDto2 := contentmodels.PlacementTestDto{
		Title:    transl.Localizable[string]{"en": "Electronic Test With Questions"},
		Status:   "active",
		Type:     "electronic",
		RefType:  "coaching",
		RefId:    coaching.Id,
		TestMark: 100,
		Questions: []contentmodels.Question{
			{
				Question: transl.Localizable[string]{
					"en": "What is 2+2?",
					"ar": "ما هو 2+2؟",
				},
				Answers: []contentmodels.Answer{
					{
						Answer: transl.Localizable[string]{
							"en": "3",
							"ar": "3",
						},
						IsCorrect: false,
					},
					{
						Answer: transl.Localizable[string]{
							"en": "4",
							"ar": "4",
						},
						IsCorrect: true,
					},
				},
			},
		},
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	}

	body2, _ := json.Marshal(ptDto2)
	req2 := httptest.NewRequest("POST", "/placement-tests", bytes.NewBuffer(body2))
	req2.Header.Set("Content-Type", "application/json")
	req2.Header.Set("x-access-token", "test-token")
	req2.Header.Set("x-client", dbConfig.Database)
	req2 = req2.WithContext(ctx)
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	require.Equal(t, http.StatusOK, w2.Code, "should allow electronic test with questions")
}

// TestPlacementTestEnrichment verifies that enrichment fields are correctly populated from referenced courses and coaching programs
func TestPlacementTestEnrichment(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{
		"authenticate",
		content.CapabilityManageAllPlacementTests,
		content.CapabilityViewAllCoachingPrograms,
		"ManageAllCourses",
		"ViewAllCourses",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Create teacher
	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)

	// Create coaching program with specific details
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Title: transl.Localizable[string]{
			"en": "Math Coaching Program",
			"ar": "برنامج الرياضيات التعليمي",
		},
		ProgramType: "foundational",
		Subject:     primitive.NewObjectID(),
		Level:       "beginner",
		Class:       []string{"Grade 10"},
		SchoolType:  "public",
		Country:     "Saudi Arabia",
		Coachs:      []primitive.ObjectID{teacher.Id},
	})

	// Create course with specific details
	courseDto := contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name: transl.Localizable[string]{
				"en": "Mathematics Course",
				"ar": "دورة الرياضيات",
			},
			Type:       "curriculum",
			Status:     "active",
			Country:    "UAE",
			SchoolType: "public",
			GradeLevel: "grade_9",
			Level:      "beginner",
		},
	}
	courseBody, _ := json.Marshal(courseDto)
	courseReq := httptest.NewRequest("POST", "/courses", bytes.NewBuffer(courseBody))
	courseReq.Header.Set("Content-Type", "application/json")
	courseReq.Header.Set("x-access-token", "test-token")
	courseReq.Header.Set("x-client", dbConfig.Database)
	courseReq = courseReq.WithContext(ctx)
	courseW := httptest.NewRecorder()
	router.ServeHTTP(courseW, courseReq)
	require.Equal(t, http.StatusOK, courseW.Code)

	var course contentmodels.Course
	require.NoError(t, json.Unmarshal(courseW.Body.Bytes(), &course))

	// Test coaching-based placement test enrichment
	coachingPT := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
		Type:    "online",
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     80,
		},
	})

	// Verify enrichment from coaching
	require.Equal(t, coaching.Country, coachingPT.Country, "Country should be enriched from coaching")
	require.Equal(t, coaching.ProgramType, coachingPT.ProgramType, "ProgramType should be enriched from coaching")
	require.Equal(t, coaching.SchoolType, coachingPT.SchoolType, "SchoolType should be enriched from coaching")
	require.Equal(t, coaching.Level, coachingPT.Stage, "Stage should be enriched from coaching Level")
	require.Equal(t, coaching.Class, coachingPT.Class, "Class should be enriched from coaching Class")

	// Test course-based placement test enrichment
	coursePT := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "course",
		RefId:   course.Id,
		Type:    "online",
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     75,
		},
	})

	// Verify enrichment from course
	require.Equal(t, course.BasicInfo.Country, coursePT.Country, "Country should be enriched from course")
	require.Equal(t, course.BasicInfo.Type, coursePT.ProgramType, "ProgramType should be enriched from course Type")
	require.Equal(t, course.BasicInfo.SchoolType, coursePT.SchoolType, "SchoolType should be enriched from course")
	require.Equal(t, course.BasicInfo.GradeLevel, coursePT.Stage, "Stage should be enriched from course GradeLevel")
	require.Equal(t, []string{course.BasicInfo.Level}, coursePT.Class, "Class should be enriched from course Level")

	// Test GET endpoint enrichment
	req := httptest.NewRequest("GET", "/placement-tests/"+coachingPT.Id.Hex(), nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var retrievedPT contentmodels.PlacementTest
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &retrievedPT))

	// Verify GET also returns enriched fields
	require.Equal(t, coaching.Country, retrievedPT.Country, "GET should return enriched Country")
	require.Equal(t, coaching.ProgramType, retrievedPT.ProgramType, "GET should return enriched ProgramType")
	require.Equal(t, coaching.Level, retrievedPT.Stage, "GET should return enriched Stage")
	require.Equal(t, coaching.Class, retrievedPT.Class, "GET should return enriched Class")
}

func TestAdminSubmitEvaluation(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllPlacementTests}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}

	// Create teacher and student
	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)
	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)

	// Create coaching and online placement test
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	pt := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
		Type:    "online",
		Settings: contentmodels.PlacementTestSettings{
			NumOfAttempts: 1,
			PassScore:     70,
		},
	})

	// Student schedules test
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	futureDate := time.Now().UTC().AddDate(0, 0, 5)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)

	schedReq := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "10:00",
		To:                "11:00",
	}
	body, _ := json.Marshal(schedReq)
	req := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code)

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))

	// Admin approves and assigns teacher
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests})
	from := "10:00"
	to := "11:00"
	updateDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &futureDate,
		From:      &from,
		To:        &to,
	}
	updateBody, _ := json.Marshal(updateDto)
	updateReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", dbConfig.Database)
	updateReq = updateReq.WithContext(ctx)
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusOK, updateW.Code)

	// Mark as completed
	completeDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status: contentenums.StudentPlacementTestStatusCompleted,
	}
	completeBody, _ := json.Marshal(completeDto)
	completeReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(completeBody))
	completeReq.Header.Set("Content-Type", "application/json")
	completeReq.Header.Set("x-access-token", "test-token")
	completeReq.Header.Set("x-client", dbConfig.Database)
	completeReq = completeReq.WithContext(ctx)
	completeW := httptest.NewRecorder()
	router.ServeHTTP(completeW, completeReq)
	require.Equal(t, http.StatusOK, completeW.Code)

	// Get student placement test ID
	var scheduleUpdated contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(completeW.Body.Bytes(), &scheduleUpdated))
	studentPlacementTestId := scheduleUpdated.StudentPlacementTestId.Hex()

	// Admin submits evaluation (Admin is NOT the assigned teacher)
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests})
	evalDto := contentmodels.TeacherEvaluation{
		Score:           90.0,
		Notes:           "Evaluated by admin",
		QuickIndecators: []string{"excellent"},
	}
	evalBody, _ := json.Marshal(evalDto)
	evalReq := httptest.NewRequest("PATCH", "/api/placement-tests/teacher/"+studentPlacementTestId+"/evaluation", bytes.NewBuffer(evalBody))
	evalReq.Header.Set("Content-Type", "application/json")
	evalReq.Header.Set("x-access-token", "test-token")
	evalReq.Header.Set("x-client", dbConfig.Database)
	evalReq = evalReq.WithContext(ctx)
	evalW := httptest.NewRecorder()
	router.ServeHTTP(evalW, evalReq)
	require.Equal(t, http.StatusOK, evalW.Code, "Admin should be able to evaluate even if not assigned teacher")

	var evaluatedTest contentmodels.StudentPlacementTest
	require.NoError(t, json.Unmarshal(evalW.Body.Bytes(), &evaluatedTest))
	require.Equal(t, 90.0, evaluatedTest.Score)
	require.Equal(t, "pass", evaluatedTest.Result)
	require.Equal(t, contentenums.StudentPlacementTestStatusEvaluated, evaluatedTest.Status)

	// Test unauthorized case (another teacher who is not assigned and not admin)
	otherTeacherID := primitive.NewObjectID().Hex()
	switchAuth(otherTeacherID, []string{"authenticate", content.CapabilityAccessTeacherEndpoints})

	evalReq2 := httptest.NewRequest("PATCH", "/api/placement-tests/teacher/"+studentPlacementTestId+"/evaluation", bytes.NewBuffer(evalBody))
	evalReq2.Header.Set("Content-Type", "application/json")
	evalReq2.Header.Set("x-access-token", "test-token")
	evalReq2.Header.Set("x-client", dbConfig.Database)
	evalReq2 = evalReq2.WithContext(ctx)
	evalW2 := httptest.NewRecorder()
	router.ServeHTTP(evalW2, evalReq2)
	require.Equal(t, http.StatusUnauthorized, evalW2.Code, "Other teacher should NOT be able to evaluate")
}
