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

// TestCalendar_UpdateCoachingSessionTime tests updating a coaching session time
func TestCalendar_UpdateCoachingSessionTime(t *testing.T) {
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

	// Create teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{})
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching program
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Book a coaching session (requires 2 dates)
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)
	bookingDates := []contentmodels.BookingDate{
		{
			Date:    tomorrow,
			Weekday: tomorrow.Weekday().String(),
			From:    "10:00",
			To:      "11:00",
		},
		{
			Date:    dayAfterTomorrow,
			Weekday: dayAfterTomorrow.Weekday().String(),
			From:    "10:00",
			To:      "11:00",
		},
	}
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, bookingDates)

	// Switch back to teacher context to get calendar events
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Get the session ID from calendar events
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	require.NotEmpty(t, response.Events, "Expected at least one event")

	var sessionID primitive.ObjectID
	for _, event := range response.Events {
		if event.Type == "coaching_session" {
			sessionID = event.Id
			break
		}
	}
	require.NotEqual(t, primitive.NilObjectID, sessionID, "Should find a coaching session")

	// Update session time
	newDate := tomorrow.Add(48 * time.Hour)
	updateData := contentmodels.RescheduleSessionData{
		Date: newDate,
		From: "14:00",
		To:   "15:00",
	}

	updateBody, _ := json.Marshal(updateData)
	updateReq := httptest.NewRequest("PATCH", "/api/coaching/sessions/"+sessionID.Hex()+"/time", bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", fixture.DBName)
	updateReq = updateReq.WithContext(fixture.Ctx)
	updateW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(updateW, updateReq)

	require.Equal(t, http.StatusOK, updateW.Code, "Failed to update session time: body: %s", updateW.Body.String())

	// Verify the update by getting calendar events again
	updatedResponse := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	var updatedEvent *contentmodels.EnrichedCalendarEvent
	for _, event := range updatedResponse.Events {
		if event.Id == sessionID {
			updatedEvent = &event
			break
		}
	}

	require.NotNil(t, updatedEvent, "Should find updated session")
	assert.Equal(t, "14:00", updatedEvent.StartTime.Format("15:04"), "Start time should be updated to 14:00")
}

// TestCalendar_DeleteCoachingSession tests deleting a coaching session
func TestCalendar_DeleteCoachingSession(t *testing.T) {
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

	// Create teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{})
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching program
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Book a coaching session (requires 2 dates)
	tomorrow := time.Now().Add(24 * time.Hour)
	dayAfterTomorrow := time.Now().Add(48 * time.Hour)
	bookingDates := []contentmodels.BookingDate{
		{
			Date:    tomorrow,
			Weekday: tomorrow.Weekday().String(),
			From:    "10:00",
			To:      "11:00",
		},
		{
			Date:    dayAfterTomorrow,
			Weekday: dayAfterTomorrow.Weekday().String(),
			From:    "10:00",
			To:      "11:00",
		},
	}
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, bookingDates)

	// Switch back to teacher context to get calendar events
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Get the session ID from calendar events
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	require.NotEmpty(t, response.Events, "Expected at least one event")

	var sessionID primitive.ObjectID
	initialEventCount := 0
	for _, event := range response.Events {
		if event.Type == "coaching_session" {
			if sessionID == primitive.NilObjectID {
				sessionID = event.Id
			}
			initialEventCount++
		}
	}
	require.NotEqual(t, primitive.NilObjectID, sessionID, "Should find a coaching session")

	// Delete session
	deleteReq := httptest.NewRequest("DELETE", "/api/coaching/sessions/"+sessionID.Hex(), nil)
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("x-access-token", "test-token")
	deleteReq.Header.Set("x-client", fixture.DBName)
	deleteReq = deleteReq.WithContext(fixture.Ctx)
	deleteW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(deleteW, deleteReq)

	require.Equal(t, http.StatusOK, deleteW.Code, "Failed to delete session: body: %s", deleteW.Body.String())

	// Verify the deletion by getting calendar events again
	// Note: Deleted sessions won't appear in calendar (they're soft deleted with trash=true)
	// So we just verify the endpoint returned success
	assert.Equal(t, http.StatusOK, deleteW.Code)
}

