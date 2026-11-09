package di

import (
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/handler"
	content "larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/shared"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

// Register registers all contact providers
func Register(i *do.Injector, r *chi.Mux) *do.Injector {
	//services
	do.Provide(i, contact.NewParentStudentRelationSvcs)
	do.Provide(i, contact.NewParentSvcs)
	do.Provide(i, contact.NewStudentSvcs)
	do.Provide(i, contact.NewStudentNoteSvcs)
	do.Provide(i, contact.NewTeacherSvcs)
	do.Provide(i, contact.NewCredSvcs)
	do.Provide(i, contact.NewSupervisorSvcs)
	do.Provide(i, contact.NewUserLookupSvc)

	// Register ParentSvcs as ParentChecker to avoid circular dependency
	do.Provide[shared.ParentChecker](i, func(i *do.Injector) (shared.ParentChecker, error) {
		return do.Invoke[contact.ParentSvcs](i)
	})

	// Register TeacherAssignmentSvcs as TeacherAssignmentBatchHelper to avoid import cycle
	// The content service implements the interface needed by contact service
	do.Provide[contact.TeacherAssignmentBatchHelper](i, func(i *do.Injector) (contact.TeacherAssignmentBatchHelper, error) {
		svc, err := content.NewTeacherAssignmentSvcs(i)
		if err != nil {
			return nil, err
		}
		return svc, nil
	})

	return i
}

// RegisterHandlers registers all contact handlers
func RegisterHandlers(i *do.Injector, r *chi.Mux) {
	handler.NewTeacherHandler(i, r)
	handler.NewSupervisorHandler(i, r)
	handler.NewStudentPublicHandler(i, r)
	handler.NewStudentHandler(i, r)
	handler.NewStudentNoteHandler(i, r)
	handler.NewParentHandler(i, r)
	handler.NewParentStudentRelationHandler(i, r)
}

// Init is kept for backward compatibility, but now just calls Register
func Init(i *do.Injector, r *chi.Mux) *do.Injector {
	return Register(i, r)
}
