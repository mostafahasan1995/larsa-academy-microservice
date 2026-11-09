package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/enums"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type CoachingHandler struct {
	coachingSvcs       content.CoachingSvcs
	reviewsvcs         content.ReviewSvcs
	commentSvcs        content.CommentSvcs
	coursesvcs         content.CourseSvcs
	validationInstance *validator.Validate
}

func NewCoachingHandler(i *do.Injector, r *chi.Mux) {
	h := &CoachingHandler{
		coachingSvcs:       do.MustInvoke[content.CoachingSvcs](i),
		reviewsvcs:         do.MustInvoke[content.ReviewSvcs](i),
		commentSvcs:        do.MustInvoke[content.CommentSvcs](i),
		coursesvcs:         do.MustInvoke[content.CourseSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	//dashboard
	r.Route("/coaching", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Delete("/{id}", helpers.Make(h.Delete))
		r.With(middleware.Auth("authenticate")).Post("/{id}/members", helpers.Make(h.GetCoachingProgramMembers))
		// Remove a member from coaching (soft-delete all sessions)
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Delete("/{id}/members/{studentId}", helpers.Make(h.RemoveCoachingProgramMember))

		//coaching assignment
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Post("/{id}/assignments", helpers.Make(h.AddAssignment))
		r.With(middleware.Auth("authenticate")).Post("/{id}/assignments/all", helpers.Make(h.GetCoachingAllAssignments))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Put("/{id}/assignments/{assignmentId}", helpers.Make(h.UpdateAssignment))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Delete("/{id}/assignments/{assignmentId}", helpers.Make(h.DeleteAssignment))
		r.With(middleware.Auth("authenticate")).Post("/{id}/assignments/{assignmentId}/submissions", helpers.Make(h.GetCoachingAssignmentStudentSubmissions))
		r.With(middleware.Auth("authenticate")).Get("/{id}/assignments/{assignmentId}/submissions/{studentAssignmentId}", helpers.Make(h.GetCoachingStudentAssignment))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Patch("/{id}/assignments/{studentAssignmentId}/review-status", helpers.Make(h.UpdateCoachingStudentAssignmentReviewStatus))
		//pricing
		r.With(middleware.Auth("authenticate", content.CapabilityManageCoachingPricing)).Post("/{id}/pricing", helpers.Make(h.AddCoachingPricing))
		r.With(middleware.Auth("authenticate", content.CapabilityManageCoachingPricing)).Put("/{id}/pricing/{pricingId}", helpers.Make(h.UpdateCoachingPricing))
		r.With(middleware.Auth("authenticate", content.CapabilityManageCoachingPricing)).Delete("/{id}/pricing/{pricingId}", helpers.Make(h.DeleteCoachingPricing))
		r.With(middleware.Auth("authenticate")).Post("/{id}/pricing/list", helpers.Make(h.GetCoachingPricing))

		//coaching test
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Post("/{id}/tests", helpers.Make(h.AddTest))
		r.With(middleware.Auth("authenticate")).Post("/{id}/tests/all", helpers.Make(h.GetCoachingAllTests))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Put("/{id}/tests/{testId}", helpers.Make(h.UpdateTest))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllCoachingPrograms)).Delete("/{id}/tests/{testId}", helpers.Make(h.DeleteTest))
		r.With(middleware.Auth("authenticate")).Post("/{id}/tests/{testId}/submissions", helpers.Make(h.GetCoachingTestStudentSubmissions))
		r.With(middleware.Auth("authenticate")).Get("/{id}/tests/{testId}/submissions/{studentTestId}", helpers.Make(h.GetCoachingStudentTest))
		r.With(middleware.Auth("authenticate")).Patch("/{id}/tests/{studentTestId}/review-status", helpers.Make(h.UpdateCoachingStudentTestReviewStatus))

		//comments
		r.With(middleware.Auth("authenticate")).Post("/{id}/comments", helpers.Make(h.GetCoachingComments))

		//reviews
		r.With(middleware.Auth("authenticate")).Get("/{id}/reviews", helpers.Make(h.GetReviews))

		//session details
		r.With(middleware.Auth("authenticate")).Get("/sessions/{sessionId}/details", helpers.Make(h.GetSessionById))
		r.With(middleware.Auth("authenticate")).Get("/{id}/sessions/{sessionNum}/resources/{resourceType}", helpers.Make(h.GetStudentSessionResources))
		r.With(middleware.Auth("authenticate")).Post("/session-details", helpers.Make(h.GetSessionDetails))

		r.With(middleware.Auth("authenticate")).Patch("/sessions/bulk", helpers.Make(h.BulkUpdateStudentSessions))
		// admin-facing unified justification requests per teacher (absence/reschedule) using query.Conditions
		r.With(middleware.Auth("authenticate")).Post("/teachers/{teacherId}/justification-requests", helpers.Make(h.GetTeacherJustificationRequests))
	})

	//mobile
	r.Route("/api/coaching", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/{id}/assignments", helpers.Make(h.AddAssignment))

		r.With(middleware.Auth("authenticate")).Post("/student/list", helpers.Make(h.GetStudentList))
		r.With(middleware.Auth("authenticate")).Post("/student/{studentId}/list", helpers.Make(h.GetParentStudentList))

		r.With(middleware.Auth("authenticate")).Get("/my-programs", helpers.Make(h.MyPrograms))
		r.With(middleware.Auth("authenticate")).Get("/parent/student/{studentId}/programs", helpers.Make(h.ParentPrograms))
		r.With(middleware.Auth("authenticate")).Post("/booking", helpers.Make(h.BookingSessions))
		r.With(middleware.Auth("authenticate")).Get("/{coachingProgramId}/teacher-availability/{teacherId}", helpers.Make(h.TeacherAvailability))
		r.With(middleware.Auth("authenticate")).Get("/{coachingProgramId}/teachers/availability", helpers.Make(h.GetCoachingTeachersAvailability))
		r.With(middleware.Auth("authenticate")).Get("/{coachingProgramId}/availability/grouped", helpers.Make(h.GetCoachingGroupedAvailability))
		r.With(middleware.Auth("authenticate")).Get("/{coachingProgramId}/sessions/{stage}", helpers.Make(h.GetSessionsByStage))
		r.With(middleware.Auth("authenticate")).Get("/{coachingProgramId}/student/{studentId}/sessions", helpers.Make(h.GetStudentSessionsForCoaching))
		r.With(middleware.Auth("authenticate")).Get("/{coachingId}/assignments/all", helpers.Make(h.GetCoachingAssignmentsForStudent))

		//
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/absence", helpers.Make(h.AddAbsenceJustification))
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/reschedule", helpers.Make(h.RescheduleRequest))
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/justification-decision", helpers.Make(h.JustificationDecision))
		//student session resources
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/update-notes", helpers.Make(h.UpdateStudentSessionNotes))
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/update-resources", helpers.Make(h.UpdateStudentSessionResources))
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/update-agendas", helpers.Make(h.UpdateStudentSessionAgendas))
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/update-feedback", helpers.Make(h.UpdateStudentSessionFeedback))
		//calendar event management
		r.With(middleware.Auth("authenticate")).Get("/sessions/{studentSessionId}", helpers.Make(h.GetStudentSessionById))
		r.With(middleware.Auth("authenticate")).Patch("/sessions/{studentSessionId}/time", helpers.Make(h.UpdateStudentSessionTime))
		r.With(middleware.Auth("authenticate")).Delete("/sessions/{studentSessionId}", helpers.Make(h.DeleteStudentSession))

		//coaching student assignment
		r.With(middleware.Auth("authenticate")).Get("/assignments/{assignmentId}/start", helpers.Make(h.StartCoachingAssignment))
		r.With(middleware.Auth("authenticate")).Post("/assignments/{studentAssignmentId}/submit", helpers.Make(h.SubmitCoachingStudentAssignment))

		//coaching student test
		r.With(middleware.Auth("authenticate")).Get("/tests/{testId}/start", helpers.Make(h.StartCoachingStudentTest))
		r.With(middleware.Auth("authenticate")).Post("/tests/{studentTestId}/submit", helpers.Make(h.SubmitCoachingStudentTest))

	})

	//teacher-facing endpoints (coaching-specific)
	r.Route("/api/teachers/coachings", func(r chi.Router) {
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/list", helpers.Make(h.GetTeacherCoachings))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/{coachingId}/sessions/list", helpers.Make(h.GetTeacherCoachingSessions))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/{coachingId}/members/list", helpers.Make(h.GetTeacherCoachingMembers))
	})

	//supervisor-facing endpoints (session details from dashboard)
	r.Route("/api/supervisors", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/{supervisorId}/sessions/{sessionId}", helpers.Make(h.GetSessionDetailsForSupervisor))
	})

	// Student enrolled coachings endpoint
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/coachings/enrolled", helpers.Make(h.GetEnrolledCoachings))

	// Student sessions and stats endpoints
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/sessions", helpers.Make(h.GetAllStudentSessions))
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/coachings/stats", helpers.Make(h.GetStudentCoachingStats))
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/learning/stats", helpers.Make(h.GetStudentLearningStats))

}

