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
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type CourseHandler struct {
	coursesvcs         content.CourseSvcs
	commentSvcs        content.CommentSvcs
	reviewsvcs         content.ReviewSvcs
	validationInstance *validator.Validate
}

func NewCourseHandler(i *do.Injector, r *chi.Mux) {
	h := &CourseHandler{
		coursesvcs:         do.MustInvoke[content.CourseSvcs](i),
		commentSvcs:        do.MustInvoke[content.CommentSvcs](i),
		reviewsvcs:         do.MustInvoke[content.ReviewSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/courses", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))

		// Course Categories (Tags)
		r.With(middleware.Auth("authenticate")).Post("/categories/list", helpers.Make(h.GetAllCourseCategories))
		r.With(middleware.Auth("authenticate")).Post("/categories", helpers.Make(h.AddCourseCategory))
		r.With(middleware.Auth("authenticate")).Get("/categories/{categoryId}", helpers.Make(h.GetCourseCategory))
		r.With(middleware.Auth("authenticate")).Put("/categories/{categoryId}", helpers.Make(h.UpdateCourseCategory))
		r.With(middleware.Auth("authenticate")).Delete("/categories/{categoryId}", helpers.Make(h.DeleteCourseCategory))

		//members
		//student courses
		r.With(middleware.Auth("authenticate")).Post("/{id}/members/list", helpers.Make(h.GetAllCourseMembers))

		//course summary
		r.With(middleware.Auth("authenticate")).Post("/{id}/summaries", helpers.Make(h.AddSummary))
		r.With(middleware.Auth("authenticate")).Post("/{id}/summaries/all", helpers.Make(h.GetAllCourseSummaries))
		r.With(middleware.Auth("authenticate")).Get("/{id}/summaries/{summaryId}", helpers.Make(h.GetCourseSummary))
		r.With(middleware.Auth("authenticate")).Put("/{id}/summaries/{summaryId}", helpers.Make(h.UpdateSummary))
		r.With(middleware.Auth("authenticate")).Delete("/{id}/summaries/{summaryId}", helpers.Make(h.DeleteSummary))

		//course assignment
		r.With(middleware.Auth("authenticate")).Post("/{id}/assignments", helpers.Make(h.AddAssignment))
		r.With(middleware.Auth("authenticate")).Post("/{id}/assignments/all", helpers.Make(h.GetAllCourseAssignments))
		r.With(middleware.Auth("authenticate")).Get("/{id}/assignments/{assignmentId}", helpers.Make(h.GetCourseAssignment))
		r.With(middleware.Auth("authenticate")).Put("/{id}/assignments/{assignmentId}", helpers.Make(h.UpdateAssignment))
		r.With(middleware.Auth("authenticate")).Delete("/{id}/assignments/{assignmentId}", helpers.Make(h.DeleteAssignment))
		r.With(middleware.Auth("authenticate")).Post("/{courseId}/assignments/{assignmentId}/submissions", helpers.Make(h.GetCourseAssignmentStudentSubmissions))
		r.With(middleware.Auth("authenticate")).Delete("/{courseId}/assignments/{assignmentId}/submissions/{submissionId}", helpers.Make(h.DeleteAssignmentSubmission))
		r.With(middleware.Auth("authenticate")).Patch("/assignments/{studentAssignmentId}/grade", helpers.Make(h.GradeStudentCourseAssignment))

		//course test
		r.With(middleware.Auth("authenticate")).Post("/{id}/tests", helpers.Make(h.AddTest))
		r.With(middleware.Auth("authenticate")).Post("/{id}/tests/all", helpers.Make(h.GetAllCourseTests))
		r.With(middleware.Auth("authenticate")).Get("/{id}/tests/{testId}", helpers.Make(h.GetCourseTest))
		r.With(middleware.Auth("authenticate")).Put("/{id}/tests/{testId}", helpers.Make(h.UpdateTest))
		r.With(middleware.Auth("authenticate")).Delete("/{id}/tests/{testId}", helpers.Make(h.DeleteTest))
		r.With(middleware.Auth("authenticate")).Post("/{courseId}/tests/{testId}/submissions", helpers.Make(h.GetCourseTestStudentSubmissions))
		r.With(middleware.Auth("authenticate")).Delete("/{courseId}/tests/{testId}/submissions/{submissionId}", helpers.Make(h.DeleteTestSubmission))

		//course faq
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Post("/{id}/faqs", helpers.Make(h.AddFaq))
		r.With(middleware.Auth("authenticate")).Post("/{id}/faqs/all", helpers.Make(h.GetAllCourseFaqs))
		r.With(middleware.Auth("authenticate")).Get("/{id}/faqs/{faqId}", helpers.Make(h.GetCourseFaq))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Put("/{id}/faqs/{faqId}", helpers.Make(h.UpdateFaq))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Delete("/{id}/faqs/{faqId}", helpers.Make(h.DeleteFaq))

		//course study note
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Post("/{id}/study-notes", helpers.Make(h.AddCourseStudyNote))
		r.With(middleware.Auth("authenticate")).Post("/{id}/study-notes/all", helpers.Make(h.GetAllCourseStudyNotes))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Put("/{id}/study-notes/{studyNoteId}", helpers.Make(h.UpdateCourseStudyNote))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Delete("/{id}/study-notes/{studyNoteId}", helpers.Make(h.DeleteCourseStudyNote))

		//course study note category
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Post("/{id}/study-note-categories", helpers.Make(h.AddCourseStudyNoteCategory))
		r.With(middleware.Auth("authenticate")).Post("/{id}/study-note-categories/all", helpers.Make(h.GetAllCourseStudyNoteCategories))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Put("/{id}/study-note-categories/{categoryId}", helpers.Make(h.UpdateCourseStudyNoteCategory))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Delete("/{id}/study-note-categories/{categoryId}", helpers.Make(h.DeleteCourseStudyNoteCategory))

		//course flashcard
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Post("/{id}/flashcards", helpers.Make(h.AddCourseFlashcard))
		r.With(middleware.Auth("authenticate")).Post("/{id}/flashcards/all", helpers.Make(h.GetAllCourseFlashcards))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Put("/{id}/flashcards/{flashcardId}", helpers.Make(h.UpdateCourseFlashcard))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Delete("/{id}/flashcards/{flashcardId}", helpers.Make(h.DeleteCourseFlashcard))

		//course flashcard category
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Post("/{id}/flashcard-categories", helpers.Make(h.AddCourseFlashcardCategory))
		r.With(middleware.Auth("authenticate")).Post("/{id}/flashcard-categories/all", helpers.Make(h.GetAllCourseFlashcardCategories))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Put("/{id}/flashcard-categories/{categoryId}", helpers.Make(h.UpdateCourseFlashcardCategory))
		r.With(middleware.Auth("authenticate", content.CapabilityEditCourseResources)).Delete("/{id}/flashcard-categories/{categoryId}", helpers.Make(h.DeleteCourseFlashcardCategory))

		//pricing
		r.With(middleware.Auth("authenticate")).Post("/{id}/pricing", helpers.Make(h.AddCoursePricing))
		r.With(middleware.Auth("authenticate")).Put("/{id}/pricing/{pricingId}", helpers.Make(h.UpdateCoursePricing))
		r.With(middleware.Auth("authenticate")).Delete("/{id}/pricing/{pricingId}", helpers.Make(h.DeleteCoursePricing))
		r.With(middleware.Auth("authenticate")).Get("/{id}/pricing", helpers.Make(h.GetCoursePricing))

		//calendar event management - live session
		r.With(middleware.Auth("authenticate")).Patch("/{id}/live-session/time", helpers.Make(h.UpdateCourseLiveSessionTime))
		r.With(middleware.Auth("authenticate")).Delete("/{id}/live-session", helpers.Make(h.DeleteCourseLiveSession))

		//comments
		r.With(middleware.Auth("authenticate")).Post("/{id}/comments", helpers.Make(h.GetCourseComments))

		//reviews
		r.With(middleware.Auth("authenticate")).Get("/{id}/reviews", helpers.Make(h.GetReviews))

		//course summary stats
		r.With(middleware.Auth("authenticate")).Get("/{id}/summary", helpers.Make(h.GetCourseSummaryStats))
	})

	r.Route("/api/teachers/courses", func(r chi.Router) {
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/list", helpers.Make(h.GetTeacherCourses))
		r.With(middleware.Auth("authenticate", content.CapabilityAccessTeacherEndpoints)).Post("/{id}/members/list", helpers.Make(h.GetTeacherCourseMembers))
	})

	r.Route("/api/courses", func(r chi.Router) {
		// r.With(middleware.Auth("authenticate")).Post("/cart", helpers.Make(h.AddToCart))
		// r.With(middleware.Auth("authenticate")).Put("/cart/{cartId}", helpers.Make(h.UpdateCartItem))
		// r.With(middleware.Auth("authenticate")).Delete("/cart/{cartId}", helpers.Make(h.DeleteCartItem))
		// r.With(middleware.Auth("authenticate")).Post("/cart/checkout", helpers.Make(h.Checkout))
		// r.With(middleware.Auth("authenticate")).Get("/cart/me", helpers.Make(h.MyCart))

		// Student facing list endpoint
		r.With(middleware.Auth("authenticate")).Post("/student/list", helpers.Make(h.GetStudentList))
		r.With(middleware.Auth("authenticate")).Post("/student/{studentId}/list", helpers.Make(h.GetParentStudentList))
		r.With(middleware.Auth("authenticate")).Get("/student/{id}", helpers.Make(h.GetEnrolledCourseById))
		//student assignments
		r.With(middleware.Auth("authenticate")).Post("/assignments/{assignmentId}/submit", helpers.Make(h.SubmitStudentCourseAssignment))
		r.With(middleware.Auth("authenticate")).Post("/assignments/all", helpers.Make(h.GetAllStudentCourseAssignment))
		r.With(middleware.Auth("authenticate")).Get("/{courseId}/assignments/all", helpers.Make(h.GetCourseAssignmentsForStudent))

		// student tests
		r.With(middleware.Auth("authenticate")).Post("/tests/{testId}/start", helpers.Make(h.StartCourseStudentTest))
		r.With(middleware.Auth("authenticate")).Post("/tests/submissions/{studentTestId}/submit", helpers.Make(h.SubmitCourseStudentTest))
		r.With(middleware.Auth("authenticate")).Get("/tests/submissions/{studentTestId}", helpers.Make(h.GetCourseStudentTest))
		r.With(middleware.Auth("authenticate")).Post("/tests/{testId}/submissions", helpers.Make(h.GetAllCourseStudentTests))
		r.With(middleware.Auth("authenticate")).Get("/tests/course/{courseId}", helpers.Make(h.GetCourseTestsForStudent))

		// course assignment submissions
		r.With(middleware.Auth("authenticate")).Post("/assignments/{assignmentId}/submissions", helpers.Make(h.GetAllCourseAssignmentSubmissions))

		// lesson completion
		r.With(middleware.Auth("authenticate")).Post("/complete-lesson", helpers.Make(h.MarkLessonAsCompleted))
	})

	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/courses/enrolled", helpers.Make(h.GetEnrolledCourses))
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/courses/stats", helpers.Make(h.GetStudentCourseStats))
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/bundles/enrolled", helpers.Make(h.GetEnrolledBundles))

}

