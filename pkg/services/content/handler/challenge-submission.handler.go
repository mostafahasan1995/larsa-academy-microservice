package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type ChallengeSubmissionHandler struct {
	challengeSubmissionSvcs content.ChallengeSubmissionSvcs
	validationInstance      *validator.Validate
}

func NewChallengeSubmissionHandler(i *do.Injector, r *chi.Mux) {
	h := &ChallengeSubmissionHandler{
		challengeSubmissionSvcs: do.MustInvoke[content.ChallengeSubmissionSvcs](i),
		validationInstance:      do.MustInvoke[*validator.Validate](i),
	}

	// Add routes directly to avoid conflict with existing /challenges route
	r.With(middleware.Auth("authenticate")).Post("/challenges/submit", helpers.Make(h.Submit))
	r.With(middleware.Auth("authenticate")).Get("/challenges/submissions/student/{studentId}", helpers.Make(h.GetStudentSubmissions))
	r.With(middleware.Auth("authenticate")).Get("/challenges/submissions/{id}", helpers.Make(h.GetSubmissionById))
	r.With(middleware.Auth("authenticate")).Get("/challenges/{challengeId}/submissions", helpers.Make(h.GetChallengeSubmissionsWithStats))
}

// Submit godoc
// @Summary Submit challenge answers
// @Description Student submits answers for a challenge
// @Tags Challenge Submission
// @Accept json
// @Produce json
// @Param submission body models.SubmitChallengeDto true "Submission Data"
// @Success 200 {object} models.ChallengeSubmission
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/submit [post]
func (h *ChallengeSubmissionHandler) Submit(w http.ResponseWriter, r *http.Request) error {
	ctx, cfg := util.AddCtxAppCfg(r)

	var data *models.SubmitChallengeDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	// Use student ID from context (authenticated user)
	studentId := cfg.User.Id.Hex()

	result, err := h.challengeSubmissionSvcs.SubmitChallenge(ctx, studentId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentSubmissions godoc
// @Summary Get student submissions
// @Description Get all submissions for a specific student
// @Tags Challenge Submission
// @Accept json
// @Produce json
// @Param studentId path string true "Student ID"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Success 200 {object} models.ChallengeSubmissionPagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/submissions/student/{studentId} [get]
func (h *ChallengeSubmissionHandler) GetStudentSubmissions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.challengeSubmissionSvcs.GetStudentSubmissions(ctx, studentId, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetSubmissionById godoc
// @Summary Get submission by ID
// @Description Get detailed information about a specific submission
// @Tags Challenge Submission
// @Accept json
// @Produce json
// @Param id path string true "Submission ID"
// @Success 200 {object} models.ChallengeSubmission
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/submissions/{id} [get]
func (h *ChallengeSubmissionHandler) GetSubmissionById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.challengeSubmissionSvcs.GetSubmissionById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetChallengeSubmissionsWithStats godoc
// @Summary Get challenge submissions with statistics
// @Description Get all submissions for a challenge with aggregate statistics
// @Tags Challenge Submission
// @Accept json
// @Produce json
// @Param challengeId path string true "Challenge ID"
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Success 200 {object} models.ChallengeSubmissionStats
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/{challengeId}/submissions [get]
func (h *ChallengeSubmissionHandler) GetChallengeSubmissionsWithStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	challengeId := chi.URLParam(r, "challengeId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.challengeSubmissionSvcs.GetChallengeSubmissionsWithStats(ctx, challengeId, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
