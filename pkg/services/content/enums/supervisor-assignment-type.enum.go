package enums

type SupervisorAssignmentType string

const (
	SupervisorAssignmentTypeDirect  SupervisorAssignmentType = "direct"
	SupervisorAssignmentTypeDynamic SupervisorAssignmentType = "dynamic"
)

type SupervisorRefType string

const (
	SupervisorRefTypeTeacher SupervisorRefType = "teacher"
	SupervisorRefTypeSubject SupervisorRefType = "subject"
)

type SupervisorAssignmentStatus string

const (
	SupervisorAssignmentStatusAssigned SupervisorAssignmentStatus = "assigned"
	SupervisorAssignmentStatusPending  SupervisorAssignmentStatus = "pending"
)
