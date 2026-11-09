package enums

type StudentSessionStatus string

const (
	StudentSessionStatusPending     StudentSessionStatus = "pending"
	StudentSessionStatusScheduled   StudentSessionStatus = "scheduled"
	StudentSessionStatusRescheduled StudentSessionStatus = "rescheduled"
	StudentSessionStatusMissed      StudentSessionStatus = "missed"
	StudentSessionStatusCancelled   StudentSessionStatus = "cancelled"
	StudentSessionStatusCompleted   StudentSessionStatus = "completed"
)

type JustificationType string

const (
	JustificationTypeAbsence    JustificationType = "absence"
	JustificationTypeReschedule JustificationType = "reschedule"
)

type JustificationDecision string

const (
	JustificationDecisionPending  JustificationDecision = "pending"
	JustificationDecisionApproved JustificationDecision = "approved"
	JustificationDecisionRejected JustificationDecision = "rejected"
)
