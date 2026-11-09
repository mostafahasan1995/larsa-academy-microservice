package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	contactconstant "larsa-academy-microservice/pkg/services/contact/constant"
	"larsa-academy-microservice/pkg/services/contact/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type StudentNoteHandler struct {
	noteService        contact.StudentNoteSvcs
	validationInstance *validator.Validate
}

func NewStudentNoteHandler(i *do.Injector, r *chi.Mux) {
	h := &StudentNoteHandler{
		noteService:        do.MustInvoke[contact.StudentNoteSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/api/students/{studentId}/notes", func(r chi.Router) {
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Create))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.GetByStudentId))
		r.With(middleware.Auth("authenticate")).Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Patch("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Patch("/{id}/position", helpers.Make(h.UpdatePosition))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

func (h *StudentNoteHandler) Create(w http.ResponseWriter, r *http.Request) error {
	ctx, cfg := util.AddCtxAppCfg(r)
	studentIdStr := chi.URLParam(r, "studentId")

	studentId, err := primitive.ObjectIDFromHex(studentIdStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid student id")
	}

	var data models.StudentNoteCreateDto
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return helpers.BadRequest(err.Error())
	}

	note, err := h.noteService.Create(ctx, studentId, &data, cfg.User.Id)
	if err != nil {
		return helpers.InternalServerError(err.Error())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusCreated, note)
}

func (h *StudentNoteHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	idStr := chi.URLParam(r, "id")

	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid note id")
	}

	note, err := h.noteService.GetById(ctx, id)
	if err != nil {
		if errors.Is(err, contactconstant.ERR_NOTE_NOT_FOUND) || errors.Is(err, mongo.ErrNoDocuments) {
			return helpers.NotFoundError("note not found")
		}
		return helpers.InternalServerError(err.Error())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, note)
}

func (h *StudentNoteHandler) GetByStudentId(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	studentIdStr := chi.URLParam(r, "studentId")

	studentId, err := primitive.ObjectIDFromHex(studentIdStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid student id")
	}

	// Parse pagination parameters
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return helpers.InvalidPagination()
	}

	// Parse query conditions from request body
	var conditions query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		// Allow empty body - treat as no filtering
		if !errors.Is(err, io.EOF) {
			return helpers.InvalidJSON()
		}
	}

	pagination, err := h.noteService.GetByStudentId(ctx, studentId, skip, limit, &conditions)
	if err != nil {
		return helpers.InternalServerError(err.Error())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, pagination)
}

func (h *StudentNoteHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, cfg := util.AddCtxAppCfg(r)
	idStr := chi.URLParam(r, "id")

	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid note id")
	}

	var data models.StudentNoteUpdateDto
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return helpers.BadRequest(err.Error())
	}

	note, err := h.noteService.Update(ctx, id, &data, cfg.User.Id)
	if err != nil {
		if errors.Is(err, contactconstant.ERR_NOTE_NOT_FOUND) || errors.Is(err, mongo.ErrNoDocuments) {
			return helpers.NotFoundError("note not found")
		}
		return helpers.InternalServerError(err.Error())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, note)
}

func (h *StudentNoteHandler) UpdatePosition(w http.ResponseWriter, r *http.Request) error {
	ctx, cfg := util.AddCtxAppCfg(r)
	studentIdStr := chi.URLParam(r, "studentId")
	noteIdStr := chi.URLParam(r, "id")

	studentId, err := primitive.ObjectIDFromHex(studentIdStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid student id")
	}

	noteId, err := primitive.ObjectIDFromHex(noteIdStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid note id")
	}

	var data models.StudentNotePositionDto
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		return helpers.InvalidJSON()
	}

	if err := h.validationInstance.Struct(data); err != nil {
		return helpers.BadRequest(err.Error())
	}

	err = h.noteService.UpdatePosition(ctx, studentId, noteId, data.Position, cfg.User.Id)
	if err != nil {
		if errors.Is(err, contactconstant.ERR_NOTE_NOT_FOUND) {
			return helpers.NotFoundError("note not found")
		}
		return helpers.InternalServerError(err.Error())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "position updated"})
}

func (h *StudentNoteHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, cfg := util.AddCtxAppCfg(r)
	idStr := chi.URLParam(r, "id")

	id, err := primitive.ObjectIDFromHex(idStr)
	if err != nil {
		return helpers.InvalidObjectIdCustomMessage("invalid note id")
	}

	err = h.noteService.Delete(ctx, id, cfg.User.Id)
	if err != nil {
		if errors.Is(err, contactconstant.ERR_NOTE_NOT_FOUND) || errors.Is(err, mongo.ErrNoDocuments) {
			return helpers.NotFoundError("note not found")
		}
		return helpers.InternalServerError(err.Error())
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "note deleted"})
}
