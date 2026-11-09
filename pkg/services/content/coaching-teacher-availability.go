package content

import (
	"context"
	"errors"
	"fmt"
	contactenums "larsa-academy-microservice/pkg/services/contact/enums"
	teachermodels "larsa-academy-microservice/pkg/services/contact/models"
	contentenums "larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"strconv"
	"strings"
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

// GetTeacherAvailability returns available time slots for a teacher for a specific coaching program
func (c *coachingsvcs) GetTeacherAvailability(ctx context.Context, teacherId string, coachingProgramId string) (*models.TeacherAvailabilityResponse, error) {
	tId, err := primitive.ObjectIDFromHex(teacherId)
	if err != nil {
		return nil, errors.New("invalid teacher id")
	}

	cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}

	// Get Teacher
	teacher, err := c.teacherRepo.GetByFilter(ctx, bson.M{"_id": tId, "trash": false})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("teacher not found")
		}
		return nil, err
	}

	// Get Coaching Program
	program, err := c.coachingRepo.GetByFilter(ctx, bson.M{"_id": cpId, "trash": false})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("coaching program not found")
		}
		return nil, err
	}

	// Determine session duration (in minutes)
	sessionDuration := program.SessionDuration
	if sessionDuration == 0 {
		// Fallback to teacher's default session duration if not set in program
		if teacher.Availability.Scheduling.SessionDuration.Value > 0 {
			sessionDuration = teacher.Availability.Scheduling.SessionDuration.Value
		}
	}

	if sessionDuration <= 0 {
		return nil, errors.New("invalid session duration configuration")
	}

	// Determine interval between sessions (buffer)
	intervalMin := teacher.Availability.Scheduling.TimeIntervalBetweenSessions.Value

	// Define period (next 7 days)
	// Start looking from today
	now := time.Now()
	// Reset to start of day for consistent date calculations
	startOfToday := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())

	daysToCheck := 7
	endDate := startOfToday.AddDate(0, 0, daysToCheck)

	// Filter for existing scheduled sessions in the range
	sessionFilter := bson.M{
		"teacherId": tId,
		"trash":     false,
		"date": bson.M{
			"$gte": startOfToday,
			"$lte": endDate,
		},
		"status": bson.M{
			"$in": []contentenums.StudentSessionStatus{
				contentenums.StudentSessionStatusScheduled,
				contentenums.StudentSessionStatusRescheduled,
			},
		},
	}

	var existingSessions []models.StudentSession
	pipeline := []bson.M{
		{"$match": sessionFilter},
	}

	if err := c.studentSessionRepo.Aggregate(ctx, pipeline, func(cur *mongo.Cursor) error {
		return cur.All(ctx, &existingSessions)
	}); err != nil {
		return nil, err
	}

	// Organize existing sessions by Date ("YYYY-MM-DD") -> list of [StartMin, EndMin]
	busySlots := make(map[string][][2]int)
	for _, s := range existingSessions {
		dateKey := s.Date.Format("2006-01-02")
		startM, err1 := parseTime(s.From)
		endM, err2 := parseTime(s.To)
		if err1 == nil && err2 == nil {
			busySlots[dateKey] = append(busySlots[dateKey], [2]int{startM, endM})
		}
	}

	availableSlotsMap := make(map[string][]models.AvailableTimeSlot)

	for i := 0; i < daysToCheck; i++ {
		currDate := startOfToday.AddDate(0, 0, i)
		weekdayStr := currDate.Weekday().String()
		dayEnum := contactenums.DayOfWeek(strings.ToLower(weekdayStr))

		availability, ok := teacher.Availability.DayOfWeekAvailability[dayEnum]
		if !ok || !availability.IsSelected {
			// Teacher does not work on this day
			continue
		}

		var workingHours []teachermodels.Shift
		if availability.IsCustom {
			workingHours = availability.WorkingHours
		} else {
			workingHours = teacher.Availability.DefaultWorkingHours
		}

		dateKey := currDate.Format("2006-01-02")
		dayBusySlots := busySlots[dateKey]

		var daySlots []models.AvailableTimeSlot

		for _, shift := range workingHours {
			// Skip empty shifts
			if shift.From == "" || shift.To == "" {
				continue
			}

			shiftStart, err := parseTime(shift.From)
			if err != nil {
				continue
			}
			shiftEnd, err := parseTime(shift.To)
			if err != nil {
				continue
			}

			// Handle overnight shifts (e.g., 22:00 to 06:00)
			isOvernightShift := shiftEnd <= shiftStart
			if isOvernightShift {
				shiftEnd += 24 * 60 // Add 24 hours to end time
			}

			// Generate slots within this shift
			currSlotStart := shiftStart

			for {
				currSlotEnd := currSlotStart + sessionDuration
				if currSlotEnd > shiftEnd {
					break
				}

				// If we are checking today, ensure slot is in the future
				if i == 0 {
					nowMin := now.Hour()*60 + now.Minute()
					if currSlotStart <= nowMin {
						// Skip this slot and try next interval
						currSlotStart = currSlotEnd + intervalMin
						continue
					}
				}

				// Check for overlap with busy slots
				isBusy := false
				for _, busy := range dayBusySlots {
					busyStart := busy[0]
					busyEnd := busy[1]
					// Handle overnight busy slots
					if busyEnd <= busyStart {
						busyEnd += 24 * 60
					}
					// Overlap condition: StartA < EndB && EndA > StartB
					if currSlotStart < busyEnd && currSlotEnd > busyStart {
						isBusy = true
						break
					}
				}

				if !isBusy {
					daySlots = append(daySlots, models.AvailableTimeSlot{
						From:      formatTime(currSlotStart),
						To:        formatTime(currSlotEnd),
						Date:      currDate,
						Available: true,
					})
				}

				// Advance to next potential slot
				currSlotStart = currSlotEnd + intervalMin
			}
		}

		if len(daySlots) > 0 {
			availableSlotsMap[strings.ToLower(weekdayStr)] = daySlots
		}
	}

	// Merge teacher firstName and lastName into full name
	teacherName := mergeLocalizableNames(teacher.FirstName, teacher.LastName)

	return &models.TeacherAvailabilityResponse{
		TeacherId:       teacher.Id,
		TeacherName:     teacherName,
		SessionDuration: sessionDuration,
		AvailableSlots:  availableSlotsMap,
	}, nil
}

