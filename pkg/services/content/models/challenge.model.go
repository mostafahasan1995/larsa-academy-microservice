package models

import (
	"errors"
	"time"

	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/transl"
	"larsa-academy-microservice/pkg/types"

	"github.com/go-playground/validator/v10"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type Challenge struct {
	Id           primitive.ObjectID `bson:"_id,omitempty" json:"_id,omitempty"`
	ChallengeDto `bson:",inline"`
	CreatedBy    primitive.ObjectID `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy    primitive.ObjectID `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	CreatedAt    time.Time          `bson:"createdAt,omitempty" json:"createdAt,omitempty"`
	UpdatedAt    time.Time          `bson:"updatedAt,omitempty" json:"updatedAt,omitempty"`
	Trash        bool               `bson:"trash" json:"trash"`

	// Creator lightweight info (enriched)
	CreatedByName    string `bson:"createdByName,omitempty" json:"createdByName,omitempty"`
	CreatedByPicture string `bson:"createdByPicture,omitempty" json:"createdByPicture,omitempty"`

	// Populated fields (not stored in DB but can be read from aggregation)
	CourseObj *Course `bson:"courseObj,omitempty" json:"courseObj,omitempty"`

	// Populated statistics (not stored in DB but can be read from aggregation)
	TotalPoints      int     `bson:"totalPoints,omitempty" json:"totalPoints,omitempty"` // Total reward points for this challenge
	TotalSubmissions int     `bson:"totalSubmissions,omitempty" json:"totalSubmissions,omitempty"`
	UniqueStudents   int     `bson:"uniqueStudents,omitempty" json:"uniqueStudents,omitempty"`
	AverageScore     float64 `bson:"averageScore,omitempty" json:"averageScore,omitempty"`

	// Populated when listing challenges for a student: this student's submissions for this challenge
	SubmissionObj []ChallengeSubmission `bson:"submissionObj,omitempty" json:"submissionObj,omitempty"`
}

type ChallengeDto struct {
	Title            transl.Localizable[string] `bson:"title" json:"title" validate:"required"`
	// Scope defines the visibility of the challenge: "global" or "course"
	Scope            string                     `bson:"scope,omitempty" json:"scope,omitempty"`
	Course           primitive.ObjectID         `bson:"course" json:"course"`
	Level            string                     `bson:"level" json:"level"`
	Subject          string                     `bson:"subject" json:"subject"`
	Date             time.Time                  `bson:"date" json:"date"`
	TimeAvailability TimeAvailability           `bson:"timeAvailablity" json:"timeAvailablity"`
	// Duration is an optional duration in minutes for the challenge
	Duration         int                        `bson:"duration,omitempty" json:"duration,omitempty"`
	Description      transl.Localizable[string] `bson:"description" json:"description"`
	Quiz             ChallengeQuiz              `bson:"questions" json:"questions"`
	Settings         ChallengeSettings          `bson:"settings" json:"settings"`
}

type TimeAvailability struct {
	StartTime time.Time `bson:"startTime" json:"startTime"`
	EndTime   time.Time `bson:"endTime" json:"endTime"`
}

func (c *ChallengeDto) Validate(v *validator.Validate) error {
	if err := helpers.GenericValidation(v, c); err != nil {
		return err
	}

	// Normalize and validate scope
	if c.Scope == "" {
		c.Scope = "global"
	}

	if c.Scope != "global" && c.Scope != "course" {
		return errors.New("scope must be either 'global' or 'course'")
	}

	if c.Scope == "course" && c.Course.IsZero() {
		return errors.New("course is required when scope is 'course'")
	}

	if c.Duration < 0 {
		return errors.New("duration must be greater than or equal to 0")
	}

	return nil
}

func (c *Challenge) CollName() string {
	return "academyChallenges"
}

// UnmarshalBSON implements custom BSON unmarshaling to support both "timeAvailablity" (typo) and
// "timeAvailability" (correct spelling) keys for backward compatibility.
func (c *Challenge) UnmarshalBSON(data []byte) error {
	type challenge Challenge
	if err := bson.Unmarshal(data, (*challenge)(c)); err != nil {
		return err
	}
	if !c.TimeAvailability.StartTime.IsZero() || !c.TimeAvailability.EndTime.IsZero() {
		return nil
	}
	var raw bson.M
	if err := bson.Unmarshal(data, &raw); err != nil {
		return nil
	}
	v, ok := raw["timeAvailability"]
	if !ok {
		return nil
	}
	m, ok := v.(bson.M)
	if !ok {
		return nil
	}
	if t, ok := m["startTime"].(primitive.DateTime); ok {
		c.TimeAvailability.StartTime = t.Time()
	}
	if t, ok := m["endTime"].(primitive.DateTime); ok {
		c.TimeAvailability.EndTime = t.Time()
	}
	return nil
}

// challenge quiz
type ChallengeQuiz struct {
	Title       transl.Localizable[string] `bson:"title" json:"title"`
	Questions   []ChallengeQuizQuestion    `bson:"questions" json:"questions"`
	AIAssistant any                        `bson:"aiAssistant" json:"aiAssistant"`
	Settings    ChallengeQuizSettings      `bson:"settings" json:"settings"`
}

type ChallengeQuizQuestion struct {
	Id       primitive.ObjectID         `bson:"_id,omitempty" json:"_id,omitempty"`
	Question transl.Localizable[string] `bson:"question" json:"question"`
	Image    types.FileField            `bson:"image,omitempty" json:"image,omitempty"` // Optional image for the question
	Answers  []QuestionAnswer           `bson:"answer" json:"answer"`
}

type QuestionAnswer struct {
	Answer    transl.Localizable[string] `bson:"option" json:"option"`
	IsCorrect bool                       `bson:"isCorrect" json:"isCorrect"`
}

type ChallengeQuizSettings struct {
	RecordGrads            bool `bson:"recordGrads" json:"recordGrads"`
	RecordPoints           bool `bson:"recordPoints" json:"recordPoints"`
	InstantAnswerFeedback  bool `bson:"instantAnswerFeedback" json:"instantAnswerFeedback"`
	ApplyCompliance        bool `bson:"applyCompliance" json:"applyCompliance"`
	EnableReviewByStudents bool `bson:"enableReviewByStudents" json:"enableReviewByStudents"`
}

// settings
type ChallengeSettings struct {
	Status                   string `bson:"status" json:"status"`
	NumOfScoreToEarnPoints   int    `bson:"numOfscoreToEarnPoints" json:"numOfscoreToEarnPoints"`
	MaxAttempts              int    `bson:"maxAttempts" json:"maxAttempts"`
	NumOfRewardPoints        int    `bson:"rewardPoints" json:"rewardPoints"`
	NumOfAvailabilityLimit   int    `bson:"availabilityLimit" json:"availabilityLimit"`
	AutoAwardPoints          bool   `bson:"autoAwardPoints" json:"autoAwardPoints"`
	NotifyForNewChallenge    bool   `bson:"notifyForNewChallenge" json:"notifyForNewChallenge"`
	SendReminderBeforeExpiry bool   `bson:"sendReminderBeforeExpiry" json:"sendReminderBeforeExpiry"`
	AllowRetries             bool   `bson:"allowRetries" json:"allowRetries"`
	AllowLateSubmission      bool   `bson:"allowLateSubmission" json:"allowLateSubmission"`
	DisplayResultAfterSubmit bool   `bson:"displayResultAfterSubmit" json:"displayResultAfterSubmit"`
}

type ChallengePagination struct {
	Challenges []Challenge      `json:"challenges"`
	Pagination types.Pagination `json:"pagination"`
}

// Challenge Submission Models

type ChallengeSubmission struct {
	Id             primitive.ObjectID       `bson:"_id,omitempty" json:"_id,omitempty"`
	StudentId      primitive.ObjectID       `bson:"studentId" json:"studentId"`
	StudentName    string                   `bson:"studentName,omitempty" json:"studentName,omitempty"` // Student name for display
	ChallengeId    primitive.ObjectID       `bson:"challengeId" json:"challengeId"`
	StudentAnswers []ChallengeStudentAnswer `bson:"studentAnswers" json:"studentAnswers"`
	AvgScore       float64                  `bson:"avgScore" json:"avgScore"`           // Score percentage (0-100)
	RewardPoints   int                      `bson:"rewardPoints" json:"rewardPoints"`   // Points earned
	StudentLevel   string                   `bson:"studentLevel" json:"studentLevel"`   // Performance level
	AttemptNumber  int                      `bson:"attemptNumber" json:"attemptNumber"` // Attempt number
	Status         string                   `bson:"status" json:"status"`               // submitted, pending, etc.
	CreatedAt      time.Time                `bson:"createdAt" json:"createdAt"`
	UpdatedAt      time.Time                `bson:"updatedAt" json:"updatedAt"`
	CreatedBy      primitive.ObjectID       `bson:"createdBy,omitempty" json:"createdBy,omitempty"`
	UpdatedBy      primitive.ObjectID       `bson:"updatedBy,omitempty" json:"updatedBy,omitempty"`
	Trash          bool                     `bson:"trash" json:"trash"`

	// Populated fields
	ChallengeObj *Challenge `bson:"challengeObj,omitempty" json:"challengeObj,omitempty"`
}

func (c *ChallengeSubmission) CollName() string {
	return "academyChallengeSubmissions"
}

type ChallengeStudentAnswer struct {
	QuestionId      primitive.ObjectID `bson:"questionId" json:"questionId"`
	SelectedAnswers []int              `bson:"selectedAnswers" json:"selectedAnswers"` // Answer indices
	// Historical snapshot of question and answers at submission time
	QuestionSnapshot    transl.Localizable[string]   `bson:"questionSnapshot,omitempty" json:"questionSnapshot,omitempty"`
	AnswersSnapshot     []QuestionAnswer             `bson:"answersSnapshot,omitempty" json:"answersSnapshot,omitempty"`
	SelectedAnswersText []transl.Localizable[string] `bson:"selectedAnswersText,omitempty" json:"selectedAnswersText,omitempty"`
}

type SubmitChallengeDto struct {
	ChallengeId    string                   `json:"challengeId" validate:"required"`
	StudentAnswers []ChallengeStudentAnswer `json:"studentAnswers" validate:"required"`
}

func (s *SubmitChallengeDto) Validate(v *validator.Validate) error {
	return helpers.GenericValidation(v, s)
}

type ChallengeSubmissionPagination struct {
	Submissions []ChallengeSubmission `json:"submissions"`
	Pagination  types.Pagination      `json:"pagination"`
}

type ChallengeSubmissionStats struct {
	Submissions      []ChallengeSubmission `json:"submissions"`
	TotalSubmissions int                   `json:"totalSubmissions"`
	UniqueStudents   int                   `json:"uniqueStudents"`
	AverageScore     float64               `json:"averageScore"`
	HighestScore     float64               `json:"highestScore"`
	LowestScore      float64               `json:"lowestScore"`
	Pagination       types.Pagination      `json:"pagination"`
}
