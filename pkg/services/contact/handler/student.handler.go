package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type StudentHandler struct {
	studentsvcs        contact.StudentSvcs
	teachersvcs        contact.TeacherSvcs
	reviewsvcs         content.ReviewSvcs
	validationInstance *validator.Validate
}

func NewStudentHandler(i *do.Injector, r *chi.Mux) {
	h := &StudentHandler{
		studentsvcs:        do.MustInvoke[contact.StudentSvcs](i),
		teachersvcs:        do.MustInvoke[contact.TeacherSvcs](i),
		reviewsvcs:         do.MustInvoke[content.ReviewSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/students", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/paginated", helpers.Make(h.Get))
		r.Get("/{id}", helpers.Make(h.GetOne))
		r.Get("/{id}/summary", helpers.Make(h.GetSummary))
		r.With(middleware.Auth("authenticate")).Post("/{id}/reviews", helpers.Make(h.GetReviews))
		r.With(middleware.Auth("authenticate")).Post("/{id}/sessions-log/coaching/{coachingProgramId}/paginated", helpers.Make(h.GetSessionsLog))
		r.Post("/all", helpers.Make(h.GetAll))

		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})

	// Student-facing API routes
	r.With(middleware.Auth("authenticate")).Post("/api/students/me/teachers", helpers.Make(h.GetStudentTeachersMe))
	r.With(middleware.Auth("authenticate")).Post("/api/students//{studentId}/teachers", helpers.Make(h.GetStudentTeachers))
}

func (h *StudentHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.studentsvcs.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.studentsvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		return err
	}

	teacher, err := h.studentsvcs.GetAll(ctx, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, teacher)
}

func (h *StudentHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data models.StudentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.studentsvcs.Add(ctx, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.StudentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.studentsvcs.Update(ctx, id, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") // package id

	if err := h.studentsvcs.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *StudentHandler) GetSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.studentsvcs.GetSummary(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentHandler) GetSessionsLog(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "id")
	coachingProgramId := chi.URLParam(r, "coachingProgramId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var q *query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		q = &query.Conditions{}
	}

	sessions, err := h.studentsvcs.GetSessionsLog(ctx, coachingProgramId, studentId, skip, limit, q)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, sessions)
}

// GetReviews retrieves paginated reviews for a specific student with optional query conditions
func (h *StudentHandler) GetReviews(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "id")

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

	result, err := h.reviewsvcs.GetReviewsByUserIdWithQuery(ctx, studentId, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentTeachersMe retrieves all teachers for the authenticated student
func (h *StudentHandler) GetStudentTeachersMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	studentId := cfg.User.Id.Hex()

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var q *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &q); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		q = nil
	}

	result, err := h.teachersvcs.GetStudentTeachers(ctx, studentId, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentTeachers retrieves all teachers for a specific student by ID
func (h *StudentHandler) GetStudentTeachers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var q *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &q); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		q = nil
	}

	result, err := h.teachersvcs.GetStudentTeachers(ctx, studentId, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// 78   64   27.5 14
