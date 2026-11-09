package models

import (
	"errors"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type CoursePagination struct {
	Courses    []EnrichedCourse `bson:"course" json:"course"`
	Pagination types.Pagination `bson:"pagination" json:"pagination"`
}

type Course struct {
	Id        primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	CourseDto `bson:",inline"`
	CreatedAt time.Time          `bson:"createdAt" json:"createdAt"`
	UpdatedAt time.Time          `bson:"updatedAt" json:"updatedAt"`
	CreatedBy primitive.ObjectID `bson:"createdBy" json:"createdBy"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy" json:"updatedBy"`
	Trash     bool               `bson:"trash" json:"trash"`
	Rating    *float64           `bson:"rating,omitempty" json:"rating,omitempty"`
}

type EnrichedCourse struct {
	Course       `bson:",inline"`
	Subjects     []Subject          `bson:"subjects,omitempty" json:"subjects,omitempty"`
	SchoolObj    *School            `bson:"schoolObj,omitempty" json:"schoolObj,omitempty"`
	StudentCount *int64             `bson:"studentCount,omitempty" json:"studentCount,omitempty"`
	TeachersObjs []TeacherBasicInfo `bson:"teachersObjs,omitempty" json:"teachersObjs,omitempty"`
	Pricing      *CoursePricing     `bson:"pricing,omitempty" json:"pricing,omitempty"`
	TagsObj      []CourseCategory   `bson:"tagsobj,omitempty" json:"tagsobj,omitempty"`
}

type StudentCourseResponse struct {
	Course       `bson:",inline"`
	Subjects     []Subject          `bson:"subjects,omitempty" json:"subjects,omitempty"`
	StudentCount *int64             `bson:"studentCount,omitempty" json:"studentCount,omitempty"`
	TeachersObjs []TeacherBasicInfo `bson:"teachersObjs,omitempty" json:"teachersObjs,omitempty"`
	Pricing      *CoursePricing     `bson:"pricing,omitempty" json:"pricing,omitempty"`
	IsFavorite   bool               `json:"isFavorite"`
	IsEnrolled   bool               `json:"isEnrolled"`
}

type StudentCoursePagination struct {
	Courses    []StudentCourseResponse `bson:"course" json:"course"`
	Pagination types.Pagination        `bson:"pagination" json:"pagination"`
}

// StudentLearningStatsResponse combines course and coaching stats for a student
type StudentLearningStatsResponse struct {
	CourseStats   *StudentCourseStatsResponse     `json:"courseStats"`
	CoachingStats *StudentCoachingStatsPagination `json:"coachingStats"`
}

// TeacherCourseResponse represents a course with student count for teacher endpoints
type TeacherCourseResponse struct {
	Course       Course `json:"course"`
	StudentCount int    `json:"studentCount"`
}

// TeacherCoursePagination represents paginated teacher courses with student counts
type TeacherCoursePagination struct {
	Courses    []TeacherCourseResponse `json:"courses"`
	Pagination types.Pagination        `json:"pagination"`
}

func (c *Course) CollName() string {
	return "academyCourses"
}

func (c *Course) GetName() transl.Localizable[string] {
	return c.BasicInfo.Name
}

type CourseDto struct {
	BasicInfo CourseBasicInfo `bson:"basicInfo" json:"basicInfo" validate:"required"`
	Content   []Section       `bson:"content" json:"content"`
	Quiz      Quiz            `bson:"quiz" json:"quiz"`
	//Pricing     []CoursePricing   `bson:"pricing" json:"pricing"`
	Certificate CourseCertificate `bson:"certificate" json:"certificate"`
	Settings    CourseSettings    `bson:"settings" json:"settings"`
}

func (c *CourseDto) Validate(v *validator.Validate) error {
	if err := helpers.GenericValidation(v, c); err != nil {
		return err
	}

	if c.BasicInfo.SchoolType == "private" {
		if c.BasicInfo.SchoolId.IsZero() {
			return errors.New("schoolId is required for private school type")
		}
	} else if c.BasicInfo.SchoolType == "public" {
		if !c.BasicInfo.SchoolId.IsZero() {
			return errors.New("schoolId should not be defined for public school type")
		}
	}

	return nil
}

type CourseBasicInfo struct {
	Name        transl.Localizable[string] `bson:"name" json:"name" validate:"required"`
	Type        string                     `bson:"type" json:"type" validate:"required,oneof=curriculum foundational"`
	Status      string                     `bson:"status" json:"status"` // active - inactive
	Version     string                     `bson:"version" json:"version"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Thumbnail   types.FileField            `bson:"thumbnail" json:"thumbnail"`
	// Media is an optional marketing gallery (images/videos) shown before purchase
	Media          []MediaItem          `bson:"media,omitempty" json:"media,omitempty"`
	LinkedSubjects []primitive.ObjectID `bson:"linkedSubjects" json:"linkedSubjects"`
	Country        string               `bson:"country" json:"country"`
	SchoolType     string               `bson:"schoolType" json:"schoolType" validate:"required,oneof=private public"`
	SchoolId       primitive.ObjectID   `bson:"schoolId,omitempty" json:"schoolId,omitempty"`
	GradeLevel     string               `bson:"gradeLevel" json:"gradeLevel"`
	Level          string               `bson:"level" json:"level" validate:"required,oneof=beginner intermediate advanced"`
	Tags           []primitive.ObjectID `bson:"tags" json:"tags"`
	MaxStudents    int                  `bson:"maxStudents" json:"maxStudents"`
	Duration       int                  `bson:"duration" json:"duration"`
	Teachers       []primitive.ObjectID `bson:"teachers" json:"teachers"`
	SupervisorIds  []primitive.ObjectID `bson:"supervisorIds,omitempty" json:"supervisorIds,omitempty"` // Direct supervisor IDs synced from SupervisorAssignment
}

type Section struct {
	Id          primitive.ObjectID         `bson:"_id" json:"_id"`
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Lessons     []Lesson                   `bson:"lessons" json:"lessons"`
	SubSections []Section                  `bson:"subSections" json:"subSections"`
}

type Lesson struct {
	Id          primitive.ObjectID         `bson:"_id" json:"_id"`
	Name        transl.Localizable[string] `bson:"name" json:"name"`
	Image       types.FileField            `bson:"image" json:"image"`
	Content     []Content                  `bson:"content" json:"content"`
	Attachments []types.FileField          `bson:"attachments" json:"attachments"`
}

type Content struct {
	Id    primitive.ObjectID         `bson:"_id,omitempty" json:"_id,omitempty"`
	Title transl.Localizable[string] `bson:"title" json:"title"`
	Type  string                     `bson:"type" json:"type"` // video - audio - file - text - quiz
	Data  map[string]any             `bson:"data" json:"data"`
	Meta  map[string]any             `bson:"meta" json:"meta"`
}

type Quiz struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Questions   []Question                 `bson:"questions" json:"questions"`
	AIAssistant QuizAIAssistant            `bson:"aiAssistant" json:"aiAssistant"`
	Settings    QuizSettings               `bson:"settings" json:"settings"`
}

type Question struct {
	Num        int                        `bson:"num" json:"num"`
	AnswerType string                     `bson:"answerType" json:"answerType" validate:"required,oneof=multi single"`
	Question   transl.Localizable[string] `bson:"question" json:"question" validate:"required"`
	Image      types.FileField            `bson:"image" json:"image"`
	Answers    []Answer                   `bson:"answers" json:"answers"`
}
type Answer struct {
	Num       int                        `bson:"num" json:"num"`
	Answer    transl.Localizable[string] `bson:"answer" json:"answer"`
	IsCorrect bool                       `bson:"isCorrect" json:"isCorrect"`
}

type QuizSettings struct {
	InstantAnswerFeedback bool `bson:"instantAnswerFeedback" json:"instantAnswerFeedback"`
	Compliance            bool `bson:"compliance" json:"compliance"`
	StudentComments       bool `bson:"studentComments" json:"studentComments"`
	PublicReview          bool `bson:"publicReview" json:"publicReview"`
}

type QuizAIAssistant struct {
	Sections []primitive.ObjectID `bson:"sections" json:"sections"`
	Lessons  []primitive.ObjectID `bson:"lessons" json:"lessons"`
}

// course settings
type CourseSettings struct {
	LiveSession LiveSessionSettings `bson:"liveSession" json:"liveSession"`
	Comments    CommentSettings     `bson:"commentSettings" json:"commentSettings"`
}

type LiveSessionSettings struct {
	Active             bool                       `bson:"active" json:"active"`
	Title              transl.Localizable[string] `bson:"title" json:"title"`
	Thumbnail          types.FileField            `bson:"thumbnail" json:"thumbnail"`
	Schedule           time.Time                  `bson:"schedule" json:"schedule"`
	Description        transl.Localizable[string] `bson:"description" json:"description"`
	UseExtrnalLink     bool                       `bson:"useExtrnalLink" json:"useExtrnalLink"`
	AppName            string                     `bson:"appName" json:"appName"`
	Url                string                     `bson:"url" json:"url"`
	UploadRecordings   bool                       `bson:"uploadRecordings" json:"uploadRecordings"`
	PublishAfterUpload bool                       `bson:"publishAfterUpload" json:"publishAfterUpload"`
	RepeatWeekly       bool                       `bson:"repeatWeekly" json:"repeatWeekly"`
}

type CommentSettings struct {
	Active             bool `bson:"active" json:"active"`
	DisableNewComments bool `bson:"disableNewComments" json:"disableNewComments"` //prev comments will remain , but no new comments will be allowed
}

// course certificate
type CourseCertificate struct {
	Title      transl.Localizable[string] `bson:"title" json:"title"`
	Subtitle   transl.Localizable[string] `bson:"subtitle" json:"subtitle"`
	Content    transl.Localizable[string] `bson:"content" json:"content"`
	Background types.FileField            `bson:"background" json:"background"`
	Header     CertificateHeader          `bson:"header" json:"header"`
	Footer     CertificateFooter          `bson:"footer" json:"footer"`
	Automation CertificateAutomation      `bson:"automation" json:"automation"`
}

type CertificateHeader struct {
	ShowCourseName  bool                       `bson:"showCourseName" json:"showCourseName"`
	CourseName      transl.Localizable[string] `bson:"courseName" json:"courseName"`
	ShowStudentName bool                       `bson:"showStudentName" json:"showStudentName"`
	ShowLogo        bool                       `bson:"showLogo" json:"showLogo"`
	Logo            types.FileField            `bson:"logo" json:"logo"`
}

type CertificateFooter struct {
	IncludeCompletionData bool                       `bson:"includeCompletionData" json:"includeCompletionData"`
	IncludeExpirationDate bool                       `bson:"includeExpirationDate" json:"includeExpirationDate"`
	ExpireAfterValue      int                        `bson:"expireAfterValue" json:"expireAfterValue"`
	ExpireAfterUnit       string                     `bson:"expireAfterUnit" json:"expireAfterUnit"`
	IncludeSerialNumber   bool                       `bson:"includeSerialNumber" json:"includeSerialNumber"`
	AddCustomText         bool                       `bson:"addCustomText" json:"addCustomText"`
	CustomText            transl.Localizable[string] `bson:"customText" json:"customText"`
}

type CertificateAutomation struct {
	Event string `bson:"event" json:"event"` // todo: reveiw use case later
}

// cart
type CourseSubscribe struct {
	CourseId     primitive.ObjectID `bson:"courseId" json:"courseId" validate:"required"`
	RenewMonthly bool               `bson:"renewMonthly" json:"renewMonthly" `
	Price        float64            `bson:"price" json:"price" `
}

// CourseSummaryStats represents statistics summary for a course
type CourseSummaryStats struct {
	EnrolledStudentsCount int      `bson:"enrolledStudentsCount" json:"enrolledStudentsCount"`
	CompletionRate        float64  `bson:"completionRate" json:"completionRate"`       // Percentage of students who completed the course
	AvgEngagementRate     float64  `bson:"avgEngagementRate" json:"avgEngagementRate"` // Average progress percentage across all enrolled students
	CourseRating          *float64 `bson:"courseRating" json:"courseRating"`           // Average rating from reviews
	TotalLessons          int      `bson:"totalLessons" json:"totalLessons"`           // Total number of lessons in the course
}

// StudentCountByCourseRow is one row for student-count-by-course analytics (admin/supervisor)
type StudentCountByCourseRow struct {
	CourseId     primitive.ObjectID         `json:"courseId"`
	CourseName   transl.Localizable[string] `json:"courseName,omitempty"`
	StudentCount int                        `json:"studentCount"`
}
