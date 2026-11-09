package content

import (
	"context"
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/db"
	contactenums "larsa-academy-microservice/pkg/services/contact/enums"
	contactmodels "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	zoommodels "larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/pkg/shared"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"log/slog"
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"git.larsa.io/mahdawi/microservices-commons.git/common"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// Calendar constants
const (
	// DefaultCourseDurationMinutes is the default duration for course live sessions (2 hours)
	DefaultCourseDurationMinutes = 120
	// MaxAvailabilityDateRangeDays is the maximum date range allowed for availability calculation (1 year)
	MaxAvailabilityDateRangeDays = 365
	// DefaultFirstDayOfWeek is the default first day of week for calendar views
	DefaultFirstDayOfWeek = "monday"
	// FallbackLanguage is the fallback language when language is not specified
	FallbackLanguage = "en"
)

// Calendar-related errors
var (
	// ErrAvailabilityDateRangeExceeded is returned when availability calculation is requested for a date range exceeding 1 year
	ErrAvailabilityDateRangeExceeded = errors.New("availability calculation limited to 1 year. Please reduce date range")
	// ErrUnauthorizedInsufficientPermissions is returned when user lacks required permissions
	ErrUnauthorizedInsufficientPermissions = errors.New("unauthorized: insufficient permissions")
	// ErrUnauthorizedOwnEventsOnly is returned when a teacher tries to view events for other teachers
	ErrUnauthorizedOwnEventsOnly = errors.New("unauthorized: can only view own events")
	// ErrUnauthorizedOwnAvailabilityOnly is returned when a teacher tries to view availability for other teachers
	ErrUnauthorizedOwnAvailabilityOnly = errors.New("unauthorized: can only view own availability")
)

// CalendarSvcs defines the interface for calendar-related operations
type CalendarSvcs interface {
	GetCalendarEvents(ctx context.Context, req *CalendarEventsRequest, viewType string, startDate, endDate time.Time, page, limit int) (*models.CalendarEventsResponse, error)
	GetCoachingSessions(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error)
	GetCourseLiveSessions(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error)
	GetPlacementTests(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error)
	GetCoachingAssignmentDeadlines(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error)
	GetCourseAssignmentDeadlines(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error)
	EnrichWithBundleInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventBundle, error)
	EnrichWithStudentInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventStudent, error)
	EnrichWithTeacherInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventTeacher, error)
	EnrichWithCourseInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventCourse, error)
	EnrichWithCoachingProgramInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventCoachingProgram, error)
	EnrichWithTypeSpecificDetails(ctx context.Context, events []models.CalendarEvent) map[string]interface{}
	CalculateDateRange(ctx context.Context, viewType string, referenceDate time.Time, timezone string) (models.DateRange, error)
	ApplyBundleFilter(events []models.CalendarEvent, bundleFilter string) []models.CalendarEvent
	GetCalendarSummary(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) (*models.CalendarSummary, error)
	GetTeacherBusySlots(ctx context.Context, teacherId primitive.ObjectID, dateRange models.DateRange) (map[string][][2]int, error)
	GetTeacherAvailabilitySlots(ctx context.Context, teacherId primitive.ObjectID, dateRange models.DateRange, coachingProgramId *primitive.ObjectID) ([]models.CalendarEvent, error)
	ApplyAuthorization(ctx context.Context, events []models.EnrichedCalendarEvent, filters models.CalendarFilters) ([]models.EnrichedCalendarEvent, error)
}

// calendarsvcs is the implementation of CalendarSvcs interface
type calendarsvcs struct {
	studentSessionRepo            db.MainRepo[models.StudentSession]
	courseRepo                    db.MainRepo[models.Course]
	placementTestScheduleRepo     db.MainRepo[models.OnlinePlacementTestSchedule]
	bundleRepo                    db.MainRepo[models.Bundle]
	studentRepo                   db.MainRepo[contactmodels.Student]
	teacherRepo                   db.MainRepo[contactmodels.Teacher]
	studentCourseRepo             db.MainRepo[models.StudentCourse]
	coachingRepo                  db.MainRepo[models.Coaching]
	coachingStudentRepo           db.MainRepo[models.CoachingStudent]
	coachingAssignmentRepo        db.MainRepo[models.CoachingAssignment]
	coachingStudentAssignmentRepo db.MainRepo[models.CoachingStudentAssignment]
	courseAssignmentRepo          db.MainRepo[models.CourseAssignment]
	courseStudentAssignmentRepo   db.MainRepo[models.CourseStudentAssignment]
	supervisorAssignmentRepo      db.MainRepo[models.SupervisorAssignment]
	zoomMeetingRepo               db.MainRepo[zoommodels.ZoomMeeting]
	teacherAssignmentSvc          TeacherAssignmentSvcs
	parentChecker                 shared.ParentChecker
}

// NewCalendarSvcs creates a new calendar service
func NewCalendarSvcs(i *do.Injector) (CalendarSvcs, error) {
	return &calendarsvcs{
		studentSessionRepo:            db.NewRepo[models.StudentSession](i),
		courseRepo:                    db.NewRepo[models.Course](i),
		placementTestScheduleRepo:     db.NewRepo[models.OnlinePlacementTestSchedule](i),
		bundleRepo:                    db.NewRepo[models.Bundle](i),
		studentRepo:                   db.NewRepo[contactmodels.Student](i),
		teacherRepo:                   db.NewRepo[contactmodels.Teacher](i),
		studentCourseRepo:             db.NewRepo[models.StudentCourse](i),
		coachingRepo:                  db.NewRepo[models.Coaching](i),
		coachingStudentRepo:           db.NewRepo[models.CoachingStudent](i),
		coachingAssignmentRepo:        db.NewRepo[models.CoachingAssignment](i),
		coachingStudentAssignmentRepo: db.NewRepo[models.CoachingStudentAssignment](i),
		courseAssignmentRepo:          db.NewRepo[models.CourseAssignment](i),
		courseStudentAssignmentRepo:   db.NewRepo[models.CourseStudentAssignment](i),
		supervisorAssignmentRepo:      db.NewRepo[models.SupervisorAssignment](i),
		zoomMeetingRepo:               db.NewRepo[zoommodels.ZoomMeeting](i),
		teacherAssignmentSvc:          do.MustInvoke[TeacherAssignmentSvcs](i),
		parentChecker:                 do.MustInvoke[shared.ParentChecker](i),
	}, nil
}

// GetCalendarEvents is the main entry point for calendar events
func (c *calendarsvcs) GetCalendarEvents(ctx context.Context, req *CalendarEventsRequest, viewType string, startDate, endDate time.Time, page, limit int) (*models.CalendarEventsResponse, error) {
	// Yearly view automatically uses summary mode
	if viewType == "yearly" {
		dateRange := models.DateRange{
			Start: startDate,
			End:   endDate,
		}
		summary, err := c.GetCalendarSummary(ctx, req.Filters, dateRange)
		if err != nil {
			return nil, err
		}
		return &models.CalendarEventsResponse{
			Events:    []models.EnrichedCalendarEvent{},
			ViewType:  viewType,
			DateRange: dateRange,
			Summary:   summary,
		}, nil
	}

	// For other views, fetch full events
	dateRange := models.DateRange{
		Start: startDate,
		End:   endDate,
	}

	// Fetch events from different sources
	allEvents := []models.CalendarEvent{}

	// Get coaching sessions
	if len(req.Filters.EventTypes) == 0 || contains(req.Filters.EventTypes, "coaching_session") {
		coachingEvents, err := c.GetCoachingSessions(ctx, req.Filters, dateRange)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, coachingEvents...)
	}

	// Get course live sessions
	if len(req.Filters.EventTypes) == 0 || contains(req.Filters.EventTypes, "course_live_session") {
		courseEvents, err := c.GetCourseLiveSessions(ctx, req.Filters, dateRange)
		if err != nil {
			fmt.Printf("DEBUG: GetCourseLiveSessions error: %v\n", err)
			return nil, err
		}
		allEvents = append(allEvents, courseEvents...)
	}

	// Get placement tests
	if len(req.Filters.EventTypes) == 0 || contains(req.Filters.EventTypes, "placement_test") {
		placementEvents, err := c.GetPlacementTests(ctx, req.Filters, dateRange)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, placementEvents...)
	}

	// Get coaching assignment deadlines
	if len(req.Filters.EventTypes) == 0 || contains(req.Filters.EventTypes, "coaching_assignment_deadline") {
		coachingAssignmentEvents, err := c.GetCoachingAssignmentDeadlines(ctx, req.Filters, dateRange)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, coachingAssignmentEvents...)
	}

	// Get course assignment deadlines
	if len(req.Filters.EventTypes) == 0 || contains(req.Filters.EventTypes, "course_assignment_deadline") {
		courseAssignmentEvents, err := c.GetCourseAssignmentDeadlines(ctx, req.Filters, dateRange)
		if err != nil {
			return nil, err
		}
		allEvents = append(allEvents, courseAssignmentEvents...)
	}

	// Get availability slots (only if single teacher and IncludeAvailability is true)
	// Note: Availability calculation is skipped for agenda view as it shows all future events
	if req.Filters.IncludeAvailability && len(req.Filters.TeacherIds) == 1 && viewType != "agenda" {
		// Check if availability_slot is requested or no event type filter
		if len(req.Filters.EventTypes) == 0 || contains(req.Filters.EventTypes, "availability_slot") {
			teacherId := req.Filters.TeacherIds[0]

			// Get coaching program ID if filtering by coaching program
			var coachingProgramId *primitive.ObjectID
			if len(req.Filters.CoachingProgramIds) == 1 {
				coachingProgramId = &req.Filters.CoachingProgramIds[0]
			}

			availabilityEvents, err := c.GetTeacherAvailabilitySlots(ctx, teacherId, dateRange, coachingProgramId)
			if err != nil {
				// If error is about date range limit, return it
				if errors.Is(err, ErrAvailabilityDateRangeExceeded) {
					return nil, err
				}
				// For other errors, log but continue (don't fail entire request)
				slog.Warn("Failed to get availability slots",
					"teacherId", teacherId.Hex(),
					"error", err.Error(),
					"dateRange", fmt.Sprintf("%v to %v", dateRange.Start, dateRange.End))
			} else {
				allEvents = append(allEvents, availabilityEvents...)
			}
		}
	}

	// Enrich with bundle info
	bundleMap, err := c.EnrichWithBundleInfo(ctx, allEvents)
	if err != nil {
		return nil, err
	}

	// Apply bundle filter
	allEvents = c.ApplyBundleFilter(allEvents, req.Filters.BundleFilter)

	// Apply bundle IDs filter
	if len(req.Filters.BundleIds) > 0 {
		filtered := []models.CalendarEvent{}
		for _, event := range allEvents {
			if event.BundleId != nil {
				for _, bundleId := range req.Filters.BundleIds {
					if event.BundleId.Hex() == bundleId.Hex() {
						filtered = append(filtered, event)
						break
					}
				}
			}
		}
		allEvents = filtered
	}

	// Enrich with other info
	studentMap, err := c.EnrichWithStudentInfo(ctx, allEvents)
	if err != nil {
		return nil, err
	}

	teacherMap, err := c.EnrichWithTeacherInfo(ctx, allEvents)
	if err != nil {
		return nil, err
	}

	courseMap, err := c.EnrichWithCourseInfo(ctx, allEvents)
	if err != nil {
		return nil, err
	}

	coachingProgramMap, err := c.EnrichWithCoachingProgramInfo(ctx, allEvents)
	if err != nil {
		return nil, err
	}

	detailsMap := c.EnrichWithTypeSpecificDetails(ctx, allEvents)

	// Combine into enriched events
	enrichedEvents := []models.EnrichedCalendarEvent{}
	for _, event := range allEvents {
		enriched := models.EnrichedCalendarEvent{
			CalendarEvent: event,
		}

		// Add bundle
		if event.BundleId != nil {
			if bundle, ok := bundleMap[*event.BundleId]; ok {
				enriched.Bundle = bundle
			}
		}

		// Add student
		if event.StudentId != nil {
			if student, ok := studentMap[*event.StudentId]; ok {
				enriched.Students = []models.EventStudent{*student}
			}
		}

		// Add teacher
		if teacher, ok := teacherMap[event.TeacherId]; ok {
			enriched.Teacher = teacher
		}

		// Add course
		if event.CourseId != nil {
			if course, ok := courseMap[*event.CourseId]; ok {
				enriched.Course = course
			}
		}

		// Add coaching program
		if event.CoachingProgramId != nil {
			if program, ok := coachingProgramMap[*event.CoachingProgramId]; ok {
				enriched.CoachingProgram = program
			}
		}

		// Add type-specific details
		if details, ok := detailsMap[event.Id.Hex()]; ok {
			switch event.Type {
			case "coaching_session":
				if d, ok := details.(*models.CoachingSessionDetails); ok {
					enriched.CoachingSessionDetails = d
				}
			case "placement_test":
				if d, ok := details.(*models.PlacementTestDetails); ok {
					enriched.PlacementTestDetails = d
				}
			case "coaching_assignment_deadline", "course_assignment_deadline":
				if d, ok := details.(*models.AssignmentDetails); ok {
					enriched.AssignmentDetails = d
				}
			}
		}

		// Add availability slot details
		if event.Type == "availability_slot" {
			// Calculate session duration from the slot
			sessionDuration := int(event.EndTime.Sub(event.StartTime).Minutes())
			enriched.AvailableSlotDetails = &models.AvailableSlotDetails{
				SessionDuration: sessionDuration,
			}
		}

		enrichedEvents = append(enrichedEvents, enriched)
	}

	// Apply refined authorization
	enrichedEvents, err = c.ApplyAuthorization(ctx, enrichedEvents, req.Filters)
	if err != nil {
		return nil, err
	}

	// Sort by start time
	sortEventsByStartTime(enrichedEvents)

	// Apply pagination for agenda view
	var pagination *types.Pagination
	if viewType == "agenda" {
		total := int64(len(enrichedEvents))
		startIdx := (page - 1) * limit
		endIdx := startIdx + limit
		if startIdx > len(enrichedEvents) {
			enrichedEvents = []models.EnrichedCalendarEvent{}
		} else if endIdx > len(enrichedEvents) {
			enrichedEvents = enrichedEvents[startIdx:]
		} else {
			enrichedEvents = enrichedEvents[startIdx:endIdx]
		}

		totalPages := int64(math.Ceil(float64(total) / float64(limit)))
		pagination = &types.Pagination{
			TotalPages: float64(totalPages),
			PerPage:    int64(limit),
			TotalCount: total,
		}
	}

	return &models.CalendarEventsResponse{
		Events:     enrichedEvents,
		ViewType:   viewType,
		DateRange:  dateRange,
		Pagination: pagination,
	}, nil
}

