package di

import (
	"larsa-academy-microservice/pkg/services/exchange"

	"github.com/samber/do"
)

func Register(i *do.Injector) *do.Injector {
	do.Provide(i, exchange.NewExchangeService)
	return i
}

