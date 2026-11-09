package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/services/sales/enums"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type PaymentHandler struct {
	paymentSvcs        sales.PaymentSvcs
	validationInstance *validator.Validate
}

func NewPaymentHandler(i *do.Injector, r *chi.Mux) {
	h := &PaymentHandler{
		paymentSvcs:        do.MustInvoke[sales.PaymentSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/payments", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Patch("/{id}/status", helpers.Make(h.UpdateStatus))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *PaymentHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.paymentSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	// Remove customerId from response
	response := map[string]interface{}{
		"_id":           result.ID,
		"invoiceId":     result.InvoiceID,
		"invoiceNum":    result.InvoiceNum,
		"amount":        result.Amount,
		"status":        result.Status,
		"paymentType":   result.PaymentType,
		"paymentMethod": result.PaymentMethod,
		"dateTime":      result.DateTime,
		"createdAt":     result.CreatedAt,
		"updatedAt":     result.UpdatedAt,
		"createdBy":     result.CreatedBy,
		"updatedBy":     result.UpdatedBy,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *PaymentHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.paymentSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}

	// Remove customerId from all payments in response
	payments := make([]map[string]interface{}, len(result.Payments))
	for i, payment := range result.Payments {
		payments[i] = map[string]interface{}{
			"_id":           payment.ID,
			"invoiceId":     payment.InvoiceID,
			"invoiceNum":    payment.InvoiceNum,
			"amount":        payment.Amount,
			"status":        payment.Status,
			"paymentType":   payment.PaymentType,
			"paymentMethod": payment.PaymentMethod,
			"dateTime":      payment.DateTime,
			"createdAt":     payment.CreatedAt,
			"updatedAt":     payment.UpdatedAt,
			"createdBy":     payment.CreatedBy,
			"updatedBy":     payment.UpdatedBy,
		}
	}

	response := map[string]interface{}{
		"payments":   payments,
		"pagination": result.Pagination,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *PaymentHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.Payment
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.paymentSvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	// Remove customerId from response
	response := map[string]interface{}{
		"_id":           result.ID,
		"invoiceId":     result.InvoiceID,
		"invoiceNum":    result.InvoiceNum,
		"amount":        result.Amount,
		"status":        result.Status,
		"paymentType":   result.PaymentType,
		"paymentMethod": result.PaymentMethod,
		"dateTime":      result.DateTime,
		"createdAt":     result.CreatedAt,
		"updatedAt":     result.UpdatedAt,
		"createdBy":     result.CreatedBy,
		"updatedBy":     result.UpdatedBy,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *PaymentHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var data *models.Payment
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	result, err := h.paymentSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	// Remove customerId from response
	response := map[string]interface{}{
		"_id":           result.ID,
		"invoiceId":     result.InvoiceID,
		"invoiceNum":    result.InvoiceNum,
		"amount":        result.Amount,
		"status":        result.Status,
		"paymentType":   result.PaymentType,
		"paymentMethod": result.PaymentMethod,
		"dateTime":      result.DateTime,
		"createdAt":     result.CreatedAt,
		"updatedAt":     result.UpdatedAt,
		"createdBy":     result.CreatedBy,
		"updatedBy":     result.UpdatedBy,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *PaymentHandler) UpdateStatus(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")

	var req struct {
		Status enums.PaymentStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return err
	}

	result, err := h.paymentSvcs.UpdateStatus(ctx, id, req.Status)
	if err != nil {
		return err
	}

	// Remove customerId from response
	response := map[string]interface{}{
		"_id":           result.ID,
		"invoiceId":     result.InvoiceID,
		"invoiceNum":    result.InvoiceNum,
		"amount":        result.Amount,
		"status":        result.Status,
		"paymentType":   result.PaymentType,
		"paymentMethod": result.PaymentMethod,
		"dateTime":      result.DateTime,
		"createdAt":     result.CreatedAt,
		"updatedAt":     result.UpdatedAt,
		"createdBy":     result.CreatedBy,
		"updatedBy":     result.UpdatedBy,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *PaymentHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.paymentSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
