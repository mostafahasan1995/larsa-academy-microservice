package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/points"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"strconv"

	"larsa-academy-microservice/pkg/query"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type StudentPointHandler struct {
	studentPointSvcs   points.StudentPointSvcs
	validationInstance *validator.Validate
}

func NewStudentPointHandler(i *do.Injector, r *chi.Mux) {
	h := &StudentPointHandler{
		studentPointSvcs:   do.MustInvoke[points.StudentPointSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/student-points", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.Get("/student/{studentId}", helpers.Make(h.GetByStudentId))
		r.With(middleware.Auth("authenticate")).Get("/statistics", helpers.Make(h.GetStatistics))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.CreateOrUpdate))
		r.With(middleware.Auth("authenticate")).Post("/student/{studentId}/update-balance", helpers.Make(h.UpdateBalance))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *StudentPointHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.studentPointSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentPointHandler) GetByStudentId(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	studentId := chi.URLParam(r, "studentId")
	result, err := h.studentPointSvcs.GetByStudentId(ctx, studentId)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentPointHandler) GetStatistics(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Get topN from query parameter, default to 3
	topNStr := r.URL.Query().Get("topN")
	topN := 3
	if topNStr != "" {
		if n, err := strconv.Atoi(topNStr); err == nil && n > 0 {
			topN = n
		}
	}

	result, err := h.studentPointSvcs.GetStatistics(ctx, topN)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentPointHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.studentPointSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentPointHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.studentPointSvcs.GetAll(ctx, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentPointHandler) CreateOrUpdate(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.StudentPoint
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.studentPointSvcs.CreateOrUpdate(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentPointHandler) UpdateBalance(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	studentId := chi.URLParam(r, "studentId")

	pointsStr := r.URL.Query().Get("points") // ✅ from query string

	points, err := strconv.Atoi(pointsStr)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{
			"error": "invalid points value",
		})
	}

	result, err := h.studentPointSvcs.UpdateBalance(ctx, studentId, points)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *StudentPointHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.studentPointSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
