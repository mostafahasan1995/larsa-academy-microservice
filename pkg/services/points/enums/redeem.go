package enums

// RedeemStatus defines the status of a redemption request
type RedeemStatus string

const (
	RedeemStatusPending  RedeemStatus = "Pending"
	RedeemStatusApproved RedeemStatus = "Approved"
	RedeemStatusRejected RedeemStatus = "Rejected"
)
