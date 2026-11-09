package models

import (
	"fmt"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"regexp"
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Coaching struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingDto `bson:",inline"`
	CreatedBy   primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash       bool               `bson:"trash" json:"trash"`
}

type CoachingDto struct {
	Title       transl.Localizable[string] `bson:"title" json:"title" validate:"required"`
	ProgramType string                     `bson:"programType" json:"programType" validate:"required,oneof=curriculum foundational"`
	Subject     primitive.ObjectID         `bson:"subject" json:"subject" `
	Level       string                     `bson:"level" json:"level" validate:"required"`
	Class       []string                   `bson:"class" json:"class" validate:"required"`
	SchoolType  string                     `bson:"schoolType" json:"schoolType" validate:"required,oneof=private public"`
	SchoolId    primitive.ObjectID         `bson:"schoolId,omitempty" json:"schoolId,omitempty"`
	Country     string                     `bson:"country" json:"country" `
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Thumbnail   types.FileField            `bson:"thumbnail" json:"thumbnail"`
	// Media is an optional marketing gallery (images/videos) shown before purchase
	Media           []MediaItem          `bson:"media,omitempty" json:"media,omitempty"`
	MinSessionNum   int                  `bson:"minSessionNum" json:"minSessionNum" `
	MaxSessionNum   int                  `bson:"maxSessionNum" json:"maxSessionNum" `
	SessionDuration int                  `bson:"sessionDuration" json:"sessionDuration" `
	Sessions        []CoachingSession    `bson:"sessions" json:"sessions"`
	Coachs          []primitive.ObjectID `bson:"coachs" json:"coachs" validate:"required"`
	SupervisorIds   []primitive.ObjectID `bson:"supervisorIds,omitempty" json:"supervisorIds,omitempty"` // Direct supervisor IDs synced from SupervisorAssignment
}

func (dto *CoachingDto) Validate(v *validator.Validate) error {
	// run generic validation first (tags)
	if err := helpers.GenericValidation(v, dto); err != nil {
		return err
	}

	// Validate programType and Class cross-field rules
	// Class values should match pattern: "Grade <digit>" (e.g. "Grade 1")
	gradeRe := regexp.MustCompile(`^Grade \d+$`)
	for _, c := range dto.Class {
		if !gradeRe.MatchString(c) {
			return fmt.Errorf("class value '%s' must match 'Grade <digit>' pattern", c)
		}
	}

	if dto.ProgramType == "curriculum" {
		if len(dto.Class) < 1 {
			return fmt.Errorf("for programType 'curriculum' class must have at least one value")
		}
	} else if dto.ProgramType == "foundational" {
		if len(dto.Class) != 1 {
			return fmt.Errorf("for programType 'foundational' class must have exactly one value")
		}
	} else {
		return fmt.Errorf("programType must be either 'curriculum' or 'foundational'")
	}

	if dto.SchoolType == "private" {
		if dto.SchoolId.IsZero() {
			return fmt.Errorf("schoolId is required for private school type")
		}
	} else if dto.SchoolType == "public" {
		if !dto.SchoolId.IsZero() {
			return fmt.Errorf("schoolId should not be defined for public school type")
		}
	}

	return nil
}

func (c *Coaching) CollName() string {
	return "academyCoachings"
}

func (c *Coaching) GetName() transl.Localizable[string] {
	return c.Title
}

// PreSave automatically generates SessionNum
func (c *Coaching) PreSave() {
	sessions := make([]CoachingSession, 0, len(c.Sessions))
	for index, session := range c.Sessions {
		session.SessionNum = index + 1
		sessions = append(sessions, session)
	}
	c.Sessions = sessions
}

