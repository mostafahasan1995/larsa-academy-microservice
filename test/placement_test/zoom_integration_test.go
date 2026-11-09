package placement

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
	"larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
)

func TestPlacementZoomMeetingCreation(t *testing.T) {
	// Setup DB and auth
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllPlacementTests}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	// Setup App with Mock Zoom Service
	injector := do.New()
	zoomMock := RegisterMockZoomService(injector)
	injector, router, _ := setup.SetupFullTestAppWithInjector(t, injector, dbConfig, authServer.URL())

	// Since PlacementTestService was already created during SetupFullTestAppWithInjector,
	// and it has a reference to the real ZoomService, we need to override it too
	// so it picks up the mock.
	do.Override(injector, content.NewPlacementTestSvcs)

	ctx := context.Background()

	// Helpers
	createTeacher := func(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contactmodels.TeacherDto) contactmodels.Teacher {
		tf := &helpers.TestFixture{Router: router.(*chi.Mux), DBName: dbName, Ctx: ctx, AuthServer: authServer}
		return tf.CreateTeacher(t, dto)
	}
	createStudent := func(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contactmodels.StudentDto) contactmodels.Student {
		tf := &helpers.TestFixture{Router: router.(*chi.Mux), DBName: dbName, Ctx: ctx, AuthServer: authServer}
		return tf.CreateStudent(t, dto)
	}
	createCoaching := func(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contentmodels.CoachingDto) contentmodels.Coaching {
		tf := &helpers.TestFixture{Router: router.(*chi.Mux), DBName: dbName, Ctx: ctx, AuthServer: authServer}
		return tf.CreateCoaching(t, dto)
	}
	createPlacementTest := func(t *testing.T, router http.Handler, dbName string, authServer *setup.MockAuthServer, ctx context.Context, dto *contentmodels.PlacementTestDto) contentmodels.PlacementTest {
		tf := &helpers.TestFixture{Router: router.(*chi.Mux), DBName: dbName, Ctx: ctx, AuthServer: authServer}
		return tf.CreatePlacementTest(t, dto)
	}

	// Create teacher and student
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

	// 1. Student schedules (pending)
	switchAuth := func(userID string, caps []string) {
		cfg := setup.DefaultMockAuthConfig()
		cfg.UserID = userID
		cfg.Capabilities = caps
		authServer.UpdateConfig(cfg)
	}
	switchAuth(student.Id.Hex(), []string{"authenticate", content.CapabilityViewPublicCoaching, content.CapabilityViewStudentCalendarEvents})

	futureDate := time.Now().UTC().AddDate(0, 0, 5)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)

	schedReq := contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   pt.Id,
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "10:00",
		To:                "11:30", // 90 minutes duration
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

	// 2. Admin approves and assigns teacher -> Should trigger Zoom creation
	switchAuth(adminUserID, []string{"authenticate", content.CapabilityManageAllPlacementTests})
	updateDto := contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
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

	// Wait for background goroutine
	time.Sleep(100 * time.Millisecond)

	// Verify Zoom call
	require.Len(t, zoomMock.CreateMeetingCalls, 1)
	call := zoomMock.CreateMeetingCalls[0]
	require.Equal(t, 90, call.Duration) // 10:00 to 11:30 = 90 mins
	require.Equal(t, "placement-test", call.RefType)
	require.Equal(t, schedule.Id, call.RefId)

	// 3. Update time -> Should trigger new Zoom creation (after deleting old one)
	newFrom := "14:00"
	newTo := "15:00" // 60 minutes
	updateDtoTime := contentmodels.UpdatePlacementTestScheduleDto{
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
	require.Equal(t, http.StatusOK, updateWTime.Code)

	// Wait for background goroutine
	time.Sleep(100 * time.Millisecond)

	// Should have another call with new duration
	require.Len(t, zoomMock.CreateMeetingCalls, 2)
	call2 := zoomMock.CreateMeetingCalls[1]
	require.Equal(t, 60, call2.Duration)
}
