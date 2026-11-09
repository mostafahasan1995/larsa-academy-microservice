package calendar

import (
	"bytes"
	"encoding/json"
	"fmt"
	content "larsa-academy-microservice/pkg/services/content"
	contentmodels "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/test/setup"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestCalendar_AssignmentDeadlines_StudentAccess(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer

	fixture.initSharedFixture()

	// 1. Create entities
	student := fixture.createStudent(t, nil)
	otherStudent := fixture.createStudent(t, nil)
	teacher := fixture.createTeacher(t, nil)

	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{
			Teachers: []primitive.ObjectID{teacher.Id},
		},
	})
	fixture.enrollStudent(t, student.Id.Hex(), course.Id.Hex())

	coaching := fixture.createCoaching(t, &contentmodels.CoachingDto{
		Coachs: []primitive.ObjectID{teacher.Id},
	})
	// In bookCoachingSessions, student is enrolled and sessions are booked
	// Default sessions num is 4, but the API expects exactly 2 dates and alternates between them
	now := time.Now()
	fixture.bookCoachingSessions(t, student.Id.Hex(), coaching.Id, teacher.Id, createBookingDates(
		[]time.Time{
			now.Add(24 * time.Hour),
			now.Add(48 * time.Hour),
		},
		"10:00", "11:00",
	))

	// 2. Create assignments as admin
	deadline := time.Now().Add(48 * time.Hour).Truncate(time.Second)

	courseAssignment := createCourseAssignment(t, fixture, course.Id, contentmodels.CourseAssignmentDto{
		Title:    transl.Localizable[string]{"en": "Course Assignment 1"},
		Deadline: deadline,
		Status:   "active",
	})

	coachingAssignment := createCoachingAssignment(t, fixture, coaching.Id, contentmodels.CoachingAssignmentDto{
		Title:   transl.Localizable[string]{"en": "Coaching Assignment 1"},
		DueDate: deadline.Add(1 * time.Hour),
		Status:  "active",
	})

	// 3. Verify student can see their own assignments
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", content.CapabilityViewStudentCalendarEvents})

	response := fixture.getCalendarEvents(t, contentmodels.CalendarFilters{})

	var foundCourseAssignment, foundCoachingAssignment *contentmodels.EnrichedCalendarEvent
	for i := range response.Events {
		event := &response.Events[i]
		if event.Id == courseAssignment.Id {
			foundCourseAssignment = event
		}
		if event.Id == coachingAssignment.Id {
			foundCoachingAssignment = event
		}
	}

	require.NotNil(t, foundCourseAssignment, "Student should see course assignment deadline")
	assert.Equal(t, "course_assignment_deadline", foundCourseAssignment.Type)
	assert.Equal(t, false, foundCourseAssignment.AssignmentDetails.IsSubmitted)
	assert.Equal(t, "Course Assignment 1", foundCourseAssignment.AssignmentDetails.Title)

	require.NotNil(t, foundCoachingAssignment, "Student should see coaching assignment deadline")
	assert.Equal(t, "coaching_assignment_deadline", foundCoachingAssignment.Type)
	assert.Equal(t, false, foundCoachingAssignment.AssignmentDetails.IsSubmitted)

	// 4. Submit one assignment and verify IsSubmitted flag
	submitCourseAssignment(t, fixture, student.Id.Hex(), courseAssignment.Id.Hex())

	response = fixture.getCalendarEvents(t, contentmodels.CalendarFilters{})
	for i := range response.Events {
		event := &response.Events[i]
		if event.Id == courseAssignment.Id {
			assert.Equal(t, true, event.AssignmentDetails.IsSubmitted, "Assignment should be marked as submitted")
		}
	}

	// 5. Verify Privacy: Other student cannot see these assignments
	fixture.switchAuthContext(otherStudent.Id.Hex(), []string{"authenticate", content.CapabilityViewStudentCalendarEvents})
	response = fixture.getCalendarEvents(t, contentmodels.CalendarFilters{})

	for _, event := range response.Events {
		assert.NotEqual(t, courseAssignment.Id, event.Id, "Other student should not see Student 1's course assignment")
		assert.NotEqual(t, coachingAssignment.Id, event.Id, "Other student should not see Student 1's coaching assignment")
	}

	// 6. Verify Teacher Restriction: Teacher cannot see student assignment deadlines in their calendar
	fixture.switchAuthContext(teacher.Id.Hex(), []string{"authenticate", content.CapabilityAccessTeacherEndpoints, content.CapabilityViewOwnCalendarEvents})
	response = fixture.getCalendarEvents(t, contentmodels.CalendarFilters{})

	for _, event := range response.Events {
		assert.NotEqual(t, "course_assignment_deadline", event.Type, "Teacher should not see assignment deadlines")
		assert.NotEqual(t, "coaching_assignment_deadline", event.Type, "Teacher should not see assignment deadlines")
	}
}

