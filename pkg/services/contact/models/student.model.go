package models

import (
	"larsa-academy-microservice/pkg/services/contact/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentDto struct {
	GeneralInfo           GeneralInfo           `bson:"generalInfo" json:"generalInfo"`
	ContactInfo           ContactInfo           `bson:"contactInfo" json:"contactInfo"`
	AcademicInfo          AcademicInfo          `bson:"academicInfo" json:"academicInfo"`
	Other                 Other                 `bson:"other" json:"other"`
	AccountStudentSetting AccountStudentSetting `bson:"accountStudentSetting" json:"accountStudentSetting"`
	AddressDetails        AddressDetails        `bson:"addressDetails" json:"addressDetails"`
	Favorites             []StudentFavorite     `bson:"favorites" json:"favorites"`
}

type GeneralInfo struct {
	FirstName   transl.Localizable[string] `bson:"firstName" json:"firstName" validate:"required"`
	LastName    transl.Localizable[string] `bson:"lastName" json:"lastName" validate:"required"`
	Gender      string                     `bson:"gender" json:"gender"`
	DateOfBirth time.Time                  `bson:"dateOfBirth" json:"dateOfBirth"`
	Nationality string                     `bson:"nationality" json:"nationality"`
	Status      enums.StudentStatus        `bson:"status" json:"status" validate:"required,oneof=active pending suspend"`
	Picture     types.FileField            `bson:"picture" json:"picture"`
}

type PhoneNumber struct {
	Prefix  string `bson:"prefix" json:"prefix" validate:"required"`   // Country code
	Content string `bson:"content" json:"content" validate:"required"` // Actual phone number
}

type ContactInfo struct {
	MobilePhoneNumber PhoneNumber `bson:"mobilePhoneNumber" json:"mobilePhoneNumber" validate:"required"`
	ParentPhoneNumber PhoneNumber `bson:"parentPhoneNumber" json:"parentPhoneNumber"`
}

type AcademicInfo struct {
	EnrolledCourse []primitive.ObjectID `bson:"enrolledCourse" json:"enrolledCourse"`

	// new
	// RegistrationDate      time.Time       `bson:"registrationDate" json:"registrationDate"`
	Stage string `bson:"stage" json:"stage"`
	Class string `bson:"class" json:"class"`

	EnrollmentDate time.Time `bson:"enrollmentDate" json:"enrollmentDate"`
}

type AddressDetails struct {
	Country        string `bson:"country" json:"country"`
	City           string `bson:"city" json:"city"`
	Street         string `bson:"street" json:"street"`
	AdditionalInfo string `bson:"additionalInfo" json:"additionalInfo"`
}

type Other struct {
	NotesList []NoteObject `bson:"notesList" json:"notesList"`
}

type NoteObject struct {
	Sort           int                `bson:"sort" json:"sort"`
	CreatedBy      primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	Description    string             `bson:"description" json:"description"`
	CreatedByName  string             `bson:"createdByName" json:"createdByName"`
	CreatedByImage types.FileField    `bson:"createdByImage" json:"createdByImage"`
	CreatedAt      time.Time          `bson:"createdAt" json:"createdAt"`
}

type StudentFavorite struct {
	Type  string             `bson:"type" json:"type" validate:"required,oneof=course coaching bundle"`
	RefId primitive.ObjectID `bson:"refId" json:"refId" validate:"required"`
	Meta  map[string]any     `bson:"meta" json:"meta"`
	Data  any                `bson:"-" json:"data,omitempty"` // Enriched data (not stored in DB)
}

// UniqueKey returns a unique identifier for the favorite (type:refId)
func (f StudentFavorite) UniqueKey() string {
	return f.Type + ":" + f.RefId.Hex()
}

// Equals checks if two favorites are the same
func (f StudentFavorite) Equals(other StudentFavorite) bool {
	return f.Type == other.Type && f.RefId == other.RefId
}

type AccountStudentSetting struct {
	IsActive          bool   `bson:"isActive" json:"isActive"`
	Email             string `bson:"email" json:"email"`
	InitialPass       string `bson:"initialPass" json:"initialPass"`
	ConfirmPass       string `bson:"confirmPass" json:"confirmPass"`
	ResetOnFirstLogin bool   `bson:"resetOnFirstLogin" json:"resetOnFirstLogin"`
	JoinStatus        string `bson:"joinStatus" json:"joinStatus"`
}

type Student struct {
	Id                     primitive.ObjectID `bson:"_id" json:"_id"`
	Seq                    string             `bson:"seq" json:"seq"`
	StudentDto             `bson:",inline"`
	ParentStudentRelations []*ParentStudentRelation `bson:"parentStudentRelations" json:"parentStudentRelations,omitempty"`

	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash     bool               `bson:"trash" json:"trash"`
}

