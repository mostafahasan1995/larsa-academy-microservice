package constant

import "larsa-academy-microservice/pkg/helpers"

var (
	// Placement Test Errors
	ErrPlacementTestNotFound      = helpers.NotFoundError("placement test not found")
	ErrOnlineTestNoAutoSubmit     = helpers.BadRequest("online placement tests do not accept automated answer submission")
	ErrPlacementTestNoQuestions   = helpers.BadRequest("placement test has no questions")
	ErrPlacementTestNotOnline     = helpers.BadRequest("placement test is not online type")
	ErrCoachingProgramMismatch    = helpers.BadRequest("coachingProgramId does not match placement test reference")
	ErrRetryNotAllowed            = helpers.BadRequest("retry not allowed for this placement test")
	ErrMaxAttemptsReached         = helpers.BadRequest("maximum number of attempts reached")
	ErrPendingTestExists          = helpers.BadRequest("there is already a pending or scheduled placement test")
	ErrFromTimeAfterToTime        = helpers.BadRequest("'from' time must be before 'to' time")
	ErrTeacherNotPartOfCoaching   = helpers.BadRequest("teacher is not part of this coaching program")
	ErrInvalidScheduleId          = helpers.BadRequest("invalid schedule id")
	ErrScheduleNotFound           = helpers.NotFoundError("placement test schedule not found")
	ErrUnauthorizedUpdate         = helpers.Unauthorized("you are not authorized to update this schedule")
	ErrUnauthorizedDelete         = helpers.Unauthorized("you are not authorized to delete this schedule")
	ErrCompletedTestUpdate        = helpers.BadRequest("cannot update a completed/evaluated placement test")
	ErrStatusChangeAuth           = helpers.Unauthorized("only admin/supervisor can change status")
	ErrTeacherRequiredForApproval = helpers.BadRequest("teacherId is required to approve and schedule")
	ErrTeacherChangeAuth          = helpers.Unauthorized("only admin/supervisor can change teacher")
	ErrTeacherNull                = helpers.BadRequest("teacherId cannot be null")
	ErrTeacherReassignAuth        = helpers.Unauthorized("you are not authorized to change teacher for this schedule")
	ErrCoachingChangeAuth         = helpers.Unauthorized("only admin/supervisor can change coaching program")

	// Student Placement Test Errors
	ErrInvalidPlacementTestId     = helpers.BadRequest("invalid placement test id")
	ErrStudentTestNotFound        = helpers.NotFoundError("student placement test not found")
	ErrPlacementTestEvaluatedAuth = helpers.Unauthorized("you are not authorized to submit teacher evaluation for this placement test")
	ErrPlacementTestNotCompleted  = helpers.BadRequest("placement test must be completed before evaluation")

	// Student Session Errors
	ErrInvalidStudentSessionId   = helpers.BadRequest("invalid student session id")
	ErrStudentSessionNotFound    = helpers.NotFoundError("student session not found")
	ErrUnauthorizedUpdateSession = helpers.Unauthorized("you are not authorized to update this session")
	ErrInvalidTimeFormat         = helpers.BadRequest("invalid time format")
	ErrInvalidStatusValue        = helpers.BadRequest("status can only be set to: missed, cancelled, or completed")
	ErrStatusRequiresPastEndTime = helpers.BadRequest("cannot set status to missed or completed: session end time must be in the past")

	// student questions
	ErrInvalidQuestionId     = helpers.BadRequest("invalid question id")
	ErrInvalidReplyId        = helpers.BadRequest("invalid reply id")
	ErrInvalidRefId          = helpers.BadRequest("invalid ref id")
	ErrQuestionNotFound      = helpers.BadRequest("question not found")
	ErrReplyNotFound         = helpers.BadRequest("reply not found")
	ErrRatingOutOfRange      = helpers.BadRequest("rating must be between 1 and 5")
	ErrCannotUpdateQuestion  = helpers.BadRequest("you can only update your own questions")
	ErrCannotDeleteQuestion  = helpers.BadRequest("you can only delete your own questions")
	ErrCannotUpdateReply     = helpers.BadRequest("you can only update your own replies")
	ErrCannotDeleteReply     = helpers.BadRequest("you can only delete your own replies")
	ErrCannotReplyToQuestion = helpers.BadRequest("you are not authorized to reply to this question")
	ErrCourseNotFound        = helpers.BadRequest("course not found")
	ErrCoachingNotFound      = helpers.BadRequest("coaching not found")
	ErrLessonNotFound        = helpers.BadRequest("lesson not found in course")
)
