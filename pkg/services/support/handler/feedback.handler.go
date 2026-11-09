package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/support"
	"larsa-academy-microservice/pkg/services/support/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type FeedbackHandler struct {
	feedbacksvcs       support.FeedbackSvcs
	validationInstance *validator.Validate
}

func NewFeedbackHandler(i *do.Injector, r *chi.Mux) {
	h := &FeedbackHandler{
		feedbacksvcs:       do.MustInvoke[support.FeedbackSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/feedbacks", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetOne))
		r.With(middleware.Auth("authenticate")).Post("/paginated", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Get("/stats", helpers.Make(h.GetStats))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})

}
func (h *FeedbackHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.feedbacksvcs.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *FeedbackHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.feedbacksvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *FeedbackHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.FeedbackDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.feedbacksvcs.Add(ctx, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *FeedbackHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.FeedbackDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.feedbacksvcs.Update(ctx, id, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *FeedbackHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //package id

	if err := h.feedbacksvcs.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *FeedbackHandler) GetStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.feedbacksvcs.GetStats(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
