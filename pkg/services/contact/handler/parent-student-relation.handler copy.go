package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type ParentStudentRelationHandler struct {
	parentStudentRelationsvcs contact.ParentStudentRelationSvcs
	validationInstance        *validator.Validate
}

func NewParentStudentRelationHandler(i *do.Injector, r *chi.Mux) {
	h := &ParentStudentRelationHandler{
		parentStudentRelationsvcs: do.MustInvoke[contact.ParentStudentRelationSvcs](i),
		validationInstance:        do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/parentStudentRelations", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetOne))
		r.With(middleware.Auth("authenticate")).Post("/paginated", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))

	})

}
func (h *ParentStudentRelationHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.parentStudentRelationsvcs.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *ParentStudentRelationHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.parentStudentRelationsvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *ParentStudentRelationHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.ParentStudentRelationDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.parentStudentRelationsvcs.Add(ctx, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *ParentStudentRelationHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.ParentStudentRelationDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.parentStudentRelationsvcs.Update(ctx, id, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ParentStudentRelationHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //package id

	if err := h.parentStudentRelationsvcs.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
