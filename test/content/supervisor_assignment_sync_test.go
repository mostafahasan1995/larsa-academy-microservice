package content

import (
	"context"
	contact "larsa-academy-microservice/pkg/services/contact"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"larsa-academy-microservice/test/helpers"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// Helper function to create supervisor
func createSupervisor(t *testing.T, injector *do.Injector, ctx context.Context, dto *contactmodels.SupervisorDTO) *contactmodels.Supervisor {
	supervisorSvc := do.MustInvoke[contact.SupervisorSvc](injector)

	if dto == nil {
		dto = &contactmodels.SupervisorDTO{
			FirstName:        transl.Localizable[string]{"en": "Test"},
			LastName:         transl.Localizable[string]{"en": "Supervisor"},
			Gender:           "male",
			DateOfBirth:      time.Date(1980, 1, 1, 0, 0, 0, 0, time.UTC),
			Nationality:      "US",
			NationalID:       helpers.RandomNationalID(),
			EmploymentStatus: "full-time",
			AccountSettings: contactmodels.AccountSettings{
				Email:       helpers.RandomEmail(),
				InitialPass: "password123",
				ConfirmPass: "password123",
				JoinStatus:  "active",
				IsActive:    true,
			},
		}
	}

	supervisor, err := supervisorSvc.Add(ctx, dto)
	require.NoError(t, err)
	return supervisor
}

// Helper function to create subject
func createSubject(t *testing.T, injector *do.Injector, ctx context.Context) *contentmodels.Subject {
	subjectSvc := do.MustInvoke[content.SubjectSvcs](injector)

	dto := &contentmodels.SubjectDto{
		Name: transl.Localizable[string]{
			"en": "Test Subject " + helpers.RandomString(4),
			"ar": "مادة اختبار",
		},
		Code: helpers.RandomString(4),
	}

	subject, err := subjectSvc.Add(ctx, dto)
	require.NoError(t, err)
	return subject
}

// Helper function to create teacher
func createTeacher(t *testing.T, injector *do.Injector, ctx context.Context) *contactmodels.Teacher {
	teacherSvc := do.MustInvoke[contact.TeacherSvcs](injector)

	dto := &contactmodels.TeacherDto{
		FirstName:        transl.Localizable[string]{"en": "Test"},
		LastName:         transl.Localizable[string]{"en": "Teacher"},
		Gender:           "male",
		DateOfBirth:      time.Date(1990, 1, 1, 0, 0, 0, 0, time.UTC),
		Nationality:      "US",
		NationalID:       helpers.RandomNationalID(),
		EmploymentStatus: "full-time",
		Email:            helpers.RandomEmail(),
		Mobile:           helpers.RandomPhone(),
		AccountSettings: contactmodels.AccountSettings{
			Email:       helpers.RandomEmail(),
			InitialPass: "password123",
			ConfirmPass: "password123",
			JoinStatus:  "active",
			IsActive:    true,
		},
		Subjects: []contactmodels.Subject{},
	}

	teacher, err := teacherSvc.Add(ctx, dto)
	require.NoError(t, err)
	return teacher
}

// TestSupervisorAssignmentSyncToCourses tests that creating a supervisor assignment
// syncs supervisor IDs to matching courses
func TestSupervisorAssignmentSyncToCourses(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities for course/coaching operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create supervisor
	supervisor := createSupervisor(t, injector, ctx, nil)

	// Create teacher (required for course)
	teacher := createTeacher(t, injector, ctx)

	// Create subject
	subject := createSubject(t, injector, ctx)

	// Create course with matching criteria
	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)
	require.Empty(t, course.BasicInfo.SupervisorIds, "Course should not have supervisor IDs initially")

	// Create supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)
	require.NotNil(t, assignment)

	// Verify course has supervisor ID synced
	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	require.NotNil(t, updatedCourse)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor ID synced")
}

