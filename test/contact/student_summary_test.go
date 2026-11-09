package contact

import (
	"context"
	"larsa-academy-microservice/pkg/services/contact"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	pointsmodels "larsa-academy-microservice/pkg/services/points/models"
	pointsenums "larsa-academy-microservice/pkg/services/points/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestStudentSummary_EnrollmentBased tests that GET summary uses enrollment data
// (StudentCourse, CoachingStudent, StudentSession) and returns correct metrics.
func TestStudentSummary_EnrollmentBased(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, db := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	fixture := &helpers.TestFixture{
		Router:     nil,
		DBName:     dbConfig.Database,
		Ctx:        ctx,
		AuthServer: authServer,
		DB:         db,
	}

	student := fixture.CreateStudent(t, nil)
	course := fixture.CreateCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:     transl.Localizable[string]{"en": "Test Course"},
			Duration: 120, // minutes
			Status:   "active",
		},
	})
	teacher := fixture.CreateTeacher(t, nil)
	coaching := fixture.CreateCoaching(t, &contentmodels.CoachingDto{
		Coachs:        []primitive.ObjectID{teacher.Id},
		MinSessionNum: 1,
		MaxSessionNum: 10,
	})

	// Enroll student in course (active)
	_, err := db.Collection("academyCourseStudents").InsertOne(ctx, contentmodels.StudentCourse{
		Id:        primitive.NewObjectID(),
		StudentId: student.Id,
		CourseId:  course.Id,
		Status:    contentenums.EnrollmentStatusActive,
		Progress:  25,
		Trash:     false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	})
	require.NoError(t, err)

	// Enroll student in coaching (active)
	_, err = db.Collection("academyCoachingStudents").InsertOne(ctx, contentmodels.CoachingStudent{
		Id:         primitive.NewObjectID(),
		StudentId:  student.Id,
		CoachingId: coaching.Id,
		Status:     contentenums.EnrollmentStatusActive,
		Trash:      false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	require.NoError(t, err)

	// Two completed sessions
	for i := 0; i < 2; i++ {
		_, err = db.Collection("academyCoachingStudentSessions").InsertOne(ctx, contentmodels.StudentSession{
			Id:                 primitive.NewObjectID(),
			StudentId:          student.Id,
			CoachingProgramId:  coaching.Id,
			TeacherId:          teacher.Id,
			Status:             contentenums.StudentSessionStatusCompleted,
			Trash:              false,
			CreatedAt:          time.Now(),
			UpdatedAt:          time.Now(),
		})
		require.NoError(t, err)
	}

	// One approved redeem (badge)
	_, err = db.Collection("academyRedeemRequests").InsertOne(ctx, pointsmodels.RedeemRequest{
		ID:         primitive.NewObjectID(),
		StudentID:  student.Id,
		Status:     pointsenums.RedeemStatusApproved,
		Trash:      false,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	})
	require.NoError(t, err)

	// Call GetSummary as the student
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		User: &types.User{Id: student.Id},
	}
	summaryCtx := context.WithValue(ctx, util.ReqAppCfg, cfg)
	svc := do.MustInvoke[contact.StudentSvcs](injector)
	result, err := svc.GetSummary(summaryCtx, student.Id.Hex())
	require.NoError(t, err)
	require.NotNil(t, result)

	// Assert enrollment-based metrics
	assert.Equal(t, 1, result.ActiveCoursesCount, "one active course enrollment")
	assert.Equal(t, 1, result.ActiveCoachingsCount, "one active coaching enrollment")
	assert.Equal(t, 2, result.TotalSessionsCompleted, "two completed sessions")
	assert.Equal(t, 1, result.BadgesEarned, "one approved redeem")
	// TotalLearningHours: one course with Duration 120 min => 2 hours
	assert.InDelta(t, 2.0, result.TotalLearningHours, 0.01, "totalLearningHours from course duration")
}
