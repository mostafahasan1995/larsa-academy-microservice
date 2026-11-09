package models

import (
	contact_models "larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentSession struct {
	Id                 primitive.ObjectID         `bson:"_id" json:"_id"`
	StudentId          primitive.ObjectID         `bson:"studentId" json:"studentId"`
	CoachingProgramId  primitive.ObjectID         `bson:"coachingProgramId" json:"coachingProgramId"`
	BundleId           primitive.ObjectID         `bson:"bundleId" json:"bundleId"`
	TeacherId          primitive.ObjectID         `bson:"teacherId" json:"teacherId"`
	SessionNum         int                        `bson:"sessionNum" json:"sessionNum"`
	SessionName        transl.Localizable[string] `bson:"sessionName" json:"sessionName"`
	Description        transl.Localizable[string] `bson:"description" json:"description"`
	Agendas            []SessionAgenda            `bson:"agendas" json:"agendas"`
	Notes              []SessionNote              `bson:"notes" json:"notes"`
	Resources          []SessionResource          `bson:"resources" json:"resources"`
	Recordings         []types.FileField          `bson:"recordings" json:"recordings"`
	Date               time.Time                  `bson:"date" json:"date"`
	From               string                     `bson:"from" json:"from"` //24 hour format
	To                 string                     `bson:"to" json:"to"`     //24 hour format
	DurationMinutes    int                        `bson:"durationMinutes" json:"durationMinutes"`
	Status             enums.StudentSessionStatus `bson:"status" json:"status"` //pending - scheduled - rescheduled - cancelled - missed  - completed
	CanReschedule      bool                       `bson:"canReschedule" json:"canReschedule"`
	PrevJustifications []SessionJustification     `bson:"prevJustifications" json:"prevJustifications"`
	Justification      *SessionJustification      `bson:"justification" json:"justification"`
	Feedback           *SessionFeedback           `bson:"feedback,omitempty" json:"feedback,omitempty"`
	StudentInfo        *SessionStudentBasic       `bson:"studentInfo,omitempty" json:"studentInfo,omitempty"`
	TeacherInfo        *SessionTeacherBasic       `bson:"teacherInfo,omitempty" json:"teacherInfo,omitempty"`
	CoachingInfo       *SessionCoachingBasic      `bson:"coachingInfo,omitempty" json:"coachingInfo,omitempty"`
	//
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash     bool               `bson:"trash" json:"trash"`

	// membership removal metadata
	MembershipRemoved   bool               `bson:"membershipRemoved,omitempty" json:"membershipRemoved,omitempty"`
	MembershipRemovedAt time.Time          `bson:"membershipRemovedAt,omitempty" json:"membershipRemovedAt,omitempty"`
	MembershipRemovedBy primitive.ObjectID `bson:"membershipRemovedBy,omitempty" json:"membershipRemovedBy,omitempty"`

	ZoomMeetingId *primitive.ObjectID `bson:"zoomMeetingId,omitempty" json:"zoomMeetingId,omitempty"`
	ZoomMeeting   *ZoomMeetingDetails `bson:"zoomMeeting,omitempty" json:"zoomMeeting,omitempty"`
}

type SessionStudentBasic struct {
	Id        primitive.ObjectID         `json:"id"`
	FirstName transl.Localizable[string] `json:"firstName"`
	LastName  transl.Localizable[string] `json:"lastName"`
	Picture   types.FileField            `json:"picture"`
}

type SessionTeacherBasic struct {
	Id        primitive.ObjectID         `json:"id"`
	FirstName transl.Localizable[string] `json:"firstName"`
	LastName  transl.Localizable[string] `json:"lastName"`
	Picture   types.FileField            `json:"picture"`
}

