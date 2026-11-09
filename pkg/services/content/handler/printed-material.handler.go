package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type PrintedMaterialHandler struct {
	printedMaterialsvcs content.PrintedMaterialSvcs
	validationInstance  *validator.Validate
}

func NewPrintedMaterialHandler(i *do.Injector, r *chi.Mux) {
	h := &PrintedMaterialHandler{
		printedMaterialsvcs: do.MustInvoke[content.PrintedMaterialSvcs](i),
		validationInstance:  do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/printed-materials", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *PrintedMaterialHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	printedMaterialId := chi.URLParam(r, "id")

	printedMaterial, err := h.printedMaterialsvcs.GetById(ctx, printedMaterialId)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, printedMaterial)
}

func (h *PrintedMaterialHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		return err
	}

	result, err := h.printedMaterialsvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PrintedMaterialHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.PrintedMaterialDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	printedMaterial, err := h.printedMaterialsvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, printedMaterial)
}

func (h *PrintedMaterialHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	printedMaterialId := chi.URLParam(r, "id")

	var data *models.PrintedMaterialDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	printedMaterial, err := h.printedMaterialsvcs.Update(ctx, printedMaterialId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, printedMaterial)
}

func (h *PrintedMaterialHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	printedMaterialId := chi.URLParam(r, "id")

	err := h.printedMaterialsvcs.Delete(ctx, printedMaterialId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
