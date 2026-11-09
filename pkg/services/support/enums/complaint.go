package enums

type ComplaintStatus string

const (
	ComplaintStatusOpen       ComplaintStatus = "open"
	ComplaintStatusInProgress ComplaintStatus = "inProgress"
	ComplaintStatusResolved   ComplaintStatus = "resolved"
)

type ComplaintUserRole string

const (
	RoleStudent    ComplaintUserRole = "student"
	RoleParent     ComplaintUserRole = "parent"
	RoleTeacher    ComplaintUserRole = "teacher"
	RoleSupervisor ComplaintUserRole = "supervisor"
)

type ChangeActionType string

const (
	ChangeActionStatus   ChangeActionType = "status"
	ChangeActionAssignTo ChangeActionType = "assignTo"
	ChangeActionReply    ChangeActionType = "reply"
)
