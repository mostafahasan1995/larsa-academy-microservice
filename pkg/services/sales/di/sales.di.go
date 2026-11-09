package di

import (
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/services/sales/handler"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all sales providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, sales.NewInvoiceSvcs)
	do.Provide(i, sales.NewCouponSvcs)
	do.Provide(i, sales.NewPaymentSvcs)
	do.Provide(i, sales.NewPricingSvcs)

	return i
}

// RegisterHandlers registers all sales handlers
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewInvoiceHandler(i, r)
	handler.NewCouponHandler(i, r)
	handler.NewPaymentHandler(i, r)
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
