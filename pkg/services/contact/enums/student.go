package enums

type StudentStatus string

const (
	StudentStatusActive   StudentStatus = "active"
	StudentStatusPending StudentStatus = "pending"
	StudentStatusSuspend StudentStatus = "suspend"
)