type CoachingSession struct {
	SessionNum  int                        `bson:"sessionNum" json:"sessionNum"`
	SessionName transl.Localizable[string] `bson:"sessionName" json:"sessionName"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Recordings  []types.FileField          `bson:"recordings" json:"recordings"`   //added by teacher
	AgendaItems []AgendaItem               `bson:"agendaItems" json:"agendaItems"` //added by teacher
	Notes       []NoteItem                 `bson:"notes" json:"notes"`
	Resources   []types.FileField          `bson:"resources" json:"resources"`
}

type AgendaItem struct {
	Item    string `bson:"item" json:"item"`
	Checked bool   `bson:"checked" json:"checked"`
}

type NoteItem struct {
	Note string    `bson:"note" json:"note"`
	Date time.Time `bson:"date" json:"date"`
}

type CoachingPagination struct {
	Coachings  []EnrichedCoaching `json:"coachings"`
	Pagination types.Pagination   `json:"pagination"`
}

// TeacherCoachingResponse represents a coaching with student count for teacher endpoints
type TeacherCoachingResponse struct {
	Coaching     Coaching `json:"coaching"`
	StudentCount int      `json:"studentCount"`
}

// TeacherCoachingPagination represents paginated teacher coachings with student counts
type TeacherCoachingPagination struct {
	Coachings  []TeacherCoachingResponse `json:"coachings"`
	Pagination types.Pagination          `json:"pagination"`
}

// coaching program subscription
type CoachingSubscribe struct {
	CoachingProgramId primitive.ObjectID `bson:"coachingProgramId" json:"coachingProgramId" validate:"required"`
	NumOfSessions     int                `bson:"numOfSessions" json:"numOfSessions" validate:"required"` // must be event number (2-4-6 ..etc)
	RenewMonthly      bool               `bson:"renewMonthly" json:"renewMonthly" `
	Price             float64            `bson:"price" json:"price" `
}

func (cs *CoachingSubscribe) Validate(v *validator.Validate) error {
	return helpers.GenericValidation(v, cs)
}

// student session schedule
type BookingSessions struct {
	CoachingProgramId primitive.ObjectID `bson:"coachingProgramId" json:"coachingProgramId" validate:"required"`
	TeacherId         primitive.ObjectID `bson:"teacherId" json:"teacherId" validate:"required"`
	Dates             []BookingDate      `bson:"dates" json:"dates" validate:"required"`
}

type BookingDate struct {
	Date    time.Time `bson:"date" json:"date"`
	Weekday string    `bson:"weekday" json:"weekday"`
	From    string    `bson:"from" json:"from"` // 24 hour format
	To      string    `bson:"to" json:"to"`     // 24 hour format
}

func (bs *BookingSessions) Validate(v *validator.Validate) error {
	return helpers.GenericValidation(v, bs)
}

// Teacher availability models
type TeacherAvailabilityResponse struct {
	TeacherId       primitive.ObjectID             `json:"teacherId"`
	TeacherName     transl.Localizable[string]     `json:"teacherName"`
	SessionDuration int                            `json:"sessionDuration"` // in minutes
	AvailableSlots  map[string][]AvailableTimeSlot `json:"availableSlots"`  // day of week -> time slots
}

type AvailableTimeSlot struct {
	From      string    `json:"from"` // 24h format HH:MM
	To        string    `json:"to"`   // 24h format HH:MM
	Date      time.Time `json:"date"` // specific date for this slot
	Available bool      `json:"available"`
}

// CoachingTeachersAvailabilityResponse - Response for all teachers with their availability
type CoachingTeachersAvailabilityResponse struct {
	CoachingId      primitive.ObjectID            `json:"coachingId"`
	CoachingTitle   string                        `json:"coachingTitle"`
	SessionDuration int                           `json:"sessionDuration"`
	Teachers        []TeacherAvailabilityResponse `json:"teachers"`
}

// GroupedAvailabilityResponse - Response for grouped availability from all teachers
type GroupedAvailabilityResponse struct {
	CoachingId      primitive.ObjectID           `json:"coachingId"`
	CoachingTitle   string                       `json:"coachingTitle"`
	SessionDuration int                          `json:"sessionDuration"`
	GroupedSlots    map[string][]GroupedTimeSlot `json:"groupedSlots"` // weekday -> slots
}

// GroupedTimeSlot - Time slot with multiple teachers available
type GroupedTimeSlot struct {
	From         string               `json:"from"` // HH:MM
	To           string               `json:"to"`   // HH:MM
	Date         time.Time            `json:"date"` // specific date for this slot
	Available    bool                 `json:"available"`
	TeacherIds   []primitive.ObjectID `json:"teacherIds"`   // teachers available at this slot
	TeacherCount int                  `json:"teacherCount"` // convenience field
}

// Enriched coach model for when enrichCoachs=true is requested
type EnrichedCoach struct {
	Id        primitive.ObjectID         `bson:"_id" json:"_id"`
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Picture   types.FileField            `bson:"picture" json:"picture"`
}

// StudentCountByCoachingRow is one row for student-count-by-coaching analytics (admin/supervisor)
type StudentCountByCoachingRow struct {
	CoachingId    primitive.ObjectID         `json:"coachingId"`
	SubjectId     primitive.ObjectID         `json:"subjectId"`
	CoachingTitle transl.Localizable[string] `json:"coachingTitle,omitempty"`
	StudentCount  int                        `json:"studentCount"`
}

// StudentCountBySubjectRow represents total students grouped by subject (across courses and coachings)
type StudentCountBySubjectRow struct {
	SubjectId    primitive.ObjectID         `json:"subjectId"`
	SubjectName  transl.Localizable[string] `json:"subjectName,omitempty"`
	StudentCount int                        `json:"studentCount"`
}

// Enriched coaching model with coach details in separate field
type EnrichedCoaching struct {
	Id          primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CoachingDto `bson:",inline"`
	CreatedBy   primitive.ObjectID   `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy   primitive.ObjectID   `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt   time.Time            `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt   time.Time            `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash       bool                 `bson:"trash" json:"trash"`
	Coachs      []primitive.ObjectID `bson:"coachs" json:"coachs" validate:"required"`         // Original coach IDs
	CoachsObjs  []EnrichedCoach      `bson:"coachsObjs,omitempty" json:"coachsObjs,omitempty"` // Enriched coach objects
	SchoolObj   *School              `bson:"schoolObj,omitempty" json:"schoolObj,omitempty"`
}