// TestSupervisorAssignmentSyncToCoachings tests that creating a supervisor assignment
// syncs supervisor IDs to matching coachings
func TestSupervisorAssignmentSyncToCoachings(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities for course/coaching operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create supervisor
	supervisor := createSupervisor(t, injector, ctx, nil)

	// Create teacher (required for coaching)
	teacher := createTeacher(t, injector, ctx)

	// Create subject
	subject := createSubject(t, injector, ctx)

	// Create coaching with matching criteria
	coachingRepo := do.MustInvoke[content.CoachingSvcs](injector)
	coachingDto := &contentmodels.CoachingDto{
		Title:      transl.Localizable[string]{"en": "Test Coaching"},
		Subject:    subject.Id,
		Country:    "US",
		SchoolType: "public",
		Level:      "elementary",
		Sessions:   []contentmodels.CoachingSession{},
		Coachs:     []primitive.ObjectID{teacher.Id},
	}
	coaching, err := coachingRepo.Add(ctx, coachingDto)
	require.NoError(t, err)
	require.Empty(t, coaching.SupervisorIds, "Coaching should not have supervisor IDs initially")

	// Create supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)
	require.NotNil(t, assignment)

	// Verify coaching has supervisor ID synced
	updatedCoaching, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	require.NotNil(t, updatedCoaching)
	assert.Contains(t, updatedCoaching.SupervisorIds, supervisor.Id, "Coaching should have supervisor ID synced")
}

// TestMultipleSupervisorsForCourse tests that multiple supervisors can be assigned
// to the same course
func TestMultipleSupervisorsForCourse(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities for course/coaching operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create two supervisors
	supervisor1 := createSupervisor(t, injector, ctx, nil)
	supervisor2 := createSupervisor(t, injector, ctx, nil)

	// Create teacher (required for course)
	teacher := createTeacher(t, injector, ctx)

	// Create subject
	subject := createSubject(t, injector, ctx)

	// Create course
	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	// Create first supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto1 := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor1.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto1)
	require.NoError(t, err)

	// Create second supervisor assignment
	assignmentDto2 := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor2.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto2)
	require.NoError(t, err)

	// Verify course has both supervisor IDs
	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor1.Id, "Course should have first supervisor ID")
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor2.Id, "Course should have second supervisor ID")
	assert.Len(t, updatedCourse.BasicInfo.SupervisorIds, 2, "Course should have exactly 2 supervisor IDs")
}

// TestSupervisorAssignmentUpdateSyncs tests that updating a supervisor assignment
// updates supervisor IDs in courses and coachings
func TestSupervisorAssignmentUpdateSyncs(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities for course/coaching operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create supervisor
	supervisor := createSupervisor(t, injector, ctx, nil)

	// Create teacher (required for course)
	teacher := createTeacher(t, injector, ctx)

	// Create subjects
	subject1 := createSubject(t, injector, ctx)
	subject2 := createSubject(t, injector, ctx)

	// Create course with subject1
	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject1.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	// Create supervisor assignment with subject1
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject1.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	// Verify course has supervisor ID
	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id)

	// Update assignment to use subject2 instead
	assignmentDto.RefIds = []primitive.ObjectID{subject2.Id}
	_, err = supervisorAssignmentSvc.Update(ctx, assignment.Id.Hex(), assignmentDto)
	require.NoError(t, err)

	// Course should no longer have supervisor ID (since it doesn't match subject2)
	updatedCourse2, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, updatedCourse2.BasicInfo.SupervisorIds, supervisor.Id, "Course should not have supervisor ID after assignment update")
}

// TestSupervisorAssignmentDeleteSyncs tests that deleting a supervisor assignment
// removes supervisor IDs from courses and coachings
func TestSupervisorAssignmentDeleteSyncs(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities for course/coaching operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create supervisor
	supervisor := createSupervisor(t, injector, ctx, nil)

	// Create teacher (required for course)
	teacher := createTeacher(t, injector, ctx)

	// Create subject
	subject := createSubject(t, injector, ctx)

	// Create course
	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	// Create supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	// Verify course has supervisor ID
	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id)

	// Delete assignment
	err = supervisorAssignmentSvc.Delete(ctx, assignment.Id.Hex())
	require.NoError(t, err)

	// Course should no longer have supervisor ID
	updatedCourse2, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, updatedCourse2.BasicInfo.SupervisorIds, supervisor.Id, "Course should not have supervisor ID after assignment deletion")
}

