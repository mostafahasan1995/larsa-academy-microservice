package enums

type AssignmentType string

const (
	AssignmentTypeCourse   AssignmentType = "course"
	AssignmentTypeCoaching AssignmentType = "coaching"
)

type AssignmentStatus string

const (
	AssignmentStatusAssigned AssignmentStatus = "assigned"
	AssignmentStatusPending  AssignmentStatus = "pending"
)

type SubjectType string

const (
	SubjectTypeGeneral    SubjectType = "general"
	SubjectTypeGovernment SubjectType = "government"
)
