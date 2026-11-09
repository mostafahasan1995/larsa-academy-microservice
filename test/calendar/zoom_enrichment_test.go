package calendar

import (
	"larsa-academy-microservice/pkg/services/content/models"
	zoommodels "larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCalendar_ZoomEnrichment_ReturnsDetailsForSessionsAndTests(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Create teacher and student
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)
	fixture.switchAuthContext(teacher.Id.Hex(), []string{
		"authenticate",
		"ViewOwnCalendarEvents",
		"ViewAllCalendarEvents",
		"ViewAllStudents",
		"ViewAllCoachingPrograms",
		"AccessTeacherEndpoints",
	})

	now := time.Now().Truncate(time.Hour)
	tomorrow := now.Add(24 * time.Hour)

	// 1. Setup Coaching Session with Zoom Meeting
	coaching := fixture.createCoaching(t, &models.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, tomorrow.Add(time.Hour)},
		"10:00", "11:00",
	))

	// Find the created session to attach Zoom Meeting
	var sessions []models.StudentSession
	cursor, err := fixture.DB.Collection("academyCoachingStudentSessions").Find(fixture.Ctx, bson.M{})
	require.NoError(t, err)
	err = cursor.All(fixture.Ctx, &sessions)
	require.NoError(t, err)
	require.GreaterOrEqual(t, len(sessions), 1)
	session := sessions[0]

	// Create Zoom Meeting for the session
	zoomAccount := primitive.NewObjectID()
	zoomUser := primitive.NewObjectID()
	zoomMeeting := &zoommodels.ZoomMeeting{
		Id:              primitive.NewObjectID(),
		ZoomAccountId:   zoomAccount,
		ZoomUserId:      zoomUser,
		RefType:         "session",
		RefId:           session.Id,
		ZoomMeetingId:   "1234567890",
		JoinUrl:         "https://zoom.us/j/1234567890",
		Password:        "abc123",
		StartTime:       tomorrow.Add(10 * time.Hour),
		DurationMinutes: 60,
		Status:          "scheduled",
	}
	_, err = fixture.DB.Collection("academyZoomMeetings").InsertOne(fixture.Ctx, zoomMeeting)
	require.NoError(t, err)

	// Update session with ZoomMeetingId
	_, err = fixture.DB.Collection("academyCoachingStudentSessions").UpdateOne(
		fixture.Ctx,
		bson.M{"_id": session.Id},
		bson.M{"$set": bson.M{"zoomMeetingId": zoomMeeting.Id}},
	)
	require.NoError(t, err)

	// 2. Setup Placement Test with Zoom Meeting
	placementTest := fixture.createPlacementTest(t, &models.PlacementTestDto{
		RefType: "coaching",
		RefId:   coaching.Id,
	})
	schedule := fixture.schedulePlacementTest(t, student.Id.Hex(), models.OnlinePlacementTestScheduleDto{
		PlacementTestId:   placementTest.Id,
		TeacherId:         types.Nullable[primitive.ObjectID]{Value: &teacher.Id},
		CoachingProgramId: coaching.Id,
		Date:              tomorrow,
		From:              "14:00",
		To:                "15:00",
	})

	// Create Zoom Meeting for the placement test
	zoomMeetingPT := &zoommodels.ZoomMeeting{
		Id:              primitive.NewObjectID(),
		ZoomAccountId:   zoomAccount,
		ZoomUserId:      zoomUser,
		RefType:         "placement_test",
		RefId:           schedule.Id,
		ZoomMeetingId:   "0987654321",
		JoinUrl:         "https://zoom.us/j/0987654321",
		Password:        "xyz789",
		StartTime:       tomorrow.Add(14 * time.Hour),
		DurationMinutes: 60,
		Status:          "scheduled",
	}
	_, err = fixture.DB.Collection("academyZoomMeetings").InsertOne(fixture.Ctx, zoomMeetingPT)
	require.NoError(t, err)

	// Update placement test schedule with ZoomMeetingId
	_, err = fixture.DB.Collection("academyOnlinePlacementTestSchedules").UpdateOne(
		fixture.Ctx,
		bson.M{"_id": schedule.Id},
		bson.M{"$set": bson.M{
			"zoomMeetingId": zoomMeetingPT.Id,
			"status":        "scheduled",
		}},
	)
	require.NoError(t, err)

	// 3. Fetch Calendar Events and Verify Zoom Details
	response := fixture.getCalendarEvents(t, models.CalendarFilters{
		EventTypes: []string{"coaching_session", "placement_test"},
	})

	require.NotEmpty(t, response.Events)

	foundSession := false
	foundPlacementTest := false

	for _, event := range response.Events {
		t.Logf("Found event: Type=%s, ID=%s", event.Type, event.Id.Hex())
		if event.Type == "coaching_session" {
			// Find the specific session we enriched
			if event.Id == session.Id {
				foundSession = true
				require.NotNil(t, event.CoachingSessionDetails, "CoachingSessionDetails should not be nil")
				require.NotNil(t, event.CoachingSessionDetails.ZoomMeeting, "ZoomMeeting details should be enriched for session")
				assert.Equal(t, zoomMeeting.ZoomMeetingId, event.CoachingSessionDetails.ZoomMeeting.ZoomMeetingId)
				assert.Equal(t, zoomMeeting.JoinUrl, event.CoachingSessionDetails.ZoomMeeting.JoinUrl)
				assert.Equal(t, zoomMeeting.Password, event.CoachingSessionDetails.ZoomMeeting.Password)
			}
		} else if event.Type == "placement_test" {
			// Find the specific placement test we enriched
			if event.Id == schedule.Id {
				foundPlacementTest = true
				require.NotNil(t, event.PlacementTestDetails, "PlacementTestDetails should not be nil")
				require.NotNil(t, event.PlacementTestDetails.ZoomMeeting, "ZoomMeeting details should be enriched for placement test")
				assert.Equal(t, zoomMeetingPT.ZoomMeetingId, event.PlacementTestDetails.ZoomMeeting.ZoomMeetingId)
				assert.Equal(t, zoomMeetingPT.JoinUrl, event.PlacementTestDetails.ZoomMeeting.JoinUrl)
				assert.Equal(t, zoomMeetingPT.Password, event.PlacementTestDetails.ZoomMeeting.Password)
			}
		}
	}

	assert.True(t, foundSession, "Should have found the coaching session in calendar")
	assert.True(t, foundPlacementTest, "Should have found the placement test in calendar")
}