func parseTime(t string) (int, error) {
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

func formatTime(m int) string {
	h := m / 60
	min := m % 60
	return fmt.Sprintf("%02d:%02d", h, min)
}

// GetCoachingTeachersAvailability returns all teachers assigned to a coaching with their availability
func (c *coachingsvcs) GetCoachingTeachersAvailability(ctx context.Context, coachingProgramId string) (*models.CoachingTeachersAvailabilityResponse, error) {
	cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}

	// Get Coaching Program
	program, err := c.coachingRepo.GetByFilter(ctx, bson.M{"_id": cpId, "trash": false})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("coaching program not found")
		}
		return nil, err
	}

	// Get coaching title (handle localization)
	coachingTitle := program.Title.GetContentByContext(ctx)
	if coachingTitle == "" {
		// Fallback to first available language if context-based lookup fails
		for _, v := range program.Title {
			coachingTitle = v
			break
		}
	}

	// Get all teachers assigned to this coaching using teacher assignment service
	teacherIds, err := c.teacherAssignmentSvc.GetCoachingTeachers(ctx, cpId)
	if err != nil {
		return nil, err
	}

	if len(teacherIds) == 0 {
		return &models.CoachingTeachersAvailabilityResponse{
			CoachingId:      program.Id,
			CoachingTitle:   coachingTitle,
			SessionDuration: program.SessionDuration,
			Teachers:        []models.TeacherAvailabilityResponse{},
		}, nil
	}

	// Fetch availability for each teacher
	teachers := make([]models.TeacherAvailabilityResponse, 0, len(teacherIds))
	for _, teacherId := range teacherIds {
		teacherAvailability, err := c.GetTeacherAvailability(ctx, teacherId.Hex(), coachingProgramId)
		if err != nil {
			// Log error but continue with other teachers
			// Optionally, you could skip teachers that fail
			continue
		}
		teachers = append(teachers, *teacherAvailability)
	}

	return &models.CoachingTeachersAvailabilityResponse{
		CoachingId:      program.Id,
		CoachingTitle:   coachingTitle,
		SessionDuration: program.SessionDuration,
		Teachers:        teachers,
	}, nil
}

