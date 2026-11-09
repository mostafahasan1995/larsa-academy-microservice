package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type SupervisorAssignmentHandler struct {
	supervisorAssignmentSvcs content.SupervisorAssignmentSvcs
}

func NewSupervisorAssignmentHandler(i *do.Injector, r *chi.Mux) {
	h := &SupervisorAssignmentHandler{
		supervisorAssignmentSvcs: do.MustInvoke[content.SupervisorAssignmentSvcs](i),
	}

	r.Route("/supervisor-assignments", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.List))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Create))
		r.With(middleware.Auth("authenticate")).Post("/matched-teachers", helpers.Make(h.GetMatchedTeachers))
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *SupervisorAssignmentHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.supervisorAssignmentSvcs.List(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *SupervisorAssignmentHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.supervisorAssignmentSvcs.Get(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *SupervisorAssignmentHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.SupervisorAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.supervisorAssignmentSvcs.Create(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *SupervisorAssignmentHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.SupervisorAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.supervisorAssignmentSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *SupervisorAssignmentHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.supervisorAssignmentSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "ok"})
}

func (h *SupervisorAssignmentHandler) GetMatchedTeachers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.SupervisorAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.supervisorAssignmentSvcs.GetMatchedTeachers(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
