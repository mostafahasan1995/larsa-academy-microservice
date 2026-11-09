package calendar

import (
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestCalendar_BundleFiltering_ReturnsFilteredEvents
// This comprehensive test verifies:
// - Bundle detection (course and coaching program)
// - Bundle filter application (bundle_only, independent_only, all)
// - Bundle enrichment
// - Multiple events from same bundle
// - Events with and without bundles
func TestCalendar_BundleFiltering_ReturnsFilteredEvents(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", "ViewAllCalendarEvents", "ViewAllBundles", "ViewAllCourses", "ViewAllCoachingPrograms"}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer

	// Create teacher
	teacher := fixture.createTeacher(t, nil)
	fixture.switchAuthContext(teacher.Id.Hex(), authConfig.Capabilities)

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching and course for bundle
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)

	coachingInBundle := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	courseInBundle := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة", "en": "Live Session"},
				Schedule: time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 10, 0, 0, 0, time.UTC),
			},
		},
	})

	// Create independent coaching and course
	coachingIndependent := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	_ = fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
		Settings: contentmodels.CourseSettings{
			LiveSession: contentmodels.LiveSessionSettings{
				Active:   true,
				Title:    transl.Localizable[string]{"ar": "جلسة مباشرة", "en": "Live Session"},
				Schedule: time.Date(dayAfterTomorrow.Year(), dayAfterTomorrow.Month(), dayAfterTomorrow.Day(), 14, 0, 0, 0, time.UTC),
			},
		},
	})

	// Create bundle
	bundle := fixture.createBundle(t, &contentmodels.BundleDto{
		Content: []contentmodels.BundleContent{
			{Type: "coaching", RefId: coachingInBundle.Id},
			{Type: "course", RefId: courseInBundle.Id},
		},
	})

	// Book sessions for both coaching programs
	fixture.bookCoachingSessions(t, student.Id.Hex(), coachingInBundle.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"11:00", "12:00",
	))

	fixture.bookCoachingSessions(t, student.Id.Hex(), coachingIndependent.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"15:00", "16:00",
	))

	// Switch back to admin context
	fixture.switchAuthContext(adminUserID, authConfig.Capabilities)

	// Test 1: bundle_only filter
	t.Run("bundle_only", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			BundleFilter: "bundle_only",
		})

		require.NotEmpty(t, response.Events, "Expected at least one event from bundle")

		// All events should have bundle ID
		for _, event := range response.Events {
			require.NotNil(t, event.BundleId, "Event %s should have bundle ID", event.Id.Hex())
			assert.Equal(t, bundle.Id, *event.BundleId, "Event %s has wrong bundle ID", event.Id.Hex())

			// Verify bundle enrichment
			require.NotNil(t, event.Bundle, "Event %s missing bundle enrichment", event.Id.Hex())
			assert.Equal(t, bundle.Id, event.Bundle.Id, "Event %s has wrong bundle enrichment ID", event.Id.Hex())
		}
	})

	// Test 2: independent_only filter
	t.Run("independent_only", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			BundleFilter: "independent_only",
		})

		require.NotEmpty(t, response.Events, "Expected at least one independent event")

		// All events should NOT have bundle ID
		for _, event := range response.Events {
			assert.Nil(t, event.BundleId, "Event %s should not have bundle ID", event.Id.Hex())
		}
	})

	// Test 3: all filter
	t.Run("all", func(t *testing.T) {
		response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
			BundleFilter: "all",
		})

		// Should have both bundled and independent events
		hasBundled := false
		hasIndependent := false

		for _, event := range response.Events {
			if event.BundleId != nil {
				hasBundled = true
			} else {
				hasIndependent = true
			}
		}

		assert.True(t, hasBundled, "Expected at least one bundled event")
		assert.True(t, hasIndependent, "Expected at least one independent event")
	})
}
