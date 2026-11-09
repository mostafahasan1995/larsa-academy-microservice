package enums

type OrderStatus string

const (
	OrderStatusPending   OrderStatus = "pending"
	OrderStatusPaid      OrderStatus = "paid"
	OrderStatusFailed    OrderStatus = "failed"
	OrderStatusCancelled OrderStatus = "cancelled"
)

type PaymentGateway string

const (
	MYFATOORAH PaymentGateway = "myFatoorah"
)
