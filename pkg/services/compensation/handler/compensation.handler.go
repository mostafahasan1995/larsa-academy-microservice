package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/compensation"
	"larsa-academy-microservice/pkg/services/compensation/constant"
	"larsa-academy-microservice/pkg/services/compensation/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type CompensationHandler struct {
	svcs               compensation.CompensationSvcs
	validationInstance *validator.Validate
}

type pricingListRequest struct {
	Conditions *query.Conditions `json:"conditions"`
	IsActive   *bool             `json:"isActive"`
}

type hoursListRequest struct {
	Conditions *query.Conditions `json:"conditions"`
	Start      *time.Time        `json:"start"`
	End        *time.Time        `json:"end"`
	Status     string            `json:"status"`
}

type summaryRequest struct {
	Conditions      *query.Conditions `json:"conditions"`
	Start           *time.Time        `json:"start"`
	End             *time.Time        `json:"end"`
	IncludeRejected bool              `json:"includeRejected"`
	HideZeroRecorded bool             `json:"hideZeroRecorded"`
}

func NewCompensationHandler(i *do.Injector, r *chi.Mux) {
	h := &CompensationHandler{
		svcs:               do.MustInvoke[compensation.CompensationSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/compensation", func(r chi.Router) {
		// pricing list (all pricing, optionally filtered by teacherId in conditions)
		r.With(middleware.Auth("authenticate", compensation.CapabilityViewTeacherCompensation)).
			Post("/pricing/list", helpers.Make(h.ListPricing))

		r.Route("/teachers/{teacherId}", func(r chi.Router) {
			// pricing
			r.With(middleware.Auth("authenticate", compensation.CapabilityManageTeacherCompensation)).
				Post("/pricing", helpers.Make(h.AddPricing))
			r.With(middleware.Auth("authenticate", compensation.CapabilityManageTeacherCompensation)).
				Put("/pricing/{pricingId}", helpers.Make(h.UpdatePricing))
			r.With(middleware.Auth("authenticate", compensation.CapabilityManageTeacherCompensation)).
				Delete("/pricing/{pricingId}", helpers.Make(h.DeletePricing))

			// hours
			r.With(middleware.Auth("authenticate", compensation.CapabilityManageTeacherCompensation)).
				Post("/hours", helpers.Make(h.AddHours))
			r.With(middleware.Auth("authenticate", compensation.CapabilityManageTeacherCompensation)).
				Put("/hours/{hoursId}", helpers.Make(h.UpdateHours))
			r.With(middleware.Auth("authenticate", compensation.CapabilityManageTeacherCompensation)).
				Post("/hours/{hoursId}/reject", helpers.Make(h.RejectHours))
			r.With(middleware.Auth("authenticate", compensation.CapabilityViewTeacherCompensation)).
				Post("/hours/list", helpers.Make(h.ListHours))
		})

		// quote
		r.With(middleware.Auth("authenticate", compensation.CapabilityViewTeacherCompensation)).
			Get("/pay/quote", helpers.Make(h.Quote))

		// breakdown (detailed per-teacher breakdown)
		r.With(middleware.Auth("authenticate", compensation.CapabilityViewTeacherCompensation)).
			Post("/breakdown", helpers.Make(h.Breakdown))

		// summary (aggregate statistics)
		r.With(middleware.Auth("authenticate", compensation.CapabilityViewTeacherCompensation)).
			Post("/summary", helpers.Make(h.Summary))
	})
}

func (h *CompensationHandler) AddPricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")

	var dto models.TeacherPricingCreateDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return helpers.InvalidJSON()
	}
	if err := helpers.GenericValidation(h.validationInstance, dto); err != nil {
		return err
	}

	res, err := h.svcs.AddTeacherPricing(ctx, teacherId, &dto)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) UpdatePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")
	pricingId := chi.URLParam(r, "pricingId")

	var dto models.TeacherPricingUpdateDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return helpers.InvalidJSON()
	}
	if err := helpers.GenericValidation(h.validationInstance, dto); err != nil {
		return err
	}

	res, err := h.svcs.UpdateTeacherPricing(ctx, teacherId, pricingId, &dto)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) DeletePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")
	pricingId := chi.URLParam(r, "pricingId")

	if err := h.svcs.DeleteTeacherPricing(ctx, teacherId, pricingId); err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CompensationHandler) ListPricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.BadRequest("invalid pagination")
	}

	var req pricingListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return helpers.InvalidJSON()
	}

	res, err := h.svcs.GetTeacherPricings(ctx, "", skip, limit, req.Conditions, req.IsActive)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) Quote(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := r.URL.Query().Get("teacherId")
	ref := r.URL.Query().Get("ref")
	refId := r.URL.Query().Get("refId")
	durationStr := r.URL.Query().Get("durationMinutes")
	if teacherId == "" || ref == "" || refId == "" || durationStr == "" {
		return helpers.BadRequest("teacherId, ref, refId, durationMinutes are required")
	}
	durationMinutes, err := strconv.ParseFloat(durationStr, 64)
	if err != nil {
		return helpers.BadRequest("invalid durationMinutes")
	}

	res, err := h.svcs.Quote(ctx, teacherId, ref, refId, durationMinutes)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) AddHours(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")

	var dto models.TeacherHoursCreateDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return helpers.InvalidJSON()
	}
	if err := helpers.GenericValidation(h.validationInstance, dto); err != nil {
		return err
	}

	res, err := h.svcs.AddTeacherHours(ctx, teacherId, &dto)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) UpdateHours(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")
	hoursId := chi.URLParam(r, "hoursId")

	var dto models.TeacherHoursUpdateDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return helpers.InvalidJSON()
	}
	if err := helpers.GenericValidation(h.validationInstance, dto); err != nil {
		return err
	}

	res, err := h.svcs.UpdateTeacherHours(ctx, teacherId, hoursId, &dto)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) RejectHours(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")
	hoursId := chi.URLParam(r, "hoursId")

	var payload struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		return helpers.InvalidJSON()
	}
	if payload.Reason == "" {
		return constant.ErrReasonRequired
	}

	res, err := h.svcs.RejectTeacherHours(ctx, teacherId, hoursId, payload.Reason)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) ListHours(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	teacherId := chi.URLParam(r, "teacherId")
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.BadRequest("invalid pagination")
	}
	var req hoursListRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return helpers.InvalidJSON()
	}

	res, err := h.svcs.GetTeacherHours(ctx, teacherId, skip, limit, req.Conditions, req.Start, req.End, req.Status)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}

func (h *CompensationHandler) Breakdown(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var req summaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return helpers.InvalidJSON()
	}

	res, err := h.svcs.GetCompensationBreakdown(ctx, req.Conditions, req.Start, req.End, req.IncludeRejected, req.HideZeroRecorded)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string][]models.TeacherCompensationSummary{
		"items": res,
	})
}

func (h *CompensationHandler) Summary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var req summaryRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
		return helpers.InvalidJSON()
	}

	res, err := h.svcs.GetCompensationSummary(ctx, req.Conditions, req.Start, req.End, req.IncludeRejected)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}