// TestSupervisorAssignmentCriteriaMatching tests that supervisor assignments
// only sync to courses/coachings that match all criteria (country, level, schoolType)
func TestSupervisorAssignmentCriteriaMatching(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities for course/coaching operations
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create supervisor
	supervisor := createSupervisor(t, injector, ctx, nil)

	// Create teacher (required for course)
	teacher := createTeacher(t, injector, ctx)

	// Create subject
	subject := createSubject(t, injector, ctx)

	// Create course matching criteria
	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	matchingCourseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Matching Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	matchingCourse, err := courseRepo.Add(ctx, matchingCourseDto)
	require.NoError(t, err)

	// Create course NOT matching criteria (different country)
	nonMatchingCourseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Non-Matching Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "UK", // Different country
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	nonMatchingCourse, err := courseRepo.Add(ctx, nonMatchingCourseDto)
	require.NoError(t, err)

	// Create supervisor assignment
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	// Verify matching course has supervisor ID
	matchingCourseUpdated, err := courseRepo.GetById(ctx, matchingCourse.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, matchingCourseUpdated.BasicInfo.SupervisorIds, supervisor.Id, "Matching course should have supervisor ID")

	// Verify non-matching course does NOT have supervisor ID
	nonMatchingCourseUpdated, err := courseRepo.GetById(ctx, nonMatchingCourse.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, nonMatchingCourseUpdated.BasicInfo.SupervisorIds, supervisor.Id, "Non-matching course should not have supervisor ID")
}

// TestSupervisorAssignmentStatusSync tests that updating supervisor assignment status
// between 'assigned' and 'pending' correctly syncs/removes supervisor IDs
func TestSupervisorAssignmentStatusSync(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	// Add supervisor role to RolesMap
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	// Add required capabilities
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
	ctx := context.Background()

	// Setup context with app config
	hp := &common.HeaderParams{
		Client:         dbConfig.Database,
		AcceptLanguage: "en",
	}
	cfg := &types.AppCfg{
		Db:   dbConfig.Database,
		Lang: "en",
		Hp:   hp,
		User: &types.User{
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	// Create supervisor, teacher, and subject
	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	// Create course
	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Status Sync Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	// Create coaching
	coachingRepo := do.MustInvoke[content.CoachingSvcs](injector)
	coachingDto := &contentmodels.CoachingDto{
		Title:      transl.Localizable[string]{"en": "Status Sync Coaching"},
		Subject:    subject.Id,
		Country:    "US",
		SchoolType: "public",
		Level:      "elementary",
		Sessions:   []contentmodels.CoachingSession{},
		Coachs:     []primitive.ObjectID{teacher.Id},
	}
	coaching, err := coachingRepo.Add(ctx, coachingDto)
	require.NoError(t, err)

	// 1. Create assignment as 'assigned'
	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
		Status:       string(contentenums.SupervisorAssignmentStatusAssigned),
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	// Verify course and coaching have supervisor ID
	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor ID when assigned")

	updatedCoaching, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.Contains(t, updatedCoaching.SupervisorIds, supervisor.Id, "Coaching should have supervisor ID when assigned")

	// 2. Update status to 'pending'
	assignmentDto.Status = string(contentenums.SupervisorAssignmentStatusPending)
	_, err = supervisorAssignmentSvc.Update(ctx, assignment.Id.Hex(), assignmentDto)
	require.NoError(t, err)

	// Verify course and coaching NO LONGER have supervisor ID
	updatedCourse2, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, updatedCourse2.BasicInfo.SupervisorIds, supervisor.Id, "Course should NOT have supervisor ID when pending")

	updatedCoaching2, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.NotContains(t, updatedCoaching2.SupervisorIds, supervisor.Id, "Coaching should NOT have supervisor ID when pending")

	// 3. Update status back to 'assigned'
	assignmentDto.Status = string(contentenums.SupervisorAssignmentStatusAssigned)
	_, err = supervisorAssignmentSvc.Update(ctx, assignment.Id.Hex(), assignmentDto)
	require.NoError(t, err)

	// Verify course and coaching have supervisor ID again
	updatedCourse3, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse3.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor ID again when assigned")

	updatedCoaching3, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.Contains(t, updatedCoaching3.SupervisorIds, supervisor.Id, "Coaching should have supervisor ID again when assigned")
}

// TestDirectSupervisorAssignmentCascadesToCourse tests that assigning a supervisor directly
// to a teacher cascades the supervisor to all courses that teacher is assigned to
func TestDirectSupervisorAssignmentCascadesToCourse(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)
	require.Empty(t, course.BasicInfo.SupervisorIds, "Course should not have supervisor IDs initially")

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor ID cascaded from teacher assignment")
}

