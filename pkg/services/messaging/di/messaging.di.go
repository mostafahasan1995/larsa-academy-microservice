package di

import (
	"larsa-academy-microservice/pkg/services/messaging"
	"larsa-academy-microservice/pkg/services/messaging/handler"
	"larsa-academy-microservice/pkg/services/messaging/repo"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all messaging providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, messaging.NewMessageSvcs)

	//repos
	do.Provide(i, repo.NewMessageRepo)

	return i
}

// RegisterHandlers registers all messaging handlers
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewMessagingHandler(i, r)
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
