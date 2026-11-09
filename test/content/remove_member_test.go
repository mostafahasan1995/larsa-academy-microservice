package content

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"

	content "larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/test/setup"
)

func TestRemoveCoachingProgramMember(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, router, database := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	fixture := &TestFixture{
		Router:     router,
		DBName:     dbConfig.Database,
		Ctx:        context.Background(),
		AuthServer: authServer,
		DB:         database,
	}

	// Create teacher and student
	teacher := fixture.createTeacher(t, nil)
	student := fixture.createStudent(t, nil)

	// Create coaching
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{Coachs: []primitive.ObjectID{teacher.Id}})

	// Insert two sessions: one completed (historical) and one future (actionable)
	completedDate := time.Now().AddDate(0, -1, 0)
	futureDate := time.Now().AddDate(0, 0, 7)

	fixture.createCoachingSession(t, student.Id, coaching.Id, teacher.Id, completedDate, contentenums.StudentSessionStatusCompleted)
	fixture.createCoachingSession(t, student.Id, coaching.Id, teacher.Id, futureDate, contentenums.StudentSessionStatusScheduled)

	// Call API as admin with capability to remove any member
	adminUserId := primitive.NewObjectID().Hex()
	fixture.switchAuthContext(adminUserId, []string{"authenticate", content.CapabilityManageAllCoachingPrograms})

	url := fmt.Sprintf("/coaching/%s/members/%s", coaching.Id.Hex(), student.Id.Hex())
	req := httptest.NewRequest("DELETE", url, nil)
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	// Reload sessions from DB
	cursor, err := fixture.DB.Collection("academyCoachingStudentSessions").Find(fixture.Ctx, map[string]interface{}{"studentId": student.Id, "coachingProgramId": coaching.Id})
	require.NoError(t, err)
	var sessions []contentmodels.StudentSession
	err = cursor.All(fixture.Ctx, &sessions)
	require.NoError(t, err)
	require.Equal(t, 2, len(sessions))

	var completedFound, futureFound bool
	for _, s := range sessions {
		if s.Status == contentenums.StudentSessionStatusCompleted {
			completedFound = true
			require.True(t, s.MembershipRemoved, "completed session should be marked membershipRemoved")
			require.False(t, s.Trash, "completed session should not be trashed")
			require.False(t, s.MembershipRemovedBy.IsZero(), "membershipRemovedBy should be set")
		} else if s.Status == contentenums.StudentSessionStatusCancelled {
			futureFound = true
			require.True(t, s.Trash, "future session should be trashed")
			require.True(t, s.MembershipRemoved, "future session should be marked membershipRemoved")
			require.False(t, s.MembershipRemovedBy.IsZero(), "membershipRemovedBy should be set")
		}
	}
	require.True(t, completedFound && futureFound)

	// Call GetCoachingProgramMembers and assert student not listed
	membersReq := httptest.NewRequest("POST", fmt.Sprintf("/coaching/%s/members?skip=0&limit=10", coaching.Id.Hex()), nil)
	membersReq.Header.Set("Content-Type", "application/json")
	membersReq.Header.Set("x-access-token", "test-token")
	membersReq.Header.Set("x-client", fixture.DBName)
	membersReq = membersReq.WithContext(fixture.Ctx)
	w = httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, membersReq)

	require.Equal(t, http.StatusOK, w.Code)

	var membersResp contentmodels.CoachingProgramMemberPagination
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &membersResp))

	// Ensure the student is not present in members
	for _, m := range membersResp.Members {
		require.NotEqual(t, student.Id, m.StudentId)
	}
}