// TestDirectSupervisorAssignmentCascadesToCoaching tests that assigning a supervisor directly
// to a teacher cascades the supervisor to all coachings that teacher is assigned to
func TestDirectSupervisorAssignmentCascadesToCoaching(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	coachingRepo := do.MustInvoke[content.CoachingSvcs](injector)
	coachingDto := &contentmodels.CoachingDto{
		Title:      transl.Localizable[string]{"en": "Test Coaching"},
		Subject:    subject.Id,
		Country:    "US",
		SchoolType: "public",
		Level:      "elementary",
		Sessions:   []contentmodels.CoachingSession{},
		Coachs:     []primitive.ObjectID{teacher.Id},
	}
	coaching, err := coachingRepo.Add(ctx, coachingDto)
	require.NoError(t, err)
	require.Empty(t, coaching.SupervisorIds, "Coaching should not have supervisor IDs initially")

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	updatedCoaching, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.Contains(t, updatedCoaching.SupervisorIds, supervisor.Id, "Coaching should have supervisor ID cascaded from teacher assignment")
}

// TestDirectSupervisorAssignmentRemovedFromCourse tests that removing a supervisor
// from a teacher removes them from courses where they only had access through that teacher
func TestDirectSupervisorAssignmentRemovedFromCourse(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor ID")

	err = supervisorAssignmentSvc.Delete(ctx, assignment.Id.Hex())
	require.NoError(t, err)

	updatedCourse2, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, updatedCourse2.BasicInfo.SupervisorIds, supervisor.Id, "Course should NOT have supervisor ID after deletion")
}

// TestDirectSupervisorAssignmentRemovedFromCoaching tests that removing a supervisor
// from a teacher removes them from coachings where they only had access through that teacher
func TestDirectSupervisorAssignmentRemovedFromCoaching(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	coachingRepo := do.MustInvoke[content.CoachingSvcs](injector)
	coachingDto := &contentmodels.CoachingDto{
		Title:      transl.Localizable[string]{"en": "Test Coaching"},
		Subject:    subject.Id,
		Country:    "US",
		SchoolType: "public",
		Level:      "elementary",
		Sessions:   []contentmodels.CoachingSession{},
		Coachs:     []primitive.ObjectID{teacher.Id},
	}
	coaching, err := coachingRepo.Add(ctx, coachingDto)
	require.NoError(t, err)

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	assignment, err := supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	updatedCoaching, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.Contains(t, updatedCoaching.SupervisorIds, supervisor.Id, "Coaching should have supervisor ID")

	err = supervisorAssignmentSvc.Delete(ctx, assignment.Id.Hex())
	require.NoError(t, err)

	updatedCoaching2, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.NotContains(t, updatedCoaching2.SupervisorIds, supervisor.Id, "Coaching should NOT have supervisor ID after deletion")
}

// TestTeacherAddedToCourseTriggersSupervisorCascade tests that adding a teacher to a course
// adds their supervisors to that course
func TestTeacherAddedToCourseTriggersSupervisorCascade(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	teacherAssignmentSvc := do.MustInvoke[content.TeacherAssignmentSvcs](injector)

	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)
	require.Empty(t, course.BasicInfo.SupervisorIds, "Course should not have supervisor IDs initially")
	require.Empty(t, course.BasicInfo.Teachers, "Course should not have teachers initially")

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	err = teacherAssignmentSvc.BulkCreateOrUpdate(ctx, &contentmodels.BulkTeacherAssignmentDto{
		Type:       "course",
		TargetId:   course.Id,
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})
	require.NoError(t, err)

	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.Teachers, teacher.Id, "Course should have teacher")
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor cascaded from teacher")
}

