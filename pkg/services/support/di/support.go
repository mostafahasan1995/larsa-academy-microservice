package di

import (
	"larsa-academy-microservice/pkg/services/support"
	"larsa-academy-microservice/pkg/services/support/handler"
	"larsa-academy-microservice/pkg/services/support/repo"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all support providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, support.NewComplaintSvcs)
	do.Provide(i, support.NewComplaintTypeSvcs)
	do.Provide(i, support.NewFeedbackSvcs)
	do.Provide(i, support.NewBenefitTypeSvcs)
	do.Provide(i, support.NewBenefitSvcs)
	do.Provide(i, support.NewTeacherBenefitSvcs)

	//repos
	do.Provide(i, repo.NewComplaintRepo)
	do.Provide(i, repo.NewComplaintTypeRepo)
	do.Provide(i, repo.NewFeedbackRepo)
	do.Provide(i, repo.NewBenefitTypeRepo)
	do.Provide(i, repo.NewBenefitRepo)
	do.Provide(i, repo.NewTeacherBenefitRepo)

	return i
}

// RegisterHandlers registers all support handlers
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewComplaintHandler(i, r)
	handler.NewComplaintTypeHandler(i, r)
	handler.NewFeedbackHandler(i, r)
	handler.NewBenefitTypeHandler(i, r)
	handler.NewBenefitHandler(i, r)
	handler.NewTeacherBenefitHandler(i, r)
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
