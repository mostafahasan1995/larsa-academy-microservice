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

type PlacementTestHandler struct {
	placementTestSvcs  content.PlacementTestSvcs
	validationInstance *validator.Validate
}

func NewPlacementTestHandler(i *do.Injector, r *chi.Mux) {
	h := &PlacementTestHandler{
		placementTestSvcs:  do.MustInvoke[content.PlacementTestSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/placement-tests", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.Get("/pathway/{pathId}", helpers.Make(h.GetByPathwayId))
		r.With(middleware.Auth("authenticate", content.CapabilityManageAllPlacementTests)).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})

	r.Route("/api/placement-tests", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/student/submit", helpers.Make(h.SubmitPlacementTestAnswers))
		r.With(middleware.Auth("authenticate")).Post("/student/schedule", helpers.Make(h.OnlinePlacementTestSchedule))
		r.With(middleware.Auth("authenticate")).Get("/student/{placementTestId}", helpers.Make(h.GetStudentPlacementTests))
		r.With(middleware.Auth("authenticate")).Patch("/teacher/{id}/evaluation", helpers.Make(h.SubmitTeacherEvaluation))
		//calendar event management
		r.With(middleware.Auth("authenticate")).Post("/schedules/list", helpers.Make(h.GetPlacementTestSchedules))
		r.With(middleware.Auth("authenticate")).Patch("/schedules/{scheduleId}/time", helpers.Make(h.UpdatePlacementTestSchedule))
		r.With(middleware.Auth("authenticate")).Delete("/schedules/{scheduleId}", helpers.Make(h.DeletePlacementTestSchedule))
		//admin - list all submissions
		r.With(middleware.Auth("authenticate")).Post("/submissions/list", helpers.Make(h.GetAllStudentPlacementTests))
		r.With(middleware.Auth("authenticate")).Get("/submissions/{id}", helpers.Make(h.GetStudentPlacementTestById))
		r.With(middleware.Auth("authenticate")).Delete("/submissions/{id}", helpers.Make(h.DeleteStudentPlacementTest))
	})

}

func (h *PlacementTestHandler) SubmitTeacherEvaluation(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") // student placement test id

	var data *models.TeacherEvaluation
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.SubmitTeacherEvaluation(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) SubmitPlacementTestAnswers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.StudentPlacementTestAnswers
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.SubmitPlacementTestAnswers(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) OnlinePlacementTestSchedule(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.OnlinePlacementTestScheduleDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.OnlinePlacementTestSchedule(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.placementTestSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) GetByPathwayId(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	pathId := chi.URLParam(r, "pathId")
	result, err := h.placementTestSvcs.GetByPathwayId(ctx, pathId)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.Get(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.PlacementTestDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.PlacementTestDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *PlacementTestHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.placementTestSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// UpdatePlacementTestSchedule updates the time of a placement test schedule (for calendar events)
func (h *PlacementTestHandler) UpdatePlacementTestSchedule(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	scheduleId := chi.URLParam(r, "scheduleId")

	var data *models.UpdatePlacementTestScheduleDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.placementTestSvcs.UpdatePlacementTestSchedule(ctx, scheduleId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// DeletePlacementTestSchedule deletes a placement test schedule (for calendar events)
func (h *PlacementTestHandler) DeletePlacementTestSchedule(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	scheduleId := chi.URLParam(r, "scheduleId")

	err := h.placementTestSvcs.DeletePlacementTestSchedule(ctx, scheduleId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// GetStudentPlacementTests retrieves all of a student's placement test submissions/schedules for a given placement test
func (h *PlacementTestHandler) GetStudentPlacementTests(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	placementTestId := chi.URLParam(r, "placementTestId")

	result, err := h.placementTestSvcs.GetStudentPlacementTests(ctx, placementTestId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetPlacementTestSchedules lists all placement test schedules with pagination and filtering
// Only admins and supervisors can access this endpoint
func (h *PlacementTestHandler) GetPlacementTestSchedules(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	// Query is optional - only decode if body is not empty
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
			return helpers.BadRequest("invalid query format: " + err.Error())
		}
	}

	result, err := h.placementTestSvcs.GetPlacementTestSchedules(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetAllStudentPlacementTests lists all student placement test submissions with pagination and filtering
// Only admins and supervisors can access this endpoint
func (h *PlacementTestHandler) GetAllStudentPlacementTests(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	// Query is optional - only decode if body is not empty
	if r.ContentLength > 0 {
		if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
			return helpers.BadRequest("invalid query format: " + err.Error())
		}
	}

	result, err := h.placementTestSvcs.GetAllStudentPlacementTests(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentPlacementTestById retrieves a single student placement test submission by ID
// Students can view their own submissions, teachers can view assigned submissions,
// and admins/supervisors can view all submissions
func (h *PlacementTestHandler) GetStudentPlacementTestById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.placementTestSvcs.GetStudentPlacementTestById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// DeleteStudentPlacementTest soft deletes a student placement test submission (admin only)
func (h *PlacementTestHandler) DeleteStudentPlacementTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.placementTestSvcs.DeleteStudentPlacementTest(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "Submission deleted successfully"})
}
