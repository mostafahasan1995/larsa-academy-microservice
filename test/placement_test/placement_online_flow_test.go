package placement_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"larsa-academy-microservice/test/helpers"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/go-chi/chi/v5"

	"larsa-academy-microservice/pkg/db"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
	content "larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/setup"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestPlacementOnlineFullFlow covers:
// 1) Student schedules online placement test (pending, no teacher).
// 2) Admin approves and assigns teacher via unified update endpoint.
// 3) Teacher updates time (allowed).
// 4) Teacher cannot change assigned teacher (forbidden).
func TestPlacementOnlineFullFlow(t *testing.T) {
	// Setup DB and auth
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllPlacementTests}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	// Helper to switch auth context
	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}

	ctx := context.Background()

	// Create teacher and student under admin context
	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, nil)
	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)

	// Create coaching and placement test
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	pt := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Student schedules without teacher (pending)
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
	require.Equal(t, http.StatusOK, w.Code, "schedule placement test failed: %s", w.Body.String())

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))
	require.Equal(t, contentenums.StudentPlacementTestStatusPending, schedule.Status)

	// Admin approves and assigns teacher
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests, content.CapabilityViewAllCalendarEvents, content.CapabilityViewOwnCalendarEvents})
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
	require.Equal(t, http.StatusOK, updateW.Code, "approve placement test failed: %s", updateW.Body.String())

	// Teacher updates time (allowed)
	switchAuth(teacher.Id.Hex(), []string{"authenticate", content.CapabilityViewOwnCalendarEvents, content.CapabilityAccessTeacherEndpoints})
	newFrom := "10:30"
	newTo := "11:30"
	newDate := futureDate.AddDate(0, 0, 1)
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, time.UTC)
	updateDtoTime := contentmodels.UpdatePlacementTestScheduleDto{
		Date: &newDate,
		From: &newFrom,
		To:   &newTo,
	}
	updateBodyTime, _ := json.Marshal(updateDtoTime)
	updateReqTime := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(updateBodyTime))
	updateReqTime.Header.Set("Content-Type", "application/json")
	updateReqTime.Header.Set("x-access-token", "test-token")
	updateReqTime.Header.Set("x-client", dbConfig.Database)
	updateReqTime = updateReqTime.WithContext(ctx)
	updateWTime := httptest.NewRecorder()
	router.ServeHTTP(updateWTime, updateReqTime)
	require.Equal(t, http.StatusOK, updateWTime.Code, "teacher time update failed: %s", updateWTime.Body.String())

	// Teacher attempts to reassign teacher (should fail)
	otherTeacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, &contactmodels.TeacherDto{})
	updateDtoTeacher := contentmodels.UpdatePlacementTestScheduleDto{
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &otherTeacher.Id},
	}
	updateBodyTeacher, _ := json.Marshal(updateDtoTeacher)
	updateReqTeacher := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(updateBodyTeacher))
	updateReqTeacher.Header.Set("Content-Type", "application/json")
	updateReqTeacher.Header.Set("x-access-token", "test-token")
	updateReqTeacher.Header.Set("x-client", dbConfig.Database)
	updateReqTeacher = updateReqTeacher.WithContext(ctx)
	updateWTeacher := httptest.NewRecorder()
	router.ServeHTTP(updateWTeacher, updateReqTeacher)
	require.Equal(t, http.StatusUnauthorized, updateWTeacher.Code, "teacher reassign should fail: %s", updateWTeacher.Body.String())
}