// Helper function to check if slice contains string
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// Helper function to sort events by start time
func sortEventsByStartTime(events []models.EnrichedCalendarEvent) {
	sort.Slice(events, func(i, j int) bool {
		return events[i].StartTime.Before(events[j].StartTime)
	})
}

// CalendarEventsRequest is a type alias for the request model
type CalendarEventsRequest = models.CalendarEventsRequest

// GetCoachingSessions retrieves coaching sessions as calendar events
func (c *calendarsvcs) GetCoachingSessions(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error) {
	// Apply basic authorization
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user has required capability
	if !isAdminUser {
		hasSupervisorCap, err := util.HasCapability(ctx, CapabilityViewSupervisedCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasTeacherCap, err := util.HasCapability(ctx, CapabilityViewOwnCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasStudentCap, err := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasParentCap, err := util.HasCapability(ctx, CapabilityViewChildrenCalendarEvents)
		if err != nil {
			return nil, err
		}

		if !hasSupervisorCap && !hasTeacherCap && !hasStudentCap && !hasParentCap {
			return nil, ErrUnauthorizedInsufficientPermissions
		}

		if hasSupervisorCap {
			// Resolve managed teachers
			managedTeacherIds, err := c.getSupervisedTeacherIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}

			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = managedTeacherIds
			} else {
				// Verify supervisor only filters by managed teachers
				managedMap := make(map[primitive.ObjectID]bool)
				for _, tid := range managedTeacherIds {
					managedMap[tid] = true
				}
				for _, tid := range filters.TeacherIds {
					if !managedMap[tid] {
						return nil, ErrUnauthorizedOwnEventsOnly
					}
				}
			}
		} else if hasTeacherCap {
			// For non-admins, enforce teacher filter
			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify teacher can only filter by themselves
				if len(filters.TeacherIds) != 1 || filters.TeacherIds[0] != user.Id {
					return nil, ErrUnauthorizedOwnEventsOnly
				}
			}
		} else if hasStudentCap {
			// For students, enforce student filter
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify student can only filter by themselves
				if len(filters.StudentIds) != 1 || filters.StudentIds[0] != user.Id {
					return nil, ErrUnauthorizedInsufficientPermissions
				}
			}
		} else if hasParentCap {
			// For parents, get their children's IDs
			childrenIds, err := c.parentChecker.GetChildrenIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}
			if len(childrenIds) == 0 {
				return []models.CalendarEvent{}, nil
			}
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = childrenIds
			} else {
				// Verify parent can only filter by their children
				childrenMap := make(map[primitive.ObjectID]bool)
				for _, cid := range childrenIds {
					childrenMap[cid] = true
				}
				for _, sid := range filters.StudentIds {
					if !childrenMap[sid] {
						return nil, ErrUnauthorizedInsufficientPermissions
					}
				}
			}
		}
	}

	now := time.Now()

	// Build filter
	filter := bson.M{
		"trash": false,
		"date": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Filter by teacher
	if len(filters.TeacherIds) > 0 {
		filter["teacherId"] = bson.M{"$in": filters.TeacherIds}
	}

	// Filter by student
	if len(filters.StudentIds) > 0 {
		filter["studentId"] = bson.M{"$in": filters.StudentIds}
	}

	// Filter by coaching program
	if len(filters.CoachingProgramIds) > 0 {
		filter["coachingProgramId"] = bson.M{"$in": filters.CoachingProgramIds}
	}

	// Filter by status
	if len(filters.Statuses) > 0 {
		filter["status"] = bson.M{"$in": filters.Statuses}
	}

	// Query sessions
	pipeline := []bson.M{
		{"$match": filter},
	}

	var sessions []models.StudentSession
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &sessions)
	}); err != nil {
		return nil, err
	}

	// Collect unique coaching program IDs for batch fetching
	programIds := []primitive.ObjectID{}
	seenProgramIds := make(map[string]bool)
	for _, session := range sessions {
		idStr := session.CoachingProgramId.Hex()
		if !seenProgramIds[idStr] {
			programIds = append(programIds, session.CoachingProgramId)
			seenProgramIds[idStr] = true
		}
	}

	// Batch fetch coaching program names
	programNameMap, err := c.fetchCoachingProgramNames(ctx, programIds)
	if err != nil {
		// Log error but continue - we'll use fallback titles
		slog.Warn("Failed to fetch coaching program names", "error", err)
		programNameMap = make(map[primitive.ObjectID]string)
	}

	// Convert to calendar events
	events := []models.CalendarEvent{}
	for _, session := range sessions {
		// Combine date with from/to to create start/end times
		startTime, endTime, err := combineDateAndTime(session.Date, session.From, session.To)
		if err != nil {
			continue // Skip invalid time format
		}

		// Include events that haven't ended yet (allow 15 min buffer before start)
		if endTime.Before(now.Add(-15 * time.Minute)) {
			continue
		}

		// Generate title with i18n and program name
		programName := ""
		if name, ok := programNameMap[session.CoachingProgramId]; ok && name != "" {
			programName = name
		}

		var title string
		if programName != "" {
			// Format: "[Program Name] - [Translated: Coaching Session #X]"
			sessionLabel := getTranslatedEventLabel(ctx, "coaching_session", map[string]interface{}{
				"sessionNum": session.SessionNum,
			})
			title = fmt.Sprintf("%s - %s", programName, sessionLabel)
		} else {
			// Fallback: just the translated label without program name
			title = getTranslatedEventLabel(ctx, "coaching_session", map[string]interface{}{
				"sessionNum": session.SessionNum,
			})
		}

		event := models.CalendarEvent{
			Id:                session.Id,
			Type:              "coaching_session",
			Title:             title,
			StartTime:         startTime,
			EndTime:           endTime,
			TeacherId:         session.TeacherId,
			StudentId:         &session.StudentId,
			CoachingProgramId: &session.CoachingProgramId,
			Status:            string(session.Status),
		}

		events = append(events, event)
	}

	return events, nil
}

// GetCourseLiveSessions retrieves course live sessions as calendar events
func (c *calendarsvcs) GetCourseLiveSessions(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error) {
	// Apply basic authorization
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user has required capability
	if !isAdminUser {
		hasSupervisorCap, err := util.HasCapability(ctx, CapabilityViewSupervisedCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasTeacherCap, err := util.HasCapability(ctx, CapabilityViewOwnCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasStudentCap, err := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasParentCap, err := util.HasCapability(ctx, CapabilityViewChildrenCalendarEvents)
		if err != nil {
			return nil, err
		}

		if !hasSupervisorCap && !hasTeacherCap && !hasStudentCap && !hasParentCap {
			return nil, ErrUnauthorizedInsufficientPermissions
		}

		if hasSupervisorCap {
			// Resolve managed teachers
			managedTeacherIds, err := c.getSupervisedTeacherIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}

			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = managedTeacherIds
			} else {
				// Verify supervisor only filters by managed teachers
				managedMap := make(map[primitive.ObjectID]bool)
				for _, tid := range managedTeacherIds {
					managedMap[tid] = true
				}
				for _, tid := range filters.TeacherIds {
					if !managedMap[tid] {
						return nil, ErrUnauthorizedOwnEventsOnly
					}
				}
			}
		} else if hasTeacherCap {
			// For non-admins, enforce teacher filter
			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify teacher can only filter by themselves
				if len(filters.TeacherIds) != 1 || filters.TeacherIds[0] != user.Id {
					return nil, ErrUnauthorizedOwnEventsOnly
				}
			}
		} else if hasStudentCap {
			// For students, enforce student filter
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify student can only filter by themselves
				if len(filters.StudentIds) != 1 || filters.StudentIds[0] != user.Id {
					return nil, ErrUnauthorizedInsufficientPermissions
				}
			}
		} else if hasParentCap {
			// For parents, get their children's IDs
			childrenIds, err := c.parentChecker.GetChildrenIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}
			if len(childrenIds) == 0 {
				return []models.CalendarEvent{}, nil
			}
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = childrenIds
			} else {
				// Verify parent can only filter by their children
				childrenMap := make(map[primitive.ObjectID]bool)
				for _, cid := range childrenIds {
					childrenMap[cid] = true
				}
				for _, sid := range filters.StudentIds {
					if !childrenMap[sid] {
						return nil, ErrUnauthorizedInsufficientPermissions
					}
				}
			}
		}
	}

	now := time.Now()

	// Build filter: include all active live sessions whose base schedule is on or before the end of the range.
	// Recurring sessions will be expanded in code below.
	filter := bson.M{
		"trash":                       false,
		"settings.liveSession.active": true,
		"settings.liveSession.schedule": bson.M{
			"$lte": dateRange.End,
		},
	}

	// Filter by course
	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	hasParentCap, _ := util.HasCapability(ctx, CapabilityViewChildrenCalendarEvents)
	if !isAdminUser && (hasStudentCap || hasParentCap) {
		var studentIds []primitive.ObjectID
		if hasStudentCap {
			studentIds = []primitive.ObjectID{user.Id}
		} else if hasParentCap {
			var err error
			studentIds, err = c.parentChecker.GetChildrenIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}
			if len(studentIds) == 0 {
				return []models.CalendarEvent{}, nil
			}
		}

		enrollmentFilter := bson.M{
			"studentId": bson.M{"$in": studentIds},
			"status":    "active",
			"trash":     false,
		}
		var enrollments []models.StudentCourse
		if err := c.studentCourseRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		}); err == nil && len(enrollments) > 0 {
			enrolledCourseIds := make([]primitive.ObjectID, len(enrollments))
			for i, e := range enrollments {
				enrolledCourseIds[i] = e.CourseId
			}

			if len(filters.CourseIds) > 0 {
				// Intersect provided CourseIds with enrolled courses
				intersectedIds := []primitive.ObjectID{}
				enrolledMap := make(map[primitive.ObjectID]bool)
				for _, id := range enrolledCourseIds {
					enrolledMap[id] = true
				}
				for _, id := range filters.CourseIds {
					if enrolledMap[id] {
						intersectedIds = append(intersectedIds, id)
					}
				}
				if len(intersectedIds) == 0 {
					return []models.CalendarEvent{}, nil
				}
				filter["_id"] = bson.M{"$in": intersectedIds}
			} else {
				filter["_id"] = bson.M{"$in": enrolledCourseIds}
			}
		} else {
			// No enrollments, no courses
			return []models.CalendarEvent{}, nil
		}
	} else if len(filters.CourseIds) > 0 {
		filter["_id"] = bson.M{"$in": filters.CourseIds}
	}

	// Filter by teacher using assignment service
	if len(filters.TeacherIds) > 0 {
		assignedMap := make(map[primitive.ObjectID]bool)
		for _, tid := range filters.TeacherIds {
			ids, err := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, tid)
			if err == nil {
				for _, id := range ids {
					assignedMap[id] = true
				}
			}
		}

		allAssignedCourseIds := make([]primitive.ObjectID, 0, len(assignedMap))
		for id := range assignedMap {
			allAssignedCourseIds = append(allAssignedCourseIds, id)
		}

		if len(allAssignedCourseIds) == 0 {
			return []models.CalendarEvent{}, nil
		}

		// Add to filter
		if existingIds, ok := filter["_id"].(bson.M); ok {
			// If already filtering by CourseIds (e.g. from student enrollment logic), intersect them
			if inClause, ok := existingIds["$in"].([]primitive.ObjectID); ok {
				intersected := []primitive.ObjectID{}
				for _, id := range inClause {
					if assignedMap[id] {
						intersected = append(intersected, id)
					}
				}
				if len(intersected) == 0 {
					return []models.CalendarEvent{}, nil
				}
				filter["_id"] = bson.M{"$in": intersected}
			}
		} else {
			filter["_id"] = bson.M{"$in": allAssignedCourseIds}
		}
	}

	// Query courses
	pipeline := []bson.M{
		{"$match": filter},
	}

	var courses []models.Course
	if err := c.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	}); err != nil {
		return nil, err
	}

	// Convert to calendar events
	events := []models.CalendarEvent{}

	// Batch fetch teachers for all courses to avoid N+1 in loop
	courseIds := make([]primitive.ObjectID, len(courses))
	for i, course := range courses {
		courseIds[i] = course.Id
	}
	allTeacherAssignments, err := c.teacherAssignmentSvc.BatchGetCourseTeachers(ctx, courseIds)
	if err != nil {
		// Log error but continue (will result in missing teacher info)
		slog.Warn("Failed to batch fetch course teachers", "error", err)
		allTeacherAssignments = make(map[primitive.ObjectID][]primitive.ObjectID)
	}
	for _, course := range courses {
		schedule := course.Settings.LiveSession.Schedule
		repeatWeekly := course.Settings.LiveSession.RepeatWeekly

		// Calculate session duration
		duration := course.BasicInfo.Duration
		if duration <= 0 {
			duration = DefaultCourseDurationMinutes
		}

		// Expand to one or more occurrences within the requested date range
		occurrences := getCourseLiveSessionOccurrences(schedule, repeatWeekly, dateRange)
		if len(occurrences) == 0 {
			continue
		}

		// Get first teacher for TeacherId from assignment service
		var teacherId primitive.ObjectID
		if teachers, ok := allTeacherAssignments[course.Id]; ok && len(teachers) > 0 {
			teacherId = teachers[0]
		}

		// Generate title
		title := course.Settings.LiveSession.Title.GetContentByContext(ctx)
		if title == "" {
			title = course.BasicInfo.Name.GetContentByContext(ctx)
		}
		if title == "" {
			title = "Course Live Session"
		}

		for _, startTime := range occurrences {
			endTime := startTime.Add(time.Duration(duration) * time.Minute)

			// Include events that haven't ended yet (allow 15 min buffer before start)
			if endTime.Before(now.Add(-15 * time.Minute)) {
				continue
			}

			event := models.CalendarEvent{
				Id:        course.Id,
				Type:      "course_live_session",
				Title:     title,
				StartTime: startTime,
				EndTime:   endTime,
				TeacherId: teacherId,
				CourseId:  &course.Id,
				Status:    "active",
			}

			events = append(events, event)
		}
	}

	return events, nil
}

