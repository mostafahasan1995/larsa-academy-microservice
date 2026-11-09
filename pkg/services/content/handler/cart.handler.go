package handler

import (
	"fmt"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type CartHandler struct {
	cartSvcs           content.CartSvcs
	validationInstance *validator.Validate
}

func NewCartHandler(i *do.Injector, r *chi.Mux) {
	h := &CartHandler{
		cartSvcs:           do.MustInvoke[content.CartSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/api/cart", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/", helpers.Make(h.MyCart))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
		r.With(middleware.Auth("authenticate")).Delete("/clear", helpers.Make(h.Clear))
		r.With(middleware.Auth("authenticate")).Post("/checkout", helpers.Make(h.Checkout))
		r.With(middleware.Auth("authenticate")).Post("/coupon", helpers.Make(h.ApplyCoupon))
		r.With(middleware.Auth("authenticate")).Delete("/coupon", helpers.Make(h.RemoveCoupon))
	})
}

func (h *CartHandler) ApplyCoupon(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data struct {
		Code string `json:"code" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.cartSvcs.ApplyCoupon(ctx, data.Code)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CartHandler) RemoveCoupon(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.cartSvcs.RemoveCoupon(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CartHandler) MyCart(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.cartSvcs.MyCart(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CartHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	println("CartHandler Add called")
	var data *models.CartItemDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		fmt.Printf("CartHandler Add DEBUG: Decode error: %v\n", err)
		return err
	}
	fmt.Printf("CartHandler Add DEBUG: Type=%s RefId=%s\n", data.Type, data.RefId.Hex())

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		fmt.Printf("CartHandler Add DEBUG: Validation error: %v\n", err)
		return err
	}

	result, err := h.cartSvcs.AddToCart(ctx, data)
	if err != nil {
		fmt.Printf("CartHandler Add DEBUG: AddToCart error: %v\n", err)
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CartHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.CartItemDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.cartSvcs.UpdateCartItem(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CartHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.cartSvcs.DeleteCartItem(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CartHandler) Clear(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	err := h.cartSvcs.ClearCart(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CartHandler) Checkout(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.cartSvcs.Checkout(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
