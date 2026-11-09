package handler

import (
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

type ReviewHandler struct {
	reviewsvcs         content.ReviewSvcs
	validationInstance *validator.Validate
}

func NewReviewHandler(i *do.Injector, r *chi.Mux) {
	h := &ReviewHandler{
		reviewsvcs:         do.MustInvoke[content.ReviewSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/reviews", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Patch("/{id}/approval", helpers.Make(h.UpdateApproval))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *ReviewHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.ReviewDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.reviewsvcs.Add(ctx, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *ReviewHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	reviewId := chi.URLParam(r, "id")

	var data models.ReviewDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.reviewsvcs.UpdateReview(ctx, reviewId, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *ReviewHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	reviewId := chi.URLParam(r, "id")

	if err := h.reviewsvcs.DeleteReview(ctx, reviewId); err != nil {
		return err
	}

	return helpers.WriteJsonString(w, http.StatusOK, "ok")

}

func (h *ReviewHandler) UpdateApproval(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	reviewId := chi.URLParam(r, "id")

	var data models.ReviewApprovalDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.reviewsvcs.UpdateApproval(ctx, reviewId, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}
