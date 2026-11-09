package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type BundleHandler struct {
	bundleSvcs         content.BundleSvcs
	commentSvcs        content.CommentSvcs
	validationInstance *validator.Validate
}

func NewBundleHandler(i *do.Injector, r *chi.Mux) {
	h := &BundleHandler{
		bundleSvcs:         do.MustInvoke[content.BundleSvcs](i),
		commentSvcs:        do.MustInvoke[content.CommentSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.With(middleware.Auth("authenticate")).Post("/api/bundles/student/list", helpers.Make(h.GetStudentList))
	r.With(middleware.Auth("authenticate")).Post("/api/bundles/parent/student/list", helpers.Make(h.GetParentStudentList))
	r.Route("/bundles", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllBundles)).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllBundles)).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllBundles)).Delete("/{id}", helpers.Make(h.Delete))
		// WARNING: Hard delete all - for staging data clearing only, will be removed later
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllBundles)).Delete("/hard-delete-all", helpers.Make(h.HardDeleteAll))
		//comments
		r.With(middleware.Auth("authenticate")).Post("/{id}/comments", helpers.Make(h.GetBundleComments))

		//pricing
		r.With(middleware.Auth("authenticate", content.CapabilityManageBundlePricing)).Post("/{id}/pricing", helpers.Make(h.AddBundlePricing))
		r.With(middleware.Auth("authenticate", content.CapabilityManageBundlePricing)).Put("/{id}/pricing/{pricingId}", helpers.Make(h.UpdateBundlePricing))
		r.With(middleware.Auth("authenticate", content.CapabilityManageBundlePricing)).Delete("/{id}/pricing/{pricingId}", helpers.Make(h.DeleteBundlePricing))
		r.With(middleware.Auth("authenticate")).Get("/{id}/pricing", helpers.Make(h.GetBundlePricing))
		r.With(middleware.Auth("authenticate")).Post("/{id}/members/list", helpers.Make(h.GetBundleMembers))
	})
}

// GetById godoc
// @Summary Get bundle by ID
// @Description Get a bundle by its ID
// @Tags Bundle
// @Accept json
// @Produce json
// @Param id path string true "Bundle ID"
// @Success 200 {object} models.Bundle
// @Failure 400 {object} helpers.ErrorResponse
// @Failure 404 {object} helpers.ErrorResponse
// @Router /bundles/{id} [get]
func (h *BundleHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.bundleSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Get godoc
// @Summary Get bundles with pagination
// @Description Get a list of bundles with pagination
// @Tags Bundle
// @Accept json
// @Produce json
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Param query query string false "Query conditions"
// @Success 200 {object} models.BundlePagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /bundles/list [post]
func (h *BundleHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.bundleSvcs.Get(ctx, skip, limit, query, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentList godoc
// @Summary Get bundles with student favorites
// @Description Get a list of bundles with pagination and IsFavorite flag
// @Tags Bundle
// @Accept json
// @Produce json
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Param query query string false "Query conditions"
// @Success 200 {object} models.StudentBundlePagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /api/bundles/student/list [post]
func (h *BundleHandler) GetStudentList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.bundleSvcs.GetStudentList(ctx, "", skip, limit, query, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

type ParentBundleStudentListRequest struct {
	StudentId string            `json:"studentId" validate:"required"`
	Query     *query.Conditions `json:"query"`
}

func (h *BundleHandler) GetParentStudentList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var req ParentBundleStudentListRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, &req); err != nil {
		return err
	}

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.bundleSvcs.GetStudentList(ctx, req.StudentId, skip, limit, req.Query, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *BundleHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.bundleSvcs.GetAll(ctx, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Add godoc
// @Summary Create a new bundle
// @Description Create a new bundle
// @Tags Bundle
// @Accept json
// @Produce json
// @Param bundle body models.BundleDto true "Bundle Data"
// @Success 200 {object} models.Bundle
// @Failure 400 {object} helpers.ErrorResponse
// @Router /bundles [post]
func (h *BundleHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.BundleDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.bundleSvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Update godoc
// @Summary Update a bundle
// @Description Update a bundle by ID
// @Tags Bundle
// @Accept json
// @Produce json
// @Param id path string true "Bundle ID"
// @Param bundle body models.BundleDto true "Bundle Data"
// @Success 200 {object} models.Bundle
// @Failure 400 {object} helpers.ErrorResponse
// @Router /bundles/{id} [put]
func (h *BundleHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.BundleDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.bundleSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Delete godoc
// @Summary Delete a bundle
// @Description Delete a bundle by ID
// @Tags Bundle
// @Accept json
// @Produce json
// @Param id path string true "Bundle ID"
// @Success 200 {object} helpers.SuccessResponse
// @Failure 400 {object} helpers.ErrorResponse
// @Router /bundles/{id} [delete]
func (h *BundleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.bundleSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

//comments

func (h *BundleHandler) GetBundleComments(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") // bundle id

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.commentSvcs.GetCommentsByRef(ctx, enums.CommentTypeBundle, id, skip, limit, query)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// pricing handlers

func (h *BundleHandler) AddBundlePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //bundle id

	var data *models.BundlePricingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.bundleSvcs.AddBundlePricing(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *BundleHandler) UpdateBundlePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	bundleId := chi.URLParam(r, "id")         //bundle id
	pricingId := chi.URLParam(r, "pricingId") //pricing id

	var data *models.BundlePricingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.bundleSvcs.UpdateBundlePricing(ctx, bundleId, pricingId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *BundleHandler) DeleteBundlePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	bundleId := chi.URLParam(r, "id")         //bundle id
	pricingId := chi.URLParam(r, "pricingId") //pricing id

	err := h.bundleSvcs.DeleteBundlePricing(ctx, bundleId, pricingId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *BundleHandler) GetBundlePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	bundleId := chi.URLParam(r, "id") //bundle id

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.bundleSvcs.GetBundlePricing(ctx, bundleId, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// HardDeleteAll godoc
// @Summary Hard delete all bundles
// @Description Hard delete all bundles from the database (for staging data clearing only)
// @Tags Bundle
// @Accept json
// @Produce json
// @Success 200 {object} helpers.SuccessResponse
// @Failure 400 {object} helpers.ErrorResponse
// @Router /bundles/hard-delete-all [delete]
// WARNING: This endpoint is for staging data clearing only and should be removed later
func (h *BundleHandler) HardDeleteAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	err := h.bundleSvcs.HardDeleteAll(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// GetBundleMembers godoc
// @Summary Get bundle members
// @Description Get a list of students enrolled in a bundle
// @Tags Bundle
// @Accept json
// @Produce json
// @Param id path string true "Bundle ID"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Param query query string false "Query conditions"
// @Success 200 {object} models.MemberBundlePagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /bundles/{id}/members [post]
func (h *BundleHandler) GetBundleMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	bundleId := chi.URLParam(r, "id")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.bundleSvcs.GetBundleMembers(ctx, bundleId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
