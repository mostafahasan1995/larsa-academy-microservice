package models

import (
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentCourse struct {
	Id                primitive.ObjectID     `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId         primitive.ObjectID     `bson:"studentId" json:"studentId"`
	CourseId          primitive.ObjectID     `bson:"courseId" json:"courseId"`
	BundleId          primitive.ObjectID     `bson:"bundleId" json:"bundleId"`
	StartDate         time.Time              `bson:"startDate" json:"startDate"`
	EndDate           time.Time              `bson:"endDate" json:"endDate"`
	Status            enums.EnrollmentStatus `bson:"status" json:"status"` // pending - active - completed - cancelled
	Certificate       types.FileField        `bson:"certificate" json:"certificate"`
	CertificateStatus string                 `bson:"certificateStatus" json:"certificateStatus"` // pending - generated - expired
	Progress          int                    `bson:"progress" json:"progress"`
	Logins            int                    `bson:"logins" json:"logins"`
	LastActivity      time.Time              `bson:"lastActivity" json:"lastActivity"`
	CompletedLessons  []primitive.ObjectID   `bson:"completedLessons" json:"completedLessons"`
	//
	CreatedBy primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash     bool               `bson:"trash" json:"trash"`
}

func (s *StudentCourse) CollName() string {
	return "academyCourseStudents"
}

type MemberCoursePagination struct {
	StudentCourses []StudentCourseRes `bson:"studentCourses" json:"studentCourses"`
	Pagination     types.Pagination   `bson:"pagination" json:"pagination"`
}

type StudentCourseRes struct {
	StudentCourse `bson:",inline"`
	FirstName     transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName      transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email         string                     `bson:"email" json:"email"`
}

type StudentCourseWithDetails struct {
	Course                     Course                     `bson:"course" json:"-"`
	Name                       transl.Localizable[string] `bson:"name" json:"name"`
	Description                transl.Localizable[string] `bson:"description" json:"description"`
	Thumbnail                  types.FileField            `bson:"thumbnail" json:"thumbnail"`
	Status                     string                     `bson:"status" json:"status"`
	Type                       string                     `bson:"type" json:"type"`
	Duration                   int                        `bson:"duration" json:"duration"`
	Content                    []Section                  `bson:"content" json:"content"`
	Rating                     *float64                   `bson:"rating" json:"rating"`
	Enrollment                 StudentCourse              `bson:"enrollment" json:"enrollment"`
	StudentStatus              string                     `bson:"studentStatus" json:"studentStatus"`
	Teachers                   []TeacherBasicInfo         `bson:"teachers" json:"teachers"`
	Supervisors                []SupervisorBasicInfo      `bson:"supervisors" json:"supervisors"`
	TotalLessons               int                        `bson:"totalLessons" json:"totalLessons"`
	CompletedLessons           int                        `bson:"completedLessons" json:"completedLessons"`
	CompletedSessionsBySection map[string]int             `bson:"completedSessionsBySection,omitempty" json:"completedSessionsBySection,omitempty"`
}

// StudentCourseStatsItem represents per-course progress stats for a student
type StudentCourseStatsItem struct {
	CourseId         primitive.ObjectID         `bson:"courseId" json:"courseId"`
	Name             transl.Localizable[string] `bson:"name" json:"name"`
	Thumbnail        types.FileField            `bson:"thumbnail" json:"thumbnail"`
	TotalLessons     int                        `bson:"totalLessons" json:"totalLessons"`
	CompletedLessons int                        `bson:"completedLessons" json:"completedLessons"`
	Progress         int                        `bson:"progress" json:"progress"` // 0-100
}

// StudentCourseStatsResponse represents aggregated course stats for a student
type StudentCourseStatsResponse struct {
	Courses           []StudentCourseStatsItem `bson:"courses" json:"courses"`
	TotalLessons      int                      `bson:"totalLessons" json:"totalLessons"`
	CompletedLessons  int                      `bson:"completedLessons" json:"completedLessons"`
	OverallProgress   float64                  `bson:"overallProgress" json:"overallProgress"`     // 0-100 across all lessons
	AverageCourseRank float64                  `bson:"averageCourseRank" json:"averageCourseRank"` // reserved for future use
}
