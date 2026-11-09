package handler

import (
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/contact/models"
	content_models "larsa-academy-microservice/pkg/services/content/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type SupervisorHandler struct {
	supSvc             contact.SupervisorSvc
	validationInstance *validator.Validate
}

func NewSupervisorHandler(i *do.Injector, r *chi.Mux) {
	h := &SupervisorHandler{
		supSvc:             do.MustInvoke[contact.SupervisorSvc](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/supervisors", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetOne))
		r.Get("/{id}/summary", helpers.Make(h.GetSummary))
		r.Get("/{id}/managed-users", helpers.Make(h.GetManagedUsers))
		r.Post("/{id}/students/list", helpers.Make(h.GetAssignedStudents))
		r.Post("/{id}/teachers/list", helpers.Make(h.GetOverseenTeachers))
		r.Post("/{id}/courses/list", helpers.Make(h.GetCoveredCourses))
		r.Post("/{id}/coaching/list", helpers.Make(h.GetCoveredCoaching))
		r.With(middleware.Auth("authenticate")).Post("/{id}/classes-grades/paginated", helpers.Make(h.GetManagedClassesGrades))

		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/paginated", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))

		r.With(middleware.Auth("authenticate")).Get("/me", helpers.Make(h.GetMe))
	})
	r.Route("/api/supervisors/{id}", func(r chi.Router) {
		r.Get("/dashboard", helpers.Make(h.GetDashboard))
		r.Get("/with-stats", helpers.Make(h.GetOneWithStats))
		r.Post("/teachers-by-supervisor", helpers.Make(h.GetTeachersBySupervisorId))
		r.Post("/students-by-supervisor", helpers.Make(h.GetStudentsBySupervisorId))
		r.Post("/teachers-extended", helpers.Make(h.GetTeachersExtended))
		r.Post("/students-extended", helpers.Make(h.GetStudentsExtended))
		r.With(middleware.Auth("authenticate")).Post("/courses/student-counts", helpers.Make(h.GetStudentCountsByCourse))
		r.With(middleware.Auth("authenticate")).Post("/coachings/student-counts", helpers.Make(h.GetStudentCountsByCoaching))
	})
	r.Route("/api/admin", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/student-counts/subjects", helpers.Make(h.GetStudentCountsBySubject))
	})
	r.Route("/api/supervisors/me", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/courses/list", helpers.Make(h.GetMeCoursesList))
		r.With(middleware.Auth("authenticate")).Post("/coaching/list", helpers.Make(h.GetMeCoachingList))
		r.With(middleware.Auth("authenticate")).Post("/programs/list", helpers.Make(h.GetMeProgramsList))
	})
}
func (h SupervisorHandler) GetOne(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.supSvc.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h SupervisorHandler) GetSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.supSvc.GetSummary(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
func (h SupervisorHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	data, err := h.supSvc.GetPaginated(ctx, skip, limit, &q)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, data)
}

func (h SupervisorHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}
	data, err := h.supSvc.GetAll(ctx, &q)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, data)
}

func (h SupervisorHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data models.SupervisorDTO
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	sup, err := h.supSvc.Add(ctx, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, sup)
}

