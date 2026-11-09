package models

import (
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

// StudentQuestion represents a question asked by a student about a course or coaching
type StudentQuestion struct {
	Id                 primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId          primitive.ObjectID `bson:"studentId" json:"studentId"`
	RefType            string             `bson:"refType" json:"refType"` // "course" or "coaching"
	StudentQuestionDto `bson:",inline"`
	Reply              *StudentQuestionReply `bson:"reply,omitempty" json:"reply,omitempty"` // Single embedded reply
	CreatedAt          time.Time             `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt          time.Time             `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy          primitive.ObjectID    `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy          primitive.ObjectID    `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash              bool                  `bson:"trash" json:"trash"`
}

func (s *StudentQuestion) CollName() string {
	return "academyStudentQuestions"
}

// StudentQuestionDto contains the question data
type StudentQuestionDto struct {
	RefId       primitive.ObjectID  `bson:"refId" json:"refId" validate:"required"`       // Course ID or Coaching ID
	LessonId    *primitive.ObjectID `bson:"lessonId,omitempty" json:"lessonId,omitempty"` // Optional lesson ID (only valid when refType is "course")
	Title       string              `bson:"title" json:"title" validate:"required"`
	Description string              `bson:"description" json:"description" validate:"required"`
}

// StudentQuestionReply represents a reply to a student question (from teacher/supervisor/admin)
// Embedded in StudentQuestion document
type StudentQuestionReply struct {
	Id                      primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	UserId                  primitive.ObjectID `bson:"userId" json:"userId"` // Teacher/Supervisor/Admin ID
	StudentQuestionReplyDto `bson:",inline"`
	CreatedAt               time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt               time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	CreatedBy               primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy               primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash                   bool               `bson:"trash" json:"trash"`
}

// StudentQuestionReplyDto contains the reply data
type StudentQuestionReplyDto struct {
	Content string `bson:"content" json:"content" validate:"required"`
}

// StudentQuestionStudentInfo contains enriched student information for student questions
type StudentQuestionStudentInfo struct {
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email     string                     `bson:"email" json:"email"`
	Picture   types.FileField           `bson:"picture,omitempty" json:"picture,omitempty"`
}

// StudentQuestionTeacherInfo contains enriched teacher information for student questions
type StudentQuestionTeacherInfo struct {
	FirstName transl.Localizable[string] `bson:"firstName" json:"firstName"`
	LastName  transl.Localizable[string] `bson:"lastName" json:"lastName"`
	Email     string                     `bson:"email" json:"email"`
	Picture   types.FileField           `bson:"picture,omitempty" json:"picture,omitempty"`
}

// StudentQuestionRefBasicInfo contains basic info for course or lesson
type StudentQuestionRefBasicInfo struct {
	Id   primitive.ObjectID         `bson:"_id" json:"_id"`
	Name transl.Localizable[string] `bson:"name" json:"name"`
}

// StudentQuestionRefInfo contains enriched ref (course or coaching) and optional lesson information
type StudentQuestionRefInfo struct {
	Course   *StudentQuestionRefBasicInfo `bson:"course,omitempty" json:"course,omitempty"`     // Only when refType is "course"
	Coaching *StudentQuestionRefBasicInfo `bson:"coaching,omitempty" json:"coaching,omitempty"` // Only when refType is "coaching"
	Lesson   *StudentQuestionRefBasicInfo `bson:"lesson,omitempty" json:"lesson,omitempty"`     // Optional, only if lessonId exists and refType is "course"
}

// StudentQuestionRes represents a question with populated user data and replies
type StudentQuestionRes struct {
	StudentQuestion `bson:",inline"`
	Reply           *StudentQuestionReplyRes    `bson:"-" json:"reply,omitempty"`                   // Not unmarshaled from DB, populated manually
	Student         *StudentQuestionStudentInfo `bson:"student,omitempty" json:"student,omitempty"` // Populated from aggregation
	ReplyTeacher    *StudentQuestionTeacherInfo `bson:"replyTeacher,omitempty" json:"-"`            // Populated from aggregation, used to populate Reply
	Ref             *StudentQuestionRefInfo     `bson:"ref,omitempty" json:"ref,omitempty"`         // Populated from aggregation
}

// StudentQuestionReplyRes represents a reply with populated user data
type StudentQuestionReplyRes struct {
	StudentQuestionReply `bson:",inline"`
	Teacher              *StudentQuestionTeacherInfo `bson:"teacher,omitempty" json:"teacher,omitempty"`
}

// StudentQuestionPagination represents paginated student questions
type StudentQuestionPagination struct {
	Questions  []StudentQuestionRes `bson:"questions" json:"questions"`
	Pagination types.Pagination     `bson:"pagination" json:"pagination"`
}

// StudentQuestionTeacherStats contains statistics for a teacher's questions
type StudentQuestionTeacherStats struct {
	TotalQuestions int64 `json:"totalQuestions"` // Total questions for courses teacher is assigned to
	TotalReplied   int64 `json:"totalReplied"`   // Questions that have replies (reply exists)
	// totalUnreplied can be calculated in frontend: totalQuestions - totalReplied
}

// StudentQuestionTeacherBreakdown contains per-teacher statistics
type StudentQuestionTeacherBreakdown struct {
	Id             primitive.ObjectID         `json:"_id"`
	FirstName      transl.Localizable[string] `json:"firstName"`
	LastName       transl.Localizable[string] `json:"lastName"`
	TotalQuestions int64                      `json:"totalQuestions"`
	TotalReplied   int64                      `json:"totalReplied"`
	AvgReplyTime   float64                    `json:"avgReplyTime"` // in hours
}

// StudentQuestionSupervisorStats contains aggregate statistics for supervisor's teachers
type StudentQuestionSupervisorStats struct {
	// Aggregate stats (same on every page)
	TotalQuestions int64   `json:"totalQuestions"`
	TotalReplied   int64   `json:"totalReplied"`
	AvgReplyTime   float64 `json:"avgReplyTime"` // in hours, weighted average
	// Paginated teachers array
	Teachers []StudentQuestionTeacherBreakdown `json:"teachers"`
	// Pagination info
	Pagination types.Pagination `json:"pagination"`
}