// getCourseLiveSessionOccurrences expands a base schedule into one or more occurrences
// within the provided date range. If repeatWeekly is false, at most one occurrence is returned.
func getCourseLiveSessionOccurrences(schedule time.Time, repeatWeekly bool, dateRange models.DateRange) []time.Time {
	occurrences := []time.Time{}

	if schedule.IsZero() {
		return occurrences
	}

	// Non-recurring: single occurrence if it falls inside the range.
	if !repeatWeekly {
		if schedule.Before(dateRange.Start) || schedule.After(dateRange.End) {
			return occurrences
		}
		return append(occurrences, schedule)
	}

	// Recurring weekly: start from the first occurrence on or after the range start.
	first := schedule
	for first.Before(dateRange.Start) {
		first = first.AddDate(0, 0, 7)
	}
	if first.After(dateRange.End) {
		return occurrences
	}

	for current := first; !current.After(dateRange.End); current = current.AddDate(0, 0, 7) {
		occurrences = append(occurrences, current)
	}

	return occurrences
}

// GetPlacementTests retrieves placement test schedules as calendar events
func (c *calendarsvcs) GetPlacementTests(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error) {
	// Apply basic authorization
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user has required capability
	if !isAdminUser {
		hasSupervisorCap, err := util.HasCapability(ctx, CapabilityViewSupervisedCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasTeacherCap, err := util.HasCapability(ctx, CapabilityViewOwnCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasStudentCap, err := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasParentCap, err := util.HasCapability(ctx, CapabilityViewChildrenCalendarEvents)
		if err != nil {
			return nil, err
		}

		if !hasSupervisorCap && !hasTeacherCap && !hasStudentCap && !hasParentCap {
			return nil, ErrUnauthorizedInsufficientPermissions
		}

		if hasSupervisorCap {
			// Resolve managed teachers
			managedTeacherIds, err := c.getSupervisedTeacherIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}

			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = managedTeacherIds
			} else {
				// Verify supervisor only filters by managed teachers
				managedMap := make(map[primitive.ObjectID]bool)
				for _, tid := range managedTeacherIds {
					managedMap[tid] = true
				}
				for _, tid := range filters.TeacherIds {
					if !managedMap[tid] {
						return nil, ErrUnauthorizedOwnEventsOnly
					}
				}
			}
		} else if hasTeacherCap {
			// For non-admins, enforce teacher filter
			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify teacher can only filter by themselves
				if len(filters.TeacherIds) != 1 || filters.TeacherIds[0] != user.Id {
					return nil, ErrUnauthorizedOwnEventsOnly
				}
			}
		} else if hasStudentCap {
			// For students, enforce student filter
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify student can only filter by themselves
				if len(filters.StudentIds) != 1 || filters.StudentIds[0] != user.Id {
					return nil, ErrUnauthorizedInsufficientPermissions
				}
			}
		} else if hasParentCap {
			// For parents, get their children's IDs
			childrenIds, err := c.parentChecker.GetChildrenIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}
			if len(childrenIds) == 0 {
				return []models.CalendarEvent{}, nil
			}
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = childrenIds
			} else {
				// Verify parent can only filter by their children
				childrenMap := make(map[primitive.ObjectID]bool)
				for _, cid := range childrenIds {
					childrenMap[cid] = true
				}
				for _, sid := range filters.StudentIds {
					if !childrenMap[sid] {
						return nil, ErrUnauthorizedInsufficientPermissions
					}
				}
			}
		}
	}

	now := time.Now()

	// Build filter
	filter := bson.M{
		"date": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
		// Only show approved/scheduled/in-progress placement tests; hide pending/rejected
		"status": bson.M{
			"$in": []enums.StudentPlacementTestStatus{
				enums.StudentPlacementTestStatusScheduled,
				enums.StudentPlacementTestStatusInProgress,
				enums.StudentPlacementTestStatusCompleted,
				enums.StudentPlacementTestStatusEvaluated,
			},
		},
	}

	// Filter by teacher
	if len(filters.TeacherIds) > 0 {
		filter["teacherId"] = bson.M{"$in": filters.TeacherIds}
	}

	// Filter by student
	if len(filters.StudentIds) > 0 {
		filter["studentId"] = bson.M{"$in": filters.StudentIds}
	}

	// Filter by coaching program
	if len(filters.CoachingProgramIds) > 0 {
		filter["coachingProgramId"] = bson.M{"$in": filters.CoachingProgramIds}
	}

	// Query schedules
	pipeline := []bson.M{
		{"$match": filter},
	}

	var schedules []models.OnlinePlacementTestSchedule
	if err := c.placementTestScheduleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &schedules)
	}); err != nil {
		return nil, err
	}

	// Collect unique coaching program IDs for batch fetching
	programIds := []primitive.ObjectID{}
	seenProgramIds := make(map[string]bool)
	for _, schedule := range schedules {
		idStr := schedule.CoachingProgramId.Hex()
		if !seenProgramIds[idStr] {
			programIds = append(programIds, schedule.CoachingProgramId)
			seenProgramIds[idStr] = true
		}
	}

	// Batch fetch coaching program names
	programNameMap, err := c.fetchCoachingProgramNames(ctx, programIds)
	if err != nil {
		// Log error but continue - we'll use fallback titles
		slog.Warn("Failed to fetch coaching program names", "error", err)
		programNameMap = make(map[primitive.ObjectID]string)
	}

	// Convert to calendar events
	events := []models.CalendarEvent{}
	for _, schedule := range schedules {
		// Combine date with from/to to create start/end times
		startTime, endTime, err := combineDateAndTime(schedule.Date, schedule.From, schedule.To)
		if err != nil {
			continue // Skip invalid time format
		}

		// Include events that haven't ended yet (allow 15 min buffer before start)
		if endTime.Before(now.Add(-15 * time.Minute)) {
			continue
		}

		// Generate title with i18n and program name
		programName := ""
		if name, ok := programNameMap[schedule.CoachingProgramId]; ok && name != "" {
			programName = name
		}

		var title string
		if programName != "" {
			// Format: "[Translated: Placement Test] - [Program Name]"
			placementTestLabel := getTranslatedEventLabel(ctx, "placement_test", nil)
			title = fmt.Sprintf("%s - %s", placementTestLabel, programName)
		} else {
			// Fallback: just the translated label
			title = getTranslatedEventLabel(ctx, "placement_test", nil)
		}

		teacherId := primitive.NilObjectID
		if schedule.TeacherId.Value != nil {
			teacherId = *schedule.TeacherId.Value
		}

		event := models.CalendarEvent{
			Id:                schedule.Id,
			Type:              "placement_test",
			Title:             title,
			StartTime:         startTime,
			EndTime:           endTime,
			TeacherId:         teacherId,
			StudentId:         &schedule.StudentId,
			CoachingProgramId: &schedule.CoachingProgramId,
			Status:            string(schedule.Status),
		}

		events = append(events, event)
	}

	return events, nil
}

// GetCoachingAssignmentDeadlines retrieves coaching assignment deadlines as calendar events
func (c *calendarsvcs) GetCoachingAssignmentDeadlines(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error) {
	// Only students can see their own assignment deadlines
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	isAdminUser, _ := isAdmin(ctx)

	// If not admin and not student, return empty (teachers/supervisors don't see assignments in calendar)
	if !isAdminUser && !hasStudentCap {
		return []models.CalendarEvent{}, nil
	}

	// Determine whose assignments we are looking for
	var studentId primitive.ObjectID
	if hasStudentCap {
		studentId = user.Id
	} else if isAdminUser && len(filters.StudentIds) == 1 {
		studentId = filters.StudentIds[0]
	} else {
		// Admins viewing all or multiple students don't see assignments by default to avoid clutter
		// or if it's a general request without a specific student focus
		return []models.CalendarEvent{}, nil
	}

	// 1. Get coaching programs the student is enrolled in
	enrollmentFilter := bson.M{
		"studentId": studentId,
		"status":    "active",
		"trash":     false,
	}
	var enrollments []models.CoachingStudent
	if err := c.coachingStudentRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	}); err != nil {
		return nil, err
	}

	if len(enrollments) == 0 {
		return []models.CalendarEvent{}, nil
	}

	enrolledProgramIds := make([]primitive.ObjectID, len(enrollments))
	for i, e := range enrollments {
		enrolledProgramIds[i] = e.CoachingId
	}

	// 2. Build assignment filter
	assignmentFilter := bson.M{
		"coachingId": bson.M{"$in": enrolledProgramIds},
		"status":     "active",
		"trash":      false,
		"dueDate": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Apply coaching program filter if provided
	if len(filters.CoachingProgramIds) > 0 {
		// Intersect
		intersected := []primitive.ObjectID{}
		filterMap := make(map[primitive.ObjectID]bool)
		for _, id := range filters.CoachingProgramIds {
			filterMap[id] = true
		}
		for _, id := range enrolledProgramIds {
			if filterMap[id] {
				intersected = append(intersected, id)
			}
		}
		if len(intersected) == 0 {
			return []models.CalendarEvent{}, nil
		}
		assignmentFilter["coachingId"] = bson.M{"$in": intersected}
	}

	var assignments []models.CoachingAssignment
	if err := c.coachingAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": assignmentFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	// 3. Convert to calendar events
	events := []models.CalendarEvent{}
	for _, a := range assignments {
		title := getTranslatedEventLabel(ctx, "coaching_assignment_deadline", map[string]interface{}{
			"title": a.Title.GetContentByContext(ctx),
		})

		event := models.CalendarEvent{
			Id:                a.Id,
			Type:              "coaching_assignment_deadline",
			Title:             title,
			StartTime:         a.DueDate,
			EndTime:           a.DueDate.Add(30 * time.Minute), // Show as a short point in time
			CoachingProgramId: &a.CoachingId,
			StudentId:         &studentId,
			Status:            a.Status,
		}
		events = append(events, event)
	}

	return events, nil
}

// GetCourseAssignmentDeadlines retrieves course assignment deadlines as calendar events
func (c *calendarsvcs) GetCourseAssignmentDeadlines(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) ([]models.CalendarEvent, error) {
	// Only students can see their own assignment deadlines
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	isAdminUser, _ := isAdmin(ctx)

	// If not admin and not student, return empty
	if !isAdminUser && !hasStudentCap {
		return []models.CalendarEvent{}, nil
	}

	// Determine whose assignments we are looking for
	var studentId primitive.ObjectID
	if hasStudentCap {
		studentId = user.Id
	} else if isAdminUser && len(filters.StudentIds) == 1 {
		studentId = filters.StudentIds[0]
	} else {
		return []models.CalendarEvent{}, nil
	}

	// 1. Get courses the student is enrolled in
	enrollmentFilter := bson.M{
		"studentId": studentId,
		"status":    "active",
		"trash":     false,
	}
	var enrollments []models.StudentCourse
	if err := c.studentCourseRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	}); err != nil {
		return nil, err
	}

	if len(enrollments) == 0 {
		return []models.CalendarEvent{}, nil
	}

	enrolledCourseIds := make([]primitive.ObjectID, len(enrollments))
	for i, e := range enrollments {
		enrolledCourseIds[i] = e.CourseId
	}

	// 2. Build assignment filter
	assignmentFilter := bson.M{
		"courseId": bson.M{"$in": enrolledCourseIds},
		"status":   "active",
		"trash":    false,
		"deadline": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Apply course filter if provided
	if len(filters.CourseIds) > 0 {
		intersected := []primitive.ObjectID{}
		filterMap := make(map[primitive.ObjectID]bool)
		for _, id := range filters.CourseIds {
			filterMap[id] = true
		}
		for _, id := range enrolledCourseIds {
			if filterMap[id] {
				intersected = append(intersected, id)
			}
		}
		if len(intersected) == 0 {
			return []models.CalendarEvent{}, nil
		}
		assignmentFilter["courseId"] = bson.M{"$in": intersected}
	}

	var assignments []models.CourseAssignment
	if err := c.courseAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": assignmentFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	// 3. Convert to calendar events
	events := []models.CalendarEvent{}
	for _, a := range assignments {
		title := getTranslatedEventLabel(ctx, "course_assignment_deadline", map[string]interface{}{
			"title": a.Title.GetContentByContext(ctx),
		})

		event := models.CalendarEvent{
			Id:        a.Id,
			Type:      "course_assignment_deadline",
			Title:     title,
			StartTime: a.Deadline,
			EndTime:   a.Deadline.Add(30 * time.Minute),
			CourseId:  &a.CourseId,
			StudentId: &studentId,
			Status:    a.Status,
		}
		events = append(events, event)
	}

	return events, nil
}

// combineDateAndTime combines a date (time.Time) with time strings (HH:MM) to create start and end times
func combineDateAndTime(date time.Time, fromStr, toStr string) (time.Time, time.Time, error) {
	// Parse from time
	fromParts := strings.Split(fromStr, ":")
	if len(fromParts) != 2 {
		return time.Time{}, time.Time{}, errors.New("invalid from time format")
	}
	fromHour := parseInt(fromParts[0])
	fromMin := parseInt(fromParts[1])

	// Parse to time
	toParts := strings.Split(toStr, ":")
	if len(toParts) != 2 {
		return time.Time{}, time.Time{}, errors.New("invalid to time format")
	}
	toHour := parseInt(toParts[0])
	toMin := parseInt(toParts[1])

	// Extract date components
	year, month, day := date.Date()

	// Create start and end times in UTC
	startTime := time.Date(year, month, day, fromHour, fromMin, 0, 0, time.UTC)
	endTime := time.Date(year, month, day, toHour, toMin, 0, 0, time.UTC)

	return startTime, endTime, nil
}

// parseInt safely parses an integer string
func parseInt(s string) int {
	var result int
	fmt.Sscanf(s, "%d", &result)
	return result
}

// eventLabelTranslations contains translations for calendar event type labels
var eventLabelTranslations = map[string]map[string]string{
	"placement_test": {
		"en": "Placement Test",
		"ar": "اختبار تحديد المستوى",
	},
	"coaching_session": {
		"en": "Coaching Session #%d",
		"ar": "جلسة تدريبية #%d",
	},
	"coaching_assignment_deadline": {
		"en": "Assignment Deadline: %s",
		"ar": "موعد تسليم الواجب: %s",
	},
	"course_assignment_deadline": {
		"en": "Assignment Deadline: %s",
		"ar": "موعد تسليم الواجب: %s",
	},
}