// TestTeacherRemovedFromCourseRemovesSupervisorCascade tests that removing a teacher from a course
// removes supervisors who only had access through that teacher
func TestTeacherRemovedFromCourseRemovesSupervisorCascade(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	teacherAssignmentSvc := do.MustInvoke[content.TeacherAssignmentSvcs](injector)

	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor")

	err = teacherAssignmentSvc.BulkCreateOrUpdate(ctx, &contentmodels.BulkTeacherAssignmentDto{
		Type:       "course",
		TargetId:   course.Id,
		TeacherIds: []primitive.ObjectID{},
	})
	require.NoError(t, err)

	updatedCourse2, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, updatedCourse2.BasicInfo.Teachers, teacher.Id, "Course should NOT have teacher")
	assert.NotContains(t, updatedCourse2.BasicInfo.SupervisorIds, supervisor.Id, "Course should NOT have supervisor after teacher removal")
}

// TestTeacherAddedToCoachingTriggersSupervisorCascade tests that adding a teacher to a coaching
// adds their supervisors to that coaching
func TestTeacherAddedToCoachingTriggersSupervisorCascade(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	coachingRepo := do.MustInvoke[content.CoachingSvcs](injector)
	teacherAssignmentSvc := do.MustInvoke[content.TeacherAssignmentSvcs](injector)

	coachingDto := &contentmodels.CoachingDto{
		Title:      transl.Localizable[string]{"en": "Test Coaching"},
		Subject:    subject.Id,
		Country:    "US",
		SchoolType: "public",
		Level:      "elementary",
		Sessions:   []contentmodels.CoachingSession{},
		Coachs:     []primitive.ObjectID{},
	}
	coaching, err := coachingRepo.Add(ctx, coachingDto)
	require.NoError(t, err)
	require.Empty(t, coaching.SupervisorIds, "Coaching should not have supervisor IDs initially")
	require.Empty(t, coaching.Coachs, "Coaching should not have coaches initially")

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	err = teacherAssignmentSvc.BulkCreateOrUpdate(ctx, &contentmodels.BulkTeacherAssignmentDto{
		Type:       "coaching",
		TargetId:   coaching.Id,
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})
	require.NoError(t, err)

	updatedCoaching, err := coachingRepo.GetById(ctx, coaching.Id.Hex(), false)
	require.NoError(t, err)
	assert.Contains(t, updatedCoaching.Coachs, teacher.Id, "Coaching should have teacher")
	assert.Contains(t, updatedCoaching.SupervisorIds, supervisor.Id, "Coaching should have supervisor cascaded from teacher")
}

// TestSupervisorWithMultipleTeachersCascadesToAllCourses tests that a supervisor assigned
// to multiple teachers gets added to all courses those teachers are on
func TestSupervisorWithMultipleTeachersCascadesToAllCourses(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher1 := createTeacher(t, injector, ctx)
	teacher2 := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)

	course1Dto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Course 1"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher1.Id},
		},
		Content: []contentmodels.Section{},
	}
	course1, err := courseRepo.Add(ctx, course1Dto)
	require.NoError(t, err)

	course2Dto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Course 2"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher2.Id},
		},
		Content: []contentmodels.Section{},
	}
	course2, err := courseRepo.Add(ctx, course2Dto)
	require.NoError(t, err)

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)
	assignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher1.Id, teacher2.Id},
	}

	_, err = supervisorAssignmentSvc.Create(ctx, assignmentDto)
	require.NoError(t, err)

	updatedCourse1, err := courseRepo.GetById(ctx, course1.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse1.BasicInfo.SupervisorIds, supervisor.Id, "Course 1 should have supervisor")

	updatedCourse2, err := courseRepo.GetById(ctx, course2.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse2.BasicInfo.SupervisorIds, supervisor.Id, "Course 2 should have supervisor")
}

