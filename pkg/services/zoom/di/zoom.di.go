package di

import (
	"larsa-academy-microservice/pkg/services/zoom"
	"larsa-academy-microservice/pkg/services/zoom/webhooks"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	do.Override(i, zoom.NewZoomService)
	return i
}

func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	webhooks.NewZoomWebhookHandler(i, r)
	zoom.NewZoomHandler(i, r)
}
