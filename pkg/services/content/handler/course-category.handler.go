package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
)

func (h *CourseHandler) GetAllCourseCategories(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.coursesvcs.GetAllCourseCategories(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetCourseCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "categoryId")

	result, err := h.coursesvcs.GetCourseCategory(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) AddCourseCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.CourseCategoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseCategory(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateCourseCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "categoryId")

	var data *models.CourseCategoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseCategory(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteCourseCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "categoryId")

	err := h.coursesvcs.DeleteCourseCategory(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