type SessionCoachingBasic struct {
	Id        primitive.ObjectID         `json:"id"`
	Title     transl.Localizable[string] `json:"title"`
	Thumbnail types.FileField            `json:"thumbnail"`
}
type SessionAgenda struct {
	MemberId primitive.ObjectID `bson:"memberId" json:"memberId"`
	Item     string             `bson:"item" json:"item"`
	Checked  bool               `bson:"checked" json:"checked"`
	Date     time.Time          `bson:"date" json:"date"`
}
type SessionNote struct {
	MemberId primitive.ObjectID `bson:"memberId" json:"memberId"`
	Note     string             `bson:"note" json:"note"`
	Date     time.Time          `bson:"date" json:"date"`
}
type SessionResource struct {
	MemberId primitive.ObjectID `bson:"memberId" json:"memberId"`
	Resource types.FileField    `bson:"resource" json:"resource"`
}

type SessionJustification struct {
	Id        primitive.ObjectID      `bson:"_id" json:"_id"`
	Type      enums.JustificationType `bson:"type" json:"type"` //absence - reschedule
	Reason    string                  `bson:"reason" json:"reason"`
	Documents []types.FileField       `bson:"documents" json:"documents"`
	Data      any                     `bson:"data" json:"data"`
	Date      time.Time               `bson:"date" json:"date"`
	//decsision
	Decision       enums.JustificationDecision `bson:"decision" json:"decision"` // approved - rejected
	Note           string                      `bson:"note" json:"note"`
	DecisionDate   time.Time                   `bson:"decisionDate" json:"decisionDate"`
	DecisionBy     primitive.ObjectID          `bson:"decisionBy" json:"decisionBy"`
	DecisionByObj  *AbsenceRequestReviewer     `bson:"decisionByObj,omitempty" json:"decisionByObj,omitempty"`
	RequestedBy    primitive.ObjectID          `bson:"requestedBy,omitempty" json:"requestedBy,omitempty"`
	RequestedByObj *AbsenceRequestReviewer     `bson:"requestedByObj,omitempty" json:"requestedByObj,omitempty"`
}

// SessionFeedback groups all feedback-related fields
type SessionFeedback struct {
	TeacherFeedback     types.Nullable[string] `bson:"teacherFeedback" json:"teacherFeedback"`
	TeacherRating       types.Nullable[int]    `bson:"teacherRating" json:"teacherRating"`
	TeacherFeedbackDate *time.Time             `bson:"teacherFeedbackDate,omitempty" json:"teacherFeedbackDate,omitempty"`
	StudentEngagement   types.Nullable[int]    `bson:"studentEngagement" json:"studentEngagement"` // Percentage 0-100
	StudentFeedback     types.Nullable[string] `bson:"studentFeedback" json:"studentFeedback"`
	StudentRating       types.Nullable[int]    `bson:"studentRating" json:"studentRating"`
	StudentFeedbackDate *time.Time             `bson:"studentFeedbackDate,omitempty" json:"studentFeedbackDate,omitempty"`
}

type SessionUpdateItem struct {
	Id              string                      `json:"id" validate:"required"`
	Agendas         []SessionAgenda             `json:"agendas,omitempty"`
	Resources       []SessionResource           `json:"resources,omitempty"`
	Notes           []SessionNote               `json:"notes,omitempty"`
	Recordings      []types.FileField           `json:"recordings,omitempty"`
	DurationMinutes *int                        `json:"durationMinutes,omitempty"`
	SessionName     *transl.Localizable[string] `json:"sessionName,omitempty"`
	Description     *transl.Localizable[string] `json:"description,omitempty"`
}

type BulkUpdateSessionsDto struct {
	Sessions []SessionUpdateItem `json:"sessions" validate:"required,dive"`
}

func (s *StudentSession) CollName() string {
	return "academyCoachingStudentSessions"
}

// note
type SessionNoteDto struct {
	Note string `bson:"note" json:"note"`
}

type UpdateSessionNotesDto struct {
	Notes []SessionNote `bson:"notes" json:"notes"`
}

type UpdateSessionAgendasDto struct {
	Agendas []SessionAgenda `bson:"agendas" json:"agendas"`
}

type UpdateSessionResourcesDto struct {
	Resources []SessionResource `bson:"resources" json:"resources"`
}

