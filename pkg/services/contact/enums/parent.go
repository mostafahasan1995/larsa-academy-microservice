package enums

type ParentStatus string

const (
	ParentStatusActive   ParentStatus = "active"
	ParentStatusPending ParentStatus = "pending"
	ParentStatusSuspend ParentStatus = "suspend"
)