func (s *Student) CollName() string {
	return "academyStudents"
}

type StudentsPagination struct {
	Students   []StudentWithStats `bson:"students" json:"students"`
	Pagination types.Pagination   `bson:"pagination" json:"pagination"`
}

type EnrichedParent struct {
	ParentShortInfo `json:",inline"`
	RelationType    string `json:"relationType"`
}

// StudentWithStats extends Student with additional statistics
type StudentWithStats struct {
	Student               `bson:",inline"`
	Parents               []EnrichedParent `json:"parents" bson:"-"` // Enrichment field for API
	TotalEnrolledCourses  int              `bson:"totalEnrolledCourses" json:"totalEnrolledCourses"`
	TotalPoints           int              `bson:"totalPoints" json:"totalPoints"`
	CourseCompletionRate  float64          `bson:"courseCompletionRate" json:"courseCompletionRate"`   // Percentage of fully completed courses
	AverageCourseProgress float64          `bson:"averageCourseProgress" json:"averageCourseProgress"` // Average progress (0-100) across all enrolled courses
}

func (s *Student) GetUserData() map[string]any {
	m := map[string]any{
		"firstName": s.GeneralInfo.FirstName.GetContentByLang("ar"),
		"lastName":  s.GeneralInfo.LastName.GetContentByLang("ar"),
		"email":     s.AccountStudentSetting.Email, // Changed from ContactInfo.EmailAddress
	}

	// Add phone number from ContactInfo (same as public registration)
	if s.ContactInfo.MobilePhoneNumber.Prefix != "" && s.ContactInfo.MobilePhoneNumber.Content != "" {
		m["phone"] = s.ContactInfo.MobilePhoneNumber.Prefix + s.ContactInfo.MobilePhoneNumber.Content
	}

	// Add picture if available
	if s.GeneralInfo.Picture.Path != "" {
		m["picture"] = s.GeneralInfo.Picture.Path
	}

	if s.AccountStudentSetting.InitialPass != "" {
		m["password"] = s.AccountStudentSetting.InitialPass
		m["passwordConfirmation"] = s.AccountStudentSetting.ConfirmPass
	}

	return m
}

type StudentRegistrationDto struct {
	// Exclude AccountStudentSetting and Status from embedded StudentDto for public registration
	GeneralInfo          GeneralInfoPublic `bson:"generalInfo" json:"generalInfo"` // Excludes Status (set to "pending" by service)
	ContactInfo          ContactInfo       `bson:"contactInfo" json:"contactInfo"`
	AcademicInfo         AcademicInfo      `bson:"academicInfo" json:"academicInfo"`
	Other                Other             `bson:"other" json:"other"`
	AddressDetails       AddressDetails    `bson:"addressDetails" json:"addressDetails"`
	Email                string            `json:"email" validate:"required,email"`                                 // Student's email (top-level, not in accountStudentSetting)
	Password             string            `json:"password" validate:"required,min=8"`                              // Student's password for registration (min 8 chars)
	PasswordConfirmation string            `json:"passwordConfirmation" validate:"required,min=8,eqfield=Password"` // Password confirmation
	PhoneOTP             *string           `json:"phoneOTP"`
}

// ToStudentDto converts StudentRegistrationDto to StudentDto for internal use
func (dto *StudentRegistrationDto) ToStudentDto() StudentDto {
	return StudentDto{
		GeneralInfo: GeneralInfo{
			FirstName:   dto.GeneralInfo.FirstName,
			LastName:    dto.GeneralInfo.LastName,
			Gender:      dto.GeneralInfo.Gender,
			DateOfBirth: dto.GeneralInfo.DateOfBirth,
			Nationality: dto.GeneralInfo.Nationality,
			Picture:     dto.GeneralInfo.Picture,
			// Status will be set to "pending" by service layer
		},
		ContactInfo:    dto.ContactInfo,
		AcademicInfo:   dto.AcademicInfo,
		Other:          dto.Other,
		AddressDetails: dto.AddressDetails,
		// AccountStudentSetting will be set by service layer
	}
}

// StudentProfileResponseDto - Safe fields for GET /me endpoint (excludes sensitive data)
type StudentProfileResponseDto struct {
	Id                     string                   `json:"id"`
	GeneralInfo            GeneralInfo              `json:"generalInfo"` // Includes Status
	ContactInfo            ContactInfo              `json:"contactInfo"`
	AcademicInfo           AcademicInfo             `json:"academicInfo"`
	AddressDetails         AddressDetails           `json:"addressDetails"`
	Other                  Other                    `json:"other"` // Includes NotesList
	Email                  string                   `json:"email"` // Only email from AccountStudentSetting
	Favorites              []StudentFavorite        `json:"favorites"`
	ParentStudentRelations []*ParentStudentRelation `json:"parentStudentRelations"`
	CreatedAt              time.Time                `json:"createdAt"`
	UpdatedAt              time.Time                `json:"updatedAt"`
}

