package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// CalendarFilters defines filters for calendar events
type CalendarFilters struct {
	TeacherIds          []primitive.ObjectID `json:"teacherIds,omitempty"`
	CourseIds           []primitive.ObjectID `json:"courseIds,omitempty"`
	CoachingProgramIds  []primitive.ObjectID `json:"coachingProgramIds,omitempty"`
	StudentIds          []primitive.ObjectID `json:"studentIds,omitempty"`
	Statuses            []string             `json:"statuses,omitempty"`
	EventTypes          []string             `json:"eventTypes,omitempty"`   // ["coaching_session", "course_live_session", "placement_test", "availability_slot", "coaching_assignment_deadline", "course_assignment_deadline"]
	BundleFilter        string               `json:"bundleFilter,omitempty"` // "bundle_only" | "independent_only" | "all"
	BundleIds           []primitive.ObjectID `json:"bundleIds,omitempty"`
	IncludeAvailability bool                 `json:"includeAvailability,omitempty"`
}

// CalendarEventsRequest is the request structure for calendar events
type CalendarEventsRequest struct {
	Filters CalendarFilters `json:"filters"`
}

// CalendarEventsResponse is the response structure for calendar events
type CalendarEventsResponse struct {
	Events     []EnrichedCalendarEvent `json:"events"`
	ViewType   string                  `json:"viewType"`
	DateRange  DateRange               `json:"dateRange"`
	Pagination *types.Pagination       `json:"pagination,omitempty"` // Only for agenda view
	Summary    *CalendarSummary        `json:"summary,omitempty"`    // Only for yearly view
}

// DateRange represents a time range
type DateRange struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// CalendarSummary is returned for yearly view instead of full events
type CalendarSummary struct {
	// Map of date (YYYY-MM-DD format) to summary data
	// Only includes dates that have events
	Dates map[string]DateSummary `json:"dates"`
}

// DateSummary contains summary information for a specific date
type DateSummary struct {
	HasEvents   bool           `json:"hasEvents"`
	EventCounts map[string]int `json:"eventCounts"` // event type -> count
	TotalEvents int            `json:"totalEvents"`
}

// CalendarEvent is the base event structure
type CalendarEvent struct {
	Id                primitive.ObjectID  `json:"id"`
	Type              string              `json:"type"` // "coaching_session" | "course_live_session" | "placement_test" | "availability_slot" | "coaching_assignment_deadline" | "course_assignment_deadline"
	Title             string              `json:"title"`
	StartTime         time.Time           `json:"startTime"`
	EndTime           time.Time           `json:"endTime"`
	TeacherId         primitive.ObjectID  `json:"teacherId"`
	StudentId         *primitive.ObjectID `json:"studentId,omitempty"`         // nullable - only for coaching_session & placement_test
	CourseId          *primitive.ObjectID `json:"courseId,omitempty"`          // nullable - only for course_live_session
	CoachingProgramId *primitive.ObjectID `json:"coachingProgramId,omitempty"` // nullable - only for coaching_session & placement_test
	Status            string              `json:"status"`
	BundleId          *primitive.ObjectID `json:"bundleId,omitempty"` // nullable - only if event is from a bundle
}

// EnrichedCalendarEvent contains the base event plus enrichment data
type EnrichedCalendarEvent struct {
	CalendarEvent `bson:",inline"`

	// Enrichment objects (populated when needed)
	Teacher         *EventTeacher         `json:"teacher,omitempty"`
	Course          *EventCourse          `json:"course,omitempty"`
	CoachingProgram *EventCoachingProgram `json:"coachingProgram,omitempty"`
	Bundle          *EventBundle          `json:"bundle,omitempty"`
	Students        []EventStudent        `json:"students,omitempty"`

	// Type-specific details
	CoachingSessionDetails *CoachingSessionDetails `json:"coachingSessionDetails,omitempty"`
	PlacementTestDetails   *PlacementTestDetails   `json:"placementTestDetails,omitempty"`
	AvailableSlotDetails   *AvailableSlotDetails   `json:"availableSlotDetails,omitempty"`
	AssignmentDetails      *AssignmentDetails      `json:"assignmentDetails,omitempty"`
}

// EventTeacher contains teacher information for enrichment
type EventTeacher struct {
	Id   primitive.ObjectID         `json:"id"`
	Name transl.Localizable[string] `json:"name"` // Full name or display name
}

// EventCourse contains course information for enrichment
type EventCourse struct {
	Id              primitive.ObjectID `json:"id"`
	Name            string             `json:"name"` // Localized name
	EnrollmentCount int                `json:"enrollmentCount"`
	Teachers        []EventTeacher     `json:"teachers"` // All teachers assigned to the course
}

// EventCoachingProgram contains coaching program information for enrichment
type EventCoachingProgram struct {
	Id   primitive.ObjectID `json:"id"`
	Name string             `json:"name"` // Title or name
}

// EventBundle contains bundle information for enrichment
type EventBundle struct {
	Id   primitive.ObjectID `json:"id"`
	Name string             `json:"name"`
}

// EventStudent contains student information for enrichment
type EventStudent struct {
	Id   primitive.ObjectID `json:"id"`
	Name string             `json:"name"` // Full name or display name
}

// CoachingSessionDetails contains type-specific details for coaching sessions
type CoachingSessionDetails struct {
	SessionNum  int                        `json:"sessionNum"`
	Name        transl.Localizable[string] `json:"name"`
	Description transl.Localizable[string] `json:"description"`
	ZoomMeeting *ZoomMeetingDetails        `json:"zoomMeeting,omitempty"`
}

// PlacementTestDetails contains type-specific details for placement tests
type PlacementTestDetails struct {
	ZoomMeeting *ZoomMeetingDetails `json:"zoomMeeting,omitempty"`
}

// ZoomMeetingDetails contains non-sensitive Zoom meeting information
type ZoomMeetingDetails struct {
	Id              primitive.ObjectID `json:"id"`
	ZoomMeetingId   string             `json:"zoomMeetingId"`
	JoinUrl         string             `json:"joinUrl"`
	Password        string             `json:"password"`
	StartTime       time.Time          `json:"startTime"`
	DurationMinutes int                `json:"durationMinutes"`
	Status          string             `json:"status"`
	ActualStartTime *time.Time         `json:"actualStartTime,omitempty"`
	ActualEndTime   *time.Time         `json:"actualEndTime,omitempty"`
	TeacherJoined   bool               `json:"teacherJoined"`
	RecordingUrl    string             `json:"recordingUrl,omitempty"`
}

// AvailableSlotDetails contains type-specific details for availability slots
type AvailableSlotDetails struct {
	SessionDuration int `json:"sessionDuration"` // in minutes
}

// AssignmentDetails contains type-specific details for assignments
type AssignmentDetails struct {
	Id          primitive.ObjectID `json:"id"`
	Title       string             `json:"title"`
	Description string             `json:"description"`
	Status      string             `json:"status"` // "active"
	IsSubmitted bool               `json:"isSubmitted"`
}