type UpdateSessionFeedbackDto struct {
	// Teacher fields
	TeacherFeedback     types.Nullable[string] `bson:"teacherFeedback,omitempty" json:"teacherFeedback,omitempty"`
	TeacherRating       types.Nullable[int]    `bson:"teacherRating,omitempty" json:"teacherRating,omitempty"`
	TeacherFeedbackDate *time.Time             `bson:"teacherFeedbackDate,omitempty" json:"teacherFeedbackDate,omitempty"`
	StudentEngagement   types.Nullable[int]    `bson:"studentEngagement,omitempty" json:"studentEngagement,omitempty"`
	// Student fields
	StudentFeedback     types.Nullable[string] `bson:"studentFeedback,omitempty" json:"studentFeedback,omitempty"`
	StudentRating       types.Nullable[int]    `bson:"studentRating,omitempty" json:"studentRating,omitempty"`
	StudentFeedbackDate *time.Time             `bson:"studentFeedbackDate,omitempty" json:"studentFeedbackDate,omitempty"`
}

type SessionAgendaDot struct {
	Item    string `bson:"item" json:"item"`
	Checked bool   `bson:"checked" json:"checked"`
}

type SessionResourceDto struct {
	Resource types.FileField `bson:"resource" json:"resource"`
}

type SessionRecordingDto struct {
	SessionId primitive.ObjectID `bson:"sessionId" json:"sessionId"`
	Recording types.FileField    `bson:"recording" json:"recording"`
}

//my program

type MyProgram struct {
	CoachingProgram *Coaching
	Sessions        []StudentSession
}

// reschedule
type RescheduleJustificationDto struct {
	RescheduleSessionData `bson:",inline"`
	Reason                string            `bson:"reason" json:"reason" validate:"required"`
	Documents             []types.FileField `bson:"documents" json:"documents"`
}

type RescheduleSessionData struct {
	Date time.Time `bson:"date" json:"date" validate:"required"`
	From string    `bson:"from" json:"from" validate:"required"`
	To   string    `bson:"to" json:"to" validate:"required"`
}

// UpdateSessionTimeData is used for updating session time and/or status
type UpdateSessionTimeData struct {
	Date            *time.Time                  `bson:"date,omitempty" json:"date,omitempty"`
	From            *string                     `bson:"from,omitempty" json:"from,omitempty"`
	To              *string                     `bson:"to,omitempty" json:"to,omitempty"`
	Status          *enums.StudentSessionStatus `bson:"status,omitempty" json:"status,omitempty" validate:"omitempty,oneof=missed cancelled completed scheduled pending rescheduled"`
	TeacherId       *primitive.ObjectID         `bson:"teacherId,omitempty" json:"teacherId,omitempty"`
	SessionNum      *int                        `bson:"sessionNum,omitempty" json:"sessionNum,omitempty"`
	DurationMinutes *int                        `bson:"durationMinutes,omitempty" json:"durationMinutes,omitempty"`
	Notes           []SessionNote               `bson:"notes,omitempty" json:"notes,omitempty"`
	Agendas         []SessionAgenda             `bson:"agendas,omitempty" json:"agendas,omitempty"`
	Resources       []SessionResource           `bson:"resources,omitempty" json:"resources,omitempty"`
	Recordings      []types.FileField           `bson:"recordings,omitempty" json:"recordings,omitempty"`
	Feedback        *UpdateSessionFeedbackDto   `bson:"feedback,omitempty" json:"feedback,omitempty"`
	SessionName     *transl.Localizable[string] `bson:"sessionName,omitempty" json:"sessionName,omitempty"`
	Description     *transl.Localizable[string] `bson:"description,omitempty" json:"description,omitempty"`
}

type JustificationDecision struct {
	Decision enums.JustificationDecision `bson:"decision" json:"decision" validate:"required,oneof=approved rejected"`
	Note     string                      `bson:"note" json:"note" validate:"required"`
}

type AbsenceJustificationDto struct {
	Reason    string            `bson:"reason"`
	Documents []types.FileField `bson:"documents" json:"documents"`
}

// StudentInfo represents student details in responses
type StudentInfo struct {
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Picture   types.FileField            `bson:"picture" json:"picture"`
}

