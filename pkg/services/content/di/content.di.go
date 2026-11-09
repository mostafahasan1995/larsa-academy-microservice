package di

import (
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/handler"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all content providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, content.NewCourseSvcs)
	do.Provide(i, content.NewCourseAuthService)
	do.Provide(i, content.NewCoachingAuthService)
	do.Provide(i, content.NewBundleAuthService)
	do.Provide(i, content.NewSubjectSvcs)
	do.Provide(i, content.NewSchoolSvcs)
	do.Provide(i, content.NewCoachingSvcs)
	do.Provide(i, content.NewBundleSvcs)
	do.Provide(i, content.NewChallengeSvcs)
	do.Provide(i, content.NewChallengeSubmissionSvcs)
	do.Provide(i, content.NewStorySvcs)
	do.Provide(i, content.NewCommentSvcs)
	do.Provide(i, content.NewReviewSvcs)
	do.Provide(i, content.NewStudentQuestionSvcs)
	do.Provide(i, content.NewPlacementTestSvcs)
	do.Provide(i, content.NewPrintedMaterialSvcs)
	do.Provide(i, content.NewCartSvcs)
	do.Provide(i, content.NewCalendarSvcs)

	do.Provide(i, content.NewTeacherAssignmentSvcs)
	do.Provide(i, content.NewSupervisorAssignmentSvcs)
	do.Provide(i, content.NewStudentContentProgressSvcs)

	do.Provide(i, content.NewOrderSvcs)
	do.Provide(i, content.NewGlobalSearchSvcs)

	return i
}

// RegisterHandlers registers all content handlers
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewCourseHandler(i, r)
	handler.NewSubjectHandler(i, r)
	handler.NewSchoolHandler(i, r)
	handler.NewCoachingHandler(i, r)
	handler.NewBundleHandler(i, r)
	handler.NewChallengeHandler(i, r)
	handler.NewChallengeSubmissionHandler(i, r)
	handler.NewStoryHandler(i, r)
	handler.NewCommentHandler(i, r)
	handler.NewReviewHandler(i, r)
	handler.NewStudentQuestionHandler(i, r)
	handler.NewPlacementTestHandler(i, r)
	handler.NewPrintedMaterialHandler(i, r)
	handler.NewCartHandler(i, r)
	handler.NewCalendarHandler(i, r)
	handler.NewGlobalSearchHandler(i, r)

	handler.NewTeacherAssignmentHandler(i, r)
	handler.NewSupervisorAssignmentHandler(i, r)
	handler.NewStudentContentProgressHandler(i, r)

	handler.NewOrderHandler(i, r)

}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
