package di

import (
	"larsa-academy-microservice/pkg/services/compensation"
	"larsa-academy-microservice/pkg/services/compensation/handler"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	do.Provide(i, compensation.NewCompensationSvcs)
	return i
}

func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewCompensationHandler(i, r)
}