func TestCalendar_AssignmentDeadlines_Summary(t *testing.T) {
	// Setup
	dbConfig := setup.SetupTestDB(t)
	defer dbConfig.Cleanup()

	authConfig := setup.DefaultMockAuthConfig()
	authServer := setup.NewMockAuthServer(authConfig)
	defer authServer.Close()

	_, _, _, fixture := SetupCalendarTest(t, dbConfig, authServer.URL())
	fixture.AuthServer = authServer

	fixture.initSharedFixture()

	student := fixture.createStudent(t, nil)
	teacher := fixture.createTeacher(t, nil)
	course := fixture.createCourse(t, &contentmodels.CourseDto{
		BasicInfo: contentmodels.CourseBasicInfo{Teachers: []primitive.ObjectID{teacher.Id}},
	})
	fixture.enrollStudent(t, student.Id.Hex(), course.Id.Hex())

	// Create assignment with deadline today
	today := time.Now().Truncate(24 * time.Hour)
	createCourseAssignment(t, fixture, course.Id, contentmodels.CourseAssignmentDto{
		Title:    transl.Localizable[string]{"en": "Today's Assignment"},
		Deadline: today.Add(12 * time.Hour),
		Status:   "active",
	})

	// Get Summary (Yearly View)
	fixture.switchAuthContext(student.Id.Hex(), []string{"authenticate", content.CapabilityViewStudentCalendarEvents})

	dateKey := today.Format("2006-01-02")
	url := fmt.Sprintf("/api/calendar/events?view=yearly&startDate=%s", dateKey)

	reqBody, _ := json.Marshal(contentmodels.CalendarEventsRequest{
		Filters: contentmodels.CalendarFilters{},
	})
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(reqBody))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", fixture.DBName)
	req = req.WithContext(fixture.Ctx)
	w := httptest.NewRecorder()
	fixture.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code)

	var response contentmodels.CalendarEventsResponse
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &response))

	require.NotNil(t, response.Summary, "Response should contain a summary")
	assert.GreaterOrEqual(t, response.Summary.Dates[dateKey].TotalEvents, 1, "Summary should include at least one event for today")
}

// Helper methods

func createCourseAssignment(t *testing.T, f *TestFixture, courseId primitive.ObjectID, dto contentmodels.CourseAssignmentDto) contentmodels.CourseAssignment {
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.Capabilities = []string{"authenticate", "ManageAllCourses"}
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig)

	body, _ := json.Marshal(dto)
	url := fmt.Sprintf("/courses/%s/assignments", courseId.Hex())
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated, "Failed to create course assignment: %s", w.Body.String())

	var assignment contentmodels.CourseAssignment
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &assignment))
	return assignment
}

func createCoachingAssignment(t *testing.T, f *TestFixture, coachingId primitive.ObjectID, dto contentmodels.CoachingAssignmentDto) contentmodels.CoachingAssignment {
	originalConfig := f.AuthServer.GetCurrentConfig()
	adminConfig := setup.DefaultMockAuthConfig()
	adminConfig.Capabilities = []string{"authenticate", content.CapabilityManageAllCoachingPrograms}
	f.AuthServer.UpdateConfig(adminConfig)
	defer f.AuthServer.UpdateConfig(originalConfig)

	body, _ := json.Marshal(dto)
	url := fmt.Sprintf("/coaching/%s/assignments", coachingId.Hex())
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.True(t, w.Code == http.StatusOK || w.Code == http.StatusCreated, "Failed to create coaching assignment: %s", w.Body.String())

	var assignment contentmodels.CoachingAssignment
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &assignment))
	return assignment
}

func submitCourseAssignment(t *testing.T, f *TestFixture, studentId string, assignmentId string) {
	originalConfig := f.AuthServer.GetCurrentConfig()
	studentConfig := setup.DefaultMockAuthConfig()
	studentConfig.UserID = studentId
	f.AuthServer.UpdateConfig(studentConfig)
	defer f.AuthServer.UpdateConfig(originalConfig)

	dto := contentmodels.CourseStudentAssignmentDto{
		Title:  transl.Localizable[string]{"en": "Submission"},
		Answer: "My answer",
		Status: "submitted",
	}
	body, _ := json.Marshal(dto)
	url := fmt.Sprintf("/api/courses/assignments/%s/submit", assignmentId)
	req := httptest.NewRequest("POST", url, bytes.NewBuffer(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-access-token", "test-token")
	req.Header.Set("x-client", f.DBName)
	req = req.WithContext(f.Ctx)
	w := httptest.NewRecorder()
	f.Router.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "Failed to submit course assignment: %s", w.Body.String())
}
