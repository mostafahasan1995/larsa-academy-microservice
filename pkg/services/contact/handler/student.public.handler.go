package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentPublicHandler struct {
	studentsvcs        contact.StudentSvcs
	validationInstance *validator.Validate
}

func NewStudentPublicHandler(i *do.Injector, r *chi.Mux) {
	h := &StudentPublicHandler{
		studentsvcs:        do.MustInvoke[contact.StudentSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/api/students", func(r chi.Router) {
		// Public registration endpoint
		r.Post("/register", helpers.Make(h.Register))

		// Student profile endpoints (authenticated)
		r.With(middleware.Auth("authenticate")).Get("/me", helpers.Make(h.GetMe))
		r.With(middleware.Auth("authenticate")).Patch("/me", helpers.Make(h.UpdateMe))
		r.With(middleware.Auth("authenticate")).Delete("/me", helpers.Make(h.DeleteMe))

		// Favorites endpoints (authenticated)
		r.With(middleware.Auth("authenticate")).Patch("/me/favorites", helpers.Make(h.UpdateFavorites))
		r.With(middleware.Auth("authenticate")).Post("/me/favorites", helpers.Make(h.AddFavorite))
		r.With(middleware.Auth("authenticate")).Delete("/me/favorites/{type}/{refId}", helpers.Make(h.RemoveFavorite))
		r.With(middleware.Auth("authenticate")).Post("/me/favorites/list", helpers.Make(h.ListFavorites))

		r.With(middleware.Auth("authenticate")).Post("/me/suggestions", helpers.Make(h.GetSuggestions))
	})
}

func (h *StudentPublicHandler) Register(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.StudentRegistrationDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	// Validate using struct tags
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.studentsvcs.Register(ctx, &data)
	if err != nil {
		return err
	}

	// Forward specific headers from users service response
	// Check for both canonical form (X-Access-Token) and lowercase (x-access-token)
	getHeaderValue := func(headers map[string][]string, key string) string {
		// Try canonical form first
		if values, ok := headers[http.CanonicalHeaderKey(key)]; ok && len(values) > 0 {
			return values[0]
		}
		// Try lowercase
		if values, ok := headers[key]; ok && len(values) > 0 {
			return values[0]
		}
		return ""
	}

	if accessToken := getHeaderValue(result.Headers, "x-access-token"); accessToken != "" {
		w.Header().Set("x-access-token", accessToken)
	}
	if refreshToken := getHeaderValue(result.Headers, "x-refresh-token"); refreshToken != "" {
		w.Header().Set("x-refresh-token", refreshToken)
	}

	// Parse and forward the user response from users service
	var userResponse map[string]any
	if err := json.Unmarshal(result.UserResponse, &userResponse); err != nil {
		// If parsing fails, still return the response but log the error
		// Fall back to returning the raw bytes
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, writeErr := w.Write(result.UserResponse)
		return writeErr
	}

	// Return the user response from users service
	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, userResponse)
}

func (h *StudentPublicHandler) GetMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Get student by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.GetOne(ctx, studentId)
	if err != nil {
		return err
	}

	// Return sanitized response (exclude sensitive fields)
	response := models.StudentProfileResponseDto{
		Id:                     result.Id.Hex(),
		GeneralInfo:            result.GeneralInfo, // Includes Status
		ContactInfo:            result.ContactInfo,
		AcademicInfo:           result.AcademicInfo,
		AddressDetails:         result.AddressDetails,
		Other:                  result.Other,                       // Includes NotesList
		Email:                  result.AccountStudentSetting.Email, // Only email, not other account settings
		Favorites:              result.Favorites,
		ParentStudentRelations: result.ParentStudentRelations,
		CreatedAt:              result.CreatedAt,
		UpdatedAt:              result.UpdatedAt,
		// Excluded: Seq, AccountStudentSetting (except email), CreatedBy, UpdatedBy, Trash
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *StudentPublicHandler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	var data models.StudentProfileUpdateDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	// Validate using struct tags
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	// Update student by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.UpdateMe(ctx, studentId, &data)
	if err != nil {
		return err
	}

	// Return sanitized response (same as GET /me)
	response := models.StudentProfileResponseDto{
		Id:                     result.Id.Hex(),
		GeneralInfo:            result.GeneralInfo, // Includes Status
		ContactInfo:            result.ContactInfo,
		AcademicInfo:           result.AcademicInfo,
		AddressDetails:         result.AddressDetails,
		Other:                  result.Other, // Includes NotesList
		Email:                  result.AccountStudentSetting.Email,
		Favorites:              result.Favorites,
		ParentStudentRelations: result.ParentStudentRelations,
		CreatedAt:              result.CreatedAt,
		UpdatedAt:              result.UpdatedAt,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

func (h *StudentPublicHandler) DeleteMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Delete student by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	if err := h.studentsvcs.Delete(ctx, studentId); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// UpdateFavorites replaces the entire favorites array for the authenticated student
func (h *StudentPublicHandler) UpdateFavorites(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	var favorites []models.StudentFavorite
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &favorites); err != nil {
		return helpers.InvalidJSON()
	}

	// Validate each favorite
	for i := range favorites {
		if err := helpers.GenericValidation(h.validationInstance, favorites[i]); err != nil {
			return err
		}
	}

	// Update favorites by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.UpdateFavorites(ctx, studentId, favorites)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetSuggestions retrieves suggested courses and coachings for the student
func (h *StudentPublicHandler) GetSuggestions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	var data models.StudentSuggestionRequestDto
	_ = json.NewDecoder(r.Body).Decode(&data) // Optional body

	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.GetSuggestions(ctx, studentId, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// AddFavorite adds a new favorite to the authenticated student's favorites list
func (h *StudentPublicHandler) AddFavorite(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	var favorite models.StudentFavorite
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &favorite); err != nil {
		return helpers.InvalidJSON()
	}

	// Validate the favorite
	if err := helpers.GenericValidation(h.validationInstance, favorite); err != nil {
		return err
	}

	// Add favorite by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.AddFavorite(ctx, studentId, favorite)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// RemoveFavorite removes a specific favorite from the authenticated student's favorites list
func (h *StudentPublicHandler) RemoveFavorite(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	// Get path parameters
	favoriteType := chi.URLParam(r, "type")
	refIdStr := chi.URLParam(r, "refId")

	if favoriteType == "" || refIdStr == "" {
		return helpers.BadRequest("missing type or refId parameter")
	}

	// Validate type
	if favoriteType != "course" && favoriteType != "coaching" && favoriteType != "bundle" {
		return helpers.BadRequest("invalid favorite type, must be 'course', 'coaching', or 'bundle'")
	}

	// Parse refId
	refId, err := primitive.ObjectIDFromHex(refIdStr)
	if err != nil {
		return helpers.BadRequest("invalid refId format")
	}

	// Remove favorite by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.RemoveFavorite(ctx, studentId, favoriteType, refId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// ListFavorites retrieves paginated and enriched favorites for the authenticated student
func (h *StudentPublicHandler) ListFavorites(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var q *query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		q = &query.Conditions{}
	}

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	// Get enriched favorites by user ID (student.Id is the user ID from users microservice)
	studentId := cfg.User.Id.Hex()
	result, err := h.studentsvcs.GetEnrichedFavorites(ctx, studentId, skip, limit, q, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