type CompleteLessonDto struct {
	CourseId string `json:"courseId" validate:"required"`
	LessonId string `json:"lessonId" validate:"required"`
}

func (h *CourseHandler) MarkLessonAsCompleted(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data CompleteLessonDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.MarkLessonAsCompleted(ctx, data.CourseId, data.LessonId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentCourseStats returns per-course lesson stats for a student
func (h *CourseHandler) GetStudentCourseStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	result, err := h.coursesvcs.GetStudentCourseStats(ctx, studentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetTeacherCourses(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.coursesvcs.GetTeacherCourses(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetCourseStudentTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentTestId := chi.URLParam(r, "studentTestId")

	result, err := h.coursesvcs.GetCourseStudentTest(ctx, studentTestId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseStudentTests(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	testId := chi.URLParam(r, "testId")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.coursesvcs.GetAllCourseStudentTests(ctx, testId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetTeacherCourseMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetTeacherCourseMembers(ctx, courseId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.coursesvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetEnrolledCourseById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.coursesvcs.GetEnrolledCourseById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichSubjects := false
	if val := r.URL.Query().Get("enrichSubjects"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichSubjects = parsed
		}
	}

	includeStudentCount := false
	if val := r.URL.Query().Get("includeStudentCount"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			includeStudentCount = parsed
		}
	}

	enrichTeachers := false
	if val := r.URL.Query().Get("enrichTeachers"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichTeachers = parsed
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

	result, err := h.coursesvcs.Get(ctx, skip, limit, query, enrichSubjects, includeStudentCount, enrichTeachers, enrichPrice)
	if err != nil {
		return err
	}

	// TEMP: Log course IDs to find corruption
	for i, course := range result.Courses {
		slog.Info("DEBUG_COURSE_ID", "index", i, "courseId", course.Id.Hex())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// @Summary Get student course list
// @Description Retrieves a paginated list of courses for students, enriched with favorite status
// @Tags Courses
// @Accept json
// @Produce json
// @Param skip query int false "Skip items for pagination"
// @Param limit query int false "Limit items for pagination"
// @Param enrichSubjects query bool false "Whether to enrich linked subjects"
// @Param includeStudentCount query bool false "Whether to include enrolled student count"
// @Param query body query.Conditions true "Query conditions for filtering and searching"
// @Success 200 {object} models.StudentCoursePagination
// @Router /api/courses/student/list [post]
func (h *CourseHandler) GetStudentList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	enrichSubjects := false
	if val := r.URL.Query().Get("enrichSubjects"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichSubjects = parsed
		}
	}

	includeStudentCount := false
	if val := r.URL.Query().Get("includeStudentCount"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			includeStudentCount = parsed
		}
	}

	enrichTeachers := false
	if val := r.URL.Query().Get("enrichTeachers"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichTeachers = parsed
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

	result, err := h.coursesvcs.GetStudentList(ctx, "", skip, limit, query, enrichSubjects, includeStudentCount, enrichTeachers, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetParentStudentList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	enrichSubjects := false
	if val := r.URL.Query().Get("enrichSubjects"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichSubjects = parsed
		}
	}

	includeStudentCount := false
	if val := r.URL.Query().Get("includeStudentCount"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			includeStudentCount = parsed
		}
	}

	enrichTeachers := false
	if val := r.URL.Query().Get("enrichTeachers"); val != "" {
		if parsed, err := strconv.ParseBool(val); err == nil {
			enrichTeachers = parsed
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

	result, err := h.coursesvcs.GetStudentList(ctx, studentId, skip, limit, query, enrichSubjects, includeStudentCount, enrichTeachers, enrichPrice)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAll(ctx, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.CourseDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.coursesvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.CourseDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.coursesvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	err := h.coursesvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course summary
func (h *CourseHandler) GetCourseSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")         //course id
	summaryId := chi.URLParam(r, "summaryId") //summary id

	result, err := h.coursesvcs.GetCourseSummary(ctx, courseId, summaryId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseSummaries(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseSummaries(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CourseHandler) AddSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseSummaryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseSummary(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CourseHandler) UpdateSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")         //course id
	summaryId := chi.URLParam(r, "summaryId") //summary id

	var data *models.CourseSummaryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseSummary(ctx, courseId, summaryId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CourseHandler) DeleteSummary(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")         //course id
	summaryId := chi.URLParam(r, "summaryId") //summary id

	err := h.coursesvcs.DeleteCourseSummary(ctx, courseId, summaryId)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course assignment
func (h *CourseHandler) GetCourseAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")               //course id
	assignmentId := chi.URLParam(r, "assignmentId") //assignment id

	result, err := h.coursesvcs.GetCourseAssignment(ctx, courseId, assignmentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseAssignments(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseAssignments(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) AddAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseAssignment(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")               //course id
	assignmentId := chi.URLParam(r, "assignmentId") //assignment id

	var data *models.CourseAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseAssignment(ctx, courseId, assignmentId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")               //course id
	assignmentId := chi.URLParam(r, "assignmentId") //assignment id

	err := h.coursesvcs.DeleteCourseAssignment(ctx, courseId, assignmentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CourseHandler) DeleteAssignmentSubmission(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "courseId")
	assignmentId := chi.URLParam(r, "assignmentId")
	submissionId := chi.URLParam(r, "submissionId")

	err := h.coursesvcs.DeleteCourseStudentAssignment(ctx, submissionId, courseId)
	if err != nil {
		return err
	}

	_ = assignmentId

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course test
func (h *CourseHandler) GetCourseTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")   //course id
	testId := chi.URLParam(r, "testId") //test id

	result, err := h.coursesvcs.GetCourseTest(ctx, courseId, testId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseTests(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseTests(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) AddTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseTestDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseTest(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")   //course id
	testId := chi.URLParam(r, "testId") //test id

	var data *models.CourseTestDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseTest(ctx, courseId, testId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")   //course id
	testId := chi.URLParam(r, "testId") //test id

	err := h.coursesvcs.DeleteCourseTest(ctx, courseId, testId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CourseHandler) DeleteTestSubmission(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "courseId")
	testId := chi.URLParam(r, "testId")
	submissionId := chi.URLParam(r, "submissionId")

	err := h.coursesvcs.DeleteCourseStudentTest(ctx, submissionId, courseId)
	if err != nil {
		return err
	}

	_ = testId

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course faq
func (h *CourseHandler) GetCourseFaq(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id") //course id
	faqId := chi.URLParam(r, "faqId") //faq id

	result, err := h.coursesvcs.GetCourseFaq(ctx, courseId, faqId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseFaqs(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseFaqs(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) AddFaq(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseFaqDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseFaq(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateFaq(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id") //course id
	faqId := chi.URLParam(r, "faqId") //faq id

	var data *models.CourseFaqDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseFaq(ctx, courseId, faqId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteFaq(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id") //course id
	faqId := chi.URLParam(r, "faqId") //faq id

	err := h.coursesvcs.DeleteCourseFaq(ctx, courseId, faqId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course study notes
func (h *CourseHandler) AddCourseStudyNote(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseStudyNoteDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseStudyNote(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)

}

func (h *CourseHandler) GetAllCourseStudyNotes(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseStudyNotes(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateCourseStudyNote(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")             //course id
	studyNoteId := chi.URLParam(r, "studyNoteId") //study note id

	var data *models.CourseStudyNoteDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseStudyNote(ctx, courseId, studyNoteId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteCourseStudyNote(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")             //course id
	studyNoteId := chi.URLParam(r, "studyNoteId") //study note id

	err := h.coursesvcs.DeleteCourseStudyNote(ctx, courseId, studyNoteId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course study note category
func (h *CourseHandler) AddCourseStudyNoteCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseStudyNoteCategoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseStudyNoteCategory(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseStudyNoteCategories(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseStudyNoteCategories(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateCourseStudyNoteCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")           //course id
	categoryId := chi.URLParam(r, "categoryId") //category id

	var data *models.CourseStudyNoteCategoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseStudyNoteCategory(ctx, courseId, categoryId, data)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteCourseStudyNoteCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")           //course id
	categoryId := chi.URLParam(r, "categoryId") //category id
	err := h.coursesvcs.DeleteCourseStudyNoteCategory(ctx, courseId, categoryId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course flashcard
func (h *CourseHandler) AddCourseFlashcard(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseFlashcardDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseFlashcard(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseFlashcards(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseFlashcards(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateCourseFlashcard(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")             //course id
	flashcardId := chi.URLParam(r, "flashcardId") //flashcard id

	var data *models.CourseFlashcardDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseFlashcard(ctx, courseId, flashcardId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteCourseFlashcard(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")             //course id
	flashcardId := chi.URLParam(r, "flashcardId") //flashcard id

	err := h.coursesvcs.DeleteCourseFlashcard(ctx, courseId, flashcardId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// course flashcard category
func (h *CourseHandler) AddCourseFlashcardCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CourseFlashcardCategoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCourseFlashcardCategory(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseFlashcardCategories(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseFlashcardCategories(ctx, id, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateCourseFlashcardCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")           //course id
	categoryId := chi.URLParam(r, "categoryId") //category id

	var data *models.CourseFlashcardCategoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseFlashcardCategory(ctx, courseId, categoryId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteCourseFlashcardCategory(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")           //course id
	categoryId := chi.URLParam(r, "categoryId") //category id

	err := h.coursesvcs.DeleteCourseFlashcardCategory(ctx, courseId, categoryId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

//comments

func (h *CourseHandler) GetCourseComments(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") // course id

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.commentSvcs.GetCommentsByRef(ctx, enums.CommentTypeCourse, id, skip, limit, query)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// reviews
func (h *CourseHandler) GetReviews(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.reviewsvcs.GetReviewsByRef(ctx, "course", courseId, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// cart handlers

// func (h *CourseHandler) MyCart(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	result, err := h.coursesvcs.MyCart(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
// }

// func (h *CourseHandler) AddToCart(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	var data *models.CourseSubscribe
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
// 		return err
// 	}

// 	result, err := h.coursesvcs.AddToCart(ctx, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
// }

// func (h *CourseHandler) UpdateCartItem(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	cartId := chi.URLParam(r, "cartId")

// 	var data *models.CourseSubscribe
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
// 		return err
// 	}

// 	result, err := h.coursesvcs.UpdateCartItem(ctx, cartId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
// }

// func (h *CourseHandler) DeleteCartItem(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	cartId := chi.URLParam(r, "cartId")

// 	err := h.coursesvcs.DeleteCartItem(ctx, cartId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }

// func (h *CourseHandler) Checkout(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	result, err := h.coursesvcs.Checkout(ctx)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
// }

// student assignments
func (h *CourseHandler) SubmitStudentCourseAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	assignmentId := chi.URLParam(r, "assignmentId")

	var data *models.CourseStudentAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coursesvcs.SubmitStudentCourseAssignment(ctx, assignmentId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllStudentCourseAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllStudentCourseAssignment(ctx, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GradeStudentCourseAssignment(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentAssignmentId := chi.URLParam(r, "studentAssignmentId")

	var data *models.GradeCourseStudentAssignmentDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coursesvcs.GradeCourseStudentAssignment(ctx, studentAssignmentId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetCourseAssignmentStudentSubmissions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "courseId")
	assignmentId := chi.URLParam(r, "assignmentId")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.coursesvcs.GetCourseAssignmentStudentSubmissions(ctx, courseId, assignmentId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetAllCourseAssignmentSubmissions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	assignmentId := chi.URLParam(r, "assignmentId")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.coursesvcs.GetAllCourseAssignmentSubmissions(ctx, assignmentId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// student courses
// GetAllCourseMembers godoc
// @Summary Get all members of a course
// @Description Retrieves a paginated list of all members (students) enrolled in a specific course.
// @Tags Courses
// @Accept json
// @Produce json
// @Param id path string true "Course ID"
// @Param skip query int false "Skip items for pagination"
// @Param limit query int false "Limit items for pagination"
// @Param query body query.Conditions true "Query conditions for filtering and searching"
// @Success 200 {object} models.MemberCoursePagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /courses/{id}/members/list [post]
func (h *CourseHandler) GetAllCourseMembers(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id") //course id

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	result, err := h.coursesvcs.GetAllCourseMembers(ctx, courseId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetEnrolledCourses retrieves all courses a student is enrolled in
func (h *CourseHandler) GetEnrolledCourses(w http.ResponseWriter, r *http.Request) error {
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

	result, err := h.coursesvcs.GetEnrolledCourses(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetEnrolledBundles retrieves all bundles a student is enrolled in
func (h *CourseHandler) GetEnrolledBundles(w http.ResponseWriter, r *http.Request) error {
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

	result, err := h.coursesvcs.GetEnrolledBundles(ctx, studentId, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// pricing handlers
func (h *CourseHandler) AddCoursePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id") //course id

	var data *models.CoursePricingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.AddCoursePricing(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) UpdateCoursePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")         //course id
	pricingId := chi.URLParam(r, "pricingId") //pricing id

	var data *models.CoursePricingDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCoursePricing(ctx, courseId, pricingId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) DeleteCoursePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")         //course id
	pricingId := chi.URLParam(r, "pricingId") //pricing id

	err := h.coursesvcs.DeleteCoursePricing(ctx, courseId, pricingId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

func (h *CourseHandler) GetCoursePricing(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id") //course id

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	result, err := h.coursesvcs.GetCoursePricing(ctx, courseId, skip, limit)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// UpdateCourseLiveSessionTime updates the schedule time of a course live session (for calendar events)
func (h *CourseHandler) UpdateCourseLiveSessionTime(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")

	var data struct {
		Schedule time.Time `json:"schedule" validate:"required"`
	}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, &data); err != nil {
		return err
	}

	result, err := h.coursesvcs.UpdateCourseLiveSessionTime(ctx, courseId, data.Schedule)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// DeleteCourseLiveSession deactivates/cancels a course live session (for calendar events)
func (h *CourseHandler) DeleteCourseLiveSession(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")

	err := h.coursesvcs.DeleteCourseLiveSession(ctx, courseId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// GetCourseSummaryStats retrieves course summary statistics
func (h *CourseHandler) GetCourseSummaryStats(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "id")

	result, err := h.coursesvcs.GetCourseSummaryStats(ctx, courseId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) StartCourseStudentTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	testId := chi.URLParam(r, "testId")

	result, err := h.coursesvcs.StartCourseStudentTest(ctx, testId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) SubmitCourseStudentTest(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentTestId := chi.URLParam(r, "studentTestId")

	var data *models.StudentCourseTestAnswersDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := helpers.GenericValidation(h.validationInstance, data); err != nil {
		return err
	}

	result, err := h.coursesvcs.SubmitCourseStudentTest(ctx, studentTestId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetCourseTestsForStudent(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "courseId")

	result, err := h.coursesvcs.GetCourseTestsForStudent(ctx, courseId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetCourseTestStudentSubmissions(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "courseId")
	testId := chi.URLParam(r, "testId")

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		query = nil
	}

	result, err := h.coursesvcs.GetCourseTestStudentSubmissions(ctx, courseId, testId, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *CourseHandler) GetCourseAssignmentsForStudent(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	courseId := chi.URLParam(r, "courseId")

	result, err := h.coursesvcs.GetCourseAssignmentsForStudent(ctx, courseId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
