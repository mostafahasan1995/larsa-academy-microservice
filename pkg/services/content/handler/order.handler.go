package handler

import (
	"encoding/json"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/samber/do"
)

type OrderHandler struct {
	orderSvcs          content.OrderSvcs
	validationInstance *validator.Validate
}

func NewOrderHandler(i *do.Injector, r *chi.Mux) {
	h := &OrderHandler{
		orderSvcs:          do.MustInvoke[content.OrderSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/api/orders", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/my", helpers.Make(h.MyOrders))
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetOrder))
		r.With(middleware.Auth("authenticate")).Post("/student/{studentId}", helpers.Make(h.GetStudentOrders))
		r.With(middleware.Auth("authenticate")).Post("/{id}/pay", helpers.Make(h.PayOrder))
		r.With(middleware.Auth("authenticate")).Post("/{id}/enroll", helpers.Make(h.EnrollStudent))
		r.With(middleware.Auth("authenticate")).Post("/{id}/coupon", helpers.Make(h.ApplyCoupon))
		r.With(middleware.Auth("authenticate")).Post("/{id}/cancel", helpers.Make(h.CancelOrder))
	})
}

func (h *OrderHandler) ApplyCoupon(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	orderId := chi.URLParam(r, "id")

	var data struct {
		Code string `json:"code" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.orderSvcs.ApplyCoupon(ctx, orderId, data.Code)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *OrderHandler) MyOrders(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil && err.Error() != "EOF" {
			return err
		}
	}

	result, err := h.orderSvcs.GetMyOrders(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *OrderHandler) GetOrder(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	orderId := chi.URLParam(r, "id")
	result, err := h.orderSvcs.GetById(ctx, orderId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *OrderHandler) GetStudentOrders(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).Decode(&query); err != nil && err.Error() != "EOF" {
			return err
		}
	}

	result, err := h.orderSvcs.GetByStudentId(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *OrderHandler) PayOrder(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	orderId := chi.URLParam(r, "id")
	paymentGateway := r.URL.Query().Get("paymentGateway")

	result, err := h.orderSvcs.PayOrder(ctx, orderId, enums.PaymentGateway(paymentGateway))
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *OrderHandler) EnrollStudent(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	orderId := chi.URLParam(r, "id")

	result, err := h.orderSvcs.EnrollStudent(ctx, orderId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *OrderHandler) CancelOrder(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	orderId := chi.URLParam(r, "id")

	result, err := h.orderSvcs.CancelOrder(ctx, orderId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