func (h *CoachingHandler) GetStudentSessionResources(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingProgramId := chi.URLParam(r, "id")
	sessionNum := chi.URLParam(r, "sessionNum")
	resourceType := chi.URLParam(r, "resourceType")

	seNum, err := strconv.Atoi(sessionNum)
	if err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetStudentSessionResources(ctx, coachingProgramId, resourceType, seNum)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetCoachingAssignmentsForStudent(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "coachingId")

	result, err := h.coachingSvcs.GetCoachingAssignmentsForStudent(ctx, coachingId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) BulkUpdateStudentSessions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.BulkUpdateSessionsDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	err := h.coachingSvcs.BulkUpdateStudentSessions(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CoachingHandler) MyPrograms(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetStudentsPrograms(ctx, cfg.User.Id.Hex(), cfg.User.Id.Hex())
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) ParentPrograms(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	studentId := chi.URLParam(r, "studentId")

	result, err := h.coachingSvcs.GetStudentsPrograms(ctx, studentId, cfg.User.Id.Hex())
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) TeacherAvailability(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	teacherId := chi.URLParam(r, "teacherId")
	coachingProgramId := chi.URLParam(r, "coachingProgramId")

	result, err := h.coachingSvcs.GetTeacherAvailability(ctx, teacherId, coachingProgramId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetCoachingTeachersAvailability(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingProgramId := chi.URLParam(r, "coachingProgramId")

	result, err := h.coachingSvcs.GetCoachingTeachersAvailability(ctx, coachingProgramId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetCoachingGroupedAvailability(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingProgramId := chi.URLParam(r, "coachingProgramId")

	result, err := h.coachingSvcs.GetCoachingGroupedAvailability(ctx, coachingProgramId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) BookingSessions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.BookingSessions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.BookingSessions(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichCoachs := false
	if val := r.URL.Query().Get("enrichCoachs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichCoachs = parsed
		}
	}

	id := chi.URLParam(r, "id")

	result, err := h.coachingSvcs.GetById(ctx, id, enrichCoachs)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentList godoc
// @Summary Get coachings with student favorites
// @Description Get a list of coachings with pagination and IsFavorite flag
// @Tags Coaching
// @Accept json
// @Produce json
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Param enrichCoachs query bool false "Enrich coaches"
// @Param includePlacementTests query bool false "Include placement tests"
// @Param query query string false "Query conditions"
// @Success 200 {object} models.StudentCoachingPagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /api/coaching/student/list [post]
func (h *CoachingHandler) GetStudentList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichCoachs := false
	if val := r.URL.Query().Get("enrichCoachs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichCoachs = parsed
		}
	}

	includePlacementTests := false
	if val := r.URL.Query().Get("includePlacementTests"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			includePlacementTests = parsed
		}
	}

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetStudentList(ctx, "", skip, limit, query, includePlacementTests, enrichCoachs, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetParentStudentList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	enrichCoachs := false
	if val := r.URL.Query().Get("enrichCoachs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichCoachs = parsed
		}
	}

	includePlacementTests := false
	if val := r.URL.Query().Get("includePlacementTests"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			includePlacementTests = parsed
		}
	}

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetStudentList(ctx, studentId, skip, limit, query, includePlacementTests, enrichCoachs, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	includePlacementTests := false
	if val := r.URL.Query().Get("includePlacementTests"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			includePlacementTests = parsed
		}
	}

	enrichCoachs := false
	if val := r.URL.Query().Get("enrichCoachs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichCoachs = parsed
		}
	}

	enrichPrice := false
	if val := r.URL.Query().Get("enrichPrice"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichPrice = parsed
		}
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.Get(ctx, skip, limit, query, includePlacementTests, enrichCoachs, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichCoachs := false
	if val := r.URL.Query().Get("enrichCoachs"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichCoachs = parsed
		}
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetAll(ctx, query, enrichCoachs)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.CoachingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.coachingSvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.CoachingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.coachingSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.coachingSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CoachingHandler) GetCoachingProgramMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingProgramId := chi.URLParam(r, "id")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var query *query.Conditions
	if r.Body != nil && r.Body != http.NoBody {
		if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
			return err
		}
	}
	result, err := h.coachingSvcs.GetCoachingProgramMembers(ctx, coachingProgramId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CoachingHandler) RemoveCoachingProgramMember(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingProgramId := chi.URLParam(r, "id")
	studentId := chi.URLParam(r, "studentId")

	if coachingProgramId == "" || studentId == "" {
		return errors.New("invalid parameters")
	}

	if err := h.coachingSvcs.RemoveCoachingProgramMember(ctx, coachingProgramId, studentId); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// Coaching Assignment Handlers
func (h *CoachingHandler) GetCoachingAllAssignments(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetCoachingAllAssignments(ctx, coachingId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) AddAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

	var data *models.CoachingAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.AddCoachingAssignment(ctx, coachingId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, result)
}

func (h *CoachingHandler) UpdateAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	assignmentId := chi.URLParam(r, "assignmentId")
	coachingId := chi.URLParam(r, "id")

	var data *models.CoachingAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateCoachingAssignment(ctx, coachingId, assignmentId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	assignmentId := chi.URLParam(r, "assignmentId")

	err := h.coachingSvcs.DeleteCoachingAssignment(ctx, assignmentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CoachingHandler) GetCoachingAssignmentStudentSubmissions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	assignmentId := chi.URLParam(r, "assignmentId")
	coachingId := chi.URLParam(r, "id")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetCoachingAssignmentStudentSubmissions(ctx, coachingId, assignmentId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetCoachingStudentAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentAssignmentId := chi.URLParam(r, "studentAssignmentId")

	result, err := h.coachingSvcs.GetCoachingStudentAssignment(ctx, studentAssignmentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Coaching Test Handlers
func (h *CoachingHandler) GetCoachingAllTests(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetCoachingAllTests(ctx, coachingId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) AddTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

	var data *models.CoachingTestDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.AddCoachingTest(ctx, coachingId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, result)
}

func (h *CoachingHandler) UpdateTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	testId := chi.URLParam(r, "testId")
	coachingId := chi.URLParam(r, "id")

	var data *models.CoachingTestDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateCoachingTest(ctx, coachingId, testId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) DeleteTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	testId := chi.URLParam(r, "testId")

	err := h.coachingSvcs.DeleteCoachingTest(ctx, testId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CoachingHandler) GetCoachingTestStudentSubmissions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")
	testId := chi.URLParam(r, "testId")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coachingSvcs.GetCoachingTestStudentSubmissions(ctx, coachingId, testId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CoachingHandler) GetCoachingStudentTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentTestId := chi.URLParam(r, "studentTestId")

	result, err := h.coachingSvcs.GetCoachingStudentTest(ctx, studentTestId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

// pricing
func (h *CoachingHandler) AddCoachingPricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

	var data *models.CoachingPricingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.AddCoachingPricing(ctx, coachingId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, result)
}

func (h *CoachingHandler) UpdateCoachingPricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")
	pricingId := chi.URLParam(r, "pricingId")

	var data *models.CoachingPricingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateCoachingPricing(ctx, coachingId, pricingId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) DeleteCoachingPricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")
	pricingId := chi.URLParam(r, "pricingId")

	err := h.coachingSvcs.DeleteCoachingPricing(ctx, coachingId, pricingId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CoachingHandler) GetCoachingPricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

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

	result, err := h.coachingSvcs.GetCoachingPricing(ctx, coachingId, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetCoachingComments(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") // coaching id

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.commentSvcs.GetCommentsByRef(ctx, enums.CommentTypeCoaching, id, skip, limit, query)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// reviews
func (h *CoachingHandler) GetReviews(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "id")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.reviewsvcs.GetReviewsByRef(ctx, "coaching", coachingId, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// session resources
func (h *CoachingHandler) UpdateStudentSessionNotes(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.UpdateSessionNotesDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateStudentSessionNotes(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) UpdateStudentSessionResources(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.UpdateSessionResourcesDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateStudentSessionResources(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) UpdateStudentSessionAgendas(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.UpdateSessionAgendasDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateStudentSessionAgendas(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) UpdateStudentSessionFeedback(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.UpdateSessionFeedbackDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateStudentSessionFeedback(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// justifications
func (h *CoachingHandler) AddAbsenceJustification(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.AbsenceJustificationDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.AbsenceJustificationRequest(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) RescheduleRequest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.RescheduleJustificationDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.RescheduleRequest(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) JustificationDecision(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.JustificationDecision
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.JustificationDecision(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) GetSessionsByStage(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingProgramId := chi.URLParam(r, "coachingProgramId")
	stage := chi.URLParam(r, "stage")

	result, err := h.coachingSvcs.GetSessionsByStage(ctx, coachingProgramId, stage)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

// GetStudentSessionsForCoaching returns a student's sessions for one coaching with date, from, to, status. Student or parent only.
func (h *CoachingHandler) GetStudentSessionsForCoaching(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	coachingProgramId := chi.URLParam(r, "coachingProgramId")
	studentId := chi.URLParam(r, "studentId")

	result, err := h.coachingSvcs.GetStudentSessionsForCoaching(ctx, coachingProgramId, studentId, cfg.User.Id.Hex())
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// coaching student assignment
func (h *CoachingHandler) StartCoachingAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	assignmentId := chi.URLParam(r, "assignmentId")

	result, err := h.coachingSvcs.StartCoachingAssignment(ctx, assignmentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) SubmitCoachingStudentAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentAssignmentId := chi.URLParam(r, "studentAssignmentId")

	var data *models.StudentCoachingAssignmentAnswers
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.SubmitCoachingStudentAssignment(ctx, studentAssignmentId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) UpdateCoachingStudentAssignmentReviewStatus(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentAssignmentId := chi.URLParam(r, "studentAssignmentId")

	var data struct {
		ReviewStatus string `json:"reviewStatus" validate:"omitempty,oneof=reviewed"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateCoachingStudentAssignmentReviewStatus(ctx, studentAssignmentId, data.ReviewStatus)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// coaching student test
func (h *CoachingHandler) StartCoachingStudentTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	testId := chi.URLParam(r, "testId")

	result, err := h.coachingSvcs.StartCoachingStudentTest(ctx, testId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) SubmitCoachingStudentTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentTestId := chi.URLParam(r, "studentTestId")

	var data *models.StudentCoachingTestAnswers
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.SubmitCoachingStudentTest(ctx, studentTestId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CoachingHandler) UpdateCoachingStudentTestReviewStatus(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentTestId := chi.URLParam(r, "studentTestId")

	var data struct {
		ReviewStatus string `json:"reviewStatus" validate:"omitempty,oneof=reviewed"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateCoachingStudentTestReviewStatus(ctx, studentTestId, data.ReviewStatus)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// TEACHER-FACING HANDLERS

// GetTeacherCoachings retrieves all coachings assigned to the authenticated teacher
func (h *CoachingHandler) GetTeacherCoachings(w http.ResponseWriter, r *http.Request) error {
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

	result, err := h.coachingSvcs.GetTeacherCoachings(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetTeacherCoachingSessions retrieves sessions for a specific coaching, filtered by teacher
func (h *CoachingHandler) GetTeacherCoachingSessions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "coachingId")

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

	result, err := h.coachingSvcs.GetTeacherCoachingSessions(ctx, coachingId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// admin-facing unified absence/reschedule justification requests for a teacher with query.Conditions filters
func (h *CoachingHandler) GetTeacherJustificationRequests(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	teacherId := chi.URLParam(r, "teacherId")

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

	result, err := h.coachingSvcs.GetTeacherJustificationRequests(ctx, teacherId, skip, limit, q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetTeacherCoachingMembers retrieves members of a coaching, filtered to only show students the teacher teaches
func (h *CoachingHandler) GetTeacherCoachingMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	coachingId := chi.URLParam(r, "coachingId")

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

	result, err := h.coachingSvcs.GetTeacherCoachingMembers(ctx, coachingId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetSessionDetailsForSupervisor - Get enriched session details for supervisor dashboard
func (h *CoachingHandler) GetSessionDetailsForSupervisor(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	supervisorId := chi.URLParam(r, "supervisorId")
	sessionId := chi.URLParam(r, "sessionId")

	result, err := h.coachingSvcs.GetSessionDetailsForSupervisor(ctx, sessionId, supervisorId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// UpdateStudentSessionTime updates the time and/or status of a coaching session (for calendar events)
func (h *CoachingHandler) UpdateStudentSessionTime(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	var data *models.UpdateSessionTimeData
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coachingSvcs.UpdateStudentSessionTime(ctx, studentSessionId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// DeleteStudentSession deletes/cancels a coaching session (for calendar events)
func (h *CoachingHandler) DeleteStudentSession(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	err := h.coachingSvcs.DeleteStudentSession(ctx, studentSessionId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// GetStudentSessionById retrieves a student session by its ID
func (h *CoachingHandler) GetStudentSessionById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentSessionId := chi.URLParam(r, "studentSessionId")

	result, err := h.coachingSvcs.GetStudentSessionById(ctx, studentSessionId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetSessionById retrieves a session by its ID
func (h *CoachingHandler) GetSessionById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	sessionId := chi.URLParam(r, "sessionId")

	result, err := h.coachingSvcs.GetSessionById(ctx, sessionId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetSessionDetails retrieves session details with student and teacher info, filtered by query.Conditions
func (h *CoachingHandler) GetSessionDetails(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var q *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &q); err != nil {
		// Allow empty body - treat as no filtering
		if !errors.Is(err, io.EOF) {
			return err
		}
		q = &query.Conditions{}
	}

	result, err := h.coachingSvcs.GetSessionDetails(ctx, skip, limit, q)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetEnrolledCoachings retrieves all coachings a student is enrolled in
func (h *CoachingHandler) GetEnrolledCoachings(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

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

	result, err := h.coachingSvcs.GetEnrolledCoachings(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetAllStudentSessions retrieves all student sessions by student ID regardless of coaching ID with dynamic filtering
func (h *CoachingHandler) GetAllStudentSessions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

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

	result, err := h.coachingSvcs.GetAllStudentSessions(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentCoachingStats retrieves all coaching info with comprehensive stats for a student
func (h *CoachingHandler) GetStudentCoachingStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

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

	result, err := h.coachingSvcs.GetStudentCoachingStats(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentLearningStats returns combined course and coaching stats for a student
func (h *CoachingHandler) GetStudentLearningStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	// Coaching stats pagination and filters
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

	courseStats, err := h.coursesvcs.GetStudentCourseStats(ctx, studentId)
	if err != nil {
		return err
	}

	coachingStats, err := h.coachingSvcs.GetStudentCoachingStats(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	result := &models.StudentLearningStatsResponse{
		CourseStats:   courseStats,
		CoachingStats: coachingStats,
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
