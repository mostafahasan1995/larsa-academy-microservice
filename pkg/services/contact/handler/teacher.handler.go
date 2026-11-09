package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	content "larsa-academy-microservice/pkg/services/content"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/types"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type TeacherHandler struct {
	teachersvcs        contact.TeacherSvcs
	reviewsvcs         content.ReviewSvcs
	validationInstance *validator.Validate
}

func NewTeacherHandler(i *do.Injector, r *chi.Mux) {
	h := &TeacherHandler{
		teachersvcs:        do.MustInvoke[contact.TeacherSvcs](i),
		reviewsvcs:         do.MustInvoke[content.ReviewSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	// Dashboard/admin routes
	r.Route("/teachers", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.Get("/{id}/summary", helpers.Make(h.GetSummary))
		r.With(middleware.Auth("authenticate")).Post("/{id}/reviews", helpers.Make(h.GetReviews))
		r.Post("/paginated", helpers.Make(h.Get))
		r.Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Post("/invite/", helpers.Make(h.AddWithInvite))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))

		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
	})

	// Teacher-facing API routes
	r.Route("/api/teachers", func(r chi.Router) {
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Get("/me", helpers.Make(h.GetMe))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Patch("/me", helpers.Make(h.UpdateMe))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Get("/me/today-summary", helpers.Make(h.GetTodaySummary))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/students/list", helpers.Make(h.GetTeacherStudents))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Get("/students/{studentId}", helpers.Make(h.GetTeacherStudentProfile))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Get("/students/{studentId}/reviews", helpers.Make(h.GetTeacherStudentReviews))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/reviews", helpers.Make(h.GetMyReviews))
	})
}

func (h *TeacherHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	teacher, err := h.teachersvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, teacher)
}

func (h *TeacherHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		return err
	}

	teacher, err := h.teachersvcs.Get(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, teacher)
}

func (h *TeacherHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		return err
	}

	teacher, err := h.teachersvcs.GetAll(ctx, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, teacher)
}

func (h *TeacherHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.TeacherDto
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	teacher, err := h.teachersvcs.Add(ctx, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, teacher)
}

func (h *TeacherHandler) AddWithInvite(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.TeacherDto
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	teacher, err := h.teachersvcs.AddWithInvite(ctx, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, teacher)
}

func (h *TeacherHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.TeacherDto
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	teacher, err := h.teachersvcs.Update(ctx, id, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, teacher)
}

