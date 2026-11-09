package contact

import (
	"context"
	"encoding/json"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestTeacherTodaySummary(t *testing.T) {
	// Setup test database
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	// Setup mock auth server
	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	// Setup full test app
	injector, router, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())

	// Get services
	teacherSvc := do.MustInvoke[contact.TeacherSvcs](injector)
	studentSessionRepo := db.NewRepo[contentmodels.StudentSession](injector)
	coachingRepo := db.NewRepo[contentmodels.Coaching](injector)
	bundleRepo := db.NewRepo[contentmodels.Bundle](injector)

	// Test 1: Teacher with no sessions today
	t.Run("NoSessionsToday_ReturnsEmptySummary", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		summary, err := teacherSvc.GetTodaySummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, time.Now().UTC().Format("2006-01-02"), summary.Date)
		assert.Equal(t, 0, summary.TotalSessionsCount)
		assert.Equal(t, 0, summary.CompletedSessionsCount)
		assert.Equal(t, 0, summary.NotCompletedSessionsCount)
		assert.Nil(t, summary.CurrentSession)
		assert.Nil(t, summary.NextSession)
		assert.Len(t, summary.AllSessions, 0)
	})

	// Test 2: Teacher with multiple sessions today
	t.Run("MultipleSessionsToday_ReturnsCorrectCounts", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		// Create a student
		student := createStudent(t, injector, ctx)

		// Create a coaching program
		coaching := &contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Title: transl.Localizable[string]{
					"en": "Test Coaching",
					"ar": "برنامج اختبار",
				},
				Coachs:      []primitive.ObjectID{teacher.Id},
				ProgramType: "foundational",
				Class:       []string{"Grade 1"},
				Sessions:    []contentmodels.CoachingSession{},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, coachingRepo.Add(ctx, coaching))

		// Create teacher assignment for the coaching
		createTeacherAssignment(t, injector, ctx, coaching.Id, "coaching", []primitive.ObjectID{teacher.Id})

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create sessions for today
		// Session 1: Completed (in the past)
		session1 := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        1,
			Date:              today.Add(2 * time.Hour), // 02:00
			From:              "09:00",
			To:                "10:00",
			Status:            contentenums.StudentSessionStatusCompleted,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, session1))

		// Session 2: Scheduled (future)
		session2 := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        2,
			Date:              today.Add(6 * time.Hour), // 06:00
			From:              "14:00",
			To:                "15:00",
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, session2))

		// Session 3: Pending
		session3 := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        3,
			Date:              today.Add(8 * time.Hour), // 08:00
			From:              "16:00",
			To:                "17:00",
			Status:            contentenums.StudentSessionStatusPending,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, session3))

		summary, err := teacherSvc.GetTodaySummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, today.Format("2006-01-02"), summary.Date)
		assert.Equal(t, 3, summary.TotalSessionsCount)
		assert.Equal(t, 1, summary.CompletedSessionsCount)
		assert.Equal(t, 2, summary.NotCompletedSessionsCount)
		assert.Len(t, summary.AllSessions, 3)
	})

	// Test 3: Teacher with current ongoing session
	t.Run("CurrentOngoingSession_ReturnsCurrentSession", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		// Create a student
		student := createStudent(t, injector, ctx)

		// Create a coaching program
		coaching := &contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Title: transl.Localizable[string]{
					"en": "Current Session Coaching",
					"ar": "برنامج الجلسة الحالية",
				},
				Coachs:      []primitive.ObjectID{teacher.Id},
				ProgramType: "foundational",
				Class:       []string{"Grade 1"},
				Sessions:    []contentmodels.CoachingSession{},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, coachingRepo.Add(ctx, coaching))

		// Create teacher assignment
		createTeacherAssignment(t, injector, ctx, coaching.Id, "coaching", []primitive.ObjectID{teacher.Id})

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Calculate time that is currently ongoing (1 hour ago to 1 hour from now)
		hourAgo := now.Add(-1 * time.Hour)
		hourFromNow := now.Add(1 * time.Hour)

		// Create a session that is currently ongoing
		ongoingSession := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        1,
			Date:              today,
			From:              formatTime(hourAgo),
			To:                formatTime(hourFromNow),
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, ongoingSession))

		summary, err := teacherSvc.GetTodaySummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.NotNil(t, summary.CurrentSession, "Should have a current session")
		if summary.CurrentSession != nil {
			// Convert to JSON and back to map for type assertion
			sessionJSON, err := json.Marshal(summary.CurrentSession)
			require.NoError(t, err)
			var currentSessionMap map[string]interface{}
			err = json.Unmarshal(sessionJSON, &currentSessionMap)
			require.NoError(t, err, "CurrentSession should be unmarshalable to map")

			// Extract ID (could be string or ObjectID in JSON)
			currentSessionId := ""
			if idStr, ok := currentSessionMap["id"].(string); ok {
				currentSessionId = idStr
			} else if idObj, ok := currentSessionMap["_id"].(map[string]interface{}); ok {
				// MongoDB ObjectID format
				if oid, ok := idObj["$oid"].(string); ok {
					currentSessionId = oid
				}
			}
			// Remove ObjectID prefix if present
			if len(currentSessionId) > 0 {
				assert.Equal(t, ongoingSession.Id.Hex(), currentSessionId, "Current session ID should match")
			}
		}
	})

	// Test 4: Teacher with next upcoming session
	t.Run("NextUpcomingSession_ReturnsNextSession", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		// Create a student
		student := createStudent(t, injector, ctx)

		// Create a coaching program
		coaching := &contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Title: transl.Localizable[string]{
					"en": "Next Session Coaching",
					"ar": "برنامج الجلسة القادمة",
				},
				Coachs:      []primitive.ObjectID{teacher.Id},
				ProgramType: "foundational",
				Class:       []string{"Grade 1"},
				Sessions:    []contentmodels.CoachingSession{},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, coachingRepo.Add(ctx, coaching))

		// Create teacher assignment
		createTeacherAssignment(t, injector, ctx, coaching.Id, "coaching", []primitive.ObjectID{teacher.Id})

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create a session in the future (2 hours from now)
		futureTime := now.Add(2 * time.Hour)
		nextSession := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        1,
			Date:              today,
			From:              formatTime(futureTime),
			To:                formatTime(futureTime.Add(1 * time.Hour)),
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, nextSession))

		summary, err := teacherSvc.GetTodaySummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.NotNil(t, summary.NextSession, "Should have a next session")
		if summary.NextSession != nil {
			// Convert to JSON and back to map for type assertion
			sessionJSON, err := json.Marshal(summary.NextSession)
			require.NoError(t, err)
			var nextSessionMap map[string]interface{}
			err = json.Unmarshal(sessionJSON, &nextSessionMap)
			require.NoError(t, err, "NextSession should be unmarshalable to map")

			// Extract ID
			nextSessionId := ""
			if idStr, ok := nextSessionMap["id"].(string); ok {
				nextSessionId = idStr
			} else if idObj, ok := nextSessionMap["_id"].(map[string]interface{}); ok {
				if oid, ok := idObj["$oid"].(string); ok {
					nextSessionId = oid
				}
			}
			if len(nextSessionId) > 0 {
				assert.Equal(t, nextSession.Id.Hex(), nextSessionId, "Next session ID should match")
			}
		}
	})

	// Test 5: Teacher with sessions from different days (should only return today's)
	t.Run("SessionsFromDifferentDays_OnlyReturnsToday", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		// Create a student
		student := createStudent(t, injector, ctx)

		// Create a coaching program
		coaching := &contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Title: transl.Localizable[string]{
					"en": "Multi-Day Coaching",
					"ar": "برنامج متعدد الأيام",
				},
				Coachs:      []primitive.ObjectID{teacher.Id},
				ProgramType: "foundational",
				Class:       []string{"Grade 1"},
				Sessions:    []contentmodels.CoachingSession{},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, coachingRepo.Add(ctx, coaching))

		// Create teacher assignment
		createTeacherAssignment(t, injector, ctx, coaching.Id, "coaching", []primitive.ObjectID{teacher.Id})

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)
		yesterday := today.AddDate(0, 0, -1)
		tomorrow := today.AddDate(0, 0, 1)

		// Create session for yesterday
		yesterdaySession := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        1,
			Date:              yesterday,
			From:              "10:00",
			To:                "11:00",
			Status:            contentenums.StudentSessionStatusCompleted,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, yesterdaySession))

		// Create session for today
		todaySession := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        2,
			Date:              today,
			From:              "14:00",
			To:                "15:00",
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, todaySession))

		// Create session for tomorrow
		tomorrowSession := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        3,
			Date:              tomorrow,
			From:              "10:00",
			To:                "11:00",
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, tomorrowSession))

		summary, err := teacherSvc.GetTodaySummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, 1, summary.TotalSessionsCount, "Should only have 1 session (today's)")
		assert.Len(t, summary.AllSessions, 1, "Should only have today's session")
	})

	// Test 6: Teacher with enriched session data (student, coaching, bundle)
	t.Run("EnrichedSessionData_ReturnsCompleteDetails", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		// Create a student
		student := createStudent(t, injector, ctx)

		// Create a bundle
		bundle := &contentmodels.Bundle{
			Id: primitive.NewObjectID(),
			BundleDto: contentmodels.BundleDto{
				Name: transl.Localizable[string]{
					"en": "Test Bundle",
					"ar": "حزمة اختبار",
				},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, bundleRepo.Add(ctx, bundle))

		// Create a coaching program
		coaching := &contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Title: transl.Localizable[string]{
					"en": "Enriched Coaching",
					"ar": "برنامج غني",
				},
				Coachs:      []primitive.ObjectID{teacher.Id},
				ProgramType: "foundational",
				Class:       []string{"Grade 1"},
				Sessions:    []contentmodels.CoachingSession{},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, coachingRepo.Add(ctx, coaching))

		// Create teacher assignment
		createTeacherAssignment(t, injector, ctx, coaching.Id, "coaching", []primitive.ObjectID{teacher.Id})

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create a session with bundle
		session := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			BundleId:          bundle.Id,
			SessionNum:        1,
			Date:              today,
			From:              "15:00",
			To:                "16:00",
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, session))

		summary, err := teacherSvc.GetTodaySummary(ctx)
		require.NoError(t, err)
		require.NotNil(t, summary)

		assert.Equal(t, 1, summary.TotalSessionsCount)
		assert.Len(t, summary.AllSessions, 1)

		// Verify enrichment
		if len(summary.AllSessions) > 0 {
			// Convert to JSON and back to map
			sessionJSON, err := json.Marshal(summary.AllSessions[0])
			require.NoError(t, err)
			var sessionMap map[string]interface{}
			err = json.Unmarshal(sessionJSON, &sessionMap)
			require.NoError(t, err, "Session should be unmarshalable to map")

			// Check student enrichment
			students, ok := sessionMap["students"].([]interface{})
			if ok && len(students) > 0 {
				studentMap, ok := students[0].(map[string]interface{})
				if ok {
					assert.NotEmpty(t, studentMap["name"], "Student name should be enriched")
				}
			}

			// Check coaching enrichment
			coachingProgram, ok := sessionMap["coachingProgram"].(map[string]interface{})
			if ok {
				assert.NotEmpty(t, coachingProgram["name"], "Coaching name should be enriched")
			}

			// Check bundle enrichment
			bundleData, ok := sessionMap["bundle"].(map[string]interface{})
			if ok {
				assert.NotEmpty(t, bundleData["name"], "Bundle name should be enriched")
			}
		}
	})

	// Test 7: HTTP endpoint test
	t.Run("HTTPEndpoint_ReturnsTodaySummary", func(t *testing.T) {
		// Create a unique teacher for this test
		tempCtx := getTestContext(t, dbConfig, primitive.NewObjectID())
		teacher := createTeacher(t, injector, tempCtx)
		ctx := getTestContext(t, dbConfig, teacher.Id)

		// Create a student
		student := createStudent(t, injector, ctx)

		// Create a coaching program
		coaching := &contentmodels.Coaching{
			Id: primitive.NewObjectID(),
			CoachingDto: contentmodels.CoachingDto{
				Title: transl.Localizable[string]{
					"en": "HTTP Test Coaching",
					"ar": "برنامج اختبار HTTP",
				},
				Coachs:      []primitive.ObjectID{teacher.Id},
				ProgramType: "foundational",
				Class:       []string{"Grade 1"},
				Sessions:    []contentmodels.CoachingSession{},
			},
			Trash:     false,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		require.NoError(t, coachingRepo.Add(ctx, coaching))

		// Create teacher assignment
		createTeacherAssignment(t, injector, ctx, coaching.Id, "coaching", []primitive.ObjectID{teacher.Id})

		now := time.Now().UTC()
		today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.UTC)

		// Create a session
		session := &contentmodels.StudentSession{
			Id:                primitive.NewObjectID(),
			StudentId:         student.Id,
			TeacherId:         teacher.Id,
			CoachingProgramId: coaching.Id,
			SessionNum:        1,
			Date:              today,
			From:              "10:00",
			To:                "11:00",
			Status:            contentenums.StudentSessionStatusScheduled,
			Trash:             false,
			CreatedAt:         time.Now(),
			UpdatedAt:         time.Now(),
		}
		require.NoError(t, studentSessionRepo.Add(ctx, session))

		// Update auth config to use teacher's ID with proper capabilities
		authConfig.UserID = teacher.Id.Hex()
		authConfig.Capabilities = []string{"AccessTeacherEndpoints"} // This matches content.CapabilityAccessTeacherEndpoints
		authConfig.ShouldAuthenticate = true
		authConfig.ShouldAuthorize = true
		authServer.UpdateConfig(authConfig)

		// Make HTTP request
		req := httptest.NewRequest("GET", "/api/teachers/me/today-summary", nil)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-access-token", "test-token")
		req.Header.Set("x-client", dbConfig.Database)
		req = req.WithContext(ctx)

		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)

		assert.Equal(t, http.StatusOK, w.Code, "Should return 200 OK. Body: %s", w.Body.String())

		var summary models.TeacherTodaySummary
		err := json.Unmarshal(w.Body.Bytes(), &summary)
		require.NoError(t, err, "Should unmarshal response")

		assert.Equal(t, today.Format("2006-01-02"), summary.Date)
		assert.Equal(t, 1, summary.TotalSessionsCount)
	})
}

// Helper functions

func getTestContext(t *testing.T, dbConfig *setup.TestDBConfig, teacherId primitive.ObjectID) context.Context {
	ctx := context.Background()
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: teacherId,
		},
	}
	return context.WithValue(ctx, util.ReqAppCfg, cfg)
}

func formatTime(t time.Time) string {
	return t.Format("15:04")
}