// GeneralInfoPublic - Public version of GeneralInfo (excludes Status)
type GeneralInfoPublic struct {
	FirstName   transl.Localizable[string] `json:"firstName"`
	LastName    transl.Localizable[string] `json:"lastName"`
	Gender      string                     `json:"gender"`
	DateOfBirth time.Time                  `json:"dateOfBirth"`
	Nationality string                     `json:"nationality"`
	Picture     types.FileField            `json:"picture"`
}

// StudentProfileUpdateDto - Only fields students can update via PATCH /me
type StudentProfileUpdateDto struct {
	GeneralInfo    GeneralInfoPublic `json:"generalInfo"` // Excludes Status
	ContactInfo    ContactInfo       `json:"contactInfo"`
	AddressDetails AddressDetails    `json:"addressDetails"`
	// AcademicInfo is excluded - enrollment should be admin-controlled
	// AccountStudentSetting is excluded - all fields are admin-only
	// Other.NotesList is excluded - admin notes only
}

type StudentSummary struct {
	Student                Student `bson:"student" json:"student"`
	TotalLearningHours     float64 `bson:"totalLearningHours" json:"totalLearningHours"`
	ActiveCoursesCount     int     `bson:"activeCoursesCount" json:"activeCoursesCount"`
	ActiveCoachingsCount   int     `bson:"activeCoachingsCount" json:"activeCoachingsCount"`
	TotalSessionsCompleted int     `bson:"totalSessionsCompleted" json:"totalSessionsCompleted"`
	QuizScoreAverage       float64 `bson:"quizScoreAverage" json:"quizScoreAverage"`
	BadgesEarned           int     `bson:"badgesEarned" json:"badgesEarned"`
	MessagesWithTeacher    int     `bson:"messagesWithTeacher" json:"messagesWithTeacher"`
}

type StudentSessionLog struct {
	CoachingId    primitive.ObjectID         `json:"coachingId"`
	CoachingTitle transl.Localizable[string] `json:"coachingTitle"`
	SessionName   transl.Localizable[string] `json:"sessionName"`
	Description   transl.Localizable[string] `json:"description"`
	Recordings    []types.FileField          `json:"recordings"`
	Notes         []interface{}              `json:"notes"`
	Resources     []types.FileField          `json:"resources"`
	CreatedAt     time.Time                  `json:"createdAt"`
}

type StudentSessionLogPagination struct {
	Sessions   []StudentSessionLog `json:"sessions"`
	Pagination types.Pagination    `json:"pagination"`
}

// StudentFavoritePagination wraps paginated enriched favorites
type StudentFavoritePagination struct {
	Favorites  []StudentFavorite `json:"favorites"`
	Pagination types.Pagination  `json:"pagination"`
}

type StudentSuggestionRequestDto struct {
	Class   string `json:"class"`
	Country string `json:"country"`
	Limit   int    `json:"limit"`
}

type StudentSuggestionResponseDto struct {
	Courses   []any `json:"courses"`
	Coachings []any `json:"coachings"`
}

// StudentNote represents a rich note for a student with markdown content
type StudentNote struct {
	Id        primitive.ObjectID `bson:"_id" json:"_id"`
	StudentId primitive.ObjectID `bson:"studentId" json:"studentId"`
	Title     string             `bson:"title" json:"title"`
	Tags      []string           `bson:"tags" json:"tags"`
	Content   string             `bson:"content" json:"content"` // Markdown content with embedded file references
	Sort      int                `bson:"sort" json:"sort"`       // For reordering notes (lower numbers = higher priority)
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash     bool               `bson:"trash" json:"trash"`
}

func (s *StudentNote) CollName() string {
	return "academyStudentNotes"
}

// StudentNoteCreateDto for creating new notes
type StudentNoteCreateDto struct {
	Title   string   `json:"title" validate:"required"`
	Tags    []string `json:"tags"`
	Content string   `json:"content" validate:"required"`
}

// StudentNoteUpdateDto for updating notes
type StudentNoteUpdateDto struct {
	Title   *string  `json:"title,omitempty"`
	Tags    []string `json:"tags,omitempty"`
	Content *string  `json:"content,omitempty"`
}

// StudentNotePositionDto for updating note position
type StudentNotePositionDto struct {
	Position int `json:"position" validate:"min=0"`
}

// StudentNotePagination wraps paginated notes
type StudentNotePagination struct {
	Notes      []StudentNote    `json:"notes"`
	Pagination types.Pagination `json:"pagination"`
}
