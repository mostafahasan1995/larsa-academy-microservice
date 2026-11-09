package handler

import (
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

type PointTransactionHandler struct {
	transactionSvcs    points.PointTransactionSvcs
	validationInstance *validator.Validate
}

func NewPointTransactionHandler(i *do.Injector, r *chi.Mux) {
	h := &PointTransactionHandler{
		transactionSvcs:    do.MustInvoke[points.PointTransactionSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/point-transactions", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.Get("/student/{studentId}", helpers.Make(h.GetByStudentId))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Post("/bulk-assign", helpers.Make(h.BulkAssign))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *PointTransactionHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.transactionSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) GetByStudentId(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	studentId := chi.URLParam(r, "studentId")
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	result, err := h.transactionSvcs.GetByStudentId(ctx, studentId, skip, limit)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.transactionSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.transactionSvcs.GetAll(ctx, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.PointTransaction
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.transactionSvcs.Add(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) BulkAssign(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var req *models.BulkPointAssignmentRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(req); err != nil {
		return err
	}
	result, err := h.transactionSvcs.BulkAssign(ctx, req)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var data *models.PointTransaction
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.transactionSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PointTransactionHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.transactionSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
