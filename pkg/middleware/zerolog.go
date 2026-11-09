package middleware

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5/middleware"
	"github.com/rs/zerolog/log"
)

// ZerologRequestLogger is a middleware that logs the start and end of each request using zerolog
func ZerologRequestLogger(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Use chi's middleware.WrapResponseWriter to capture the response status code
		ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)

		// Proceed to next middleware/handler
		next.ServeHTTP(ww, r)

		// Calculate duration
		duration := time.Since(start)

		// Create a logger event
		event := log.Info().
			Str("method", r.Method).
			Str("path", r.URL.Path).
			Int("status", ww.Status()).
			Dur("duration", duration).
			Str("ip", r.RemoteAddr).
			Str("user_agent", r.UserAgent())

		// Log useful information for peak times and load
		// You can analyze these logs to find:
		// - Request count per minute/hour (using timestamp)
		// - Peak response times (using duration)
		// - Error rates (using status)

		// Flag slow requests
		if duration > 500*time.Millisecond {
			event.Bool("slow_request", true)
		}

		event.Msg("http_request")
	})
}
