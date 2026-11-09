package models

import (
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CredData interface {
	GetUserData() map[string]interface{}
}

type TeacherDto struct {
	EmploymentStatus   string                     `bson:"employmentStatus" json:"employmentStatus"`
	FirstName          transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName           transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Gender             string                     `bson:"gender" json:"gender"`
	DateOfBirth        time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality        string                     `bson:"nationality" json:"nationality"`
	NationalID         string                     `bson:"nationalID" json:"nationalID"`
	HireDate           time.Time                  `bson:"hireDate" json:"hireDate"`
	JoiningDate        time.Time                  `bson:"joiningDate" json:"joiningDate"`
	Picture            types.FileField            `bson:"picture" json:"picture"`
	IdDocument         types.FileField            `bson:"idDocument" json:"idDocument"`
	Mobile             string                     `bson:"mobile" json:"mobile"`
	Email              string                     `bson:"email" json:"email"`
	Qualification      Qualification              `bson:"qualification" json:"qualification"`
	Subjects           []Subject                  `bson:"subjects" json:"subjects"`
	Address            Address                    `bson:"address" json:"address"`
	AccountSettings    AccountSettings            `bson:"accountSettings" json:"accountSettings"`
	Availability       Availability               `bson:"availability" json:"availability"`
	RoleAndPermissions RoleAndPermissions         `bson:"roleAndPermissions" json:"roleAndPermissions"`
}

type RoleAndPermissions struct {
	Roles       []primitive.ObjectID `bson:"roles" json:"roles"`
	Permissions []primitive.ObjectID `permissions:"_id" json:"permissions"`
}
type Qualification struct {
	AcademicSpecialization []string          `bson:"academicSpecialization" json:"academicSpecialization"`
	YearOfExperience       int               `bson:"yearOfExperience" json:"yearOfExperience"`
	HighestDegree          string            `bson:"highestDegree" json:"highestDegree"`
	University             string            `bson:"university" json:"university"`
	Cv                     types.FileField   `bson:"cv" json:"cv"`
	Certificates           []types.FileField `bson:"certificates" json:"certificates"`
	Contract               []types.FileField `bson:"contracts" json:"contracts"`
	Skills                 string            `bson:"skills" json:"skills"`
}

type Subject struct {
	SubjectID    string             `bson:"subjectID" json:"subjectID"`
	SubjectName  string             `bson:"subjectName" json:"subjectName"`
	Stage        string             `bson:"stage" json:"stage"`
	Class        []string           `bson:"class" json:"class"`
	Note         string             `bson:"note" json:"note"`
	SupervisorId primitive.ObjectID `bson:"supervisorId" json:"supervisorId"`
}

type Address struct {
	Country        string `bson:"country" json:"country"`
	City           string `bson:"city" json:"city"`
	Area           string `bson:"area" json:"area"`
	AdditionalInfo string `bson:"additionalInfo" json:"additionalInfo"`
}

type AccountSettings struct {
	IsActive          bool   `bson:"isActive" json:"isActive"`
	Email             string `bson:"email" json:"email"`
	InitialPass       string `bson:"initialPass" json:"initialPass"`
	ConfirmPass       string `bson:"confirmPass" json:"confirmPass"`
	ResetOnFirstLogin bool   `bson:"resetOnFirstLogin" json:"resetOnFirstLogin"`
	JoinStatus        string `bson:"joinStatus" json:"joinStatus"`
}

type Availability struct {
	DefaultWorkingHours   []Shift                                   `bson:"defaultWorkingHours" json:"defaultWorkingHours"`
	DayOfWeekAvailability map[enums.DayOfWeek]DayOfWeekAvailability `bson:"dayOfWeekAvailability" json:"dayOfWeekAvailability"`
	Scheduling            Scheduling                                `bson:"scheduling" json:"scheduling"`
}

type Scheduling struct {
	MaxNumOfReservations        SchedulingUnit `bson:"maxNumOfReservations" json:"maxNumOfReservations"`
	MinNotice                   SchedulingUnit `bson:"minNotice" json:"minNotice"`
	SessionDuration             SchedulingUnit `bson:"sessionDuration" json:"sessionDuration"`
	TimeIntervalBetweenSessions SchedulingUnit `bson:"timeIntervalBetweenSessions" json:"timeIntervalBetweenSessions"`
	TeacherCancelSessionGrace   SchedulingUnit `bson:"teacherCancelSessionGrace" json:"teacherCancelSessionGrace"`
	StudentCancelSessionGrace   SchedulingUnit `bson:"studentCancelSessionGrace" json:"studentCancelSessionGrace"`
}
type SchedulingUnit struct {
	Value int    `json:"value"`
	Unit  string `json:"unit"`
}

type DayOfWeekAvailability struct {
	IsCustom     bool    `bson:"isCustom" json:"isCustom"`
	IsSelected   bool    `bson:"isSelected" json:"isSelected"`
	WorkingHours []Shift `bson:"workingHours" json:"workingHours"`
}

type Shift struct {
	From string `bson:"from" json:"from"` //start time - 24h format
	To   string `bson:"to" json:"to"`     //end time - 24h format
}

func (t *Teacher) SetDefaults() {
	dayOfWeekAvailability := map[enums.DayOfWeek]DayOfWeekAvailability{
		enums.Sunday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
		enums.Monday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
		enums.Tuesday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
		enums.Wednesday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
		enums.Thursday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
		enums.Friday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
		enums.Saturday: {
			IsCustom:     false,
			IsSelected:   false,
			WorkingHours: []Shift{},
		},
	}

	t.Availability = Availability{
		DefaultWorkingHours:   []Shift{},
		DayOfWeekAvailability: dayOfWeekAvailability,
	}
}

type Teacher struct {
	Id         primitive.ObjectID `bson:"_id" json:"_id"`
	Seq        string             `bson:"seq" json:"seq"`
	TeacherDto `bson:",inline"`
	CreatedAt  time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt  time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy  primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy  primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash      bool               `bson:"trash" json:"trash"`
}

func (t *Teacher) CollName() string {
	return "academyTeachers"
}

// TeacherWithStats extends Teacher with additional calculated statistics
type TeacherWithStats struct {
	Teacher `bson:",inline"`
	Rating  float64 `bson:"rating,omitempty" json:"rating,omitempty"`
}

type TeacherPagination struct {
	Teachers   []TeacherWithStats `bson:"teachers" json:"teachers"`
	Pagination types.Pagination   `bson:"pagination" json:"pagination"`
}

type TeacherSummary struct {
	Teacher                Teacher       `bson:"teacher" json:"teacher"`
	AssignedStudents       []interface{} `bson:"assignedStudents" json:"assignedStudents"`
	TotalDeliveredSessions int           `bson:"totalDeliveredSessions" json:"totalDeliveredSessions"`
	AssignmentsCreated     int           `bson:"assignmentsCreated" json:"assignmentsCreated"`
	RatingsAvg             float64       `bson:"ratingsAvg" json:"ratingsAvg"`
	MessagesReceived       float64       `bson:"messagesReceived" json:"messagesReceived"`
}

// TeacherTodaySummary provides a summary of today's sessions for a teacher
// Note: Session fields use interface{} to avoid import cycle with content package
// They should contain EnrichedCalendarEvent objects from content/models package
type TeacherTodaySummary struct {
	Date                      string        `json:"date"`                      // Today's date (YYYY-MM-DD)
	TotalSessionsCount        int           `json:"totalSessionsCount"`        // Total number of sessions today
	CompletedSessionsCount    int           `json:"completedSessionsCount"`    // Number of completed sessions
	NotCompletedSessionsCount int           `json:"notCompletedSessionsCount"` // Number of not completed sessions
	CurrentSession            interface{}   `json:"currentSession,omitempty"`  // Ongoing session (EnrichedCalendarEvent)
	NextSession               interface{}   `json:"nextSession,omitempty"`     // Next upcoming session (EnrichedCalendarEvent)
	AllSessions               []interface{} `json:"allSessions"`               // All today's sessions (EnrichedCalendarEvent[])
}

func (t *Teacher) GetUserData() map[string]any {
	// Extract default language (ar) from Localizable fields
	firstName := t.FirstName.GetContentByLang("ar")
	lastName := t.LastName.GetContentByLang("ar")

	m := map[string]any{
		"firstName": firstName,
		"lastName":  lastName,
		"email":     t.AccountSettings.Email,
	}

	// Add picture if available
	if t.Picture.Path != "" {
		m["picture"] = t.Picture.Path
	}

	if t.Mobile != "" {
		m["phone"] = t.Mobile
	}

	if t.AccountSettings.InitialPass != "" {
		m["password"] = t.AccountSettings.InitialPass
		m["passwordConfirmation"] = t.AccountSettings.ConfirmPass
	}

	return m
}

// TeacherProfileResponseDto - Safe fields for GET /me endpoint (excludes sensitive data)
type TeacherProfileResponseDto struct {
	Id            string                     `json:"id"`
	FirstName     transl.Localizable[string] `json:"firstName"`
	LastName      transl.Localizable[string] `json:"lastName"`
	Gender        string                     `json:"gender"`
	DateOfBirth   time.Time                  `json:"dateOfBirth"`
	Nationality   string                     `json:"nationality"`
	Picture       types.FileField            `json:"picture"`
	Mobile        string                     `json:"mobile"`
	Email         string                     `json:"email"` // Only email from AccountSettings
	Qualification Qualification              `json:"qualification"`
	Subjects      []Subject                  `json:"subjects"`
	Address       Address                    `json:"address"`
	Availability  Availability               `json:"availability"`
	CreatedAt     time.Time                  `json:"createdAt"`
	UpdatedAt     time.Time                  `json:"updatedAt"`
	// Excluded: Seq, EmploymentStatus, NationalID, HireDate, JoiningDate, IdDocument,
	// AccountSettings (except email), RoleAndPermissions, CreatedBy, UpdatedBy, Trash
}

// TeacherProfileUpdateDto - Only fields teachers can update via PATCH /me
type TeacherProfileUpdateDto struct {
	FirstName     transl.Localizable[string] `json:"firstName"`
	LastName      transl.Localizable[string] `json:"lastName"`
	Gender        string                     `json:"gender"`
	DateOfBirth   time.Time                  `json:"dateOfBirth"`
	Nationality   string                     `json:"nationality"`
	Picture       types.FileField            `json:"picture"`
	Mobile        string                     `json:"mobile"`
	Qualification Qualification              `json:"qualification"`
	Address       Address                    `json:"address"`
	Availability  Availability               `json:"availability"`
	// Excluded: Email (account settings), Subjects (admin-controlled), EmploymentStatus,
	// NationalID, HireDate, JoiningDate, IdDocument, RoleAndPermissions
}
