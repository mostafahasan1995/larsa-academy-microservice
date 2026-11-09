package handler

import (
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/sales"
	"larsa-academy-microservice/pkg/services/sales/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"larsa-academy-microservice/pkg/query"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type InvoiceHandler struct {
	invoiceSvcs        sales.InvoiceSvcs
	validationInstance *validator.Validate
}

func NewInvoiceHandler(i *do.Injector, r *chi.Mux) {
	h := &InvoiceHandler{
		invoiceSvcs:        do.MustInvoke[sales.InvoiceSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/invoices", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Get("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
		r.With(middleware.Auth("authenticate")).Post("/calculate-products", helpers.Make(h.CalculateProducts))
		r.With(middleware.Auth("authenticate")).Post("/calculate-invoice-total", helpers.Make(h.CalculateInvoiceTotal))
	})
}

func (h *InvoiceHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.invoiceSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *InvoiceHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}
	result, pagination, err := h.invoiceSvcs.Get(ctx, skip, limit, query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]interface{}{
		"invoices":   result,
		"pagination": pagination,
	})
}

func (h *InvoiceHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	result, err := h.invoiceSvcs.GetAll(ctx)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *InvoiceHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.Invoice
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.invoiceSvcs.Add(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *InvoiceHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var data *models.Invoice
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	result, err := h.invoiceSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *InvoiceHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.invoiceSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// CalculateProducts handles API 1: Calculate products with their discounts and coupons
func (h *InvoiceHandler) CalculateProducts(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var req models.ProductCalculationRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return fmt.Errorf("JSON decoding error: %v", err)
	}

	// Validate that products are not empty
	if len(req.Products) == 0 {
		return errors.New("no products provided for calculation")
	}

	// Validate each product
	for i, product := range req.Products {
		if product == nil {
			return fmt.Errorf("product at index %d is nil", i)
		}
		if product.Price < 0 {
			return fmt.Errorf("invalid price for product at index %d", i)
		}
	}

	result, err := h.invoiceSvcs.CalculateProducts(ctx, &req)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// CalculateInvoiceTotal handles API 2: Calculate invoice-level discount, coupon, and fees
func (h *InvoiceHandler) CalculateInvoiceTotal(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var req models.InvoiceTotalCalculationRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return fmt.Errorf("JSON decoding error: %v", err)
	}

	// Validate subtotal
	if req.SubTotal < 0 {
		return errors.New("subtotal cannot be negative")
	}

	result, err := h.invoiceSvcs.CalculateInvoiceTotal(ctx, &req)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
