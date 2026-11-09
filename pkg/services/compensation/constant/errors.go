package constant

import "larsa-academy-microservice/pkg/helpers"

var (
	ErrActivePricingExists     = helpers.BadRequest("active pricing already exists for teacher/ref/refId")
	ErrInvalidDuration         = helpers.BadRequest("durationMinutes must be > 0")
	ErrInvalidSessionId        = helpers.BadRequest("invalid sessionId")
	ErrCannotEditRejectedHours = helpers.BadRequest("cannot edit rejected hours")
	ErrReasonRequired          = helpers.BadRequest("reason is required")
	ErrInvalidRef              = helpers.BadRequest("ref must be course or coaching")
	ErrUnsupportedCurrency     = helpers.BadRequest("unsupported currency")
	ErrInvalidTeacherId        = helpers.BadRequest("invalid teacherId")
	ErrInvalidPricingId        = helpers.BadRequest("invalid pricingId")
	ErrInvalidRefId            = helpers.BadRequest("invalid refId")
	ErrInvalidHoursId          = helpers.BadRequest("invalid hoursId")
	ErrQueryNotValid           = helpers.BadRequest("query not valid")
	ErrPricingInUse            = helpers.BadRequest("pricing is referenced by recorded hours and cannot be deleted")
)
