package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/points"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type RedeemRequestHandler struct {
	redeemRequestSvcs  points.RedeemRequestSvcs
	validationInstance *validator.Validate
}

func NewRedeemRequestHandler(i *do.Injector, r *chi.Mux) {
	h := &RedeemRequestHandler{
		redeemRequestSvcs:  do.MustInvoke[points.RedeemRequestSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/redeem-requests", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Create))
		r.With(middleware.Auth("authenticate")).Post("/{id}/approve", helpers.Make(h.Approve))
		r.With(middleware.Auth("authenticate")).Post("/{id}/reject", helpers.Make(h.Reject))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *RedeemRequestHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.redeemRequestSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RedeemRequestHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.redeemRequestSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RedeemRequestHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.redeemRequestSvcs.GetAll(ctx, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RedeemRequestHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.RedeemRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.redeemRequestSvcs.Create(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RedeemRequestHandler) Approve(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.redeemRequestSvcs.Approve(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RedeemRequestHandler) Reject(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var reqBody struct {
		Reason string `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &reqBody); err != nil {
		return err
	}
	result, err := h.redeemRequestSvcs.Reject(ctx, id, reqBody.Reason)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RedeemRequestHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.redeemRequestSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
