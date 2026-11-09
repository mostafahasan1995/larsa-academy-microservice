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
	"github.com/goccy/go-json"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StudentContentProgressHandler struct {
	progressSvc content.StudentContentProgressSvcs
	courseSvc   content.CourseSvcs
}

func NewStudentContentProgressHandler(i *do.Injector, r *chi.Mux) {
	h := &StudentContentProgressHandler{
		progressSvc: do.MustInvoke[content.StudentContentProgressSvcs](i),
		courseSvc:   do.MustInvoke[content.CourseSvcs](i),
	}

	r.Route("/api/student-content-progress", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.UpsertProgress))
		r.With(middleware.Auth("authenticate")).Post("/get", helpers.Make(h.GetProgress))
	})

	r.Route("/student-content-progress", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.AdminList))
	})
}

type UpsertContentProgressDto struct {
	ContentId          string `json:"contentId" validate:"required"`
	CourseId           string `json:"courseId"` // Optional but recommended for perfect performance
	LessonId           string `json:"lessonId"` // Optional but recommended for perfect performance
	ContentType        string `json:"contentType" validate:"required,oneof=video"`
	CurrentTimeSeconds int    `json:"currentTimeSeconds" validate:"required,min=0"`
}

type GetContentProgressDto struct {
	ContentId string `json:"contentId" validate:"required"`
	CourseId  string `json:"courseId"` // Optional but recommended for perfect performance
	LessonId  string `json:"lessonId"` // Optional but recommended for perfect performance
}

func (h *StudentContentProgressHandler) UpsertProgress(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	studentId := cfg.User.Id

	var dto UpsertContentProgressDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if dto.ContentType != "video" {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "contentType must be 'video'"})
	}

	contentId, err := primitive.ObjectIDFromHex(dto.ContentId)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid contentId"})
	}

	var courseId, lessonId primitive.ObjectID
	if dto.CourseId != "" {
		courseId, err = primitive.ObjectIDFromHex(dto.CourseId)
		if err != nil {
			return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid courseId"})
		}
	}
	if dto.LessonId != "" {
		lessonId, err = primitive.ObjectIDFromHex(dto.LessonId)
		if err != nil {
			return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid lessonId"})
		}
	}

	// Validate content exists and is video type
	contentType, err := h.progressSvc.ValidateContent(ctx, contentId)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusNotFound, map[string]string{"error": "content not found"})
	}

	if contentType != "video" {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "content is not a video"})
	}

	// If courseId and lessonId provided, validate they match the content
	if !courseId.IsZero() && !lessonId.IsZero() {
		validCourseId, validLessonId, _, err := h.progressSvc.FindContentInCourse(ctx, contentId)
		if err != nil {
			return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "failed to validate content location"})
		}
		// Check if provided courseId/lessonId match the actual location
		if validCourseId != courseId || validLessonId != lessonId {
			return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "courseId and lessonId do not match the contentId location"})
		}
	}

	progress := &models.StudentContentProgress{
		StudentId:          studentId,
		ContentId:          contentId,
		CourseId:           courseId,
		LessonId:           lessonId,
		CurrentTimeSeconds: dto.CurrentTimeSeconds,
	}

	result, err := h.progressSvc.UpsertProgress(ctx, progress)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *StudentContentProgressHandler) GetProgress(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	cfg, err := util.GetReqAppCfg(ctx)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusUnauthorized, map[string]string{"error": "authentication required"})
	}
	studentId := cfg.User.Id

	var dto GetContentProgressDto
	if err := json.NewDecoder(r.Body).Decode(&dto); err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid request body"})
	}

	if dto.ContentId == "" {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "contentId is required"})
	}

	contentId, err := primitive.ObjectIDFromHex(dto.ContentId)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid contentId"})
	}

	var courseId, lessonId primitive.ObjectID
	if dto.CourseId != "" {
		courseId, err = primitive.ObjectIDFromHex(dto.CourseId)
		if err != nil {
			return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid courseId"})
		}
	}
	if dto.LessonId != "" {
		lessonId, err = primitive.ObjectIDFromHex(dto.LessonId)
		if err != nil {
			return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid lessonId"})
		}
	}

	progress, err := h.progressSvc.GetProgress(ctx, studentId, contentId, courseId, lessonId)
	if err != nil {
		return err
	}

	if progress == nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusOK, &models.StudentContentProgress{
			StudentId:          studentId,
			ContentId:          contentId,
			CourseId:           courseId,
			LessonId:           lessonId,
			CurrentTimeSeconds: 0,
		})
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, progress)
}

func (h *StudentContentProgressHandler) AdminList(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.WriteJsonCtx(ctx, w, http.StatusBadRequest, map[string]string{"error": "invalid pagination parameters"})
	}

	var queryConditions query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &queryConditions); err != nil {
		queryConditions = query.Conditions{}
	}

	query := &queryConditions

	result, err := h.progressSvc.List(ctx, skip, limit, query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}
