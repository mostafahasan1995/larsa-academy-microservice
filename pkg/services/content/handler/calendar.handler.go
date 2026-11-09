package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"strconv"
	"time"

	"errors"
	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type CalendarHandler struct {
	calendarSvcs       content.CalendarSvcs
	validationInstance *validator.Validate
}

func NewCalendarHandler(i *do.Injector, r *chi.Mux) {
	h := &CalendarHandler{
		calendarSvcs:       do.MustInvoke[content.CalendarSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/api/calendar", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/events", helpers.Make(h.GetCalendarEvents))
	})
}

// GetCalendarEvents handles POST /api/calendar/events
// Query Parameters:
//   - view: string (daily|weekly|monthly|yearly|agenda) [required]
//   - startDate: string (ISO 8601 date, required for daily/weekly/monthly)
//   - endDate: string (ISO 8601 date, optional, auto-calculated if not provided)
//   - timezone: string (IANA timezone, optional, default: UTC)
//   - page: int (for agenda view pagination, default: 1)
//   - limit: int (for agenda view pagination, default: 50, max: 100)
func (h *CalendarHandler) GetCalendarEvents(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Parse query parameters
	viewType := r.URL.Query().Get("view")
	if viewType == "" {
		return helpers.BadRequest("view parameter is required")
	}

	// Validate view type
	validViews := map[string]bool{
		"daily":   true,
		"weekly":  true,
		"monthly": true,
		"yearly":  true,
		"agenda":  true,
	}
	if !validViews[viewType] {
		return helpers.BadRequest("invalid view type. Must be one of: daily, weekly, monthly, yearly, agenda")
	}

	// Parse timezone (default to UTC)
	timezone := r.URL.Query().Get("timezone")
	if timezone == "" {
		timezone = "UTC"
	}

	// Parse startDate
	startDateStr := r.URL.Query().Get("startDate")
	if startDateStr == "" && viewType != "agenda" {
		return helpers.BadRequest("startDate parameter is required for this view type")
	}

	var startDate, endDate time.Time
	var err error

	if startDateStr != "" {
		startDate, err = time.Parse("2006-01-02", startDateStr)
		if err != nil {
			// Try ISO 8601 format
			startDate, err = time.Parse(time.RFC3339, startDateStr)
			if err != nil {
				return helpers.BadRequest("invalid startDate format. Use YYYY-MM-DD or ISO 8601")
			}
		}
	} else {
		// For agenda view, default to current time
		startDate = time.Now()
	}

	// Parse endDate (optional)
	endDateStr := r.URL.Query().Get("endDate")
	if endDateStr != "" {
		endDate, err = time.Parse("2006-01-02", endDateStr)
		if err != nil {
			// Try ISO 8601 format
			endDate, err = time.Parse(time.RFC3339, endDateStr)
			if err != nil {
				return helpers.BadRequest("invalid endDate format. Use YYYY-MM-DD or ISO 8601")
			}
		}
	} else {
		// Calculate endDate based on view type
		dateRange, err := h.calendarSvcs.CalculateDateRange(ctx, viewType, startDate, timezone)
		if err != nil {
			return helpers.BadRequest(err.Error())
		}
		startDate = dateRange.Start
		endDate = dateRange.End
	}

	// Validate date range (skip for agenda view as it has no end date)
	if viewType != "agenda" && endDate.Before(startDate) {
		return helpers.BadRequest("endDate must be after startDate")
	}

	// Parse pagination (for agenda view)
	page := 1
	limit := 50
	if viewType == "agenda" {
		pageStr := r.URL.Query().Get("page")
		if pageStr != "" {
			page, err = strconv.Atoi(pageStr)
			if err != nil || page < 1 {
				return helpers.BadRequest("page must be a positive integer")
			}
		}

		limitStr := r.URL.Query().Get("limit")
		if limitStr != "" {
			limit, err = strconv.Atoi(limitStr)
			if err != nil || limit < 1 {
				return helpers.BadRequest("limit must be a positive integer")
			}
			if limit > 100 {
				limit = 100 // Max limit
			}
		}
	}

	// Parse request body
	var req models.CalendarEventsRequest
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &req); err != nil {
		return helpers.BadRequest("invalid request body: " + err.Error())
	}

	// Validate bundle filter if provided
	if req.Filters.BundleFilter != "" {
		validBundleFilters := map[string]bool{
			"all":              true,
			"bundle_only":      true,
			"independent_only": true,
		}
		if !validBundleFilters[req.Filters.BundleFilter] {
			return helpers.BadRequest("invalid bundleFilter. Must be one of: all, bundle_only, independent_only")
		}
	}

	// Call service
	result, err := h.calendarSvcs.GetCalendarEvents(ctx, &req, viewType, startDate, endDate, page, limit)
	if err != nil {
		if errors.Is(err, content.ErrUnauthorizedInsufficientPermissions) ||
			errors.Is(err, content.ErrUnauthorizedOwnEventsOnly) ||
			errors.Is(err, content.ErrUnauthorizedOwnAvailabilityOnly) {
			return helpers.NewApiError(http.StatusForbidden, err)
		}
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
