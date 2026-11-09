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

type CommentHandler struct {
	commentSvcs        content.CommentSvcs
	validationInstance *validator.Validate
}

func NewCommentHandler(i *do.Injector, r *chi.Mux) {
	h := &CommentHandler{
		commentSvcs:        do.MustInvoke[content.CommentSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/comments", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})

}

func (h *CommentHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var comment models.CommentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &comment); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, comment); err != nil {
		return err
	}

	result, err := h.commentSvcs.AddComment(ctx, &comment)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CommentHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var comment models.CommentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &comment); err != nil {
		return err
	}

	result, err := h.commentSvcs.UpdateComments(ctx, id, &comment)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CommentHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //comment id

	err := h.commentSvcs.DeleteComment(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