// getTranslatedEventLabel returns a translated label for calendar event types
// eventType: "placement_test", "coaching_session", "coaching_assignment_deadline", or "course_assignment_deadline"
// params: map with optional "sessionNum" (int) or "title" (string) key
// Returns: translated string with placeholders filled if applicable
func getTranslatedEventLabel(ctx context.Context, eventType string, params map[string]interface{}) string {
	// Get language from context
	cfg, err := util.GetReqAppCfg(ctx)
	lang := FallbackLanguage // default to "en"
	if err == nil && cfg.Lang != "" {
		lang = cfg.Lang
	}

	// Get translation map for event type
	langMap, exists := eventLabelTranslations[eventType]
	if !exists {
		// Fallback to English if event type not found
		if enMap, ok := eventLabelTranslations["placement_test"]; ok {
			langMap = enMap
		} else {
			return "" // Should not happen, but handle gracefully
		}
	}

	// Get translation for the language (fallback to English)
	translated, exists := langMap[lang]
	if !exists {
		translated = langMap[FallbackLanguage]
	}

	// Replace placeholders if any
	if params != nil {
		if sessionNum, ok := params["sessionNum"].(int); ok {
			return fmt.Sprintf(translated, sessionNum)
		}
		if title, ok := params["title"].(string); ok {
			return fmt.Sprintf(translated, title)
		}
	}

	return translated
}

// fetchCoachingProgramNames batch fetches coaching program names for given IDs
// Returns: map[programId] -> programName (localized)
// This is optimized to fetch all programs in a single query
func (c *calendarsvcs) fetchCoachingProgramNames(ctx context.Context, programIds []primitive.ObjectID) (map[primitive.ObjectID]string, error) {
	programNameMap := make(map[primitive.ObjectID]string)

	if len(programIds) == 0 {
		return programNameMap, nil
	}

	// Query coaching programs
	filter := bson.M{
		"_id":   bson.M{"$in": programIds},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var programs []models.Coaching
	if err := c.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &programs)
	}); err != nil {
		return nil, err
	}

	// Create map of program ID to localized name
	for _, program := range programs {
		name := program.Title.GetContentByContext(ctx)
		// If name is empty, we'll use fallback in title generation
		programNameMap[program.Id] = name
	}

	return programNameMap, nil
}

// EnrichWithBundleInfo determines bundle association for each event
func (c *calendarsvcs) EnrichWithBundleInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventBundle, error) {
	bundleMap := make(map[primitive.ObjectID]*models.EventBundle)

	// Collect all course and coaching program IDs
	courseIds := []primitive.ObjectID{}
	coachingProgramIds := []primitive.ObjectID{}

	for _, event := range events {
		if event.CourseId != nil {
			courseIds = append(courseIds, *event.CourseId)
		}
		if event.CoachingProgramId != nil {
			coachingProgramIds = append(coachingProgramIds, *event.CoachingProgramId)
		}
	}

	// Query bundles that contain these courses or coaching programs
	if len(courseIds) == 0 && len(coachingProgramIds) == 0 {
		return bundleMap, nil
	}

	filter := bson.M{
		"trash": false,
		"$or": []bson.M{
			{
				"content.type":  "course",
				"content.refId": bson.M{"$in": courseIds},
			},
			{
				"content.type":  "coaching",
				"content.refId": bson.M{"$in": coachingProgramIds},
			},
		},
	}

	pipeline := []bson.M{
		{"$match": filter},
		{"$unwind": "$content"},
		{"$match": bson.M{
			"$or": []bson.M{
				{
					"content.type":  "course",
					"content.refId": bson.M{"$in": courseIds},
				},
				{
					"content.type":  "coaching",
					"content.refId": bson.M{"$in": coachingProgramIds},
				},
			},
		}},
		{"$group": bson.M{
			"_id":     "$_id",
			"name":    bson.M{"$first": "$name"},
			"content": bson.M{"$push": "$content"},
		}},
	}

	var bundles []models.Bundle
	if err := c.bundleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &bundles)
	}); err != nil {
		return nil, err
	}

	// Create a map of refId -> bundle
	refToBundle := make(map[primitive.ObjectID]*models.Bundle)
	for i := range bundles {
		bundle := &bundles[i]
		// Get the bundle name (localized)
		bundleName := bundle.Name.GetContentByContext(ctx)
		if bundleName == "" {
			bundleName = "Bundle"
		}

		// Map each content refId to this bundle
		for _, content := range bundle.Content {
			if content.Type == "course" || content.Type == "coaching" {
				refToBundle[content.RefId] = bundle
			}
		}

		// Store bundle info
		bundleMap[bundle.Id] = &models.EventBundle{
			Id:   bundle.Id,
			Name: bundleName,
		}
	}

	// Update events with bundle IDs
	for i := range events {
		event := &events[i]
		if event.CourseId != nil {
			if bundle, ok := refToBundle[*event.CourseId]; ok {
				event.BundleId = &bundle.Id
			}
		} else if event.CoachingProgramId != nil {
			if bundle, ok := refToBundle[*event.CoachingProgramId]; ok {
				event.BundleId = &bundle.Id
			}
		}
	}

	return bundleMap, nil
}

// EnrichWithStudentInfo batch lookup student names
func (c *calendarsvcs) EnrichWithStudentInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventStudent, error) {
	studentMap := make(map[primitive.ObjectID]*models.EventStudent)

	// Collect unique student IDs
	studentIds := []primitive.ObjectID{}
	seen := make(map[string]bool)
	for _, event := range events {
		if event.StudentId != nil {
			idStr := event.StudentId.Hex()
			if !seen[idStr] {
				studentIds = append(studentIds, *event.StudentId)
				seen[idStr] = true
			}
		}
	}

	if len(studentIds) == 0 {
		return studentMap, nil
	}

	// Query students
	filter := bson.M{
		"_id":   bson.M{"$in": studentIds},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var students []contactmodels.Student
	if err := c.studentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &students)
	}); err != nil {
		return nil, err
	}

	// Create map
	for _, student := range students {
		firstName := student.GeneralInfo.FirstName.GetContentByContext(ctx)
		lastName := student.GeneralInfo.LastName.GetContentByContext(ctx)
		name := strings.TrimSpace(firstName + " " + lastName)
		if name == "" {
			name = "Student"
		}

		studentMap[student.Id] = &models.EventStudent{
			Id:   student.Id,
			Name: name,
		}
	}

	return studentMap, nil
}

// EnrichWithTeacherInfo batch lookup teacher names
func (c *calendarsvcs) EnrichWithTeacherInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventTeacher, error) {
	teacherMap := make(map[primitive.ObjectID]*models.EventTeacher)

	// Collect unique teacher IDs
	teacherIds := []primitive.ObjectID{}
	seen := make(map[string]bool)
	for _, event := range events {
		idStr := event.TeacherId.Hex()
		if !seen[idStr] {
			teacherIds = append(teacherIds, event.TeacherId)
			seen[idStr] = true
		}
	}

	if len(teacherIds) == 0 {
		return teacherMap, nil
	}

	// Query teachers
	filter := bson.M{
		"_id":   bson.M{"$in": teacherIds},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var teachers []contactmodels.Teacher
	if err := c.teacherRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &teachers)
	}); err != nil {
		return nil, err
	}

	// Create map - merge firstName and lastName preserving Localizable structure
	for _, teacher := range teachers {
		name := mergeLocalizableNames(teacher.FirstName, teacher.LastName)
		// Ensure at least one language exists
		if len(name) == 0 {
			name = transl.Localizable[string]{"en": "Teacher"}
		}

		teacherMap[teacher.Id] = &models.EventTeacher{
			Id:   teacher.Id,
			Name: name,
		}
	}

	return teacherMap, nil
}

// EnrichWithCourseInfo batch lookup course names and enrollment counts
func (c *calendarsvcs) EnrichWithCourseInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventCourse, error) {
	courseMap := make(map[primitive.ObjectID]*models.EventCourse)

	// Collect unique course IDs
	courseIds := []primitive.ObjectID{}
	seen := make(map[string]bool)
	for _, event := range events {
		if event.CourseId != nil {
			idStr := event.CourseId.Hex()
			if !seen[idStr] {
				courseIds = append(courseIds, *event.CourseId)
				seen[idStr] = true
			}
		}
	}

	if len(courseIds) == 0 {
		return courseMap, nil
	}

	// Query courses
	filter := bson.M{
		"_id":   bson.M{"$in": courseIds},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var courses []models.Course
	if err := c.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	}); err != nil {
		return nil, err
	}

	// Get teacher assignments for all courses in batch
	// Reuse courseIds from above (already contains the course IDs we need)
	teacherAssignments, err := c.teacherAssignmentSvc.BatchGetCourseTeachers(ctx, courseIds)
	if err != nil {
		return nil, err
	}

	// Collect all unique teacher IDs from all courses for batch lookup
	allTeacherIdsMap := make(map[string]primitive.ObjectID)
	for _, course := range courses {
		teachers := teacherAssignments[course.Id]
		for _, teacherId := range teachers {
			allTeacherIdsMap[teacherId.Hex()] = teacherId
		}
	}

	// Convert to slice for batch query
	allTeacherIds := make([]primitive.ObjectID, 0, len(allTeacherIdsMap))
	for _, teacherId := range allTeacherIdsMap {
		allTeacherIds = append(allTeacherIds, teacherId)
	}

	// Batch query all teachers in one query
	teacherMap := make(map[primitive.ObjectID]contactmodels.Teacher)
	if len(allTeacherIds) > 0 {
		teacherFilter := bson.M{
			"_id":   bson.M{"$in": allTeacherIds},
			"trash": false,
		}
		teacherPipeline := []bson.M{
			{"$match": teacherFilter},
		}
		var allTeachers []contactmodels.Teacher
		if err := c.teacherRepo.Aggregate(ctx, teacherPipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &allTeachers)
		}); err == nil {
			for _, t := range allTeachers {
				teacherMap[t.Id] = t
			}
		}
	}

	// Batch count enrollments for all courses
	enrollmentCounts := make(map[primitive.ObjectID]int)
	if len(courseIds) > 0 {
		pipeline := []bson.M{
			{"$match": bson.M{
				"courseId": bson.M{"$in": courseIds},
				"status":   "active",
				"trash":    false,
			}},
			{"$group": bson.M{
				"_id":   "$courseId",
				"count": bson.M{"$sum": 1},
			}},
		}
		var results []struct {
			ID    primitive.ObjectID `bson:"_id"`
			Count int                `bson:"count"`
		}
		if err := c.studentCourseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &results)
		}); err == nil {
			for _, res := range results {
				enrollmentCounts[res.ID] = res.Count
			}
		}
	}

	// Combine info for each course
	for _, course := range courses {
		// Get enrollment count from pre-fetched map
		enrollmentCount := enrollmentCounts[course.Id]

		// Get course name
		courseName := course.BasicInfo.Name.GetContentByContext(ctx)
		if courseName == "" {
			courseName = "Course"
		}

		// Get all teachers for this course from the assignment service
		teachers := []models.EventTeacher{}
		courseTeachers := teacherAssignments[course.Id]
		for _, teacherId := range courseTeachers {
			if t, ok := teacherMap[teacherId]; ok {
				name := mergeLocalizableNames(t.FirstName, t.LastName)
				// Ensure at least one language exists
				if len(name) == 0 {
					name = transl.Localizable[string]{"en": "Teacher"}
				}
				teachers = append(teachers, models.EventTeacher{
					Id:   t.Id,
					Name: name,
				})
			}
		}

		courseMap[course.Id] = &models.EventCourse{
			Id:              course.Id,
			Name:            courseName,
			EnrollmentCount: int(enrollmentCount),
			Teachers:        teachers,
		}
	}

	return courseMap, nil
}

// EnrichWithCoachingProgramInfo batch lookup coaching program names
func (c *calendarsvcs) EnrichWithCoachingProgramInfo(ctx context.Context, events []models.CalendarEvent) (map[primitive.ObjectID]*models.EventCoachingProgram, error) {
	programMap := make(map[primitive.ObjectID]*models.EventCoachingProgram)

	// Collect unique coaching program IDs
	programIds := []primitive.ObjectID{}
	seen := make(map[string]bool)
	for _, event := range events {
		if event.CoachingProgramId != nil {
			idStr := event.CoachingProgramId.Hex()
			if !seen[idStr] {
				programIds = append(programIds, *event.CoachingProgramId)
				seen[idStr] = true
			}
		}
	}

	if len(programIds) == 0 {
		return programMap, nil
	}

	// Query coaching programs
	filter := bson.M{
		"_id":   bson.M{"$in": programIds},
		"trash": false,
	}

	pipeline := []bson.M{
		{"$match": filter},
	}

	var programs []models.Coaching
	if err := c.coachingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &programs)
	}); err != nil {
		return nil, err
	}

	// Create map
	for _, program := range programs {
		name := program.Title.GetContentByContext(ctx)
		if name == "" {
			name = "Coaching Program"
		}

		programMap[program.Id] = &models.EventCoachingProgram{
			Id:   program.Id,
			Name: name,
		}
	}

	return programMap, nil
}

