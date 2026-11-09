package handler

import (
	"errors"
	"fmt"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/points"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type PointsRuleHandler struct {
	pointsRuleSvcs     points.PointsRuleSvcs
	validationInstance *validator.Validate
}

func NewPointsRuleHandler(i *do.Injector, r *chi.Mux) {
	h := &PointsRuleHandler{
		pointsRuleSvcs:     do.MustInvoke[points.PointsRuleSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/points-rules", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Post("/bulk", helpers.Make(h.BulkAdd))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Patch("/bulk", helpers.Make(h.BulkPatch))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *PointsRuleHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.pointsRuleSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointsRuleHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.pointsRuleSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointsRuleHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.pointsRuleSvcs.GetAll(ctx, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointsRuleHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.PointsRule
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.pointsRuleSvcs.Add(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointsRuleHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var data *models.PointsRule
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	result, err := h.pointsRuleSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointsRuleHandler) BulkAdd(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var req struct {
		Rules []*models.PointsRule `json:"rules"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return err
	}

	if len(req.Rules) == 0 {
		return errors.New("no points rules provided")
	}

	// Validate each rule
	for i, rule := range req.Rules {
		if rule == nil {
			return fmt.Errorf("rule at index %d is nil", i)
		}
		if err := h.validationInstance.Struct(rule); err != nil {
			return err
		}
	}

	result, err := h.pointsRuleSvcs.BulkAdd(ctx, req.Rules)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointsRuleHandler) BulkPatch(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var req struct {
		Updates []models.BulkPointsRuleUpdate `json:"updates"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return err
	}

	if len(req.Updates) == 0 {
		return errors.New("no updates provided")
	}

	err := h.pointsRuleSvcs.BulkPatch(ctx, req.Updates)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]interface{}{
		"message": "bulk update completed successfully",
		"count":   len(req.Updates),
	})
}

func (h *PointsRuleHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.pointsRuleSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
