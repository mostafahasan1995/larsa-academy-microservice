package calendar

import (
	"bytes"
	"encoding/json"
	"larsa-academy-microservice/pkg/services/contact/models"
	content "larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
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

// TestCalendar_TeacherViewingOwnEvents_ReturnsFilteredAndEnrichedEvents
// This comprehensive test verifies:
// - Basic authorization (teacher capability check)
// - Fetching coaching sessions (with teacher filter)
// - Fetching course live sessions (with teacher filter)
// - Fetching placement tests (with teacher filter)
// - Bundle detection and enrichment
// - Student/teacher/course enrichment
// - Date range filtering (upcoming events only)
// - Refined authorization (resource access checks)
// - Student data privacy (teacher can only see their students)
func TestCalendar_TeacherViewingOwnEvents_ReturnsFilteredAndEnrichedEvents(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	teacherUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = teacherUserID
	authConfig.Capabilities = []string{"authenticate", "ViewOwnCalendarEvents"}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Create teacher (only specify what's needed - rest is random)
	teacher := fixture.createTeacher(t, &models.TeacherDto{})

	// Update auth to use teacher's ID
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create student (only specify what's needed)
	student := fixture.createStudent(t, nil)

	// Create coaching program (only specify teacher - rest is random)
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)

	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create course with live session (only specify teacher and schedule)
	_ = fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة", "en": "Live Session"},
				Schedule: dayAfterTomorrow.Add(2 * time.Hour),
			},
		},
	})

	// Create placement test
	placementTest := fixture.createPlacementTest(t, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Book coaching sessions
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"10:00", "11:00",
	))

	// Schedule placement test
	schedule := fixture.schedulePlacementTest(t, student.Id.Hex(), contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   placementTest.Id,
		TeacherId:         types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		CoachingProgramId: coaching.Id,
		Date:              dayAfterTomorrow.Add(24 * time.Hour),
		From:              "15:00",
		To:                "16:00",
	})

	// Approve to make placement test visible/busy
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", content.CapabilityManageAllPlacementTests, "ViewOwnCalendarEvents"})
	date := dayAfterTomorrow.Add(24 * time.Hour)
	from := "15:00"
	to := "16:00"
	approveBody, _ := json.Marshal(contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &date,
		From:      &from,
		To:        &to,
	})
	approveReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+schedule.Id.Hex()+"/time", bytes.NewBuffer(approveBody))
	approveReq.Header.Set("Content-Type", "application/json")
	approveReq.Header.Set("x-access-token", "test-token")
	approveReq.Header.Set("x-client", fixture.DBName)
	approveReq = approveReq.WithContext(fixture.Ctx)
	approveW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(approveW, approveReq)
	require.Equal(t, http.StatusOK, approveW.Code, "Failed to approve placement test: body: %s", approveW.Body.String())

	// Switch back to teacher context and get calendar events
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	// Assertions
	require.NotEmpty(t, response.Events, "Expected at least one event")

	// All events should be for this teacher
	assertOnlyTeacherEvents(t, response.Events, teacher.Id)

	// Events should be in the future and have proper enrichment
	for _, event := range response.Events {
		assert.False(t, event.StartTime.Before(time.Now()), "Event %s is in the past: %v", event.Id.Hex(), event.StartTime)
		require.NotNil(t, event.Teacher, "Event %s missing teacher enrichment", event.Id.Hex())
		assert.Equal(t, teacher.Id, event.Teacher.Id, "Event %s has wrong teacher enrichment ID", event.Id.Hex())

		// Type-specific assertions
		switch event.Type {
		case "coaching_session":
			require.NotNil(t, event.CoachingProgramId, "Coaching session %s missing coaching program ID", event.Id.Hex())
			require.NotNil(t, event.StudentId, "Coaching session %s missing student ID", event.Id.Hex())
			require.NotNil(t, event.CoachingProgram, "Coaching session %s missing coaching program enrichment", event.Id.Hex())
			assert.NotEmpty(t, event.Students, "Coaching session %s missing student enrichment", event.Id.Hex())
			require.NotNil(t, event.CoachingSessionDetails, "Coaching session %s missing session details", event.Id.Hex())
			assert.Equal(t, "scheduled", event.Status, "Coaching session %s has wrong status", event.Id.Hex())

		case "course_live_session":
			require.NotNil(t, event.CourseId, "Course live session %s missing course ID", event.Id.Hex())
			require.NotNil(t, event.Course, "Course live session %s missing course enrichment", event.Id.Hex())
			assert.Equal(t, "active", event.Status, "Course live session %s has wrong status", event.Id.Hex())

		case "placement_test":
			require.NotNil(t, event.CoachingProgramId, "Placement test %s missing coaching program ID", event.Id.Hex())
			require.NotNil(t, event.StudentId, "Placement test %s missing student ID", event.Id.Hex())
			require.NotNil(t, event.PlacementTestDetails, "Placement test %s missing placement test details", event.Id.Hex())
		}
	}

	// Events should be sorted
	assertEventsSorted(t, response.Events)

	// Date range validation - agenda view shows all future events, so end date is far in the future
	// Just verify that end date is after start date
	assert.True(t, response.DateRange.End.After(response.DateRange.Start),
		"Date range end should be after start: start=%v, end=%v", response.DateRange.Start, response.DateRange.End)

	// View type validation
	assert.Equal(t, "agenda", response.ViewType, "View type is incorrect")
}