// EnrichWithTypeSpecificDetails extracts type-specific details
func (c *calendarsvcs) EnrichWithTypeSpecificDetails(ctx context.Context, events []models.CalendarEvent) map[string]interface{} {
	detailsMap := make(map[string]interface{})

	// Collect coaching session IDs and placement test schedule IDs
	coachingSessionIds := []primitive.ObjectID{}
	placementScheduleIds := []primitive.ObjectID{}
	coachingAssignmentIds := []primitive.ObjectID{}
	courseAssignmentIds := []primitive.ObjectID{}

	for _, event := range events {
		if event.Type == "coaching_session" {
			coachingSessionIds = append(coachingSessionIds, event.Id)
		} else if event.Type == "placement_test" {
			placementScheduleIds = append(placementScheduleIds, event.Id)
		} else if event.Type == "coaching_assignment_deadline" {
			coachingAssignmentIds = append(coachingAssignmentIds, event.Id)
		} else if event.Type == "course_assignment_deadline" {
			courseAssignmentIds = append(courseAssignmentIds, event.Id)
		}
	}

	// Query coaching assignments details and submissions
	if len(coachingAssignmentIds) > 0 {
		var assignments []models.CoachingAssignment
		if err := c.coachingAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": coachingAssignmentIds}}}}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &assignments)
		}); err == nil {
			for _, a := range assignments {
				detailsMap[a.Id.Hex()] = &models.AssignmentDetails{
					Id:     a.Id,
					Title:  a.Title.GetContentByContext(ctx),
					Status: a.Status,
				}
			}
		}

		studentIds := []primitive.ObjectID{}
		for _, e := range events {
			if e.Type == "coaching_assignment_deadline" && e.StudentId != nil {
				studentIds = append(studentIds, *e.StudentId)
			}
		}
		if len(studentIds) > 0 {
			var submissions []models.CoachingStudentAssignment
			filter := bson.M{
				"assignmentId": bson.M{"$in": coachingAssignmentIds},
				"studentId":    bson.M{"$in": studentIds},
				"trash":        false,
			}
			if err := c.coachingStudentAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &submissions)
			}); err == nil {
				subMap := make(map[string]bool)
				for _, s := range submissions {
					subMap[s.AssignmentId.Hex()+s.StudentId.Hex()] = true
				}

				for _, e := range events {
					if e.Type == "coaching_assignment_deadline" && e.StudentId != nil {
						if details, ok := detailsMap[e.Id.Hex()].(*models.AssignmentDetails); ok {
							if subMap[e.Id.Hex()+e.StudentId.Hex()] {
								details.IsSubmitted = true
							}
						}
					}
				}
			}
		}
	}

	// Query course assignments details and submissions
	if len(courseAssignmentIds) > 0 {
		var assignments []models.CourseAssignment
		if err := c.courseAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": bson.M{"_id": bson.M{"$in": courseAssignmentIds}}}}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &assignments)
		}); err == nil {
			for _, a := range assignments {
				detailsMap[a.Id.Hex()] = &models.AssignmentDetails{
					Id:          a.Id,
					Title:       a.Title.GetContentByContext(ctx),
					Description: a.Description.GetContentByContext(ctx),
					Status:      a.Status,
				}
			}
		}

		studentIds := []primitive.ObjectID{}
		for _, e := range events {
			if e.Type == "course_assignment_deadline" && e.StudentId != nil {
				studentIds = append(studentIds, *e.StudentId)
			}
		}
		if len(studentIds) > 0 {
			var submissions []models.CourseStudentAssignment
			filter := bson.M{
				"assignmentId": bson.M{"$in": courseAssignmentIds},
				"studentId":    bson.M{"$in": studentIds},
				"trash":        false,
			}
			if err := c.courseStudentAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &submissions)
			}); err == nil {
				subMap := make(map[string]bool)
				for _, s := range submissions {
					subMap[s.AssignmentId.Hex()+s.StudentId.Hex()] = true
				}

				for _, e := range events {
					if e.Type == "course_assignment_deadline" && e.StudentId != nil {
						if details, ok := detailsMap[e.Id.Hex()].(*models.AssignmentDetails); ok {
							if subMap[e.Id.Hex()+e.StudentId.Hex()] {
								details.IsSubmitted = true
							}
						}
					}
				}
			}
		}
	}

	// Used to track which Zoom meeting IDs we need to fetch
	zoomMeetingIdsToFetch := []primitive.ObjectID{}
	zoomMeetingIdMap := make(map[primitive.ObjectID]primitive.ObjectID) // eventId -> zoomMeetingId

	// Query coaching sessions to get sessionNum and ZoomMeetingId
	if len(coachingSessionIds) > 0 {
		filter := bson.M{
			"_id": bson.M{"$in": coachingSessionIds},
		}
		pipeline := []bson.M{
			{"$match": filter},
		}

		var sessions []models.StudentSession
		if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &sessions)
		}); err == nil {
			for _, session := range sessions {
				detailsMap[session.Id.Hex()] = &models.CoachingSessionDetails{
					SessionNum:  session.SessionNum,
					Name:        session.SessionName,
					Description: session.Description,
				}
				if session.ZoomMeetingId != nil {
					zoomMeetingIdsToFetch = append(zoomMeetingIdsToFetch, *session.ZoomMeetingId)
					zoomMeetingIdMap[session.Id] = *session.ZoomMeetingId
				}
			}
		}
	}

	// Query placement test schedules to get ZoomMeetingId
	if len(placementScheduleIds) > 0 {
		filter := bson.M{
			"_id": bson.M{"$in": placementScheduleIds},
		}
		pipeline := []bson.M{
			{"$match": filter},
		}

		var schedules []models.OnlinePlacementTestSchedule
		if err := c.placementTestScheduleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &schedules)
		}); err == nil {
			for _, schedule := range schedules {
				detailsMap[schedule.Id.Hex()] = &models.PlacementTestDetails{}
				if schedule.ZoomMeetingId != nil {
					zoomMeetingIdsToFetch = append(zoomMeetingIdsToFetch, *schedule.ZoomMeetingId)
					zoomMeetingIdMap[schedule.Id] = *schedule.ZoomMeetingId
				}
			}
		}
	}

	// Fetch Zoom meeting details if any
	if len(zoomMeetingIdsToFetch) > 0 {
		filter := bson.M{
			"_id": bson.M{"$in": zoomMeetingIdsToFetch},
		}
		pipeline := []bson.M{
			{"$match": filter},
		}

		var meetings []zoommodels.ZoomMeeting
		if err := c.zoomMeetingRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &meetings)
		}); err == nil {
			meetingsMap := make(map[primitive.ObjectID]*models.ZoomMeetingDetails)
			for _, m := range meetings {
				meetingsMap[m.Id] = &models.ZoomMeetingDetails{
					Id:              m.Id,
					ZoomMeetingId:   m.ZoomMeetingId,
					JoinUrl:         m.JoinUrl,
					Password:        m.Password,
					StartTime:       m.StartTime,
					DurationMinutes: m.DurationMinutes,
					Status:          m.Status,
					ActualStartTime: m.ActualStartTime,
					ActualEndTime:   m.ActualEndTime,
					TeacherJoined:   m.TeacherJoined,
					RecordingUrl:    m.RecordingUrl,
				}
			}

			// Map Zoom details back to the events
			for eventId, zoomId := range zoomMeetingIdMap {
				if zoomDetails, ok := meetingsMap[zoomId]; ok {
					details := detailsMap[eventId.Hex()]
					if d, ok := details.(*models.CoachingSessionDetails); ok {
						d.ZoomMeeting = zoomDetails
					} else if d, ok := details.(*models.PlacementTestDetails); ok {
						d.ZoomMeeting = zoomDetails
					}
				}
			}
		}
	}

	return detailsMap
}

// CalculateDateRange calculates date range for a view type
func (c *calendarsvcs) CalculateDateRange(ctx context.Context, viewType string, referenceDate time.Time, timezone string) (models.DateRange, error) {
	// Default to UTC if timezone not provided
	loc := time.UTC
	if timezone != "" {
		var err error
		loc, err = time.LoadLocation(timezone)
		if err != nil {
			return models.DateRange{}, fmt.Errorf("invalid timezone: %v", err)
		}
	}

	// Convert reference date to the specified timezone
	refInTz := referenceDate.In(loc)

	var start, end time.Time

	switch viewType {
	case "daily":
		start = time.Date(refInTz.Year(), refInTz.Month(), refInTz.Day(), 0, 0, 0, 0, loc)
		end = time.Date(refInTz.Year(), refInTz.Month(), refInTz.Day(), 23, 59, 59, 999999999, loc)
	case "weekly":
		// Get first day of week from options (default to Monday)
		firstDayOfWeek := DefaultFirstDayOfWeek
		cfg, err := util.GetReqAppCfg(ctx)
		if err == nil && cfg != nil && cfg.Hp != nil {
			val, err := common.GetOptionValue("CALENDAR_FIRST_DAY_OF_WEEK", cfg.Hp)
			if err == nil {
				if str, ok := val.(string); ok && str != "" {
					firstDayOfWeek = strings.ToLower(str)
				}
			}
		}

		// Calculate start day based on configurable first day of week
		weekday := int(refInTz.Weekday())
		if weekday == 0 {
			weekday = 7 // Sunday = 7
		}

		var daysFromFirstDay int
		switch firstDayOfWeek {
		case "sunday", "sun":
			// Sunday is first day (0 in Go, but we use 7)
			if weekday == 7 {
				daysFromFirstDay = 0
			} else {
				daysFromFirstDay = weekday
			}
		case "monday", "mon":
			// Monday is first day (default)
			daysFromFirstDay = weekday - 1
		case "saturday", "sat":
			// Saturday is first day
			if weekday == 6 {
				daysFromFirstDay = 0
			} else if weekday == 7 {
				daysFromFirstDay = 1
			} else {
				daysFromFirstDay = weekday + 1
			}
		default:
			// Default to Monday if invalid value
			daysFromFirstDay = weekday - 1
		}

		start = time.Date(refInTz.Year(), refInTz.Month(), refInTz.Day()-daysFromFirstDay, 0, 0, 0, 0, loc)
		end = start.AddDate(0, 0, 6)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, loc)
	case "monthly":
		start = time.Date(refInTz.Year(), refInTz.Month(), 1, 0, 0, 0, 0, loc)
		// Last day of month
		nextMonth := start.AddDate(0, 1, 0)
		end = nextMonth.AddDate(0, 0, -1)
		end = time.Date(end.Year(), end.Month(), end.Day(), 23, 59, 59, 999999999, loc)
	case "yearly":
		start = time.Date(refInTz.Year(), 1, 1, 0, 0, 0, 0, loc)
		end = time.Date(refInTz.Year(), 12, 31, 23, 59, 59, 999999999, loc)
	case "agenda":
		// Agenda shows future events, limited to a reasonable range (3 months)
		start = refInTz
		end = start.AddDate(0, 3, 0)
	default:
		return models.DateRange{}, fmt.Errorf("invalid view type: %s", viewType)
	}

	// Convert back to UTC for database queries
	return models.DateRange{
		Start: start.UTC(),
		End:   end.UTC(),
	}, nil
}

// ApplyBundleFilter filters events based on bundle filter
func (c *calendarsvcs) ApplyBundleFilter(events []models.CalendarEvent, bundleFilter string) []models.CalendarEvent {
	if bundleFilter == "" || bundleFilter == "all" {
		return events
	}

	filtered := []models.CalendarEvent{}
	for _, event := range events {
		switch bundleFilter {
		case "bundle_only":
			if event.BundleId != nil {
				filtered = append(filtered, event)
			}
		case "independent_only":
			if event.BundleId == nil {
				filtered = append(filtered, event)
			}
		default:
			filtered = append(filtered, event)
		}
	}

	return filtered
}

// Authorization helper functions

// getUserFromContext extracts user from context
func getUserFromContext(ctx context.Context) (*types.User, error) {
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return nil, err
	}
	if cfg.User == nil {
		return nil, errors.New("user not found in context")
	}
	return cfg.User, nil
}

// isAdmin checks if user has admin capability (ViewAllCalendarEvents)
func isAdmin(ctx context.Context) (bool, error) {
	return util.HasCapability(ctx, CapabilityViewAllCalendarEvents)
}

// getSupervisedTeacherIds retrieves all teacher IDs managed by the supervisor
func (c *calendarsvcs) getSupervisedTeacherIds(ctx context.Context, supervisorId primitive.ObjectID) ([]primitive.ObjectID, error) {
	filter := bson.M{
		"supervisorId": supervisorId,
		"status":       "assigned",
		"trash":        false,
	}

	var assignments []models.SupervisorAssignment
	if err := c.supervisorAssignmentRepo.Aggregate(ctx, []bson.M{{"$match": filter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &assignments)
	}); err != nil {
		return nil, err
	}

	teacherIdsMap := make(map[primitive.ObjectID]bool)
	for _, assignment := range assignments {
		if assignment.AssignType == "direct" && assignment.RefType == "teacher" {
			for _, tid := range assignment.RefIds {
				teacherIdsMap[tid] = true
			}
		} else if assignment.AssignType == "dynamic" && assignment.RefType == "subject" {
			// For dynamic subject assignments, matchedTeachers contains the resolved teacher IDs
			for _, tid := range assignment.MatchedTeachers {
				teacherIdsMap[tid] = true
			}
		}
	}

	teacherIds := make([]primitive.ObjectID, 0, len(teacherIdsMap))
	for tid := range teacherIdsMap {
		teacherIds = append(teacherIds, tid)
	}

	return teacherIds, nil
}

// convertSchedulingUnitToMinutes converts a SchedulingUnit to minutes
func convertSchedulingUnitToMinutes(unit contactmodels.SchedulingUnit) int {
	switch unit.Unit {
	case "hours":
		return unit.Value * 60
	case "minutes":
		return unit.Value
	case "days":
		return unit.Value * 24 * 60
	default:
		// Default to Value (assume minutes for backward compatibility)
		return unit.Value
	}
}

// parseTimeMinutes parses a time string (HH:MM) and returns minutes since midnight
func parseTimeMinutes(t string) (int, error) {
	parts := strings.Split(t, ":")
	if len(parts) != 2 {
		return 0, errors.New("invalid time format")
	}
	h, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, err
	}
	m, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, err
	}
	return h*60 + m, nil
}

// formatTimeMinutes formats minutes since midnight to HH:MM string
func formatTimeMinutes(m int) string {
	h := m / 60
	min := m % 60
	return fmt.Sprintf("%02d:%02d", h, min)
}

