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

// TestCalendar_MultipleFilters_ReturnsCorrectlyFilteredEvents
// This comprehensive test verifies:
// - Combining multiple filters (teacher, student, course, coaching program, status, bundle)
// - Filter intersection (AND logic)
// - Multiple teachers filtering
// - Multiple students filtering
// - Status filtering
// - Event type filtering
func TestCalendar_MultipleFilters_ReturnsCorrectlyFilteredEvents(t *testing.T) {
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
	fixture.initSharedFixture()

	// Create two teachers
	teacher1 := fixture.createTeacher(t, &models.TeacherDto{})
	teacher2 := fixture.createTeacher(t, &models.TeacherDto{})

	// Update auth to use teacher1's ID for creation
	fixture.switchAuthContext(teacher1.Id.Hex(), []string{"authenticate", "ViewAllCalendarEvents"})

	// Create three students
	student1 := fixture.createStudent(t, nil)
	student2 := fixture.createStudent(t, nil)
	student3 := fixture.createStudent(t, nil)

	// Create coaching programs
	coaching1 := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher1.Id},
	})
	coaching2 := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher2.Id},
	})
	coaching3 := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher1.Id},
	})

	// Create courses
	course1 := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher1.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة 1", "en": "Session 1"},
				Schedule: time.Now().Add(24 * time.Hour).Add(10 * time.Hour),
			},
		},
	})
	_ = fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher2.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة 2", "en": "Session 2"},
				Schedule: time.Now().Add(24 * time.Hour).Add(14 * time.Hour),
			},
		},
	})

	// Create bundle containing coaching1 and course1
	bundle := fixture.createBundle(t, &contentmodels.BundleDto{
		Content: []contentmodels.BundleContent{
			{Type: "coaching", RefId: coaching1.Id},
			{Type: "course", RefId: course1.Id},
		},
	})

	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)

	// Book sessions: student1 with teacher1/coaching1, student2 with teacher1/coaching3, student3 with teacher2/coaching2
	fixture.bookCoachingSessions(t, student1.Id.Hex(), coaching1.Id, teacher1.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"10:00", "11:00",
	))
	fixture.bookCoachingSessions(t, student2.Id.Hex(), coaching3.Id, teacher1.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"12:00", "13:00",
	))
	fixture.bookCoachingSessions(t, student3.Id.Hex(), coaching2.Id, teacher2.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"14:00", "15:00",
	))

	// Switch back to admin context
	fixture.switchAuthContext(adminUserID, []string{"authenticate", "ViewAllCalendarEvents", "ViewAllBundles", "ViewAllCourses", "ViewAllCoachingPrograms", "ViewAllStudents"})

	// Test 1: Filter by teacher1 only - should return events from teacher1
	t.Run("teacher_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			TeacherIds: []primitive.ObjectID{teacher1.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events from teacher1")
		for _, event := range response.Events {
			assert.Equal(t, teacher1.Id, event.TeacherId, "Event %s should belong to teacher1", event.Id.Hex())
		}
	})

	// Test 2: Filter by student1 only - should return only student1's events
	t.Run("student_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			StudentIds: []primitive.ObjectID{student1.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events for student1")
		for _, event := range response.Events {
			if event.Type == "coaching_session" {
				require.NotNil(t, event.StudentId, "Coaching session %s missing student ID", event.Id.Hex())
				assert.Equal(t, student1.Id, *event.StudentId, "Event %s should belong to student1", event.Id.Hex())
			}
		}
	})

	// Test 3: Filter by coaching program - should return only events from that program
	t.Run("coaching_program_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			CoachingProgramIds: []primitive.ObjectID{coaching1.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events from coaching1")
		for _, event := range response.Events {
			if event.Type == "coaching_session" {
				require.NotNil(t, event.CoachingProgramId, "Coaching session %s missing coaching program ID", event.Id.Hex())
				assert.Equal(t, coaching1.Id, *event.CoachingProgramId, "Event %s should belong to coaching1", event.Id.Hex())
			}
		}
	})

	// Test 4: Filter by course - should return only course live sessions
	t.Run("course_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			CourseIds: []primitive.ObjectID{course1.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events from course1")
		for _, event := range response.Events {
			if event.Type == "course_live_session" {
				require.NotNil(t, event.CourseId, "Course live session %s missing course ID", event.Id.Hex())
				assert.Equal(t, course1.Id, *event.CourseId, "Event %s should belong to course1", event.Id.Hex())
			}
		}
	})

	// Test 5: Filter by bundle - should return only events from bundle
	t.Run("bundle_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			BundleFilter: "bundle_only",
		})

		require.NotEmpty(t, response.Events, "Expected events from bundle")
		for _, event := range response.Events {
			require.NotNil(t, event.BundleId, "Event %s should have bundle ID", event.Id.Hex())
			assert.Equal(t, bundle.Id, *event.BundleId, "Event %s should belong to bundle", event.Id.Hex())
		}
	})

	// Test 6: Filter by status - should return only events with that status
	t.Run("status_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			Statuses: []string{"scheduled"},
		})

		require.NotEmpty(t, response.Events, "Expected events with scheduled status")
		for _, event := range response.Events {
			if event.Type == "coaching_session" {
				assert.Equal(t, "scheduled", event.Status, "Event %s should have scheduled status", event.Id.Hex())
			}
		}
	})

	// Test 7: Filter by event type - should return only that event type
	t.Run("event_type_filter", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			EventTypes: []string{"course_live_session"},
		})

		require.NotEmpty(t, response.Events, "Expected course live sessions")
		for _, event := range response.Events {
			assert.Equal(t, "course_live_session", event.Type, "Event %s should be course_live_session", event.Id.Hex())
		}
	})

	// Test 8: Combined filters - teacher1 AND student1 - should return intersection
	t.Run("combined_teacher_student", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			TeacherIds: []primitive.ObjectID{teacher1.Id},
			StudentIds: []primitive.ObjectID{student1.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events from teacher1 for student1")
		for _, event := range response.Events {
			assert.Equal(t, teacher1.Id, event.TeacherId, "Event %s should belong to teacher1", event.Id.Hex())
			if event.Type == "coaching_session" {
				require.NotNil(t, event.StudentId, "Coaching session %s missing student ID", event.Id.Hex())
				assert.Equal(t, student1.Id, *event.StudentId, "Event %s should belong to student1", event.Id.Hex())
			}
		}
	})

	// Test 9: Combined filters - teacher1 AND coaching1 AND bundle
	t.Run("combined_teacher_coaching_bundle", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			TeacherIds:         []primitive.ObjectID{teacher1.Id},
			CoachingProgramIds: []primitive.ObjectID{coaching1.Id},
			BundleFilter:       "bundle_only",
		})

		require.NotEmpty(t, response.Events, "Expected events matching all filters")
		for _, event := range response.Events {
			assert.Equal(t, teacher1.Id, event.TeacherId, "Event %s should belong to teacher1", event.Id.Hex())
			if event.Type == "coaching_session" {
				require.NotNil(t, event.CoachingProgramId, "Coaching session %s missing coaching program ID", event.Id.Hex())
				assert.Equal(t, coaching1.Id, *event.CoachingProgramId, "Event %s should belong to coaching1", event.Id.Hex())
			}
			require.NotNil(t, event.BundleId, "Event %s should have bundle ID", event.Id.Hex())
			assert.Equal(t, bundle.Id, *event.BundleId, "Event %s should belong to bundle", event.Id.Hex())
		}
	})

	// Test 10: Multiple teachers - should return events from both
	t.Run("multiple_teachers", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			TeacherIds: []primitive.ObjectID{teacher1.Id, teacher2.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events from both teachers")
		hasTeacher1 := false
		hasTeacher2 := false
		for _, event := range response.Events {
			if event.TeacherId == teacher1.Id {
				hasTeacher1 = true
			}
			if event.TeacherId == teacher2.Id {
				hasTeacher2 = true
			}
		}
		assert.True(t, hasTeacher1, "Should have events from teacher1")
		assert.True(t, hasTeacher2, "Should have events from teacher2")
	})

	// Test 11: Multiple students - should return events from both
	t.Run("multiple_students", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			StudentIds: []primitive.ObjectID{student1.Id, student2.Id},
		})

		require.NotEmpty(t, response.Events, "Expected events from both students")
		hasStudent1 := false
		hasStudent2 := false
		for _, event := range response.Events {
			if event.Type == "coaching_session" && event.StudentId != nil {
				if *event.StudentId == student1.Id {
					hasStudent1 = true
				}
				if *event.StudentId == student2.Id {
					hasStudent2 = true
				}
			}
		}
		assert.True(t, hasStudent1, "Should have events from student1")
		assert.True(t, hasStudent2, "Should have events from student2")
	})

	// Verify all responses are sorted
	t.Run("all_responses_sorted", func(t *testing.T) {
		// Test a few filter combinations to ensure sorting works
		filters := []contentmodels.CalendarFilters{
			{TeacherIds: []primitive.ObjectID{teacher1.Id}},
			{StudentIds: []primitive.ObjectID{student1.Id}},
			{CoachingProgramIds: []primitive.ObjectID{coaching1.Id}},
			{BundleFilter: "bundle_only"},
		}

		for _, filter := range filters {
			response := fixture.getCalendarEvents(t, filter)
			assertEventsSorted(t, response.Events)
		}
	})
}
