package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/content"
	"larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type StudentQuestionHandler struct {
	studentQuestionSvcs content.StudentQuestionSvcs
	validationInstance  *validator.Validate
}

func NewStudentQuestionHandler(i *do.Injector, r *chi.Mux) {
	h := &StudentQuestionHandler{
		studentQuestionSvcs: do.MustInvoke[content.StudentQuestionSvcs](i),
		validationInstance:  do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/student-questions", func(r chi.Router) {
		// Get questions with pagination and role-based filtering
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))

		// Get questions by ref (course)
		r.With(middleware.Auth("authenticate")).Post("/ref/{refType}/{refId}", helpers.Make(h.GetByRef))

		// Get teacher statistics
		r.With(middleware.Auth("authenticate")).Get("/teacher/stats", helpers.Make(h.GetStats))

		// Get supervisor statistics
		r.With(middleware.Auth("authenticate")).Get("/supervisor/stats", helpers.Make(h.GetSupervisorStats))

		// Get single question
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetOne))

		// Student can create a question
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))

		// Student can update their own question
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))

		// Student can delete their own question
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))

		// Teacher/Supervisor/Admin can reply to a question
		r.With(middleware.Auth("authenticate")).Post("/{questionId}/replies", helpers.Make(h.AddReply))

		// Update a reply
		r.With(middleware.Auth("authenticate")).Patch("/replies/{replyId}", helpers.Make(h.UpdateReply))

		// Delete a reply
		r.With(middleware.Auth("authenticate")).Delete("/replies/{replyId}", helpers.Make(h.DeleteReply))
	})
}

// Get retrieves all questions from all refs with pagination and role-based filtering
func (h *StudentQuestionHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.BadRequest("invalid pagination")
	}

	var queryConditions query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &queryConditions); err != nil {
		queryConditions = query.Conditions{}
	}
	query := &queryConditions

	result, err := h.studentQuestionSvcs.GetStudentQuestions(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetByRef retrieves all questions for a specific ref (course or coaching)
func (h *StudentQuestionHandler) GetByRef(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	refType := chi.URLParam(r, "refType")
	refId := chi.URLParam(r, "refId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.BadRequest("invalid pagination")
	}

	var queryConditions query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &queryConditions); err != nil {
		queryConditions = query.Conditions{}
	}
	query := &queryConditions

	result, err := h.studentQuestionSvcs.GetStudentQuestionsByRef(ctx, refType, refId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStats retrieves statistics for the current teacher's questions
func (h *StudentQuestionHandler) GetStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Get optional query parameters
	refType := r.URL.Query().Get("refType")
	refId := r.URL.Query().Get("refId")

	var refTypePtr *string
	var refIdPtr *string

	if refType != "" {
		refTypePtr = &refType
	}
	if refId != "" {
		refIdPtr = &refId
	}

	result, err := h.studentQuestionSvcs.GetTeacherQuestionStats(ctx, refTypePtr, refIdPtr)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetSupervisorStats retrieves statistics for supervisor's teachers
func (h *StudentQuestionHandler) GetSupervisorStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Get optional query parameters
	refType := r.URL.Query().Get("refType")
	refId := r.URL.Query().Get("refId")

	var refTypePtr *string
	var refIdPtr *string

	if refType != "" {
		refTypePtr = &refType
	}
	if refId != "" {
		refIdPtr = &refId
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.BadRequest("invalid pagination")
	}

	result, err := h.studentQuestionSvcs.GetSupervisorQuestionStats(ctx, refTypePtr, refIdPtr, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetOne retrieves a single question by ID with replies
func (h *StudentQuestionHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.studentQuestionSvcs.GetStudentQuestion(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Add creates a new student question
func (h *StudentQuestionHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	// Get refType from query parameter, default to "course" for backward compatibility
	refType := r.URL.Query().Get("refType")
	if refType == "" {
		refType = "course"
	}

	var data models.StudentQuestionDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.studentQuestionSvcs.AddStudentQuestion(ctx, refType, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Update updates an existing student question
func (h *StudentQuestionHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.StudentQuestionDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.studentQuestionSvcs.UpdateStudentQuestion(ctx, id, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Delete soft deletes a student question
func (h *StudentQuestionHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	if err := h.studentQuestionSvcs.DeleteStudentQuestion(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// AddReply adds a reply to a student question
func (h *StudentQuestionHandler) AddReply(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	questionId := chi.URLParam(r, "questionId")

	var data models.StudentQuestionReplyDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.studentQuestionSvcs.AddReply(ctx, questionId, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// UpdateReply updates an existing reply
func (h *StudentQuestionHandler) UpdateReply(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	replyId := chi.URLParam(r, "replyId")

	var data models.StudentQuestionReplyDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.studentQuestionSvcs.UpdateReply(ctx, replyId, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// DeleteReply hard deletes a reply
func (h *StudentQuestionHandler) DeleteReply(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	replyId := chi.URLParam(r, "replyId")

	if err := h.studentQuestionSvcs.DeleteReply(ctx, replyId); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