func (h *TeacherHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") // package id

	if err := h.teachersvcs.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *TeacherHandler) GetSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.teachersvcs.GetSummary(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *TeacherHandler) GetReviews(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	// Read body once
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	var q *query.Conditions

	// Try to decode as nested query.condition structure first
	if len(bodyBytes) > 0 {
		var requestBody struct {
			Query struct {
				Condition *query.Conditions `json:"condition"`
			} `json:"query"`
		}

		if err := json.Unmarshal(bodyBytes, &requestBody); err == nil && requestBody.Query.Condition != nil {
			// Successfully decoded nested structure
			q = requestBody.Query.Condition
		} else {
			// Try to decode as direct query.Conditions (backward compatibility)
			if err := json.Unmarshal(bodyBytes, &q); err != nil {
				q = &query.Conditions{}
			}
		}
	} else {
		q = &query.Conditions{}
	}

	result, err := h.teachersvcs.GetReviews(ctx, id, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// TEACHER-FACING API HANDLERS

// GetTeacherStudents retrieves all students assigned to the authenticated teacher across all coachings
func (h *TeacherHandler) GetTeacherStudents(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		// Allow empty body - treat as no filtering
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.teachersvcs.GetTeacherStudents(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetTeacherStudentProfile retrieves comprehensive student profile for the authenticated teacher
func (h *TeacherHandler) GetTeacherStudentProfile(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	result, err := h.teachersvcs.GetTeacherStudentProfile(ctx, studentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetTeacherStudentReviews retrieves paginated reviews for a specific student (filtered by teacher's coachings)
func (h *TeacherHandler) GetTeacherStudentReviews(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	// Get coaching IDs where teacher is assigned with this student
	coachingIds, err := h.teachersvcs.GetTeacherStudentCoachingIds(ctx, studentId)
	if err != nil {
		return err
	}

	// If no coachings, return empty result
	if len(coachingIds) == 0 {
		return helpers.WriteJsonCtx(ctx, w, http.StatusOK, &content_models.ReviewPagination{
			Reviews:    []content_models.ReviewRes{},
			Pagination: types.Pagination{TotalPages: 0, PerPage: 0, TotalCount: 0},
		})
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.reviewsvcs.GetReviewsByUserIdAndCoachings(ctx, studentId, coachingIds, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetMyReviews retrieves reviews for the authenticated teacher
func (h *TeacherHandler) GetMyReviews(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	// Read body once
	bodyBytes, err := io.ReadAll(r.Body)
	if err != nil && !errors.Is(err, io.EOF) {
		return err
	}

	var q *query.Conditions

	// Try to decode as nested query.condition structure first
	if len(bodyBytes) > 0 {
		var requestBody struct {
			Query struct {
				Condition *query.Conditions `json:"condition"`
			} `json:"query"`
		}

		if err := json.Unmarshal(bodyBytes, &requestBody); err == nil && requestBody.Query.Condition != nil {
			// Successfully decoded nested structure
			q = requestBody.Query.Condition
		} else {
			// Try to decode as direct query.Conditions (backward compatibility)
			if err := json.Unmarshal(bodyBytes, &q); err != nil {
				q = &query.Conditions{}
			}
		}
	} else {
		q = &query.Conditions{}
	}

	result, err := h.teachersvcs.GetMyReviews(ctx, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetMe retrieves the authenticated teacher's profile
func (h *TeacherHandler) GetMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Get teacher by user ID (teacher.Id is the user ID from users microservice)
	result, err := h.teachersvcs.GetMe(ctx)
	if err != nil {
		return err
	}

	// Return sanitized response (exclude sensitive fields)
	response := models.TeacherProfileResponseDto{
		Id:            result.Id.Hex(),
		FirstName:     result.FirstName,
		LastName:      result.LastName,
		Gender:        result.Gender,
		DateOfBirth:   result.DateOfBirth,
		Nationality:   result.Nationality,
		Picture:       result.Picture,
		Mobile:        result.Mobile,
		Email:         result.AccountSettings.Email, // Only email, not other account settings
		Qualification: result.Qualification,
		Subjects:      result.Subjects,
		Address:       result.Address,
		Availability:  result.Availability,
		CreatedAt:     result.CreatedAt,
		UpdatedAt:     result.UpdatedAt,
		// Excluded: Seq, EmploymentStatus, NationalID, HireDate, JoiningDate, IdDocument,
		// AccountSettings (except email), RoleAndPermissions, CreatedBy, UpdatedBy, Trash
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

// UpdateMe updates the authenticated teacher's profile
func (h *TeacherHandler) UpdateMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	var data models.TeacherProfileUpdateDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	// Validate using struct tags
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	// Update teacher by user ID (teacher.Id is the user ID from users microservice)
	teacherId := cfg.User.Id.Hex()
	result, err := h.teachersvcs.UpdateMe(ctx, teacherId, &data)
	if err != nil {
		return err
	}

	// Return sanitized response (same as GET /me)
	response := models.TeacherProfileResponseDto{
		Id:            result.Id.Hex(),
		FirstName:     result.FirstName,
		LastName:      result.LastName,
		Gender:        result.Gender,
		DateOfBirth:   result.DateOfBirth,
		Nationality:   result.Nationality,
		Picture:       result.Picture,
		Mobile:        result.Mobile,
		Email:         result.AccountSettings.Email,
		Qualification: result.Qualification,
		Subjects:      result.Subjects,
		Address:       result.Address,
		Availability:  result.Availability,
		CreatedAt:     result.CreatedAt,
		UpdatedAt:     result.UpdatedAt,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, response)
}

// GetTodaySummary retrieves today's session summary for the authenticated teacher
func (h *TeacherHandler) GetTodaySummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.teachersvcs.GetTodaySummary(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