// Reject flow and calendar visibility: pending should not show; rejected stays hidden
func TestPlacementOnlineRejectFlowAndCalendarVisibility(t *testing.T) {
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

	// Create teacher and student
	teacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, &contactmodels.TeacherDto{})
	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)

	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	pt := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Student schedules (pending)
	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})

	futureDate := time.Now().UTC().AddDate(0, 0, 3)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)

	schedReq := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "09:00",
		To:                "10:00",
	}
	body, _ := json.Marshal(schedReq)
	req := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "schedule placement test failed: %s", w.Body.String())

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))
	require.Equal(t, contentenums.StudentPlacementTestStatusPending, schedule.Status)

	// Calendar should not show pending event (use teacher context for calendar access)
	switchAuth(teacher.Id.Hex(), []string{"authenticate", content.CapabilityViewOwnCalendarEvents})
	calendarFilters := contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	}
	events := getCalendarEvents(t, router, dbConfig.Database, authServer, ctx, calendarFilters)
	require.Empty(t, events.Events, "pending placement test should not appear in calendar")

	// Reject
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests, content.CapabilityViewAllCalendarEvents})
	reason := "No availability"
	updateDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:          contentenums.StudentPlacementTestStatusRejected,
		RejectionReason: &reason,
	}
	updateBody, _ := json.Marshal(updateDto)
	updateReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", dbConfig.Database)
	updateReq = updateReq.WithContext(ctx)
	updateW := httptest.NewRecorder()
	router.ServeHTTP(updateW, updateReq)
	require.Equal(t, http.StatusOK, updateW.Code, "reject placement test failed: %s", updateW.Body.String())

	// Calendar still should not show
	switchAuth(teacher.Id.Hex(), []string{"authenticate", content.CapabilityViewOwnCalendarEvents})
	events2 := getCalendarEvents(t, router, dbConfig.Database, authServer, ctx, calendarFilters)
	require.Empty(t, events2.Events, "rejected placement test should not appear in calendar")
}

// Supervisor can only manage teachers they oversee
func TestPlacementOnlineSupervisorScoping(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	supervisorID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = supervisorID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageSupervisedPlacementTests, content.CapabilityViewSupervisedCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{Id: mustHex(supervisorID)},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Teacher supervised by supervisor
	supervisedTeacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, &contactmodels.TeacherDto{})
	// Teacher not supervised
	otherTeacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, &contactmodels.TeacherDto{})

	// Persist supervisor assignment linking supervisor to supervisedTeacher
	assignRepo := db.NewRepo[contentmodels.SupervisorAssignment](injector)
	assign := &contentmodels.SupervisorAssignment{
		Id: primitive.NewObjectID(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId: mustHex(supervisorID),
			AssignType:   "direct",
			RefType:      "teacher",
			RefIds:       []primitive.ObjectID{supervisedTeacher.Id},
			Status:       "assigned",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		CreatedBy: mustHex(supervisorID),
		UpdatedBy: mustHex(supervisorID),
	}
	err := assignRepo.Add(ctx, assign)
	require.NoError(t, err)

	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)
	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{supervisedTeacher.Id, otherTeacher.Id},
	})
	pt := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Student schedules pending
	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	futureDate := time.Now().UTC().AddDate(0, 0, 2)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)
	schedReq := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "08:00",
		To:                "09:00",
	}
	body, _ := json.Marshal(schedReq)
	req := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "schedule placement test failed: %s", w.Body.String())

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))

	// Supervisor approves with supervised teacher -> succeed
	switchAuth(supervisorID, []string{"authenticate", content.CapabilityManageSupervisedPlacementTests})
	from := "08:00"
	to := "09:00"
	updateDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &supervisedTeacher.Id},
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
	require.Equal(t, http.StatusOK, updateW.Code, "supervisor approve should succeed: %s", updateW.Body.String())

	// Supervisor tries assigning non-supervised teacher -> fail
	updateDto2 := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &otherTeacher.Id},
		Date:      &futureDate,
		From:      &from,
		To:        &to,
	}
	updateBody2, _ := json.Marshal(updateDto2)
	updateReq2 := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(updateBody2))
	updateReq2.Header.Set("Content-Type", "application/json")
	updateReq2.Header.Set("x-access-token", "test-token")
	updateReq2.Header.Set("x-client", dbConfig.Database)
	updateReq2 = updateReq2.WithContext(ctx)
	updateW2 := httptest.NewRecorder()
	router.ServeHTTP(updateW2, updateReq2)
	require.Equal(t, http.StatusUnauthorized, updateW2.Code, "supervisor assigning non-supervised teacher should fail")
}

