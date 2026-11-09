package di

import (
	"larsa-academy-microservice/pkg/gateway"

	"github.com/samber/do"
)

// Register registers all gateway providers
func Register(i *do.Injector) *do.Injector {
	do.Provide(i, gateway.NewUsersGw)
	do.Provide(i, gateway.NewRolesGw)
	do.Provide(i, gateway.NewGateway)
	do.Provide(i, gateway.NewPaymentGw)
	return i
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector) *do.Injector {
	return Register(i)
}