// SessionDetailsResponse - Enriched session for supervisor view
type SessionDetailsResponse struct {
	StudentSession `bson:",inline" json:",inline"`
	Student        StudentInfo                `bson:"student" json:"student"`
	TeacherName    transl.Localizable[string] `bson:"teacherName" json:"teacherName"`
	CoachingTitle  transl.Localizable[string] `bson:"coachingTitle" json:"coachingTitle"`
}

// AbsenceRequestReviewer carries lightweight reviewer details resolved from users service
type AbsenceRequestReviewer struct {
	Id        primitive.ObjectID `bson:"_id" json:"_id"`
	FirstName string             `bson:"firstName" json:"firstName"`
	LastName  string             `bson:"lastName" json:"lastName"`
	Email     string             `bson:"email" json:"email"`
	Picture   string             `bson:"picture" json:"picture"`
}

// AbsenceRequestRow is the admin-facing view of an absence justification tied to a session
type AbsenceRequestRow struct {
	Id                 primitive.ObjectID          `bson:"_id" json:"_id"`
	StudentId          primitive.ObjectID          `bson:"studentId" json:"studentId"`
	Student            StudentInfo                 `bson:"student" json:"student"`
	CoachingTitle      transl.Localizable[string]  `bson:"coachingTitle" json:"coachingTitle"`
	Date               time.Time                   `bson:"date" json:"date"`
	From               string                      `bson:"from" json:"from"`
	To                 string                      `bson:"to" json:"to"`
	DurationMinutes    int                         `bson:"durationMinutes" json:"durationMinutes"`
	Reason             string                      `bson:"reason" json:"reason"`
	Documents          []types.FileField           `bson:"documents" json:"documents"`
	Status             enums.JustificationDecision `bson:"status" json:"status"`
	DecisionNote       string                      `bson:"decisionNote" json:"decisionNote"`
	DecisionDate       *time.Time                  `bson:"decisionDate" json:"decisionDate"`
	DecisionBy         primitive.ObjectID          `bson:"decisionBy" json:"decisionBy"`
	DecisionByObj      *AbsenceRequestReviewer     `bson:"decisionByObj" json:"decisionByObj,omitempty"`
	PrevJustifications []SessionJustification      `bson:"prevJustifications" json:"prevJustifications,omitempty"`
}

type AbsenceRequestPagination struct {
	Requests   []AbsenceRequestRow `bson:"requests" json:"requests"`
	Pagination types.Pagination    `bson:"pagination" json:"pagination"`
}

// Unified justification row (absence or reschedule)
type JustificationRequestRow struct {
	Id                 primitive.ObjectID          `bson:"_id" json:"_id"`
	StudentId          primitive.ObjectID          `bson:"studentId" json:"studentId"`
	Student            StudentInfo                 `bson:"student" json:"student"`
	CoachingTitle      transl.Localizable[string]  `bson:"coachingTitle" json:"coachingTitle"`
	Type               enums.JustificationType     `bson:"type" json:"type"`
	Date               time.Time                   `bson:"date" json:"date"`
	From               string                      `bson:"from" json:"from"`
	To                 string                      `bson:"to" json:"to"`
	DurationMinutes    int                         `bson:"durationMinutes" json:"durationMinutes"`
	Reason             string                      `bson:"reason" json:"reason"`
	Documents          []types.FileField           `bson:"documents" json:"documents"`
	Status             enums.JustificationDecision `bson:"status" json:"status"`
	DecisionNote       string                      `bson:"decisionNote" json:"decisionNote"`
	DecisionDate       *time.Time                  `bson:"decisionDate" json:"decisionDate"`
	DecisionBy         primitive.ObjectID          `bson:"decisionBy" json:"decisionBy"`
	DecisionByObj      *AbsenceRequestReviewer     `bson:"decisionByObj" json:"decisionByObj,omitempty"`
	RequestedBy        primitive.ObjectID          `bson:"requestedBy" json:"requestedBy"`
	RequestedByObj     *AbsenceRequestReviewer     `bson:"requestedByObj" json:"requestedByObj,omitempty"`
	PrevJustifications []SessionJustification      `bson:"prevJustifications" json:"prevJustifications,omitempty"`
	// Reschedule-specific requested slot (nil for absence)
	NewDate *time.Time `bson:"newDate" json:"newDate"`
	NewFrom string     `bson:"newFrom" json:"newFrom"`
	NewTo   string     `bson:"newTo" json:"newTo"`
}

