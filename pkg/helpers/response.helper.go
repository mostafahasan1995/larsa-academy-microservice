package helpers

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/goccy/go-json"
	//"encoding/json"
)

type APIFunc func(w http.ResponseWriter, r *http.Request) error

func Make(h APIFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {

		err := h(w, r)
		if err != nil {
			if apiErr, ok := err.(APIError); ok {
				WriteJsonCtx(r.Context(), w, apiErr.StatusCode, apiErr)
			} else {
				//WriteJson(w, http.StatusBadRequest, err.Error())
				errResp := map[string]any{
					"statusCode": http.StatusBadRequest,
					"message":    err.Error(),
				}
				WriteJsonCtx(r.Context(), w, http.StatusBadRequest, errResp)
			}
			if flag.Lookup("test.v") == nil {
				slog.Error("HTTP API error", "err", err.Error(), "path", r.URL.Path)
			}
		}
		// Suppress per-request success logs during `go test` runs to keep test output quiet.
		if flag.Lookup("test.v") == nil {
			slog.Info("HTTP API success", "path", r.URL.Path)
		}
	}
}

func WriteJsonCtx(ctx context.Context, w http.ResponseWriter, status int, v any) error {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).EncodeContext(ctx, v); err != nil {
		return err
	}
	return nil
}

func WriteJsonString(w http.ResponseWriter, status int, v string) error {
	fmt.Println("serve form cache")
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(v)); err != nil {
		return err
	}

	return nil
}