// TestDirectAndDynamicSupervisorAssignmentCombined tests that a supervisor can be both
// directly assigned to a teacher AND via dynamic assignment criteria
func TestDirectAndDynamicSupervisorAssignmentCombined(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)

	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)

	directAssignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}
	_, err = supervisorAssignmentSvc.Create(ctx, directAssignmentDto)
	require.NoError(t, err)

	dynamicAssignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}
	_, err = supervisorAssignmentSvc.Create(ctx, dynamicAssignmentDto)
	require.NoError(t, err)

	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should have supervisor")
	assert.Len(t, updatedCourse.BasicInfo.SupervisorIds, 1, "Supervisor should only appear once even with both assignment types")
}

// TestSupervisorNotRemovedWhenTeacherRemovedButStillOnCourseViaDynamic tests that a supervisor
// is NOT removed from a course if the teacher is removed but the supervisor is still
// assigned via dynamic criteria
func TestSupervisorNotRemovedWhenTeacherRemovedButStillOnCourseViaDynamic(t *testing.T) {
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	supervisorRoleID := primitive.NewObjectID()
	authConfig.RolesMap["supervisor"] = supervisorRoleID
	authConfig.Capabilities = []string{
		"authenticate",
		"ManageAllCourses",
		"ManageAssignedCourses",
		"ViewAllCourses",
		"ViewAllCoachingPrograms",
		"ManageAllCoachingPrograms",
	}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	injector, _, _ := setup.SetupFullTestApp(t, dbConfig, authServer.URL())
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
			Id: primitive.NewObjectID(),
		},
	}
	ctx = context.WithValue(ctx, util.ReqAppCfg, cfg)

	supervisor := createSupervisor(t, injector, ctx, nil)
	teacher := createTeacher(t, injector, ctx)
	subject := createSubject(t, injector, ctx)

	courseRepo := do.MustInvoke[content.CourseSvcs](injector)
	teacherAssignmentSvc := do.MustInvoke[content.TeacherAssignmentSvcs](injector)

	courseDto := &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Name:           transl.Localizable[string]{"en": "Test Course"},
			Type:           "curriculum",
			Status:         "active",
			LinkedSubjects: []primitive.ObjectID{subject.Id},
			Country:        "US",
			SchoolType:     "public",
			Level:          "elementary",
			Teachers:       []primitive.ObjectID{teacher.Id},
		},
		Content: []contentmodels.Section{},
	}
	course, err := courseRepo.Add(ctx, courseDto)
	require.NoError(t, err)

	supervisorAssignmentSvc := do.MustInvoke[content.SupervisorAssignmentSvcs](injector)

	directAssignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDirect),
		RefType:      string(contentenums.SupervisorRefTypeTeacher),
		RefIds:       []primitive.ObjectID{teacher.Id},
	}
	_, err = supervisorAssignmentSvc.Create(ctx, directAssignmentDto)
	require.NoError(t, err)

	dynamicAssignmentDto := &contentmodels.SupervisorAssignmentDto{
		SupervisorId: supervisor.Id,
		AssignType:   string(contentenums.SupervisorAssignmentTypeDynamic),
		RefType:      string(contentenums.SupervisorRefTypeSubject),
		RefIds:       []primitive.ObjectID{subject.Id},
		SchoolType:   "public",
		Country:      []string{"US"},
		Level:        []string{"elementary"},
	}
	_, err = supervisorAssignmentSvc.Create(ctx, dynamicAssignmentDto)
	require.NoError(t, err)

	err = teacherAssignmentSvc.BulkCreateOrUpdate(ctx, &contentmodels.BulkTeacherAssignmentDto{
		Type:       "course",
		TargetId:   course.Id,
		TeacherIds: []primitive.ObjectID{},
	})
	require.NoError(t, err)

	updatedCourse, err := courseRepo.GetById(ctx, course.Id.Hex())
	require.NoError(t, err)
	assert.NotContains(t, updatedCourse.BasicInfo.Teachers, teacher.Id, "Course should NOT have teacher")
	assert.Contains(t, updatedCourse.BasicInfo.SupervisorIds, supervisor.Id, "Course should STILL have supervisor via dynamic assignment")
}
