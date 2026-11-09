package enums

type EnrollmentStatus string

const (
	EnrollmentStatusPending   EnrollmentStatus = "pending"
	EnrollmentStatusActive    EnrollmentStatus = "active"
	EnrollmentStatusSuspended EnrollmentStatus = "suspended"
	EnrollmentStatusCompleted EnrollmentStatus = "completed"
	EnrollmentStatusCancelled EnrollmentStatus = "cancelled"
)