// Supervisor approval should also respect supervision scope when approving/scheduling
func TestPlacementOnlineSupervisorApprovalRequiresSupervisedTeacher(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	supervisorID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = supervisorID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageSupervisedPlacementTests, content.CapabilityViewSupervisedCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Teachers: one supervised, one not supervised
	supervisedTeacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, &contactmodels.TeacherDto{
		Subjects: []contactmodels.Subject{
			{SupervisorId: mustHex(supervisorID)},
		},
	})
	otherTeacher := createTeacher(t, router, dbConfig.Database, authServer, ctx, &contactmodels.TeacherDto{})
	student := createStudent(t, router, dbConfig.Database, authServer, ctx, nil)

	coaching := createCoaching(t, router, dbConfig.Database, authServer, ctx, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{supervisedTeacher.Id, otherTeacher.Id},
	})
	pt := createPlacementTest(t, router, dbConfig.Database, authServer, ctx, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Student schedules pending (no teacher assignment)
	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})
	futureDate := time.Now().UTC().AddDate(0, 0, 2)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)
	schedReq := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "08:00",
		To:                "09:00",
	}
	body, _ := json.Marshal(schedReq)
	req := httptest.NewRequest("POST", "/api/placement-tests/student/schedule", bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbConfig.Database)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "schedule placement test failed: %s", w.Body.String())

	var schedule contentmodels.OnlinePlacementTestSchedule
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &schedule))

	// Supervisor tries to approve with non-supervised teacher -> expect unauthorized
	switchAuth(supervisorID, []string{"authenticate", content.CapabilityManageSupervisedPlacementTests})
	from := "08:00"
	to := "09:00"
	updateDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &otherTeacher.Id},
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
	require.Equal(t, http.StatusUnauthorized, updateW.Code, "supervisor approving with non-supervised teacher should fail")
}

// getCalendarEvents helper for placement tests
func getCalendarEvents(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, filters contentmodels.CalendarFilters) contentmodels.CalendarEventsResponse {
	reqBody, _ := json.Marshal(filters)
	req := httptest.NewRequest("POST", "/api/calendar/events?view=agenda", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", dbName)
	req = req.WithContext(ctx)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)
	require.Equal(t, http.StatusOK, w.Code, "calendar events request failed: %s", w.Body.String())
	var resp contentmodels.CalendarEventsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &resp))
	return resp
}

// mustHex converts hex string to ObjectID or fails the test
func mustHex(hex string) primitive.ObjectID {
	oid, err := primitive.ObjectIDFromHex(hex)
	if err != nil {
		panic(err)
	}
	return oid
}

// helper wrappers using shared helpers.TestFixture functions directly
func createTeacher(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contactmodels.TeacherDto) contactmodels.Teacher {
	tf := helpersFixture(router, dbName, authServer, ctx)
	return tf.CreateTeacher(t, dto)
}

func createStudent(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contactmodels.StudentDto) contactmodels.Student {
	tf := helpersFixture(router, dbName, authServer, ctx)
	return tf.CreateStudent(t, dto)
}

func createCoaching(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contentmodels.CoachingDto) contentmodels.Coaching {
	tf := helpersFixture(router, dbName, authServer, ctx)
	return tf.CreateCoaching(t, dto)
}

func helpersFixture(router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context) *helpers.TestFixture {
	return &helpers.TestFixture{
		Router:     router.(*chi.Mux),
		DBName:     dbName,
		Ctx:        ctx,
		AuthServer: authServer,
	}
}

func createPlacementTest(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contentmodels.PlacementTestDto) contentmodels.PlacementTest {
	tf := helpersFixture(router, dbName, authServer, ctx)
	return tf.CreatePlacementTest(t, dto)
}
