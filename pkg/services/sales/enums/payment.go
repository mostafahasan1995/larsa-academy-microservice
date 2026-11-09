package enums

// PaymentStatus defines status values for payments

type PaymentStatus string

const (
	PaymentStatusPaid    PaymentStatus = "Paid"
	PaymentStatusPending PaymentStatus = "Pending"
	PaymentStatusFailed  PaymentStatus = "Failed"
)

// PaymentType defines method (Electronic or Offline)

type PaymentType string

const (
	PaymentTypeElectronic PaymentType = "Electronic"
	PaymentTypeOffline    PaymentType = "Offline"
)
