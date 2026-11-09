package calendar

import (
	contactenums "larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/services/contact/models"
	content "larsa-academy-microservice/pkg/services/content"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// TestCalendar_SupervisorViewingManagedTeacherEvents verifies that:
// - A supervisor can see events for teachers they manage
// - Enrichment works correctly for supervisors
// - Data privacy is maintained (supervisors can see student names for their teachers)
func TestCalendar_SupervisorViewingManagedTeacherEvents(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	supervisorUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = supervisorUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityViewSupervisedCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Create supervisor and teacher
	supervisor := fixture.createTeacher(t, &models.TeacherDto{}) // Supervisors are also teachers in our system
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Managed"},
		LastName:  transl.Localizable[string]{"en": "Teacher"},
	})

	// Create supervisor assignment
	assignment := contentmodels.SupervisorAssignment{
		Id: primitive.NewObjectID(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId: supervisor.Id,
			AssignType:   "direct",
			RefType:      "teacher",
			RefIds:       []primitive.ObjectID{teacher.Id},
			Status:       "assigned",
			Trash:        false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := fixture.DB.Collection(assignment.CollName()).InsertOne(fixture.Ctx, assignment)
	require.NoError(t, err)

	// Create student
	student := fixture.createStudent(t, nil)

	// Create coaching program for the managed teacher
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"10:00", "11:00",
	))

	// Switch auth to supervisor
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate", content.CapabilityViewSupervisedCalendarEvents})

	// Get calendar events for the managed teacher
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	// Assertions
	require.NotEmpty(t, response.Events, "Supervisor should see managed teacher's events")
	assert.Equal(t, teacher.Id, response.Events[0].TeacherId)
	require.NotNil(t, response.Events[0].Teacher)
	// Enrichment combines names. Managed Teacher -> "Teacher Managed" (depending on how fixture creates it)
	// But we know it should have a name.
	assert.NotEmpty(t, response.Events[0].Teacher.Name)

	// Data privacy: supervisor should see student names for their teachers
	require.NotEmpty(t, response.Events[0].Students)
	assert.NotEmpty(t, response.Events[0].Students[0].Name)
}

// TestCalendar_SupervisorViewingAvailability_ReturnsSlotsForManagedTeacher verifies that:
// - A supervisor can view the availability slots of a teacher they manage
func TestCalendar_SupervisorViewingAvailability_ReturnsSlotsForManagedTeacher(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	supervisorUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = supervisorUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityViewSupervisedCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Create supervisor and teacher with availability
	supervisor := fixture.createTeacher(t, &models.TeacherDto{})
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		Availability: models.Availability{
			Scheduling: models.Scheduling{
				SessionDuration: models.SchedulingUnit{Value: 60, Unit: "minutes"},
			},
			DayOfWeekAvailability: map[contactenums.DayOfWeek]models.DayOfWeekAvailability{
				contactenums.Monday:    {IsSelected: true, IsCustom: false},
				contactenums.Tuesday:   {IsSelected: true, IsCustom: false},
				contactenums.Wednesday: {IsSelected: true, IsCustom: false},
				contactenums.Thursday:  {IsSelected: true, IsCustom: false},
				contactenums.Friday:    {IsSelected: true, IsCustom: false},
				contactenums.Saturday:  {IsSelected: true, IsCustom: false},
				contactenums.Sunday:    {IsSelected: true, IsCustom: false},
			},
			DefaultWorkingHours: []models.Shift{
				{From: "09:00", To: "17:00"},
			},
		},
	})

	// Create supervisor assignment
	assignment := contentmodels.SupervisorAssignment{
		Id: primitive.NewObjectID(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId: supervisor.Id,
			AssignType:   "direct",
			RefType:      "teacher",
			RefIds:       []primitive.ObjectID{teacher.Id},
			Status:       "assigned",
			Trash:        false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := fixture.DB.Collection(assignment.CollName()).InsertOne(fixture.Ctx, assignment)
	require.NoError(t, err)

	// Switch auth to supervisor
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate", content.CapabilityViewSupervisedCalendarEvents})

	// Get availability slots for the managed teacher
	response := fixture.getCalendarEventsWithView(t, contentmodels.CalendarFilters{
		TeacherIds:          []primitive.ObjectID{teacher.Id},
		IncludeAvailability: true,
		EventTypes:          []string{"availability_slot"},
	}, "weekly", time.Now().Format("2006-01-02"), "")

	// Assertions
	require.NotEmpty(t, response.Events, "Supervisor should see managed teacher's availability")
	assert.Equal(t, "availability_slot", response.Events[0].Type)
}

// TestCalendar_SupervisorViewingDynamicManagedTeacherEvents verifies that:
// - A supervisor can see events for teachers managed via dynamic (subject-based) assignments
func TestCalendar_SupervisorViewingDynamicManagedTeacherEvents(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	supervisorUserID := primitive.NewObjectID().Hex()
	authConfig := setup.DefaultMockAuthConfig()
	authConfig.UserID = supervisorUserID
	authConfig.Capabilities = []string{"authenticate", content.CapabilityViewSupervisedCalendarEvents}
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer
	fixture.initSharedFixture()

	// Create supervisor and teacher
	supervisor := fixture.createTeacher(t, &models.TeacherDto{})
	teacher := fixture.createTeacher(t, &models.TeacherDto{
		FirstName: transl.Localizable[string]{"en": "Dynamic"},
		LastName:  transl.Localizable[string]{"en": "Teacher"},
	})

	// Create supervisor assignment (dynamic via subject)
	assignment := contentmodels.SupervisorAssignment{
		Id: primitive.NewObjectID(),
		SupervisorAssignmentDto: contentmodels.SupervisorAssignmentDto{
			SupervisorId:    supervisor.Id,
			AssignType:      "dynamic",
			RefType:         "subject",
			RefIds:          []primitive.ObjectID{primitive.NewObjectID()}, // Dummy subject ID
			MatchedTeachers: []primitive.ObjectID{teacher.Id},              // Resolved teachers
			Status:          "assigned",
			Trash:           false,
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	_, err := fixture.DB.Collection(assignment.CollName()).InsertOne(fixture.Ctx, assignment)
	require.NoError(t, err)

	// Create student and coaching session
	student := fixture.createStudent(t, nil)
	now := time.Now()
	tomorrow := now.Add(24 * time.Hour)
	dayAfterTomorrow := now.Add(48 * time.Hour)
	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})

	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{tomorrow, dayAfterTomorrow},
		"10:00", "11:00",
	))

	// Switch auth to supervisor
	fixture.switchAuthContext(supervisor.Id.Hex(), []string{"authenticate", content.CapabilityViewSupervisedCalendarEvents})

	// Get calendar events for the managed teacher
	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{
		TeacherIds: []primitive.ObjectID{teacher.Id},
	})

	// Assertions
	require.NotEmpty(t, response.Events, "Supervisor should see dynamically managed teacher's events")
	assert.Equal(t, teacher.Id, response.Events[0].TeacherId)
}
