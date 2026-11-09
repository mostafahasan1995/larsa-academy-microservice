package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/messaging"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

type MessagingHandler struct {
	messagesvcs messaging.MessageSvcs
}

func NewMessagingHandler(i *do.Injector, r *chi.Mux) {
	h := &MessagingHandler{
		messagesvcs: do.MustInvoke[messaging.MessageSvcs](i),
	}

	r.Route("/messages", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/", helpers.Make(h.GetAll))
	})

}

func (h *MessagingHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.messagesvcs.GetAll(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
