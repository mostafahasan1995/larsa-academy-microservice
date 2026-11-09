package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type CouponHandler struct {
	couponSvcs         sales.CouponSvcs
	validationInstance *validator.Validate
}

func NewCouponHandler(i *do.Injector, r *chi.Mux) {
	h := &CouponHandler{
		couponSvcs:         do.MustInvoke[sales.CouponSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/coupons", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Get("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *CouponHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.couponSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CouponHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}
	result, err := h.couponSvcs.Get(ctx, skip, limit, query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CouponHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	result, err := h.couponSvcs.GetAll(ctx)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *CouponHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.Coupon
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.couponSvcs.Add(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CouponHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var data *models.Coupon
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	result, err := h.couponSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CouponHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.couponSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