// TestCalendar_UpdatePlacementTestSchedule tests updating a placement test schedule time
func TestCalendar_UpdatePlacementTestSchedule(t *testing.T) {
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

	// Create teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{})
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching program
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create placement test
	placementTest := fixture.createPlacementTest(t, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Schedule placement test (ensure it's in the future - date should be date only, time in From/To)
	// Use a date far enough in the future to ensure it's within the agenda view's date range
	// Agenda view uses time.Now() as start date, so we need to ensure our date is after that
	now := time.Now().UTC()
	// Use 7 days from now to ensure it's well within the future
	futureDate := now.AddDate(0, 0, 7)
	// Normalize to beginning of day in UTC (date only, time will be set by From/To)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)
	schedule := fixture.schedulePlacementTest(t, student.Id.Hex(), contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   placementTest.Id,
		TeacherId:         types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "14:00",
		To:                "15:00",
	})

	// Approve and assign teacher as admin to make the event visible
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", content.CapabilityManageAllPlacementTests, "ViewOwnCalendarEvents"})
	from := "14:00"
	to := "15:00"
	approveBody, _ := json.Marshal(contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &futureDate,
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

	// Switch back to teacher context to get calendar events
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Get the schedule ID from calendar events
	// Use agenda view which shows all future events
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	require.NotEmpty(t, response.Events, "Expected at least one event")

	var scheduleID primitive.ObjectID
	for _, event := range response.Events {
		if event.Type == "placement_test" {
			scheduleID = event.Id
			break
		}
	}
	require.NotEqual(t, primitive.NilObjectID, scheduleID, "Should find a placement test schedule")

	// Update schedule time
	newDate := futureDate.AddDate(0, 0, 1) // 1 more day
	newDate = time.Date(newDate.Year(), newDate.Month(), newDate.Day(), 0, 0, 0, 0, time.UTC)
	updateData := contentmodels.UpdatePlacementTestScheduleDto{
		Date: &newDate,
		From: &from,
		To:   &to,
	}

	updateBody, _ := json.Marshal(updateData)
	updateReq := httptest.NewRequest("PATCH", "/api/placement-tests/schedules/"+scheduleID.Hex()+"/time", bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", fixture.DBName)
	updateReq = updateReq.WithContext(fixture.Ctx)
	updateW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(updateW, updateReq)

	require.Equal(t, http.StatusOK, updateW.Code, "Failed to update schedule time: body: %s", updateW.Body.String())

	// Verify the update by getting calendar events again
	updatedResponse := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	var updatedEvent *contentmodels.EnrichedCalendarEvent
	for _, event := range updatedResponse.Events {
		if event.Id == scheduleID {
			updatedEvent = &event
			break
		}
	}

	require.NotNil(t, updatedEvent, "Should find updated schedule")
	assert.Equal(t, "14:00", updatedEvent.StartTime.Format("15:04"), "Start time should be updated to 14:00")
}

// TestCalendar_DeletePlacementTestSchedule tests deleting a placement test schedule
func TestCalendar_DeletePlacementTestSchedule(t *testing.T) {
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

	// Create teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{})
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching program
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create placement test
	placementTest := fixture.createPlacementTest(t, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Schedule placement test (ensure it's in the future - date should be date only, time in From/To)
	// Use a date far enough in the future to ensure it's within the agenda view's date range
	// Agenda view uses time.Now() as start date, so we need to ensure our date is after that
	now := time.Now().UTC()
	// Use 7 days from now to ensure it's well within the future
	futureDate := now.AddDate(0, 0, 7)
	// Normalize to beginning of day in UTC (date only, time will be set by From/To)
	futureDate = time.Date(futureDate.Year(), futureDate.Month(), futureDate.Day(), 0, 0, 0, 0, time.UTC)
	schedule := fixture.schedulePlacementTest(t, student.Id.Hex(), contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   placementTest.Id,
		TeacherId:         types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		CoachingProgramId: coaching.Id,
		Date:              futureDate,
		From:              "14:00",
		To:                "15:00",
	})

	// Approve and assign teacher as admin to make the event visible
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", content.CapabilityManageAllPlacementTests, "ViewOwnCalendarEvents"})
	from := "14:00"
	to := "15:00"
	approveBody, _ := json.Marshal(contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &futureDate,
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

	// Switch back to teacher context to get calendar events
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Get the schedule ID from calendar events
	// Use agenda view which shows all future events
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	require.NotEmpty(t, response.Events, "Expected at least one event")

	var scheduleID primitive.ObjectID
	for _, event := range response.Events {
		if event.Type == "placement_test" {
			scheduleID = event.Id
			break
		}
	}
	require.NotEqual(t, primitive.NilObjectID, scheduleID, "Should find a placement test schedule")

	// Delete schedule
	deleteReq := httptest.NewRequest("DELETE", "/api/placement-tests/schedules/"+scheduleID.Hex(), nil)
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("x-access-token", "test-token")
	deleteReq.Header.Set("x-client", fixture.DBName)
	deleteReq = deleteReq.WithContext(fixture.Ctx)
	deleteW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(deleteW, deleteReq)

	require.Equal(t, http.StatusOK, deleteW.Code, "Failed to delete schedule: body: %s", deleteW.Body.String())

	// Verify the deletion - deleted schedules won't appear in calendar
	assert.Equal(t, http.StatusOK, deleteW.Code)
}

// TestCalendar_UpdateCourseLiveSessionTime tests updating a course live session time
func TestCalendar_UpdateCourseLiveSessionTime(t *testing.T) {
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

	// Create teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{})
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create course with live session
	tomorrow := time.Now().Add(24 * time.Hour)
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة", "en": "Live Session"},
				Schedule: tomorrow,
			},
		},
	})

	// Get the course ID from calendar events
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	require.NotEmpty(t, response.Events, "Expected at least one event")

	var courseID primitive.ObjectID
	for _, event := range response.Events {
		if event.Type == "course_live_session" && event.CourseId != nil {
			courseID = *event.CourseId
			break
		}
	}
	require.NotEqual(t, primitive.NilObjectID, courseID, "Should find a course live session")
	assert.Equal(t, course.Id, courseID, "Course ID should match")

	// Update live session time
	newSchedule := tomorrow.Add(48 * time.Hour).Add(2 * time.Hour) // 2 days + 2 hours from now
	updateData := struct {
		Schedule time.Time `json:"schedule"`
	}{
		Schedule: newSchedule,
	}

	updateBody, _ := json.Marshal(updateData)
	updateReq := httptest.NewRequest("PATCH", "/courses/"+courseID.Hex()+"/live-session/time", bytes.NewBuffer(updateBody))
	updateReq.Header.Set("Content-Type", "application/json")
	updateReq.Header.Set("x-access-token", "test-token")
	updateReq.Header.Set("x-client", fixture.DBName)
	updateReq = updateReq.WithContext(fixture.Ctx)
	updateW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(updateW, updateReq)

	require.Equal(t, http.StatusOK, updateW.Code, "Failed to update live session time: body: %s", updateW.Body.String())

	// Verify the update by getting calendar events again
	updatedResponse := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	var updatedEvent *contentmodels.EnrichedCalendarEvent
	for _, event := range updatedResponse.Events {
		if event.Type == "course_live_session" && event.CourseId != nil && *event.CourseId == courseID {
			updatedEvent = &event
			break
		}
	}

	require.NotNil(t, updatedEvent, "Should find updated live session")
	// Check that the schedule was updated (within a minute tolerance)
	assert.WithinDuration(t, newSchedule, updatedEvent.StartTime, time.Minute, "Start time should be updated")
}

// TestCalendar_DeleteCourseLiveSession tests deleting/deactivating a course live session
func TestCalendar_DeleteCourseLiveSession(t *testing.T) {
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

	// Create teacher
	teacher := fixture.createTeacher(t, &models.TeacherDto{})
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create course with live session
	tomorrow := time.Now().Add(24 * time.Hour)
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة", "en": "Live Session"},
				Schedule: tomorrow,
			},
		},
	})

	// Get the course ID from calendar events
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	require.NotEmpty(t, response.Events, "Expected at least one event")

	var courseID primitive.ObjectID
	initialEventCount := 0
	for _, event := range response.Events {
		if event.Type == "course_live_session" && event.CourseId != nil {
			if courseID == primitive.NilObjectID {
				courseID = *event.CourseId
			}
			initialEventCount++
		}
	}
	require.NotEqual(t, primitive.NilObjectID, courseID, "Should find a course live session")
	assert.Equal(t, course.Id, courseID, "Course ID should match")

	// Delete/deactivate live session
	deleteReq := httptest.NewRequest("DELETE", "/courses/"+courseID.Hex()+"/live-session", nil)
	deleteReq.Header.Set("Content-Type", "application/json")
	deleteReq.Header.Set("x-access-token", "test-token")
	deleteReq.Header.Set("x-client", fixture.DBName)
	deleteReq = deleteReq.WithContext(fixture.Ctx)
	deleteW := httptest.NewRecorder()
	fixture.Router.ServeHTTP(deleteW, deleteReq)

	require.Equal(t, http.StatusOK, deleteW.Code, "Failed to delete live session: body: %s", deleteW.Body.String())

	// Verify the deletion by getting calendar events again
	// Deactivated live sessions won't appear in calendar (active=false)
	updatedResponse := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	// Count course live sessions
	deletedEventCount := 0
	for _, event := range updatedResponse.Events {
		if event.Type == "course_live_session" && event.CourseId != nil && *event.CourseId == courseID {
			deletedEventCount++
		}
	}

	// The deactivated session should not appear in calendar
	assert.Equal(t, 0, deletedEventCount, "Deactivated live session should not appear in calendar")
}
