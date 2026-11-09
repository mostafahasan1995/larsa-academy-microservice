package handler

import (
	"errors"
	"io"
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

type ComplaintHandler struct {
	complaintsvcs      support.ComplaintSvcs
	validationInstance *validator.Validate
}

func NewComplaintHandler(i *do.Injector, r *chi.Mux) {
	h := &ComplaintHandler{
		complaintsvcs:      do.MustInvoke[support.ComplaintSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}
	r.Route("/complaints", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetOne))
		r.Get("/stats", helpers.Make(h.GetStats))
		r.With(middleware.Auth("authenticate")).Post("/paginated", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
		r.With(middleware.Auth("authenticate")).Put("/{id}/change-status", helpers.Make(h.ChangeStatus))
		r.With(middleware.Auth("authenticate")).Put("/{id}/assignTo/{assignToId}", helpers.Make(h.AssignTo))
		r.With(middleware.Auth("authenticate")).Post("/{id}/reply", helpers.Make(h.Reply))

	})
	r.Route("/api/complaints", func(r chi.Router) {
		// Support both POST and GET for assigned complaints to work with different clients
		r.With(middleware.Auth("authenticate")).Get("/assigned", helpers.Make(h.GetAssignedComplaints))
		r.With(middleware.Auth("authenticate")).Post("/assigned", helpers.Make(h.GetAssignedComplaints))
		r.With(middleware.Auth("authenticate")).Post("/parent", helpers.Make(h.GetParentComplaints))
	})
}
func (h *ComplaintHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.complaintsvcs.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
// Get returns paginated complaints. Request body supports filter columns (query.Conditions).
// Supported filter columns (must match BSON field names): status, assignTo, complaintType,
// relatedStudent, complaintUserRole, summary, description, createdAt, updatedAt, createdBy, updatedBy.
// Operators: eq, neq, gt, gte, lt, lte, like, in, nin.
func (h *ComplaintHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.complaintsvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *ComplaintHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.ComplaintDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.complaintsvcs.Add(ctx, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *ComplaintHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.ComplaintDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.complaintsvcs.Update(ctx, id, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *ComplaintHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //package id

	if err := h.complaintsvcs.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
func (h *ComplaintHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.ComplaintStatusDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.complaintsvcs.ChangeStatus(ctx, id, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h *ComplaintHandler) AssignTo(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")
	assignToId := chi.URLParam(r, "assignToId")

	var data models.AssignToDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.complaintsvcs.AssignTo(ctx, id, assignToId, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ComplaintHandler) Reply(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.ReplyDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.complaintsvcs.Reply(ctx, id, &data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ComplaintHandler) GetStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Read query parameters: period, from, to, status (open, inProgress, resolved)
	period := r.URL.Query().Get("period")
	from := r.URL.Query().Get("from")
	to := r.URL.Query().Get("to")
	statusFilter := r.URL.Query().Get("status")

	result, err := h.complaintsvcs.GetStats(ctx, period, from, to, statusFilter)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ComplaintHandler) GetAssignedComplaints(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}

	result, err := h.complaintsvcs.GetAssignedComplaints(ctx, cfg.User.Id.Hex(), skip, limit, &query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ComplaintHandler) GetParentComplaints(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
	}

	result, err := h.complaintsvcs.GetParentComplaints(ctx, cfg.User.Id, skip, limit, &query)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
