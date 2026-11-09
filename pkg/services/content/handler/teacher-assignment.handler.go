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

type TeacherAssignmentHandler struct {
	teacherAssignmentSvcs content.TeacherAssignmentSvcs
}

func NewTeacherAssignmentHandler(i *do.Injector, r *chi.Mux) {
	h := &TeacherAssignmentHandler{
		teacherAssignmentSvcs: do.MustInvoke[content.TeacherAssignmentSvcs](i),
	}

	r.Route("/teacher-assignments", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.List))
		r.With(middleware.Auth("authenticate")).Get("/stats", helpers.Make(h.GetStats))
		r.With(middleware.Auth("authenticate")).Post("/bulk", helpers.Make(h.BulkCreateOrUpdate))
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *TeacherAssignmentHandler) List(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.teacherAssignmentSvcs.List(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherAssignmentHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.teacherAssignmentSvcs.Get(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherAssignmentHandler) GetStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.teacherAssignmentSvcs.GetStats(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherAssignmentHandler) BulkCreateOrUpdate(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.BulkTeacherAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	err := h.teacherAssignmentSvcs.BulkCreateOrUpdate(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "ok"})
}

func (h *TeacherAssignmentHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.TeacherAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.teacherAssignmentSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherAssignmentHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.teacherAssignmentSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "ok"})
}
