package enums

// StudentPlacementTestStatus represents the lifecycle of an online placement test request/session.
type StudentPlacementTestStatus string

const (
	StudentPlacementTestStatusPending    StudentPlacementTestStatus = "pending"
	StudentPlacementTestStatusScheduled  StudentPlacementTestStatus = "scheduled"
	StudentPlacementTestStatusRejected   StudentPlacementTestStatus = "rejected"
	StudentPlacementTestStatusInProgress StudentPlacementTestStatus = "in_progress"
	StudentPlacementTestStatusCompleted  StudentPlacementTestStatus = "completed"
	StudentPlacementTestStatusEvaluated  StudentPlacementTestStatus = "evaluated"
	StudentPlacementTestStatusCancelled  StudentPlacementTestStatus = "cancelled"
	StudentPlacementTestStatusMissed     StudentPlacementTestStatus = "missed"
)