// GetTeacherBusySlots aggregates all busy time slots for a teacher
func (c *calendarsvcs) GetTeacherBusySlots(ctx context.Context, teacherId primitive.ObjectID, dateRange models.DateRange) (map[string][][2]int, error) {
	busySlots := make(map[string][][2]int)
	now := time.Now()

	// 1. Get coaching sessions with status scheduled or rescheduled
	coachingFilter := bson.M{
		"teacherId": teacherId,
		"trash":     false,
		"date": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
		"status": bson.M{
			"$in": []enums.StudentSessionStatus{
				enums.StudentSessionStatusScheduled,
				enums.StudentSessionStatusRescheduled,
			},
		},
	}

	var coachingSessions []models.StudentSession
	pipeline := []bson.M{
		{"$match": coachingFilter},
	}
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &coachingSessions)
	}); err != nil {
		return nil, err
	}

	for _, session := range coachingSessions {
		// Only include if start time is in the future
		startTime, _, err := combineDateAndTime(session.Date, session.From, session.To)
		if err != nil || startTime.Before(now) {
			continue
		}

		dateKey := session.Date.Format("2006-01-02")
		startM, err1 := parseTimeMinutes(session.From)
		endM, err2 := parseTimeMinutes(session.To)
		if err1 == nil && err2 == nil {
			busySlots[dateKey] = append(busySlots[dateKey], [2]int{startM, endM})
		}
	}

	// 2. Get active course live sessions
	courseFilter := bson.M{
		"trash":                       false,
		"settings.liveSession.active": true,
	}

	// Optimized: Get courses assigned to teacher using teacherAssignmentSvc first
	assignedCourseIds, err := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, teacherId)
	if err != nil || len(assignedCourseIds) == 0 {
		// Either error or no courses assigned, nothing to add to busy slots from courses
	} else {
		courseFilter["_id"] = bson.M{"$in": assignedCourseIds}

		var courses []models.Course
		coursePipeline := []bson.M{
			{"$match": courseFilter},
		}
		if err := c.courseRepo.Aggregate(ctx, coursePipeline, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &courses)
		}); err != nil {
			return nil, err
		}

		for _, course := range courses {
			schedule := course.Settings.LiveSession.Schedule
			repeatWeekly := course.Settings.LiveSession.RepeatWeekly

			duration := course.BasicInfo.Duration
			if duration <= 0 {
				duration = DefaultCourseDurationMinutes
			}

			occurrences := getCourseLiveSessionOccurrences(schedule, repeatWeekly, dateRange)
			for _, start := range occurrences {
				if start.Before(now) {
					continue
				}

				// Ensure UTC for all calculations
				startUTC := start.UTC()
				endTime := startUTC.Add(time.Duration(duration) * time.Minute)

				dateKey := startUTC.Format("2006-01-02")
				startM := startUTC.Hour()*60 + startUTC.Minute()
				endM := endTime.Hour()*60 + endTime.Minute()
				busySlots[dateKey] = append(busySlots[dateKey], [2]int{startM, endM})
			}
		}
	}

	// 3. Get placement test schedules
	placementFilter := bson.M{
		"teacherId": teacherId,
		"date": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	var placementSchedules []models.OnlinePlacementTestSchedule
	placementPipeline := []bson.M{
		{"$match": placementFilter},
	}
	if err := c.placementTestScheduleRepo.Aggregate(ctx, placementPipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &placementSchedules)
	}); err != nil {
		return nil, err
	}

	for _, schedule := range placementSchedules {
		startTime, _, err := combineDateAndTime(schedule.Date, schedule.From, schedule.To)
		if err != nil || startTime.Before(now) {
			continue
		}

		dateKey := schedule.Date.Format("2006-01-02")
		startM, err1 := parseTimeMinutes(schedule.From)
		endM, err2 := parseTimeMinutes(schedule.To)
		if err1 == nil && err2 == nil {
			busySlots[dateKey] = append(busySlots[dateKey], [2]int{startM, endM})
		}
	}

	return busySlots, nil
}

// GetTeacherAvailabilitySlots calculates available time slots for a teacher
func (c *calendarsvcs) GetTeacherAvailabilitySlots(ctx context.Context, teacherId primitive.ObjectID, dateRange models.DateRange, coachingProgramId *primitive.ObjectID) ([]models.CalendarEvent, error) {
	// Verify authorization
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Verify teacher can only view their own availability, or user is admin
	if !isAdminUser && teacherId != user.Id {
		// Check if user is a supervisor for this teacher
		hasSupervisorCap, _ := util.HasCapability(ctx, CapabilityViewSupervisedCalendarEvents)
		if hasSupervisorCap {
			managedTeacherIds, err := c.getSupervisedTeacherIds(ctx, user.Id)
			if err == nil {
				isManaged := false
				for _, tid := range managedTeacherIds {
					if tid == teacherId {
						isManaged = true
						break
					}
				}
				if !isManaged {
					return nil, ErrUnauthorizedOwnAvailabilityOnly
				}
			} else {
				return nil, err
			}
		} else {
			return nil, ErrUnauthorizedOwnAvailabilityOnly
		}
	}

	// Validate date range (max 1 year)
	daysDiff := int(dateRange.End.Sub(dateRange.Start).Hours() / 24)
	if daysDiff > MaxAvailabilityDateRangeDays {
		return nil, ErrAvailabilityDateRangeExceeded
	}

	// Get teacher
	teacher, err := c.teacherRepo.GetByFilter(ctx, bson.M{"_id": teacherId, "trash": false})
	if err != nil {
		return nil, fmt.Errorf("teacher not found: %v", err)
	}

	// Determine session duration
	var sessionDuration int
	if coachingProgramId != nil {
		// Priority 1: Use coaching program's SessionDuration
		program, err := c.coachingRepo.GetByFilter(ctx, bson.M{"_id": *coachingProgramId, "trash": false})
		if err == nil && program.SessionDuration > 0 {
			sessionDuration = program.SessionDuration
		}
	}

	// Priority 2: Fallback to teacher's session duration
	if sessionDuration <= 0 {
		sessionDuration = convertSchedulingUnitToMinutes(teacher.Availability.Scheduling.SessionDuration)
	}

	if sessionDuration <= 0 {
		// Log warning but don't error - return empty slots
		return []models.CalendarEvent{}, nil
	}

	// Determine interval between sessions
	intervalMin := convertSchedulingUnitToMinutes(teacher.Availability.Scheduling.TimeIntervalBetweenSessions)
	if intervalMin < 0 {
		intervalMin = 0
	}

	// Get busy slots
	busySlots, err := c.GetTeacherBusySlots(ctx, teacherId, dateRange)
	if err != nil {
		return nil, err
	}

	// Generate available slots
	now := time.Now()
	events := []models.CalendarEvent{}

	// Iterate through each day in date range
	for d := dateRange.Start; !d.After(dateRange.End); d = d.AddDate(0, 0, 1) {
		// Get day of week
		weekdayStr := d.Weekday().String()
		dayEnum := contactenums.DayOfWeek(weekdayStr)

		// Check if day is selected
		availability, ok := teacher.Availability.DayOfWeekAvailability[dayEnum]
		if !ok || !availability.IsSelected {
			continue
		}

		// Determine working hours
		var workingHours []contactmodels.Shift
		if availability.IsCustom && len(availability.WorkingHours) > 0 {
			workingHours = availability.WorkingHours
		} else if len(teacher.Availability.DefaultWorkingHours) > 0 {
			workingHours = teacher.Availability.DefaultWorkingHours
		} else {
			continue // No working hours for this day
		}

		dateKey := d.Format("2006-01-02")
		dayBusySlots := busySlots[dateKey]

		// Generate slots for each shift
		for _, shift := range workingHours {
			shiftStart, err := parseTimeMinutes(shift.From)
			if err != nil {
				continue
			}
			shiftEnd, err := parseTimeMinutes(shift.To)
			if err != nil {
				continue
			}

			// Generate slots within this shift
			currSlotStart := shiftStart

			for {
				currSlotEnd := currSlotStart + sessionDuration
				if currSlotEnd > shiftEnd {
					break
				}

				// Create slot time to check if it's in the past
				slotStartTime := time.Date(d.Year(), d.Month(), d.Day(), currSlotStart/60, currSlotStart%60, 0, 0, time.UTC)

				// Exclude past slots - check for any day, not just today
				if slotStartTime.Before(now) {
					currSlotStart = currSlotEnd + intervalMin
					continue
				}

				// Check for overlap with busy slots
				isBusy := false
				for _, busy := range dayBusySlots {
					// Overlap condition: StartA < EndB && EndA > StartB
					if currSlotStart < busy[1] && currSlotEnd > busy[0] {
						isBusy = true
						break
					}
				}

				if !isBusy {
					// Create calendar event for this slot (reuse slotStartTime from above)
					slotEndTime := time.Date(d.Year(), d.Month(), d.Day(), currSlotEnd/60, currSlotEnd%60, 0, 0, time.UTC)

					event := models.CalendarEvent{
						Id:        primitive.NewObjectID(),
						Type:      "availability_slot",
						Title:     "Available",
						StartTime: slotStartTime,
						EndTime:   slotEndTime,
						TeacherId: teacherId,
						Status:    "available",
					}

					events = append(events, event)
				}

				// Advance to next potential slot
				currSlotStart = currSlotEnd + intervalMin
			}
		}
	}

	return events, nil
}

// Access verification methods

// verifyBundleAccess checks if user can access a bundle
func (c *calendarsvcs) verifyBundleAccess(ctx context.Context, bundleId primitive.ObjectID, userId primitive.ObjectID) (bool, error) {
	// Check if user has ViewAllBundles capability
	hasAllBundles, err := util.HasCapability(ctx, CapabilityViewAllBundles)
	if err != nil {
		return false, err
	}
	if hasAllBundles {
		return true, nil
	}

	// For now, assume bundles are accessible if user can view calendar
	// This can be enhanced with specific bundle permissions later
	return true, nil
}

// verifyCourseAccess checks if user can view a course
func (c *calendarsvcs) verifyCourseAccess(ctx context.Context, courseId primitive.ObjectID, userId primitive.ObjectID) (bool, error) {
	hasAllCourses, err := util.HasCapability(ctx, CapabilityViewAllCourses)
	if err != nil {
		return false, err
	}
	if hasAllCourses {
		return true, nil
	}

	// Check if user is a student with an active enrollment
	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	if hasStudentCap {
		enrollmentFilter := bson.M{
			"studentId": userId,
			"courseId":  courseId,
			"status":    "active",
			"trash":     false,
		}
		count, err := c.studentCourseRepo.Count(ctx, enrollmentFilter)
		if err == nil && count > 0 {
			return true, nil
		}
	}

	// Check if user is assigned to the course (teacher) via assignment service
	teachers, err := c.teacherAssignmentSvc.GetCourseTeachers(ctx, courseId)
	if err != nil {
		return false, err
	}

	for _, teacherId := range teachers {
		if teacherId == userId {
			return true, nil
		}
	}

	return false, nil
}

// verifyCoachingProgramAccess checks if user can view a coaching program
func (c *calendarsvcs) verifyCoachingProgramAccess(ctx context.Context, programId primitive.ObjectID, userId primitive.ObjectID) (bool, error) {
	hasAllPrograms, err := util.HasCapability(ctx, CapabilityViewAllCoachingPrograms)
	if err != nil {
		return false, err
	}
	if hasAllPrograms {
		return true, nil
	}

	// Check if user is a student with any sessions in this program
	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	if hasStudentCap {
		sessionFilter := bson.M{
			"studentId":         userId,
			"coachingProgramId": programId,
			"trash":             false,
		}
		count, err := c.studentSessionRepo.Count(ctx, sessionFilter)
		if err == nil && count > 0 {
			return true, nil
		}

		// Also check placement tests
		placementFilter := bson.M{
			"studentId":         userId,
			"coachingProgramId": programId,
		}
		count, err = c.placementTestScheduleRepo.Count(ctx, placementFilter)
		if err == nil && count > 0 {
			return true, nil
		}
	}

	// Check if user is assigned to the program (coach) via assignment service
	teachers, err := c.teacherAssignmentSvc.GetCoachingTeachers(ctx, programId)
	if err != nil {
		return false, err
	}

	for _, coachId := range teachers {
		if coachId == userId {
			return true, nil
		}
	}

	return false, nil
}

// canViewStudentData checks if teacher can view student's data
func (c *calendarsvcs) canViewStudentData(ctx context.Context, studentId primitive.ObjectID, teacherId primitive.ObjectID) (bool, error) {
	hasAllStudents, err := util.HasCapability(ctx, CapabilityViewAllStudents)
	if err != nil {
		return false, err
	}
	if hasAllStudents {
		return true, nil
	}

	// Check if teacher has any relationship with student
	// Option 1: Check coaching sessions
	sessionFilter := bson.M{
		"studentId": studentId,
		"teacherId": teacherId,
		"trash":     false,
	}
	session, err := c.studentSessionRepo.GetByFilter(ctx, sessionFilter)
	if err == nil && session != nil {
		return true, nil
	}

	// Option 2: Check if student is enrolled in courses where teacher is assigned
	// Query teacher assignments to find courses where teacher is assigned
	courseIds, err := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, teacherId)
	if err == nil && len(courseIds) > 0 {
		// Check if student is enrolled in any of these courses
		enrollmentFilter := bson.M{
			"studentId": studentId,
			"courseId":  bson.M{"$in": courseIds},
			"status":    "active",
			"trash":     false,
		}
		enrollment, err := c.studentCourseRepo.GetByFilter(ctx, enrollmentFilter)
		if err == nil && enrollment != nil {
			return true, nil
		}
	}

	// Option 3: Check if teacher has scheduled a placement test for this student
	placementTestFilter := bson.M{
		"studentId": studentId,
		"teacherId": teacherId,
	}
	placementTestSchedule, err := c.placementTestScheduleRepo.GetByFilter(ctx, placementTestFilter)
	if err == nil && placementTestSchedule != nil {
		return true, nil
	}

	return false, nil
}

