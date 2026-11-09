package zoom

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/goccy/go-json"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type ZoomHandler struct {
	zoomSvc ZoomService
}

func NewZoomHandler(i *do.Injector, r *chi.Mux) {
	h := &ZoomHandler{
		zoomSvc: do.MustInvoke[ZoomService](i),
	}

	r.Route("/api/zoom", func(r chi.Router) {
		r.With(middleware.Auth("authenticate", CapabilityManageZoomAccounts)).Post("/accounts/list", helpers.Make(h.ListAccounts))
		r.With(middleware.Auth("authenticate", CapabilityManageZoomAccounts)).Post("/accounts", helpers.Make(h.AddAccount))
		r.With(middleware.Auth("authenticate", CapabilityManageZoomAccounts)).Delete("/accounts/{id}", helpers.Make(h.DeleteAccount))
		r.With(middleware.Auth("authenticate", CapabilityManageZoomUsers)).Post("/accounts/{id}/sync-users", helpers.Make(h.SyncUsers))
		r.With(middleware.Auth("authenticate", CapabilityManageZoomUsers)).Post("/users/list", helpers.Make(h.ListUsers))
		r.With(middleware.Auth("authenticate", CapabilityManageZoomUsers)).Post("/users", helpers.Make(h.AddUser))
		r.With(middleware.Auth("authenticate", CapabilityManageZoomUsers)).Delete("/users/{id}", helpers.Make(h.DeleteUser))
		r.With(middleware.Auth("authenticate", CapabilityCreateZoomMeeting)).Post("/meetings", helpers.Make(h.CreateMeeting))
		r.With(middleware.Auth("authenticate", CapabilityCreateZoomMeeting)).Post("/meetings/list", helpers.Make(h.ListMeetings))
	})
}

type AddAccountDto struct {
	Email         string `json:"email" validate:"required,email"`
	AccountName   string `json:"accountName" validate:"required"`
	AccountId     string `json:"accountId" validate:"required"`
	ClientId      string `json:"clientId" validate:"required"`
	ClientSecret  string `json:"clientSecret" validate:"required"`
	WebhookSecret string `json:"webhookSecret" validate:"required"`
}

type CreateMeetingDto struct {
	RefType   string             `json:"refType" validate:"required"`
	RefId     primitive.ObjectID `json:"refId" validate:"required"`
	StartTime time.Time          `json:"startTime" validate:"required"`
	Duration  int                `json:"duration" validate:"required"`
	Topic     string             `json:"topic" validate:"required"`
}

func (h *ZoomHandler) AddAccount(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data AddAccountDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.zoomSvc.AddAccount(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ZoomHandler) ListAccounts(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var queryCond query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &queryCond); err != nil {
		if err.Error() != "EOF" {
			return err
		}
	}

	result, err := h.zoomSvc.ListAccounts(ctx, &queryCond)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ZoomHandler) DeleteAccount(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return helpers.BadRequest("invalid account id")
	}

	if err := h.zoomSvc.DeleteAccount(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "account deleted"})
}

func (h *ZoomHandler) SyncUsers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return helpers.BadRequest("invalid account id")
	}

	count, err := h.zoomSvc.SyncUsers(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]any{
		"message": "users synced",
		"count":   count,
	})
}

func (h *ZoomHandler) ListUsers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var queryCond query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &queryCond); err != nil {
		if err.Error() != "EOF" {
			return err
		}
	}

	result, err := h.zoomSvc.ListUsers(ctx, &queryCond)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ZoomHandler) AddUser(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data AddUserDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.zoomSvc.AddUser(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ZoomHandler) DeleteUser(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	idStr := chi.URLParam(r, "id")
	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return helpers.BadRequest("invalid user id")
	}

	if err := h.zoomSvc.DeleteUser(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "user deleted"})
}

func (h *ZoomHandler) CreateMeeting(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data CreateMeetingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.zoomSvc.CreateMeeting(ctx, data.RefType, data.RefId, data.StartTime, data.Duration, data.Topic)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *ZoomHandler) ListMeetings(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var queryCond query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &queryCond); err != nil {
		if err.Error() != "EOF" {
			return err
		}
	}

	result, err := h.zoomSvc.ListMeetings(ctx, &queryCond)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
