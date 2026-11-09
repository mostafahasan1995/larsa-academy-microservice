package zoom

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"
	"time"

	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/zoom"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/test/setup"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestListMeetings(t *testing.T) {
	// Setup DB and auth
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	adminUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = adminUserID
	authConfig.Capabilities = []string{"authenticate", zoom.CapabilityCreateZoomMeeting}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	// Setup App
	_, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	ctx := context.Background()
	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        ctx,
		AuthServer: authServer,
		DB:         setup.GetTestDatabase(dbConfig),
	}
	fixture.initSharedFixture()

	// 1. Create test data
	acc1 := fixture.createZoomAccount(t, "acc1@test.com", "acc1", "client1", "secret1", "webhook1")
	acc2 := fixture.createZoomAccount(t, "acc2@test.com", "acc2", "client2", "secret2", "webhook2")

	user1 := fixture.createZoomUser(t, acc1.Id, "user1@test.com", "zu1", "User", "1", "active")
	user2 := fixture.createZoomUser(t, acc2.Id, "user2@test.com", "zu2", "User", "2", "active")

	fixture.createZoomMeeting(t, acc1.Id, user1.Id, "session", primitive.NewObjectID(), time.Now(), 60, "scheduled")
	fixture.createZoomMeeting(t, acc1.Id, user1.Id, "session", primitive.NewObjectID(), time.Now(), 60, "scheduled")
	fixture.createZoomMeeting(t, acc2.Id, user2.Id, "session", primitive.NewObjectID(), time.Now(), 60, "scheduled")

	t.Run("List all meetings", func(t *testing.T) {
		w := fixture.callListMeetings(t, nil)
		require.Equal(t, http.StatusOK, w.Code)

		var results []models.ZoomMeeting
		err := json.Unmarshal(w.Body.Bytes(), &results)
		require.NoError(t, err)
		require.Len(t, results, 3)
	})

	t.Run("Filter by accountId", func(t *testing.T) {
		w := fixture.callListMeetings(t, &query.Conditions{
			Columns: []query.Column{
				{Name: "zoomAccountId", Exp: "eq", Value: acc1.Id.Hex()},
			},
		})
		require.Equal(t, http.StatusOK, w.Code)

		var results []models.ZoomMeeting
		err := json.Unmarshal(w.Body.Bytes(), &results)
		require.NoError(t, err)
		require.Len(t, results, 2)
		for _, m := range results {
			require.Equal(t, acc1.Id, m.ZoomAccountId)
		}
	})

	t.Run("Filter by userId", func(t *testing.T) {
		w := fixture.callListMeetings(t, &query.Conditions{
			Columns: []query.Column{
				{Name: "zoomUserId", Exp: "eq", Value: user2.Id.Hex()},
			},
		})
		require.Equal(t, http.StatusOK, w.Code)

		var results []models.ZoomMeeting
		err := json.Unmarshal(w.Body.Bytes(), &results)
		require.NoError(t, err)
		require.Len(t, results, 1)
		require.Equal(t, user2.Id, results[0].ZoomUserId)
	})

	t.Run("Filter by both", func(t *testing.T) {
		w := fixture.callListMeetings(t, &query.Conditions{
			Columns: []query.Column{
				{Name: "zoomAccountId", Exp: "eq", Value: acc1.Id.Hex()},
				{Name: "zoomUserId", Exp: "eq", Value: user1.Id.Hex()},
			},
		})
		require.Equal(t, http.StatusOK, w.Code)

		var results []models.ZoomMeeting
		err := json.Unmarshal(w.Body.Bytes(), &results)
		require.NoError(t, err)
		require.Len(t, results, 2)
	})
}