// ApplyAuthorization applies permission checks and filters events
func (c *calendarsvcs) ApplyAuthorization(ctx context.Context, events []models.EnrichedCalendarEvent, filters models.CalendarFilters) ([]models.EnrichedCalendarEvent, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// For admins, no further checks needed currently
	if isAdminUser {
		return events, nil
	}

	// Check for supervisor capability
	hasSupervisorCap, err := util.HasCapability(ctx, CapabilityViewSupervisedCalendarEvents)
	if err != nil {
		return nil, err
	}

	var managedTeacherIds []primitive.ObjectID
	managedMap := make(map[primitive.ObjectID]bool)
	if hasSupervisorCap {
		managedTeacherIds, err = c.getSupervisedTeacherIds(ctx, user.Id)
		if err != nil {
			return nil, err
		}
		for _, tid := range managedTeacherIds {
			managedMap[tid] = true
		}
	}

	// Prepare data for batch checks
	courseIds := []primitive.ObjectID{}
	coachingProgramIds := []primitive.ObjectID{}
	studentIdsToCheck := []primitive.ObjectID{}

	seenCourses := make(map[primitive.ObjectID]bool)
	seenPrograms := make(map[primitive.ObjectID]bool)
	seenStudents := make(map[primitive.ObjectID]bool)

	for _, event := range events {
		if event.CourseId != nil && !seenCourses[*event.CourseId] {
			courseIds = append(courseIds, *event.CourseId)
			seenCourses[*event.CourseId] = true
		}
		if event.CoachingProgramId != nil && !seenPrograms[*event.CoachingProgramId] {
			coachingProgramIds = append(coachingProgramIds, *event.CoachingProgramId)
			seenPrograms[*event.CoachingProgramId] = true
		}
		if event.StudentId != nil && *event.StudentId != user.Id && !seenStudents[*event.StudentId] {
			studentIdsToCheck = append(studentIdsToCheck, *event.StudentId)
			seenStudents[*event.StudentId] = true
		}
	}

	// Batch check course access
	accessibleCourses := make(map[primitive.ObjectID]bool)
	if len(courseIds) > 0 {
		// Check active enrollments for student user
		hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
		if hasStudentCap {
			enrollmentFilter := bson.M{
				"studentId": user.Id,
				"courseId":  bson.M{"$in": courseIds},
				"status":    "active",
				"trash":     false,
			}
			var enrollments []models.StudentCourse
			c.studentCourseRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
				return cur.All(ctx, &enrollments)
			})
			for _, e := range enrollments {
				accessibleCourses[e.CourseId] = true
			}
		}

		// Check teacher assignments
		assignedCourseIds, _ := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, user.Id)
		for _, id := range assignedCourseIds {
			accessibleCourses[id] = true
		}

		// Check supervisor access (courses of managed teachers)
		if hasSupervisorCap && len(managedTeacherIds) > 0 {
			for _, tid := range managedTeacherIds {
				supervisedCourseIds, _ := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, tid)
				for _, id := range supervisedCourseIds {
					accessibleCourses[id] = true
				}
			}
		}

		// Check ViewAllCourses capability (though already checked isAdminUser, let's be thorough)
		hasAllCourses, _ := util.HasCapability(ctx, CapabilityViewAllCourses)
		if hasAllCourses {
			for _, id := range courseIds {
				accessibleCourses[id] = true
			}
		}
	}

	// Batch check coaching program access
	accessiblePrograms := make(map[primitive.ObjectID]bool)
	if len(coachingProgramIds) > 0 {
		hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
		if hasStudentCap {
			// Check sessions
			sessionFilter := bson.M{
				"studentId":         user.Id,
				"coachingProgramId": bson.M{"$in": coachingProgramIds},
				"trash":             false,
			}
			c.studentSessionRepo.Aggregate(ctx, []bson.M{{"$match": sessionFilter}, {"$group": bson.M{"_id": "$coachingProgramId"}}}, func(cur *mongo.Cursor) error {
				var results []bson.M
				if err := cur.All(ctx, &results); err == nil {
					for _, r := range results {
						if id, ok := r["_id"].(primitive.ObjectID); ok {
							accessiblePrograms[id] = true
						}
					}
				}
				return nil
			})

			// Check placement tests
			placementFilter := bson.M{
				"studentId":         user.Id,
				"coachingProgramId": bson.M{"$in": coachingProgramIds},
			}
			c.placementTestScheduleRepo.Aggregate(ctx, []bson.M{{"$match": placementFilter}, {"$group": bson.M{"_id": "$coachingProgramId"}}}, func(cur *mongo.Cursor) error {
				var results []bson.M
				if err := cur.All(ctx, &results); err == nil {
					for _, r := range results {
						if id, ok := r["_id"].(primitive.ObjectID); ok {
							accessiblePrograms[id] = true
						}
					}
				}
				return nil
			})
		}

		// Check teacher assignments
		assignedProgramIds, _ := c.teacherAssignmentSvc.GetCoachingsByTeacher(ctx, user.Id)
		for _, id := range assignedProgramIds {
			accessiblePrograms[id] = true
		}

		// Check supervisor access (coachings of managed teachers)
		if hasSupervisorCap && len(managedTeacherIds) > 0 {
			for _, tid := range managedTeacherIds {
				supervisedProgramIds, _ := c.teacherAssignmentSvc.GetCoachingsByTeacher(ctx, tid)
				for _, id := range supervisedProgramIds {
					accessiblePrograms[id] = true
				}
			}
		}

		hasAllPrograms, _ := util.HasCapability(ctx, CapabilityViewAllCoachingPrograms)
		if hasAllPrograms {
			for _, id := range coachingProgramIds {
				accessiblePrograms[id] = true
			}
		}
	}

	// Batch check student data privacy
	accessibleStudents := make(map[primitive.ObjectID]bool)
	if len(studentIdsToCheck) > 0 {
		hasAllStudents, _ := util.HasCapability(ctx, CapabilityViewAllStudents)
		if hasAllStudents {
			for _, id := range studentIdsToCheck {
				accessibleStudents[id] = true
			}
		} else {
			// Check relationships (coaching sessions)
			sessionFilter := bson.M{
				"studentId": bson.M{"$in": studentIdsToCheck},
				"teacherId": user.Id,
				"trash":     false,
			}

			// If supervisor, also check managed teachers
			if hasSupervisorCap && len(managedTeacherIds) > 0 {
				sessionFilter["teacherId"] = bson.M{"$in": append(managedTeacherIds, user.Id)}
			}

			c.studentSessionRepo.Aggregate(ctx, []bson.M{{"$match": sessionFilter}, {"$group": bson.M{"_id": "$studentId"}}}, func(cur *mongo.Cursor) error {
				var results []bson.M
				if err := cur.All(ctx, &results); err == nil {
					for _, r := range results {
						if id, ok := r["_id"].(primitive.ObjectID); ok {
							accessibleStudents[id] = true
						}
					}
				}
				return nil
			})

			// Check relationships (enrolled in teacher's courses)
			teacherSearchIds := []primitive.ObjectID{user.Id}
			if hasSupervisorCap {
				teacherSearchIds = append(teacherSearchIds, managedTeacherIds...)
			}

			allRelevantCourseIdsMap := make(map[primitive.ObjectID]bool)
			for _, tid := range teacherSearchIds {
				courseIds, _ := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, tid)
				for _, cid := range courseIds {
					allRelevantCourseIdsMap[cid] = true
				}
			}

			allRelevantCourseIds := make([]primitive.ObjectID, 0, len(allRelevantCourseIdsMap))
			for cid := range allRelevantCourseIdsMap {
				allRelevantCourseIds = append(allRelevantCourseIds, cid)
			}

			if len(allRelevantCourseIds) > 0 {
				enrollmentFilter := bson.M{
					"studentId": bson.M{"$in": studentIdsToCheck},
					"courseId":  bson.M{"$in": allRelevantCourseIds},
					"status":    "active",
					"trash":     false,
				}
				c.studentCourseRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}, {"$group": bson.M{"_id": "$studentId"}}}, func(cur *mongo.Cursor) error {
					var results []bson.M
					if err := cur.All(ctx, &results); err == nil {
						for _, r := range results {
							if id, ok := r["_id"].(primitive.ObjectID); ok {
								accessibleStudents[id] = true
							}
						}
					}
					return nil
				})
			}

			// Check relationships (placement tests)
			placementFilter := bson.M{
				"studentId": bson.M{"$in": studentIdsToCheck},
				"teacherId": user.Id,
			}
			if hasSupervisorCap && len(managedTeacherIds) > 0 {
				placementFilter["teacherId"] = bson.M{"$in": append(managedTeacherIds, user.Id)}
			}

			c.placementTestScheduleRepo.Aggregate(ctx, []bson.M{{"$match": placementFilter}, {"$group": bson.M{"_id": "$studentId"}}}, func(cur *mongo.Cursor) error {
				var results []bson.M
				if err := cur.All(ctx, &results); err == nil {
					for _, r := range results {
						if id, ok := r["_id"].(primitive.ObjectID); ok {
							accessibleStudents[id] = true
						}
					}
				}
				return nil
			})
		}
	}

	filteredEvents := []models.EnrichedCalendarEvent{}
	for _, event := range events {
		shouldInclude := true

		// Strict check for assignment deadlines: only visible to the student owner or admin
		if (event.Type == "coaching_assignment_deadline" || event.Type == "course_assignment_deadline") && !isAdminUser {
			if event.StudentId == nil || *event.StudentId != user.Id {
				shouldInclude = false
			}
		}

		if shouldInclude {
			// For supervisors, allow viewing events for their managed teachers even if other checks fail
			if hasSupervisorCap && managedMap[event.TeacherId] {
				// Include it
			} else if event.StudentId != nil && *event.StudentId == user.Id {
				// Always allow students to see their own events
				// This bypasses the coaching program check which might lock them out
				// before they have formal access to the program (e.g. during placement tests)
			} else {
				// Check resource permissions
				if event.CourseId != nil {
					if !accessibleCourses[*event.CourseId] {
						shouldInclude = false
					}
				}

				if shouldInclude && event.CoachingProgramId != nil {
					if !accessiblePrograms[*event.CoachingProgramId] {
						shouldInclude = false
					}
				}

				// Check student data privacy
				if shouldInclude && event.StudentId != nil && *event.StudentId != user.Id {
					if !accessibleStudents[*event.StudentId] {
						shouldInclude = false
					}
				}
			}
		}

		if shouldInclude {
			filteredEvents = append(filteredEvents, event)
		}
	}

	return filteredEvents, nil
}

// GetCalendarSummary returns summary data for yearly view
func (c *calendarsvcs) GetCalendarSummary(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange) (*models.CalendarSummary, error) {
	// Apply basic authorization (same as GetCalendarEvents)
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Check if user has required capability
	if !isAdminUser {
		hasSupervisorCap, err := util.HasCapability(ctx, CapabilityViewSupervisedCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasTeacherCap, err := util.HasCapability(ctx, CapabilityViewOwnCalendarEvents)
		if err != nil {
			return nil, err
		}
		hasStudentCap, err := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
		if err != nil {
			return nil, err
		}

		if !hasSupervisorCap && !hasTeacherCap && !hasStudentCap {
			return nil, ErrUnauthorizedInsufficientPermissions
		}

		if hasSupervisorCap {
			// Resolve managed teachers
			managedTeacherIds, err := c.getSupervisedTeacherIds(ctx, user.Id)
			if err != nil {
				return nil, err
			}

			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = managedTeacherIds
			} else {
				// Verify supervisor only filters by managed teachers
				managedMap := make(map[primitive.ObjectID]bool)
				for _, tid := range managedTeacherIds {
					managedMap[tid] = true
				}
				for _, tid := range filters.TeacherIds {
					if !managedMap[tid] {
						return nil, ErrUnauthorizedOwnEventsOnly
					}
				}
			}
		} else if hasTeacherCap {
			// For non-admins, enforce teacher filter
			if len(filters.TeacherIds) == 0 {
				filters.TeacherIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify teacher can only filter by themselves
				if len(filters.TeacherIds) != 1 || filters.TeacherIds[0] != user.Id {
					return nil, ErrUnauthorizedOwnEventsOnly
				}
			}
		} else if hasStudentCap {
			// For students, enforce student filter
			if len(filters.StudentIds) == 0 {
				filters.StudentIds = []primitive.ObjectID{user.Id}
			} else {
				// Verify student can only filter by themselves
				if len(filters.StudentIds) != 1 || filters.StudentIds[0] != user.Id {
					return nil, ErrUnauthorizedInsufficientPermissions
				}
			}
		}
	}

	now := time.Now()
	summary := &models.CalendarSummary{
		Dates: make(map[string]models.DateSummary),
	}

	// Aggregate coaching sessions by date
	coachingCounts, err := c.aggregateCoachingSessionsByDate(ctx, filters, dateRange, now)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate coaching sessions: %w", err)
	}

	// Aggregate course live sessions by date
	courseCounts, err := c.aggregateCourseLiveSessionsByDate(ctx, filters, dateRange, now)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate course live sessions: %w", err)
	}

	// Aggregate placement tests by date
	placementCounts, err := c.aggregatePlacementTestsByDate(ctx, filters, dateRange, now)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate placement tests: %w", err)
	}

	// Aggregate coaching assignment deadlines by date
	coachingAssignmentCounts, err := c.aggregateCoachingAssignmentDeadlinesByDate(ctx, filters, dateRange, now)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate coaching assignment deadlines: %w", err)
	}

	// Aggregate course assignment deadlines by date
	courseAssignmentCounts, err := c.aggregateCourseAssignmentDeadlinesByDate(ctx, filters, dateRange, now)
	if err != nil {
		return nil, fmt.Errorf("failed to aggregate course assignment deadlines: %w", err)
	}

	// Combine all counts into summary
	allDates := make(map[string]bool)
	for date := range coachingCounts {
		allDates[date] = true
	}
	for date := range courseCounts {
		allDates[date] = true
	}
	for date := range placementCounts {
		allDates[date] = true
	}
	for date := range coachingAssignmentCounts {
		allDates[date] = true
	}
	for date := range courseAssignmentCounts {
		allDates[date] = true
	}

	// Build summary for each date that has events
	for date := range allDates {
		dateSummary := models.DateSummary{
			HasEvents:   true,
			EventCounts: make(map[string]int),
			TotalEvents: 0,
		}

		if count, ok := coachingCounts[date]; ok {
			dateSummary.EventCounts["coaching_session"] = count
			dateSummary.TotalEvents += count
		}
		if count, ok := courseCounts[date]; ok {
			dateSummary.EventCounts["course_live_session"] = count
			dateSummary.TotalEvents += count
		}
		if count, ok := placementCounts[date]; ok {
			dateSummary.EventCounts["placement_test"] = count
			dateSummary.TotalEvents += count
		}
		if count, ok := coachingAssignmentCounts[date]; ok {
			dateSummary.EventCounts["coaching_assignment_deadline"] = count
			dateSummary.TotalEvents += count
		}
		if count, ok := courseAssignmentCounts[date]; ok {
			dateSummary.EventCounts["course_assignment_deadline"] = count
			dateSummary.TotalEvents += count
		}

		// Only include dates with events (sparse map)
		if dateSummary.TotalEvents > 0 {
			summary.Dates[date] = dateSummary
		}
	}

	return summary, nil
}

