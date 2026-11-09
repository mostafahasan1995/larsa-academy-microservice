package di

import (
	"larsa-academy-microservice/pkg/services/points"
	"larsa-academy-microservice/pkg/services/points/handler"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all points providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, points.NewStudentPointSvcs)
	do.Provide(i, points.NewRewardSvcs)
	do.Provide(i, points.NewRedeemRequestSvcs)
	do.Provide(i, points.NewPointsRuleSvcs)
	do.Provide(i, points.NewPointTransactionSvcs)

	return i
}

// RegisterHandlers registers all points handlers
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewStudentPointHandler(i, r)
	handler.NewRewardHandler(i, r)
	handler.NewRedeemRequestHandler(i, r)
	handler.NewPointsRuleHandler(i, r)
	handler.NewPointTransactionHandler(i, r)
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
