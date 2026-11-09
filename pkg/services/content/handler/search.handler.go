package handler

import (
	"encoding/json"
	"fmt"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
)

type GlobalSearchHandler struct {
	searchSvcs content.GlobalSearchSvcs
}

func NewGlobalSearchHandler(i *do.Injector, r *chi.Mux) {
	h := &GlobalSearchHandler{
		searchSvcs: do.MustInvoke[content.GlobalSearchSvcs](i),
	}

	r.Route("/search", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Search))
	})
}

func (h *GlobalSearchHandler) Search(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var req models.GlobalSearchRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		return helpers.BadRequest(err.Error())
	}

	res, err := h.searchSvcs.Search(ctx, &req)
	if err != nil {
		fmt.Printf("Search Error: %v\n", err)
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, res)
}
