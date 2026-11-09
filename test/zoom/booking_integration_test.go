package zoom

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/samber/do"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"

	"larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/test/placement_test" // Reusing RegisterMockZoomService from here
	"larsa-academy-microservice/test/setup"
)

func TestCoachingBookingZoomIntegration(t *testing.T) {
	// 1. Setup DB and Mock Auth
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	studentId := primitive.NewObjectID()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = studentId.Hex()
	authConfig.Capabilities = []string{"authenticate", content.CapabilityViewPublicCoaching}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	// 2. Setup App with Mock Zoom Service
	injector := do.New()
	zoomMock := placement.RegisterMockZoomService(injector)
	injector, router, _ := setup.SetupFullTestAppWithInjector(t, injector, dbConfig, authServer.URL())

	// Override services to use the mock
	do.Override(injector, content.NewCoachingSvcs)

	ctx := context.Background()

	// 3. Seed Data
	database := dbConfig.Client.Database(dbConfig.Database)
	// Create Teacher
	teacherId := primitive.NewObjectID()
	_, _ = database.Collection("academyTeachers").InsertOne(ctx, bson.M{
		"_id": teacherId,
		"generalInfo": bson.M{
			"firstName": "John",
			"lastName":  "Doe",
		},
	})

	// Create Coaching Program
	coachingId := primitive.NewObjectID()
	_, _ = database.Collection("academyCoachings").InsertOne(ctx, bson.M{
		"_id":    coachingId,
		"title":  "Math Coaching",
		"coachs": []primitive.ObjectID{teacherId},
	})

	// Create 4 Pending Sessions for the student
	sessionIds := make([]primitive.ObjectID, 4)
	for i := 0; i < 4; i++ {
		sessionIds[i] = primitive.NewObjectID()
		_, err := database.Collection("academyCoachingStudentSessions").InsertOne(ctx, contentmodels.StudentSession{
			Id:                sessionIds[i],
			StudentId:         studentId,
			CoachingProgramId: coachingId,
			Status:            contentenums.StudentSessionStatusPending,
			SessionNum:        i + 1,
			DurationMinutes:   60,
			Trash:             false,
		})
		require.NoError(t, err)
	}

	// 4. Test Case: Successful Booking (All Zoom Meetings Created)
	t.Run("Successful Booking with Zoom Rollback check", func(t *testing.T) {
		bookingDto := contentmodels.BookingSessions{
			CoachingProgramId: coachingId,
			TeacherId:         teacherId,
			Dates: []contentmodels.BookingDate{
				{Date: time.Now().AddDate(0, 0, 1), From: "10:00", To: "11:00", Weekday: "Monday"},
				{Date: time.Now().AddDate(0, 0, 2), From: "14:00", To: "15:00", Weekday: "Tuesday"},
			},
		}

		body, _ := json.Marshal(bookingDto)
		req := httptest.NewRequest("POST", "/api/coaching/booking", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", dbConfig.Database)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		require.Equal(t, http.StatusOK, w.Code)
		// Should have 4 CreateMeeting calls (2 per date * 2 dates = 4 sessions)
		require.Len(t, zoomMock.CreateMeetingCalls, 4)

		// Verify sessions are updated in DB with ZoomMeetingId
		for _, sid := range sessionIds {
			var s contentmodels.StudentSession
			err := database.Collection("academyCoachingStudentSessions").FindOne(ctx, bson.M{"_id": sid}).Decode(&s)
			require.NoError(t, err)
			require.NotNil(t, s.ZoomMeetingId)
			require.Equal(t, contentenums.StudentSessionStatusScheduled, s.Status)
		}
	})

	// 5. Test Case: Failed Booking (Zoom Creation Fails midway) -> Rollback check
	t.Run("Failed Booking triggers Rollback", func(t *testing.T) {
		// Reset state
		zoomMock.CreateMeetingCalls = nil
		deletedMeetingIds := []primitive.ObjectID{}
		zoomMock.DeleteMeetingFunc = func(ctx context.Context, zoomMeetingId primitive.ObjectID) error {
			deletedMeetingIds = append(deletedMeetingIds, zoomMeetingId)
			return nil
		}

		// Make 3rd meeting fail
		callCount := 0
		zoomMock.CreateMeetingFunc = func(ctx context.Context, refType string, refId primitive.ObjectID, startTime time.Time, duration int, topic string) (*models.ZoomMeeting, error) {
			callCount++
			if callCount == 3 {
				return nil, fmt.Errorf("zoom capacity exceeded")
			}
			return &models.ZoomMeeting{Id: primitive.NewObjectID(), ZoomMeetingId: "mock-id"}, nil
		}

		// Create new pending sessions
		newSessionIds := make([]primitive.ObjectID, 4)
		for i := 0; i < 4; i++ {
			newSessionIds[i] = primitive.NewObjectID()
			_, err := database.Collection("academyCoachingStudentSessions").InsertOne(ctx, contentmodels.StudentSession{
				Id:                newSessionIds[i],
				StudentId:         studentId,
				CoachingProgramId: coachingId,
				Status:            contentenums.StudentSessionStatusPending,
				Trash:             false,
			})
			require.NoError(t, err)
		}

		bookingDto := contentmodels.BookingSessions{
			CoachingProgramId: coachingId,
			TeacherId:         teacherId,
			Dates: []contentmodels.BookingDate{
				{Date: time.Now().AddDate(0, 0, 7), From: "10:00", To: "11:00", Weekday: "Monday"},
				{Date: time.Now().AddDate(0, 0, 8), From: "14:00", To: "15:00", Weekday: "Tuesday"},
			},
		}

		body, _ := json.Marshal(bookingDto)
		req := httptest.NewRequest("POST", "/api/coaching/booking", bytes.NewBuffer(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", dbConfig.Database)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		// Booking should fail
		// Note: The service might return 400 if it's considered a user error or general failure depending on handler
		require.Equal(t, http.StatusBadRequest, w.Code)

		// Rollback should have deleted the first 2 meetings
		require.Len(t, deletedMeetingIds, 2)

		// Database should NOT be updated (BulkWrite shouldn't have been called or should have failed)
		for _, sid := range newSessionIds {
			var s contentmodels.StudentSession
			err := database.Collection("academyCoachingStudentSessions").FindOne(ctx, bson.M{"_id": sid}).Decode(&s)
			require.NoError(t, err)
			require.Nil(t, s.ZoomMeetingId)
			require.Equal(t, contentenums.StudentSessionStatusPending, s.Status)
		}
	})

}