type JustificationRequestPagination struct {
	Requests   []JustificationRequestRow `bson:"requests" json:"requests"`
	Pagination types.Pagination          `bson:"pagination" json:"pagination"`
}

type CoachingProgramMember struct {
	StudentId     primitive.ObjectID         `bson:"_id" json:"_id"`
	Sessions      []StudentSession           `bson:"sessions" json:"sessions"`
	FirstName     transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName      transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email         string                     `bson:"email" json:"email"`
	Picture       types.FileField            `bson:"picture" json:"picture"`
	Progress      int                        `bson:"progress" json:"progress"`
	NumOfSessions int                        `bson:"numOfSessions" json:"numOfSessions"`
	StartDate     time.Time                  `bson:"startDate" json:"startDate"`
	Logins        int                        `bson:"logins" json:"logins"`
}

type CoachingProgramMemberPagination struct {
	Members    []CoachingProgramMember `bson:"members" json:"members"`
	Pagination types.Pagination        `bson:"pagination" json:"pagination"`
}

type StudentSessionPagination struct {
	Sessions   []StudentSession `bson:"sessions" json:"sessions"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

// TeacherStudent represents a student assigned to a teacher across all coachings
type TeacherStudent struct {
	StudentId         primitive.ObjectID      `bson:"_id" json:"_id"`
	Student           *contact_models.Student `bson:"student" json:"student"` // Full student object from academyStudents collection
	TotalSessions     int                     `bson:"totalSessions" json:"totalSessions"`
	CompletedSessions int                     `bson:"completedSessions" json:"completedSessions"`
	NumOfCoachings    int                     `bson:"numOfCoachings" json:"numOfCoachings"`
	Coachings         []primitive.ObjectID    `bson:"coachings" json:"coachings"` // List of coaching IDs
	LastSessionDate   time.Time               `bson:"lastSessionDate" json:"lastSessionDate"`
	FirstSessionDate  time.Time               `bson:"firstSessionDate" json:"firstSessionDate"`
	Progress          int                     `bson:"progress" json:"progress"` // Overall progress percentage
}

type TeacherStudentPagination struct {
	Students   []TeacherStudent `bson:"students" json:"students"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

// StudentTeacher represents a teacher assigned to a student across all coachings
type StudentTeacher struct {
	TeacherId         primitive.ObjectID      `bson:"_id" json:"_id"`
	Teacher           *contact_models.Teacher `bson:"teacher" json:"teacher"`
	TotalSessions     int                     `bson:"totalSessions" json:"totalSessions"`
	CompletedSessions int                     `bson:"completedSessions" json:"completedSessions"`
	NumOfCoachings    int                     `bson:"numOfCoachings" json:"numOfCoachings"`
	Coachings         []primitive.ObjectID    `bson:"coachings" json:"coachings"`
	LastSessionDate   time.Time               `bson:"lastSessionDate" json:"lastSessionDate"`
	FirstSessionDate  time.Time               `bson:"firstSessionDate" json:"firstSessionDate"`
	Progress          int                     `bson:"progress" json:"progress"`
}

type StudentTeacherPagination struct {
	Teachers   []StudentTeacher `bson:"teachers" json:"teachers"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

// TeacherStudentProfile represents a comprehensive student profile for teachers
type TeacherStudentProfile struct {
	Student        TeacherStudentProfileInfo `bson:"student" json:"student"`
	Coachings      []CoachingBasicInfo       `bson:"coachings" json:"coachings"`
	Courses        []CoachingBasicInfo       `bson:"courses" json:"courses"`
	Points         PointsSummary             `bson:"points" json:"points"`
	AttendanceRate float64                   `bson:"attendanceRate" json:"attendanceRate"` // Percentage
	ReviewSummary  ReviewSummary             `bson:"reviewSummary" json:"reviewSummary"`
	ParentInfo     []ParentBasicInfo         `bson:"parentInfo" json:"parentInfo"`
}

type TeacherStudentProfileInfo struct {
	Id          primitive.ObjectID         `bson:"_id" json:"_id"`
	FirstName   transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName    transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email       string                     `bson:"email" json:"email"`
	Picture     types.FileField            `bson:"picture" json:"picture"`
	Gender      string                     `bson:"gender" json:"gender"`
	DateOfBirth time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality string                     `bson:"nationality" json:"nationality"`
	Status      string                     `bson:"status" json:"status"`
	Phone       string                     `bson:"phone" json:"phone"`
	Class       string                     `bson:"class" json:"class"`
	Stage       string                     `bson:"stage" json:"stage"`
}

type CoachingBasicInfo struct {
	Id           primitive.ObjectID         `bson:"_id" json:"_id"`
	Title        transl.Localizable[string] `bson:"title" json:"title"`
	Description  transl.Localizable[string] `bson:"description" json:"description"`
	StudentCount int                        `bson:"studentCount,omitempty" json:"studentCount,omitempty"`
}

type PointsSummary struct {
	TotalPoints     int `bson:"totalPoints" json:"totalPoints"`
	RemainingPoints int `bson:"remainingPoints" json:"remainingPoints"`
	RedeemedPoints  int `bson:"redeemedPoints" json:"redeemedPoints"`
}

type ReviewSummary struct {
	TotalCount    int     `bson:"totalCount" json:"totalCount"`
	AverageRating float64 `bson:"averageRating" json:"averageRating"`
}

type ParentBasicInfo struct {
	Id           primitive.ObjectID         `bson:"_id" json:"_id"`
	FirstName    transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName     transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email        string                     `bson:"email" json:"email"`
	Phone        string                     `bson:"phone" json:"phone"`
	RelationType string                     `bson:"relationType" json:"relationType"`
}

type PersonInfo struct {
	Id        primitive.ObjectID         `json:"id"`
	FirstName transl.Localizable[string] `json:"firstName"`
	LastName  transl.Localizable[string] `json:"lastName"`
	Picture   types.FileField            `json:"picture"`
}

type SessionDetails struct {
	SessionId           *primitive.ObjectID        `bson:"sessionId" json:"sessionId,omitempty"`
	NextSessionId       *primitive.ObjectID        `bson:"nextSessionId" json:"nextSessionId,omitempty"`
	PreviousSessionId   *primitive.ObjectID        `bson:"previousSessionId" json:"previousSessionId,omitempty"`
	DurationMinutes     int                        `bson:"durationMinutes" json:"durationMinutes"`
	Student             PersonInfo                 `json:"student"`
	Teacher             PersonInfo                 `json:"teacher"`
	Class               string                     `json:"class"` // Student's class
	CoachingId          primitive.ObjectID         `json:"coachingId"`
	CoachingTitle       transl.Localizable[string] `json:"coachingTitle"`
	NextSessionDate     *time.Time                 `json:"nextSessionDate,omitempty"`     // null if no next session
	PreviousSessionDate *time.Time                 `json:"previousSessionDate,omitempty"` // null if no previous session
	StartDate           time.Time                  `json:"startDate"`                     // min date from sessions
	EndDate             time.Time                  `json:"endDate"`                       // max date from sessions
	Status              string                     `json:"status"`                        // status of next session, or previous if no next
}

type SessionDetailsPagination struct {
	Sessions   []SessionDetails `json:"sessions"`
	Pagination types.Pagination `json:"pagination"`
}

// StudentCoachingStatsResponse represents comprehensive coaching stats for a student
type StudentCoachingStatsResponse struct {
	CoachingId              primitive.ObjectID         `bson:"coachingId" json:"coachingId"`
	CoachingTitle           transl.Localizable[string] `bson:"coachingTitle" json:"coachingTitle"`
	CoachingThumbnail       types.FileField            `bson:"coachingThumbnail" json:"coachingThumbnail"`
	NumOfSessions           int                        `bson:"numOfSessions" json:"numOfSessions"`
	NumOfCompletedSessions  int                        `bson:"numOfCompletedSessions" json:"numOfCompletedSessions"`
	RestSessions            []StudentSession           `bson:"restSessions" json:"restSessions"` // Upcoming sessions
	NumOfAbsenceSessions    int                        `bson:"numOfAbsenceSessions" json:"numOfAbsenceSessions"`
	NumOfAllAssignments     int                        `bson:"numOfAllAssignments" json:"numOfAllAssignments"`
	NumOfDoneAssignments    int                        `bson:"numOfDoneAssignments" json:"numOfDoneAssignments"`
	NumOfNotDoneAssignments int                        `bson:"numOfNotDoneAssignments" json:"numOfNotDoneAssignments"`
	NumOfPositiveRatings    int                        `bson:"numOfPositiveRatings" json:"numOfPositiveRatings"` // Rating >= 3
	NumOfNegativeRatings    int                        `bson:"numOfNegativeRatings" json:"numOfNegativeRatings"` // Rating < 3
}

// StudentCoachingStatsPagination wraps paginated student coaching stats
type StudentCoachingStatsPagination struct {
	Stats      []StudentCoachingStatsResponse `bson:"stats" json:"stats"`
	Pagination types.Pagination               `bson:"pagination" json:"pagination"`
}

// StudentSessionListPagination wraps paginated student sessions list
type StudentSessionListPagination struct {
	Sessions   []StudentSession `bson:"sessions" json:"sessions"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

// StudentCoachingSessionSlot is a minimal session view with date/from/to for mobile
type StudentCoachingSessionSlot struct {
	Id         primitive.ObjectID          `json:"id"`
	Date       time.Time                   `json:"date"`
	From       string                      `json:"from"`
	To         string                      `json:"to"`
	Status     enums.StudentSessionStatus  `json:"status"`
	SessionNum int                         `json:"sessionNum"`
}

// StudentCoachingSessionsResponse is the response for a student's sessions in one coaching program
type StudentCoachingSessionsResponse struct {
	Sessions    []StudentCoachingSessionSlot  `json:"sessions"`
	NextSession *StudentCoachingSessionSlot   `json:"nextSession,omitempty"`
}

type MyCoachingProgramResponse struct {
	Id          primitive.ObjectID         `bson:"_id" json:"_id"`
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Thumbnail   types.FileField            `bson:"thumbnail" json:"thumbnail"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Sessions    []StudentSession           `bson:"sessions" json:"sessions"`
	Coachs      []primitive.ObjectID       `bson:"coachs" json:"coachs"`
	Supervisors []SupervisorBasicInfo      `bson:"supervisors" json:"supervisors"`
	Progress    int                        `bson:"progress" json:"progress"`
	EnrolledAt  time.Time                  `bson:"enrolledAt" json:"enrolledAt"`
	SchoolObj   *School                    `bson:"schoolObj,omitempty" json:"schoolObj,omitempty"`
}

type StudentResourceInfo struct {
	GeneralInfo struct {
		FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
		LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
		Picture   types.FileField            `bson:"picture" json:"picture"`
	} `bson:"generalInfo" json:"generalInfo"`
	AccountStudentSetting struct {
		Email string `bson:"email" json:"email"`
	} `bson:"accountStudentSetting" json:"accountStudentSetting"`
}

type StudentSessionResourceResponse struct {
	StudentId  primitive.ObjectID  `bson:"studentId" json:"studentId"`
	Student    StudentResourceInfo `bson:"student" json:"student"`
	Notes      []SessionNote       `bson:"notes,omitempty" json:"notes,omitempty"`
	Agendas    []SessionAgenda     `bson:"agendas,omitempty" json:"agendas,omitempty"`
	Resources  []SessionResource   `bson:"resources,omitempty" json:"resources,omitempty"`
	Recordings []types.FileField   `bson:"recordings,omitempty" json:"recordings,omitempty"`
}
