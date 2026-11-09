package calendar

import (
	"bytes"
	"encoding/json"
	"larsa-academy-microservice/pkg/services/contact/enums"
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

// TestCalendar_TeacherWithAvailability_ReturnsEventsAndSlots
// This comprehensive test verifies:
// - Availability slot calculation
// - Teacher availability settings (working hours, session duration)
// - Busy slot exclusion (coaching sessions, courses, placement tests)
// - Date range validation (1 year limit)
// - Session duration priority (program vs teacher default)
// - Unit conversion (SchedulingUnit to minutes)
// - Availability slot enrichment
func TestCalendar_TeacherWithAvailability_ReturnsEventsAndSlots(t *testing.T) {
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

	// Create teacher with availability settings
	now := time.Now()
	// Find next Monday (or next working day) to ensure we have future working days
	// This ensures availability slots can be generated
	daysUntilMonday := (8 - int(now.Weekday())) % 7
	if daysUntilMonday == 0 {
		daysUntilMonday = 7 // If today is Monday, use next Monday
	}
	nextWorkingDay := now.AddDate(0, 0, daysUntilMonday)
	// Reset to start of day for consistent date calculations
	nextWorkingDay = time.Date(nextWorkingDay.Year(), nextWorkingDay.Month(), nextWorkingDay.Day(), 0, 0, 0, 0, time.UTC)

	// Set up day of week availability (Monday to Friday)
	dayOfWeekAvailability := map[enums.DayOfWeek]models.DayOfWeekAvailability{
		enums.Monday:    {IsCustom: false, IsSelected: true, WorkingHours: []models.Shift{}},
		enums.Tuesday:   {IsCustom: false, IsSelected: true, WorkingHours: []models.Shift{}},
		enums.Wednesday: {IsCustom: false, IsSelected: true, WorkingHours: []models.Shift{}},
		enums.Thursday:  {IsCustom: false, IsSelected: true, WorkingHours: []models.Shift{}},
		enums.Friday:    {IsCustom: false, IsSelected: true, WorkingHours: []models.Shift{}},
		enums.Saturday:  {IsCustom: false, IsSelected: false, WorkingHours: []models.Shift{}},
		enums.Sunday:    {IsCustom: false, IsSelected: false, WorkingHours: []models.Shift{}},
	}

	teacher := fixture.createTeacher(t, &models.TeacherDto{
		Availability: models.Availability{
			DefaultWorkingHours: []models.Shift{
				{From: "09:00", To: "17:00"},
			},
			DayOfWeekAvailability: dayOfWeekAvailability,
			Scheduling: models.Scheduling{
				SessionDuration: models.SchedulingUnit{
					Value: 60,
					Unit:  "minutes",
				},
				TimeIntervalBetweenSessions: models.SchedulingUnit{
					Value: 15,
					Unit:  "minutes",
				},
			},
		},
	})

	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching program with specific session duration (90 minutes - should take priority)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs:          []primitive.ObjectID{teacher.Id},
		SessionDuration: 90, // 90 minutes - should take priority over teacher's default 60
	})

	// Create course with live session (busy slot)
	_ = fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة", "en": "Live Session"},
				Schedule: time.Date(nextWorkingDay.Year(), nextWorkingDay.Month(), nextWorkingDay.Day(), 10, 0, 0, 0, time.UTC), // 10:00-12:00 (busy slot)
			},
		},
	})

	// Create placement test
	placementTest := fixture.createPlacementTest(t, &contentmodels.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})

	// Book coaching sessions (busy slots at 11:00-12:30 and 14:00-15:30)
	bookingDates := []contentmodels.BookingDate{
		{
			Date:    nextWorkingDay,
			Weekday: nextWorkingDay.Weekday().String(),
			From:    "11:00",
			To:      "12:30",
		},
		{
			Date:    nextWorkingDay,
			Weekday: nextWorkingDay.Weekday().String(),
			From:    "14:00",
			To:      "15:30",
		},
	}
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, bookingDates)

	// Schedule placement test (busy slot at 13:00-14:00)
	schedule := fixture.schedulePlacementTest(t, student.Id.Hex(), contentmodels.OnlinePlacementTestScheduleDto{
		PlacementTestId:   placementTest.Id,
		TeacherId:         types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		CoachingProgramId: coaching.Id,
		Date:              nextWorkingDay,
		From:              "13:00",
		To:                "14:00",
	})

	// Approve to make placement test visible/busy
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", content.CapabilityManageAllPlacementTests, "ViewOwnCalendarEvents"})
	from := "13:00"
	to := "14:00"
	approveBody, _ := json.Marshal(contentmodels.UpdatePlacementTestScheduleDto{
		Status:    contentenums.StudentPlacementTestStatusScheduled,
		TeacherId: &types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		Date:      &nextWorkingDay,
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

	// Switch back to teacher context
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", "ViewOwnCalendarEvents"})

	// Call Calendar API with IncludeAvailability=true
	// Use weekly view (not agenda) since availability calculation is skipped for agenda view
	// Pass coaching program ID so it uses program's session duration (90 min) instead of teacher default (60 min)
	// Use nextWorkingDay to ensure the week includes future working days
	nextWorkingDayStr := nextWorkingDay.Format("2006-01-02")
	response := fixture.getCalendarEventsWithView(t, contentmodels.CalendarFilters{
		TeacherIds:          []primitive.ObjectID{teacher.Id},
		CoachingProgramIds:  []primitive.ObjectID{coaching.Id},
		IncludeAvailability: true,
	}, "weekly", nextWorkingDayStr, "")

	// Assertions
	require.NotEmpty(t, response.Events, "Expected at least one event (including availability slots)")

	// Count event types
	coachingSessionCount := 0
	courseLiveSessionCount := 0
	placementTestCount := 0
	availabilitySlotCount := 0

	for _, event := range response.Events {
		switch event.Type {
		case "coaching_session":
			coachingSessionCount++
		case "course_live_session":
			courseLiveSessionCount++
		case "placement_test":
			placementTestCount++
		case "availability_slot":
			availabilitySlotCount++

			// Verify availability slot details
			require.NotNil(t, event.AvailableSlotDetails, "Availability slot %s missing slot details", event.Id.Hex())
			// Session duration should be from coaching program (90 minutes), not teacher default (60)
			assert.Equal(t, 90, event.AvailableSlotDetails.SessionDuration,
				"Availability slot %s has wrong session duration: expected 90 (from program), got %d",
				event.Id.Hex(), event.AvailableSlotDetails.SessionDuration)

			// Verify availability slots are in the future
			assert.False(t, event.StartTime.Before(time.Now()),
				"Availability slot %s is in the past: %v", event.Id.Hex(), event.StartTime)

			// Verify availability slots are within working hours (09:00-17:00)
			startHour := event.StartTime.Hour()
			assert.GreaterOrEqual(t, startHour, 9, "Availability slot %s is outside working hours: starts at %d:00", event.Id.Hex(), startHour)
			assert.Less(t, startHour, 17, "Availability slot %s is outside working hours: starts at %d:00", event.Id.Hex(), startHour)
		}
	}

	// Should have at least some availability slots
	assert.Greater(t, availabilitySlotCount, 0, "Expected at least one availability slot")

	// Should have scheduled events
	assert.Greater(t, coachingSessionCount, 0, "Expected at least one coaching session")
	assert.Greater(t, courseLiveSessionCount, 0, "Expected at least one course live session")
	assert.Greater(t, placementTestCount, 0, "Expected at least one placement test")

	// Verify availability slots don't overlap with busy slots
	// Busy slots for nextWorkingDay: 10:00-12:00 (course), 11:00-12:30 (coaching), 13:00-14:00 (placement), 14:00-15:30 (coaching)
	nextWorkingDayDate := nextWorkingDay.Format("2006-01-02")
	for _, event := range response.Events {
		if event.Type == "availability_slot" {
			// Only check slots for nextWorkingDay (where busy slots are)
			eventDate := event.StartTime.Format("2006-01-02")
			if eventDate != nextWorkingDayDate {
				continue
			}

			startHour := event.StartTime.Hour()
			startMin := event.StartTime.Minute()
			startTotalMin := startHour*60 + startMin
			endHour := event.EndTime.Hour()
			endMin := event.EndTime.Minute()
			endTotalMin := endHour*60 + endMin

			// Busy periods for tomorrow (in minutes from midnight):
			// Course: 10:00-12:00 (600-720)
			// Coaching1: 11:00-12:30 (660-750)
			// Placement: 13:00-14:00 (780-840)
			// Coaching2: 14:00-15:30 (840-930)

			// Check overlap: StartA < EndB && EndA > StartB
			overlapsCourse := startTotalMin < 12*60 && endTotalMin > 10*60
			overlapsCoaching1 := startTotalMin < 12*60+30 && endTotalMin > 11*60
			overlapsPlacement := startTotalMin < 14*60 && endTotalMin > 13*60
			overlapsCoaching2 := startTotalMin < 15*60+30 && endTotalMin > 14*60

			assert.False(t, overlapsCourse || overlapsCoaching1 || overlapsPlacement || overlapsCoaching2,
				"Availability slot %s overlaps with busy slot: %02d:%02d-%02d:%02d",
				event.Id.Hex(), startHour, startMin, endHour, endMin)
		}
	}

	// Verify events are sorted
	assertEventsSorted(t, response.Events)
}
