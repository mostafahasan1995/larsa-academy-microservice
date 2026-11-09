package di

import (
	"larsa-academy-microservice/pkg/services/seed"
	"larsa-academy-microservice/pkg/services/seed/repo"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all seed providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, seed.NewSeedSvcs)

	//repos
	do.Provide(i, repo.NewSeedRepo)

	return i
}

// RegisterHandlers registers all seed handlers (none for seed service)
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	// No handlers for seed service
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