// GetCoachingGroupedAvailability returns all available time slots from all teachers, grouped by exact match
func (c *coachingsvcs) GetCoachingGroupedAvailability(ctx context.Context, coachingProgramId string) (*models.GroupedAvailabilityResponse, error) {
	cpId, err := primitive.ObjectIDFromHex(coachingProgramId)
	if err != nil {
		return nil, errors.New("invalid coaching program id")
	}

	// Get Coaching Program
	program, err := c.coachingRepo.GetByFilter(ctx, bson.M{"_id": cpId, "trash": false})
	if err != nil {
		if err == mongo.ErrNoDocuments {
			return nil, errors.New("coaching program not found")
		}
		return nil, err
	}

	// Get coaching title (handle localization)
	coachingTitle := program.Title.GetContentByContext(ctx)
	if coachingTitle == "" {
		// Fallback to first available language if context-based lookup fails
		for _, v := range program.Title {
			coachingTitle = v
			break
		}
	}

	// Get all teachers assigned to this coaching using teacher assignment service
	teacherIds, err := c.teacherAssignmentSvc.GetCoachingTeachers(ctx, cpId)
	if err != nil {
		return nil, err
	}

	if len(teacherIds) == 0 {
		return &models.GroupedAvailabilityResponse{
			CoachingId:      program.Id,
			CoachingTitle:   coachingTitle,
			SessionDuration: program.SessionDuration,
			GroupedSlots:    make(map[string][]models.GroupedTimeSlot),
		}, nil
	}

	// Get availability for each teacher
	allTeacherSlots := make([]models.TeacherAvailabilityResponse, 0, len(teacherIds))
	for _, teacherId := range teacherIds {
		teacherAvailability, err := c.GetTeacherAvailability(ctx, teacherId.Hex(), coachingProgramId)
		if err != nil {
			// Log error but continue with other teachers
			continue
		}
		allTeacherSlots = append(allTeacherSlots, *teacherAvailability)
	}

	// Group slots by exact match: weekday + date + from + to
	// Map key: "weekday|date|from|to" -> []teacherIds
	slotMap := make(map[string][]primitive.ObjectID)

	for _, teacher := range allTeacherSlots {
		for weekday, slots := range teacher.AvailableSlots {
			for _, slot := range slots {
				// Create unique key: weekday + date + from + to
				dateKey := slot.Date.Format("2006-01-02")
				key := fmt.Sprintf("%s|%s|%s|%s", weekday, dateKey, slot.From, slot.To)

				// Check if this teacher is already in the list for this slot
				teacherIdsForSlot := slotMap[key]
				found := false
				for _, tid := range teacherIdsForSlot {
					if tid == teacher.TeacherId {
						found = true
						break
					}
				}
				if !found {
					slotMap[key] = append(slotMap[key], teacher.TeacherId)
				}
			}
		}
	}

	// Convert map to grouped slots organized by weekday
	groupedSlots := make(map[string][]models.GroupedTimeSlot)

	for key, teacherIds := range slotMap {
		parts := strings.Split(key, "|")
		if len(parts) != 4 {
			continue
		}
		weekday := parts[0]
		dateStr := parts[1]
		from := parts[2]
		to := parts[3]

		date, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			continue
		}

		groupedSlot := models.GroupedTimeSlot{
			From:         from,
			To:           to,
			Date:         date,
			Available:    true,
			TeacherIds:   teacherIds,
			TeacherCount: len(teacherIds),
		}

		groupedSlots[weekday] = append(groupedSlots[weekday], groupedSlot)
	}

	// Sort slots within each weekday by time (optional but recommended)
	// This would require importing sort package, but for now we'll leave it unsorted
	// as the frontend can sort if needed

	return &models.GroupedAvailabilityResponse{
		CoachingId:      program.Id,
		CoachingTitle:   coachingTitle,
		SessionDuration: program.SessionDuration,
		GroupedSlots:    groupedSlots,
	}, nil
}