// aggregateCoachingSessionsByDate aggregates coaching sessions by date using MongoDB aggregation
func (c *calendarsvcs) aggregateCoachingSessionsByDate(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange, now time.Time) (map[string]int, error) {
	// Build filter
	filter := bson.M{
		"trash": false,
		"date": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Filter by teacher
	if len(filters.TeacherIds) > 0 {
		filter["teacherId"] = bson.M{"$in": filters.TeacherIds}
	}

	// Filter by student
	if len(filters.StudentIds) > 0 {
		filter["studentId"] = bson.M{"$in": filters.StudentIds}
	}

	// Filter by coaching program
	if len(filters.CoachingProgramIds) > 0 {
		filter["coachingProgramId"] = bson.M{"$in": filters.CoachingProgramIds}
	}

	// Filter by status
	if len(filters.Statuses) > 0 {
		filter["status"] = bson.M{"$in": filters.Statuses}
	}

	// Aggregation pipeline to count events per date
	// We'll filter by date range first, then calculate start time and filter by upcoming events
	pipeline := []bson.M{
		{"$match": filter},
		// Add fields to calculate start time from date + from time
		{"$addFields": bson.M{
			"fromHour": bson.M{
				"$toInt": bson.M{
					"$arrayElemAt": []interface{}{
						bson.M{"$split": []interface{}{"$from", ":"}},
						0,
					},
				},
			},
			"fromMin": bson.M{
				"$toInt": bson.M{
					"$arrayElemAt": []interface{}{
						bson.M{"$split": []interface{}{"$from", ":"}},
						1,
					},
				},
			},
		}},
		// Calculate start time by adding hours and minutes to date
		{"$addFields": bson.M{
			"startTime": bson.M{
				"$dateAdd": bson.M{
					"startDate": bson.M{
						"$dateAdd": bson.M{
							"startDate": "$date",
							"unit":      "hour",
							"amount":    "$fromHour",
						},
					},
					"unit":   "minute",
					"amount": "$fromMin",
				},
			},
		}},
		// Filter to only include upcoming events
		{"$match": bson.M{
			"startTime": bson.M{"$gte": now},
		}},
		// Extract date portion (YYYY-MM-DD)
		{"$project": bson.M{
			"dateStr": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$date",
				},
			},
		}},
		// Group by date and count
		{"$group": bson.M{
			"_id":   "$dateStr",
			"count": bson.M{"$sum": 1},
		}},
	}

	type result struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	var results []result
	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, r := range results {
		counts[r.ID] = r.Count
	}

	return counts, nil
}

// aggregateCoachingAssignmentDeadlinesByDate aggregates coaching assignment deadlines by date
func (c *calendarsvcs) aggregateCoachingAssignmentDeadlinesByDate(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange, now time.Time) (map[string]int, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	isAdminUser, _ := isAdmin(ctx)

	if !isAdminUser && !hasStudentCap {
		return map[string]int{}, nil
	}

	var studentId primitive.ObjectID
	if hasStudentCap {
		studentId = user.Id
	} else if isAdminUser && len(filters.StudentIds) == 1 {
		studentId = filters.StudentIds[0]
	} else {
		return map[string]int{}, nil
	}

	// Get active coaching program enrollments
	enrollmentFilter := bson.M{
		"studentId": studentId,
		"status":    "active",
		"trash":     false,
	}
	var enrollments []models.CoachingStudent
	if err := c.coachingStudentRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	}); err != nil {
		return nil, err
	}

	if len(enrollments) == 0 {
		return map[string]int{}, nil
	}

	programIds := make([]primitive.ObjectID, len(enrollments))
	for i, e := range enrollments {
		programIds[i] = e.CoachingId
	}

	// Build assignment filter
	assignmentFilter := bson.M{
		"coachingId": bson.M{"$in": programIds},
		"status":     "active",
		"trash":      false,
		"dueDate": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Apply filter if provided
	if len(filters.CoachingProgramIds) > 0 {
		intersected := []primitive.ObjectID{}
		fMap := make(map[primitive.ObjectID]bool)
		for _, id := range filters.CoachingProgramIds {
			fMap[id] = true
		}
		for _, id := range programIds {
			if fMap[id] {
				intersected = append(intersected, id)
			}
		}
		if len(intersected) == 0 {
			return map[string]int{}, nil
		}
		assignmentFilter["coachingId"] = bson.M{"$in": intersected}
	}

	pipeline := []bson.M{
		{"$match": assignmentFilter},
		// Filter to only include upcoming deadlines
		{"$match": bson.M{
			"dueDate": bson.M{"$gte": now},
		}},
		{"$project": bson.M{
			"dateStr": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$dueDate",
				},
			},
		}},
		{"$group": bson.M{
			"_id":   "$dateStr",
			"count": bson.M{"$sum": 1},
		}},
	}

	type result struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	var results []result
	if err := c.coachingAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, r := range results {
		counts[r.ID] = r.Count
	}

	return counts, nil
}

// aggregateCourseAssignmentDeadlinesByDate aggregates course assignment deadlines by date
func (c *calendarsvcs) aggregateCourseAssignmentDeadlinesByDate(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange, now time.Time) (map[string]int, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	isAdminUser, _ := isAdmin(ctx)

	if !isAdminUser && !hasStudentCap {
		return map[string]int{}, nil
	}

	var studentId primitive.ObjectID
	if hasStudentCap {
		studentId = user.Id
	} else if isAdminUser && len(filters.StudentIds) == 1 {
		studentId = filters.StudentIds[0]
	} else {
		return map[string]int{}, nil
	}

	// Get courses the student is enrolled in
	enrollmentFilter := bson.M{
		"studentId": studentId,
		"status":    "active",
		"trash":     false,
	}
	var enrollments []models.StudentCourse
	if err := c.studentCourseRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &enrollments)
	}); err != nil {
		return nil, err
	}

	if len(enrollments) == 0 {
		return map[string]int{}, nil
	}

	courseIds := make([]primitive.ObjectID, len(enrollments))
	for i, e := range enrollments {
		courseIds[i] = e.CourseId
	}

	// Build assignment filter
	assignmentFilter := bson.M{
		"courseId": bson.M{"$in": courseIds},
		"status":   "active",
		"trash":    false,
		"deadline": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Apply course filter if provided
	if len(filters.CourseIds) > 0 {
		intersected := []primitive.ObjectID{}
		fMap := make(map[primitive.ObjectID]bool)
		for _, id := range filters.CourseIds {
			fMap[id] = true
		}
		for _, id := range courseIds {
			if fMap[id] {
				intersected = append(intersected, id)
			}
		}
		if len(intersected) == 0 {
			return map[string]int{}, nil
		}
		assignmentFilter["courseId"] = bson.M{"$in": intersected}
	}

	pipeline := []bson.M{
		{"$match": assignmentFilter},
		// Filter to only include upcoming deadlines
		{"$match": bson.M{
			"deadline": bson.M{"$gte": now},
		}},
		{"$project": bson.M{
			"dateStr": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$deadline",
				},
			},
		}},
		{"$group": bson.M{
			"_id":   "$dateStr",
			"count": bson.M{"$sum": 1},
		}},
	}

	type result struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	var results []result
	if err := c.courseAssignmentRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, r := range results {
		counts[r.ID] = r.Count
	}

	return counts, nil
}

// aggregateCourseLiveSessionsByDate aggregates course live sessions by date
func (c *calendarsvcs) aggregateCourseLiveSessionsByDate(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange, now time.Time) (map[string]int, error) {
	user, err := getUserFromContext(ctx)
	if err != nil {
		return nil, err
	}

	isAdminUser, err := isAdmin(ctx)
	if err != nil {
		return nil, err
	}

	// Build filter: include all active live sessions whose base schedule is on or before the end of the range.
	filter := bson.M{
		"trash":                       false,
		"settings.liveSession.active": true,
		"settings.liveSession.schedule": bson.M{
			"$lte": dateRange.End,
		},
	}

	// Filter by course
	hasStudentCap, _ := util.HasCapability(ctx, CapabilityViewStudentCalendarEvents)
	if !isAdminUser && hasStudentCap {
		enrollmentFilter := bson.M{
			"studentId": user.Id,
			"status":    "active",
			"trash":     false,
		}
		var enrollments []models.StudentCourse
		if err := c.studentCourseRepo.Aggregate(ctx, []bson.M{{"$match": enrollmentFilter}}, func(cur *mongo.Cursor) error {
			return cur.All(ctx, &enrollments)
		}); err == nil && len(enrollments) > 0 {
			enrolledCourseIds := make([]primitive.ObjectID, len(enrollments))
			for i, e := range enrollments {
				enrolledCourseIds[i] = e.CourseId
			}

			if len(filters.CourseIds) > 0 {
				// Intersect provided CourseIds with enrolled courses
				intersectedIds := []primitive.ObjectID{}
				enrolledMap := make(map[primitive.ObjectID]bool)
				for _, id := range enrolledCourseIds {
					enrolledMap[id] = true
				}
				for _, id := range filters.CourseIds {
					if enrolledMap[id] {
						intersectedIds = append(intersectedIds, id)
					}
				}
				if len(intersectedIds) == 0 {
					return map[string]int{}, nil
				}
				filter["_id"] = bson.M{"$in": intersectedIds}
			} else {
				filter["_id"] = bson.M{"$in": enrolledCourseIds}
			}
		} else {
			// No enrollments, no courses
			return map[string]int{}, nil
		}
	} else if len(filters.CourseIds) > 0 {
		filter["_id"] = bson.M{"$in": filters.CourseIds}
	}

	// Filter by teacher using assignment service
	if len(filters.TeacherIds) > 0 {
		assignedMap := make(map[primitive.ObjectID]bool)
		for _, tid := range filters.TeacherIds {
			ids, err := c.teacherAssignmentSvc.GetCoursesByTeacher(ctx, tid)
			if err == nil {
				for _, id := range ids {
					assignedMap[id] = true
				}
			}
		}

		allAssignedCourseIds := make([]primitive.ObjectID, 0, len(assignedMap))
		for id := range assignedMap {
			allAssignedCourseIds = append(allAssignedCourseIds, id)
		}

		if len(allAssignedCourseIds) == 0 {
			return map[string]int{}, nil
		}

		// Add to filter
		if existingIds, ok := filter["_id"].(bson.M); ok {
			if inClause, ok := existingIds["$in"].([]primitive.ObjectID); ok {
				intersected := []primitive.ObjectID{}
				for _, id := range inClause {
					if assignedMap[id] {
						intersected = append(intersected, id)
					}
				}
				if len(intersected) == 0 {
					return map[string]int{}, nil
				}
				filter["_id"] = bson.M{"$in": intersected}
			}
		} else {
			filter["_id"] = bson.M{"$in": allAssignedCourseIds}
		}
	}

	// Query courses
	pipeline := []bson.M{
		{"$match": filter},
	}

	var courses []models.Course
	if err := c.courseRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &courses)
	}); err != nil {
		return nil, err
	}

	// Group by date and count, expanding recurring sessions within the range
	counts := make(map[string]int)
	for _, course := range courses {
		schedule := course.Settings.LiveSession.Schedule
		repeatWeekly := course.Settings.LiveSession.RepeatWeekly

		occurrences := getCourseLiveSessionOccurrences(schedule, repeatWeekly, dateRange)
		for _, start := range occurrences {
			if start.Before(now) {
				continue
			}
			dateStr := start.Format("2006-01-02")
			counts[dateStr]++
		}
	}

	return counts, nil
}

// aggregatePlacementTestsByDate aggregates placement tests by date using MongoDB aggregation
func (c *calendarsvcs) aggregatePlacementTestsByDate(ctx context.Context, filters models.CalendarFilters, dateRange models.DateRange, now time.Time) (map[string]int, error) {
	// Build filter
	filter := bson.M{
		"date": bson.M{
			"$gte": dateRange.Start,
			"$lte": dateRange.End,
		},
	}

	// Filter by teacher
	if len(filters.TeacherIds) > 0 {
		filter["teacherId"] = bson.M{"$in": filters.TeacherIds}
	}

	// Filter by student
	if len(filters.StudentIds) > 0 {
		filter["studentId"] = bson.M{"$in": filters.StudentIds}
	}

	// Filter by coaching program
	if len(filters.CoachingProgramIds) > 0 {
		filter["coachingProgramId"] = bson.M{"$in": filters.CoachingProgramIds}
	}

	// Aggregation pipeline to count events per date
	// We'll filter by date range first, then calculate start time and filter by upcoming events
	pipeline := []bson.M{
		{"$match": filter},
		// Add fields to calculate start time from date + from time
		{"$addFields": bson.M{
			"fromHour": bson.M{
				"$toInt": bson.M{
					"$arrayElemAt": []interface{}{
						bson.M{"$split": []interface{}{"$from", ":"}},
						0,
					},
				},
			},
			"fromMin": bson.M{
				"$toInt": bson.M{
					"$arrayElemAt": []interface{}{
						bson.M{"$split": []interface{}{"$from", ":"}},
						1,
					},
				},
			},
		}},
		// Calculate start time by adding hours and minutes to date
		{"$addFields": bson.M{
			"startTime": bson.M{
				"$dateAdd": bson.M{
					"startDate": bson.M{
						"$dateAdd": bson.M{
							"startDate": "$date",
							"unit":      "hour",
							"amount":    "$fromHour",
						},
					},
					"unit":   "minute",
					"amount": "$fromMin",
				},
			},
		}},
		// Filter to only include upcoming events
		{"$match": bson.M{
			"startTime": bson.M{"$gte": now},
		}},
		// Extract date portion (YYYY-MM-DD)
		{"$project": bson.M{
			"dateStr": bson.M{
				"$dateToString": bson.M{
					"format": "%Y-%m-%d",
					"date":   "$date",
				},
			},
		}},
		// Group by date and count
		{"$group": bson.M{
			"_id":   "$dateStr",
			"count": bson.M{"$sum": 1},
		}},
	}

	type result struct {
		ID    string `bson:"_id"`
		Count int    `bson:"count"`
	}

	var results []result
	if err := c.placementTestScheduleRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &results)
	}); err != nil {
		return nil, err
	}

	counts := make(map[string]int)
	for _, r := range results {
		counts[r.ID] = r.Count
	}

	return counts, nil
}
