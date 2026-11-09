package calendar

import (
	"larsa-academy-microservice/pkg/services/contact/models"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestCalendar_AdminViewingAllEvents_ReturnsAllEventsWithFullAccess
// This comprehensive test verifies:
// - Admin capability check (ViewAllCalendarEvents)
// - No teacher filtering applied for admins
// - Access to all resources (courses, coaching programs, bundles)
// - Access to all student data
// - Multiple teachers' events visible
// - All enrichment data populated
func TestCalendar_AdminViewingAllEvents_ReturnsAllEventsWithFullAccess(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", "ViewAllCalendarEvents", "ViewAllBundles", "ViewAllCourses", "ViewAllCoachingPrograms", "ViewAllStudents"}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer

	// Create multiple teachers (only specify what's needed)
	teacher1 := fixture.createTeacher(t, &models.TeacherDto{
		Availability: models.Availability{
			Scheduling: models.Scheduling{
				SessionDuration: models.SchedulingUnit{
					Value: 60,
					Unit:  "minutes",
				},
			},
		},
	})

	fixture.switchAuthContext(teacher1.Id.Hex(), authConfig.Capabilities)

	teacher2 := fixture.createTeacher(t, &models.TeacherDto{
		Availability: models.Availability{
			DefaultWorkingHours: []models.Shift{
				{From: "10:00", To: "18:00"},
			},
			Scheduling: models.Scheduling{
				SessionDuration: models.SchedulingUnit{
					Value: 90,
					Unit:  "minutes",
				},
			},
		},
	})

	// Create students
	student1 := fixture.createStudent(t, nil)
	student2 := fixture.createStudent(t, nil)

	// Create coaching programs
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)

	coaching1 := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs:          []primitive.ObjectID{teacher1.Id},
		SessionDuration: 60,
	})

	coaching2 := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs:          []primitive.ObjectID{teacher2.Id},
		SessionDuration: 90,
	})

	// Create courses
	course1 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher1.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة 1", "en": "Live Session 1"},
				Schedule: dayAfterTomorrow.Add(2 * time.Hour),
			},
		},
	})

	course2 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher2.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة 2", "en": "Live Session 2"},
				Schedule: dayAfterTomorrow.Add(4 * time.Hour),
			},
		},
	})

	// Book coaching sessions
	fixture.bookCoachingSessions(t, student1.Id.Hex(), coaching1.Id, teacher1.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"10:00", "11:00",
	))

	nextDay := dayAfterTomorrow.Add(24 * time.Hour)
	fixture.bookCoachingSessions(t, student2.Id.Hex(), coaching2.Id, teacher2.Id, createBookingDates(
		[]time.Time{dayAfterTomorrow, nextDay},
		"14:00", "15:30",
	))

	// Switch back to admin context
	fixture.switchAuthContext(adminUserID, authConfig.Capabilities)

	// Call Calendar API as Admin (no teacher filter - should see all events)
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{})

	// Assertions
	require.NotEmpty(t, response.Events, "Expected at least one event")

	// Verify events from both teachers are present
	teacher1Events := 0
	teacher2Events := 0
	hasCourse1 := false
	hasCourse2 := false
	hasCoaching1 := false
	hasCoaching2 := false

	for _, event := range response.Events {
		if event.TeacherId == teacher1.Id {
			teacher1Events++
			if event.CourseId != nil && *event.CourseId == course1.Id {
				hasCourse1 = true
			}
			if event.CoachingProgramId != nil && *event.CoachingProgramId == coaching1.Id {
				hasCoaching1 = true
			}
		}
		if event.TeacherId == teacher2.Id {
			teacher2Events++
			if event.CourseId != nil && *event.CourseId == course2.Id {
				hasCourse2 = true
			}
			if event.CoachingProgramId != nil && *event.CoachingProgramId == coaching2.Id {
				hasCoaching2 = true
			}
		}

		// Admin should see all enrichment data
		require.NotNil(t, event.Teacher, "Event %s missing teacher enrichment", event.Id.Hex())
		if event.Type == "course_live_session" {
			require.NotNil(t, event.Course, "Course event %s missing course enrichment", event.Id.Hex())
		}
		if event.Type == "coaching_session" {
			require.NotNil(t, event.CoachingProgram, "Coaching event %s missing coaching program enrichment", event.Id.Hex())
			assert.NotEmpty(t, event.Students, "Coaching event %s missing student enrichment", event.Id.Hex())
		}
	}

	// Verify admin can see events from both teachers
	assert.Greater(t, teacher1Events, 0, "Admin should see events from teacher1")
	assert.Greater(t, teacher2Events, 0, "Admin should see events from teacher2")

	// Verify admin can see both courses
	assert.True(t, hasCourse1, "Admin should see course1 live session")
	assert.True(t, hasCourse2, "Admin should see course2 live session")

	// Verify admin can see both coaching programs
	assert.True(t, hasCoaching1, "Admin should see coaching1 sessions")
	assert.True(t, hasCoaching2, "Admin should see coaching2 sessions")

	// Verify events are sorted
	assertEventsSorted(t, response.Events)
}
