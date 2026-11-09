package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// EnrolledCourseResponse represents a course with enriched enrollment data
// EnrolledCourseResponse represents a course with enriched enrollment data
type EnrolledCourseResponse struct {
	Course                     CourseEnriched        `bson:"course" json:"course"`
	Enrollment                 StudentCourse         `bson:"enrollment" json:"enrollment"`
	StudentStatus              string                `bson:"studentStatus" json:"studentStatus"`
	Teachers                   []TeacherBasicInfo    `bson:"teachers" json:"teachers"`
	Supervisors                []SupervisorBasicInfo `bson:"supervisors" json:"supervisors"`
	TotalLessons               int                   `bson:"totalLessons" json:"totalLessons"`
	CompletedLessons           int                   `bson:"completedLessons" json:"completedLessons"`
	CompletedSessionsBySection map[string]int        `bson:"completedSessionsBySection,omitempty" json:"completedSessionsBySection,omitempty"`
	UpcomingLiveSession        *LiveSessionInfo      `bson:"upcomingLiveSession,omitempty" json:"upcomingLiveSession,omitempty"`
}

// CourseEnriched contains full course details
type CourseEnriched struct {
	Id          primitive.ObjectID         `bson:"_id" json:"_id"`
	BasicInfo   CourseBasicInfo            `bson:"basicInfo" json:"basicInfo"`
	Content     []Section                  `bson:"content,omitempty" json:"-"`
	Thumbnail   types.FileField            `bson:"thumbnail" json:"thumbnail"`
	Description transl.Localizable[string] `bson:"description" json:"description"`
	Status      string                     `bson:"status" json:"status"`
	Type        string                     `bson:"type" json:"type"`
	Duration    int                        `bson:"duration" json:"duration"`
	Rating      *float64                   `bson:"rating" json:"rating"`
	CreatedAt   time.Time                  `bson:"createdAt" json:"createdAt"`
	UpdatedAt   time.Time                  `bson:"updatedAt" json:"updatedAt"`
}

// TeacherBasicInfo contains basic teacher information
type TeacherBasicInfo struct {
	Id        primitive.ObjectID         `bson:"_id" json:"_id"`
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email     string                     `bson:"email" json:"email"`
	Picture   types.FileField            `bson:"picture" json:"picture"`
}

// SupervisorBasicInfo contains basic supervisor information
type SupervisorBasicInfo struct {
	Id        primitive.ObjectID         `bson:"_id" json:"_id"`
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email     string                     `bson:"email" json:"email"`
	Picture   types.FileField            `bson:"picture" json:"picture"`
}

// LiveSessionInfo contains information about upcoming live session
type LiveSessionInfo struct {
	Title     transl.Localizable[string] `bson:"title" json:"title"`
	Schedule  time.Time                  `bson:"schedule" json:"schedule"`
	Thumbnail types.FileField            `bson:"thumbnail" json:"thumbnail"`
	Url       string                     `bson:"url" json:"url"`
	Active    bool                       `bson:"active" json:"active"`
}

// EnrolledCoursePagination wraps paginated enrolled courses
type EnrolledCoursePagination struct {
	Courses    []EnrolledCourseResponse `bson:"courses" json:"courses"`
	Pagination types.Pagination         `bson:"pagination" json:"pagination"`
}