func (h SupervisorHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data models.SupervisorDTO
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return err
	}

	teacher, err := h.supSvc.Update(ctx, id, &data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, teacher)
}
func (h SupervisorHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //package id

	if err := h.supSvc.Delete(ctx, id); err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h SupervisorHandler) GetAssignedStudents(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetAssignedStudents(ctx, id, skip, limit, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h SupervisorHandler) GetOverseenTeachers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetOverseenTeachers(ctx, id, skip, limit, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h SupervisorHandler) GetCoveredCourses(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	enrichTeachersStr := r.URL.Query().Get("enrichTeachers")
	enrichTeachers := enrichTeachersStr == "true"

	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetCoveredCourses(ctx, id, skip, limit, &q, enrichTeachers)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h SupervisorHandler) GetCoveredCoaching(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	enrichTeachersStr := r.URL.Query().Get("enrichTeachers")
	enrichTeachers := enrichTeachersStr == "true"

	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetCoveredCoaching(ctx, id, skip, limit, &q, enrichTeachers)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h SupervisorHandler) GetManagedClassesGrades(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		return err
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetManagedClassesGrades(ctx, id, skip, limit, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetOneWithStats gets a supervisor with totalNumStudents and totalNumTeachers
func (h SupervisorHandler) GetOneWithStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.supSvc.GetOneWithStats(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetTeachersBySupervisorId gets all teachers assigned to a supervisor
func (h SupervisorHandler) GetTeachersBySupervisorId(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&q)
	}

	result, err := h.supSvc.GetTeachersBySupervisorId(ctx, id, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentsBySupervisorId gets all students enrolled in courses/coaching taught by supervisor's teachers
func (h SupervisorHandler) GetStudentsBySupervisorId(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&q)
	}

	result, err := h.supSvc.GetStudentsBySupervisorId(ctx, id, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetManagedUsers returns the IDs of teachers managed by a supervisor, and optionally student IDs
func (h SupervisorHandler) GetManagedUsers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	// Read query parameter
	withStudentStr := r.URL.Query().Get("withStudent")
	withStudent := withStudentStr == "true"

	result, err := h.supSvc.GetManagedUsers(ctx, id, withStudent)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetDashboard gets supervisor dashboard with stats and filtered sessions
func (h SupervisorHandler) GetDashboard(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	// Get query parameters for filtering
	statusFilter := r.URL.Query().Get("status")    // optional: "pending", "scheduled", "completed", etc.
	todayOnlyStr := r.URL.Query().Get("todayOnly") // optional: "true" or "false"

	todayOnly := false
	if todayOnlyStr == "true" {
		todayOnly = true
	}

	// Use filtered version if any filters provided, otherwise use default
	var result *content_models.SupervisorDashboard
	var err error

	if statusFilter != "" || todayOnly {
		result, err = h.supSvc.GetDashboardWithFilter(ctx, id, statusFilter, todayOnly)
	} else {
		result, err = h.supSvc.GetDashboard(ctx, id)
	}

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetTeachersExtended gets teachers separated into direct and indirect
func (h SupervisorHandler) GetTeachersExtended(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&q)
	}

	result, err := h.supSvc.GetTeachersExtended(ctx, id, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentsExtended gets students from courses and coachings
func (h SupervisorHandler) GetStudentsExtended(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var q query.Conditions
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&q)
	}

	result, err := h.supSvc.GetStudentsExtended(ctx, id, &q)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentCountsByCourse returns number of students grouped by course (for supervisor id, or all when id is "all")
func (h SupervisorHandler) GetStudentCountsByCourse(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")
	supervisorId := id
	if id == "all" {
		supervisorId = ""
	}

	result, err := h.supSvc.GetStudentCountsByCourse(ctx, supervisorId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentCountsByCoaching returns number of students grouped by coaching (for supervisor id, or all when id is "all")
func (h SupervisorHandler) GetStudentCountsByCoaching(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")
	supervisorId := id
	if id == "all" {
		supervisorId = ""
	}

	result, err := h.supSvc.GetStudentCountsByCoaching(ctx, supervisorId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentCountsBySubject returns number of students grouped by subject across courses and coachings (admin only)
func (h SupervisorHandler) GetStudentCountsBySubject(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	result, err := h.supSvc.GetStudentCountsBySubject(ctx)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h SupervisorHandler) GetMe(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	id := cfg.User.Id.Hex()
	result, err := h.supSvc.GetOne(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetMeCoursesList returns courses assigned to the supervisor from the JWT token
func (h SupervisorHandler) GetMeCoursesList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	enrichTeachersStr := r.URL.Query().Get("enrichTeachers")
	enrichTeachers := enrichTeachersStr == "true"

	var q query.Conditions
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&q)
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetCoveredCourses(ctx, cfg.User.Id.Hex(), skip, limit, &q, enrichTeachers)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetMeCoachingList returns coachings assigned to the supervisor from the JWT token
func (h SupervisorHandler) GetMeCoachingList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	enrichTeachersStr := r.URL.Query().Get("enrichTeachers")
	enrichTeachers := enrichTeachersStr == "true"

	var q query.Conditions
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&q)
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.supSvc.GetCoveredCoaching(ctx, cfg.User.Id.Hex(), skip, limit, &q, enrichTeachers)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetMeProgramsList returns both courses and coachings assigned to the supervisor (JWT) in one response.
func (h SupervisorHandler) GetMeProgramsList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return err
	}

	enrichTeachersStr := r.URL.Query().Get("enrichTeachers")
	enrichTeachers := enrichTeachersStr == "true"

	var q query.Conditions
	if r.Body != nil {
		_ = json.NewDecoder(r.Body).Decode(&q)
	}

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	courses, err := h.supSvc.GetCoveredCourses(ctx, cfg.User.Id.Hex(), skip, limit, &q, enrichTeachers)
	if err != nil {
		return err
	}
	coachings, err := h.supSvc.GetCoveredCoaching(ctx, cfg.User.Id.Hex(), skip, limit, &q, enrichTeachers)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, struct {
		Courses   *content_models.CoursePagination   `json:"courses"`
		Coachings *content_models.CoachingPagination `json:"coachings"`
	}{Courses: courses, Coachings: coachings})
}
