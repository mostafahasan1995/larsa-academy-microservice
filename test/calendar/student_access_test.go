package calendar

import (
	"bytes"
	"encoding/json"
	content "larsa-academy-microservice/pkg/services/content"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCalendar_StudentViewingOwnEvents_ReturnsFilteredEvents(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	studentUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = studentUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityViewStudentCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Create student
	student := fixture.createStudent(t, nil)
	// Update auth to use student's ID and proper capabilities
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", content.CapabilityViewStudentCalendarEvents})

	// Create teacher
	teacher := fixture.createTeacher(t, nil)

	// Create another student (to verify privacy)
	otherStudent := fixture.createStudent(t, nil)

	// Create coaching program
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)

	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	// Create course (enrolled)
	courseEnrolled := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"en": "Enrolled Course Session"},
				Schedule: dayAfterTomorrow,
			},
		},
	})
	fixture.enrollStudent(t, student.Id.Hex(), courseEnrolled.Id.Hex())

	// Create course (not enrolled)
	_ = fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"en": "Not Enrolled Course Session"},
				Schedule: dayAfterTomorrow.Add(1 * time.Hour),
			},
		},
	})

	// Book coaching session for this student
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, tomorrow.Add(1 * time.Hour)},
		"10:00", "11:00",
	))

	// Book coaching session for another student (should not be visible)
	fixture.bookCoachingSessions(t, otherStudent.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, tomorrow.Add(1 * time.Hour)},
		"12:00", "13:00",
	))

	// Get calendar events
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", content.CapabilityViewStudentCalendarEvents})
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{})

	// Assertions
	require.NotEmpty(t, response.Events, "Expected at least one event")

	hasCoaching := false
	hasCourse := false
	hasOtherStudentEvent := false
	hasNotEnrolledCourse := false

	for _, event := range response.Events {
		if event.Type == "coaching_session" {
			if *event.StudentId == student.Id {
				hasCoaching = true
			} else {
				hasOtherStudentEvent = true
			}
		}
		if event.Type == "course_live_session" {
			if *event.CourseId == courseEnrolled.Id {
				hasCourse = true
			} else {
				hasNotEnrolledCourse = true
			}
		}
	}

	assert.True(t, hasCoaching, "Should see own coaching session")
	assert.True(t, hasCourse, "Should see enrolled course session")
	assert.False(t, hasOtherStudentEvent, "Should NOT see other student's coaching session")
	assert.False(t, hasNotEnrolledCourse, "Should NOT see session for course not enrolled in")
}

func TestCalendar_StudentTryingToFilterByOtherStudent_Forbidden(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	student := primitive.NewObjectID()
	otherStudent := primitive.NewObjectID()

	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = student.Hex()
	authConfig.Capabilities = []string{"authenticate", content.CapabilityViewStudentCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Try to filter by other student ID
	reqBody, _ := json.Marshal(contentmodels.CalendarEventsRequest{
		Filters: contentmodels.CalendarFilters{
			StudentIds: []primitive.ObjectID{otherStudent},
		},
	})
	req := httptest.NewRequest("POST", "/api/calendar/events?view=agenda", bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	// Should be unauthorized/forbidden
	assert.Equal(t, http.StatusForbidden, w.Code)
}
