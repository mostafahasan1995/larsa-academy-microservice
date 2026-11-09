package enums

// InvoiceStatus represents status values for invoice
// e.g., Paid, Pending, Canceled

type InvoiceStatus string

const (
	InvoiceStatusPaid     InvoiceStatus = "paid"
	InvoiceStatusPending  InvoiceStatus = "pending"
	InvoiceStatusCanceled InvoiceStatus = "canceled"
)

type ProductType string

const (
	ProductTypeCourse   ProductType = "course"
	ProductTypeBundle   ProductType = "bundle"
	ProductTypeCoaching ProductType = "coaching"
	ProductTypeMaterial ProductType = "material"
	ProductTypeOther    ProductType = "Other"
)
