package webhooks

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"larsa-academy-microservice/pkg/db"
	"larsa-academy-microservice/pkg/services/zoom/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson"
)

type ZoomWebhookHandler struct {
	i           *do.Injector
	meetingRepo db.MainRepo[models.ZoomMeeting]
	userRepo    db.MainRepo[models.ZoomUser]
	accountRepo db.MainRepo[models.ZoomAccount]
}

func NewZoomWebhookHandler(i *do.Injector, r *chi.Mux) *ZoomWebhookHandler {
	h := &ZoomWebhookHandler{
		i:           i,
		meetingRepo: db.NewRepo[models.ZoomMeeting](i),
		userRepo:    db.NewRepo[models.ZoomUser](i),
		accountRepo: db.NewRepo[models.ZoomAccount](i),
	}

	r.Post("/api/webhooks/zoom", h.Handle)
	return h
}

type ZoomWebhookEvent struct {
	Event   string `json:"event"`
	EventTs int64  `json:"event_ts"`
	Payload struct {
		AccountID string `json:"account_id"`
		Object    struct {
			ID        interface{} `json:"id"` // Can be string (meeting) or int64 (recording)
			UUID      string      `json:"uuid"`
			StartTime string      `json:"start_time"`
			EndTime   string      `json:"end_time"`
			ShareURL  string      `json:"share_url"`
		} `json:"object"`
		PlainToken string `json:"plainToken"`
	} `json:"payload"`
}

func (h *ZoomWebhookHandler) Handle(w http.ResponseWriter, r *http.Request) {
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, "could not read body", http.StatusInternalServerError)
		return
	}
	r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

	var event ZoomWebhookEvent
	if err := json.NewDecoder(bytes.NewBuffer(bodyBytes)).Decode(&event); err != nil {
		http.Error(w, "invalid request", http.StatusBadRequest)
		return
	}

	ctx := r.Context()

	// 1. Determine Secret to use
	secret := util.GetEnv("ZOOM_WEBHOOK_SECRET_TOKEN", "")
	if event.Payload.AccountID != "" {
		account, err := h.accountRepo.GetByFilter(ctx, bson.M{"credentials.accountId": event.Payload.AccountID})
		if err == nil && account != nil && account.Credentials.WebhookSecret != "" {
			secret = account.Credentials.WebhookSecret
		}
	}

	// 2. Handle Zoom Validation (Challenge-Response)
	if event.Event == "endpoint.url_validation" {
		h.handleValidation(w, event.Payload.PlainToken, secret)
		return
	}

	// 3. Verify Webhook Signature
	receivedSignature := r.Header.Get("x-zm-signature")
	timestamp := r.Header.Get("x-zm-request-timestamp")

	if !h.verifySignature(secret, timestamp, receivedSignature, bodyBytes) {
		if secret != "" {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	// 4. Process Events
	// Special handling for user events which might not have a numeric ID object
	if event.Event == "user.deleted" || event.Event == "user.deactivated" || event.Event == "user.updated" {
		h.handleUserEvent(ctx, event)
		w.WriteHeader(http.StatusOK)
		return
	}

	zoomMeetingId := fmt.Sprintf("%v", event.Payload.Object.ID)

	switch event.Event {
	case "meeting.started":
		startTime, _ := time.Parse(time.RFC3339, event.Payload.Object.StartTime)
		if startTime.IsZero() {
			startTime = time.Now()
		}
		_, _ = h.meetingRepo.Patch(ctx, bson.M{"zoomMeetingId": zoomMeetingId}, bson.M{
			"$set": bson.M{
				"status":          "started",
				"actualStartTime": &startTime,
			},
		})

	case "meeting.ended":
		endTime, _ := time.Parse(time.RFC3339, event.Payload.Object.EndTime)
		if endTime.IsZero() {
			endTime = time.Now()
		}
		_, _ = h.meetingRepo.Patch(ctx, bson.M{"zoomMeetingId": zoomMeetingId}, bson.M{
			"$set": bson.M{
				"status":        "completed",
				"actualEndTime": &endTime,
			},
		})

	case "recording.completed":
		shareUrl := event.Payload.Object.ShareURL
		if shareUrl != "" {
			_, _ = h.meetingRepo.Patch(ctx, bson.M{"zoomMeetingId": zoomMeetingId}, bson.M{
				"$set": bson.M{
					"recordingUrl": shareUrl,
				},
			})
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *ZoomWebhookHandler) handleValidation(w http.ResponseWriter, plainToken string, secret string) {

	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(plainToken))
	encryptedToken := hex.EncodeToString(hash.Sum(nil))

	response := map[string]string{
		"plainToken":     plainToken,
		"encryptedToken": encryptedToken,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (h *ZoomWebhookHandler) handleUserEvent(ctx context.Context, event ZoomWebhookEvent) {
	userId := fmt.Sprintf("%v", event.Payload.Object.ID)
	if userId == "" || userId == "<nil>" {
		return
	}

	switch event.Event {
	case "user.deleted", "user.deactivated":
		_, _ = h.userRepo.Patch(ctx, bson.M{"zoomUserId": userId}, bson.M{
			"$set": bson.M{"status": "inactive"},
		})
	case "user.updated":
		// Could update email/name here if needed, but for now just ensure status is reflected
		// Zoom sends 'status' in payload for updated events sometimes
	}
}

func (h *ZoomWebhookHandler) verifySignature(secret, timestamp, signature string, body []byte) bool {
	if secret == "" {
		return true
	}

	message := fmt.Sprintf("v0:%s:%s", timestamp, string(body))
	hash := hmac.New(sha256.New, []byte(secret))
	hash.Write([]byte(message))
	expectedSignature := fmt.Sprintf("v0=%s", hex.EncodeToString(hash.Sum(nil)))

	return expectedSignature == signature
}
