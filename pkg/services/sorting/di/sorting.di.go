package di

import (
	"larsa-academy-microservice/pkg/services/sorting"
	"larsa-academy-microservice/pkg/services/sorting/repo"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all sorting providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, sorting.NewSortingSvcs)

	//repos
	do.Provide(i, repo.NewSortingRepo)

	return i
}

// RegisterHandlers registers all sorting handlers (none for sorting service)
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	// No handlers for sorting service
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
