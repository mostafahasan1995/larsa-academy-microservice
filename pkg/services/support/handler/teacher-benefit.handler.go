package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/support"
	"larsa-academy-microservice/pkg/services/support/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type TeacherBenefitHandler struct {
	teacherBenefitsvcs support.TeacherBenefitSvcs
	validationInstance *validator.Validate
}

func NewTeacherBenefitHandler(i *do.Injector, r *chi.Mux) {
	h := &TeacherBenefitHandler{
		teacherBenefitsvcs: do.MustInvoke[support.TeacherBenefitSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/teacher-benefits", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetOne))
		r.With(middleware.Auth("authenticate")).Post("/paginated", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/grouped", helpers.Make(h.GetGrouped))
		r.With(middleware.Auth("authenticate")).Get("/teacher/{teacherId}", helpers.Make(h.GetByTeacher))
		r.With(middleware.Auth("authenticate")).Get("/stats", helpers.Make(h.GetStats))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Post("/bulk-add", helpers.Make(h.BulkAdd))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Patch("/bulk-update", helpers.Make(h.BulkUpdate))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *TeacherBenefitHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.teacherBenefitsvcs.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) GetGrouped(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.GetGrouped(ctx, skip, limit, &query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) GetByTeacher(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	teacherId := chi.URLParam(r, "teacherId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.GetByTeacher(ctx, teacherId, skip, limit)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) GetStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.teacherBenefitsvcs.GetStats(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.TeacherBenefitDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.Add(ctx, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.TeacherBenefitDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.Update(ctx, id, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	if err := h.teacherBenefitsvcs.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *TeacherBenefitHandler) BulkAdd(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.BulkTeacherBenefitRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.BulkAdd(ctx, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherBenefitHandler) BulkUpdate(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.BulkTeacherBenefitUpdateRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}

	result, err := h.teacherBenefitsvcs.BulkUpdate(ctx, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
