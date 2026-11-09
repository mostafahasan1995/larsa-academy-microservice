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

type SubjectHandler struct {
	subjectsvcs        content.SubjectSvcs
	validationInstance *validator.Validate
}

func NewSubjectHandler(i *do.Injector, r *chi.Mux) {
	h := &SubjectHandler{
		subjectsvcs:        do.MustInvoke[content.SubjectSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/subjects", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))

		//subject study note
		// r.Get("/{id}/study-notes/{studyNoteId}", helpers.Make(h.GetSubjectStudyNote))
		// r.Post("/{id}/study-notes/all", helpers.Make(h.GetAllSubjectStudyNotes))
		// r.With(middleware.Auth("authenticate")).Post("/{id}/study-notes", helpers.Make(h.AddSubjectStudyNote))
		// r.With(middleware.Auth("authenticate")).Put("/{id}/study-notes/{studyNoteId}", helpers.Make(h.UpdateSubjectStudyNote))
		// r.With(middleware.Auth("authenticate")).Delete("/{id}/study-notes/{studyNoteId}", helpers.Make(h.DeleteSubjectStudyNote))

		// //subject study note category
		// r.Post("/{id}/study-note-categories/all", helpers.Make(h.GetAllSubjectStudyNoteCategories))
		// r.With(middleware.Auth("authenticate")).Post("/{id}/study-note-categories", helpers.Make(h.AddSubjectStudyNoteCategory))
		// r.With(middleware.Auth("authenticate")).Put("/{id}/study-note-categories/{categoryId}", helpers.Make(h.UpdateSubjectStudyNoteCategory))
		// r.With(middleware.Auth("authenticate")).Delete("/{id}/study-note-categories/{categoryId}", helpers.Make(h.DeleteSubjectStudyNoteCategory))

		// //subject flashcard
		// r.Get("/{id}/flashcards/{flashcardId}", helpers.Make(h.GetSubjectFlashcard))
		// r.Post("/{id}/flashcards/list", helpers.Make(h.GetAllSubjectFlashcards))
		// r.With(middleware.Auth("authenticate")).Post("/{id}/flashcards", helpers.Make(h.AddSubjectFlashcard))
		// r.With(middleware.Auth("authenticate")).Put("/{id}/flashcards/{flashcardId}", helpers.Make(h.UpdateSubjectFlashcard))
		// r.With(middleware.Auth("authenticate")).Delete("/{id}/flashcards/{flashcardId}", helpers.Make(h.DeleteSubjectFlashcard))

		// //subject flashcard category
		// r.Post("/{id}/flashcard-categories/list", helpers.Make(h.GetAllSubjectFlashcardCategories))
		// r.With(middleware.Auth("authenticate")).Post("/{id}/flashcard-categories", helpers.Make(h.AddSubjectFlashcardCategory))
		// r.With(middleware.Auth("authenticate")).Put("/{id}/flashcard-categories/{categoryId}", helpers.Make(h.UpdateSubjectFlashcardCategory))
		// r.With(middleware.Auth("authenticate")).Delete("/{id}/flashcard-categories/{categoryId}", helpers.Make(h.DeleteSubjectFlashcardCategory))

		// //subject assignments
		// r.Get("/{id}/assignments/{assignmentId}", helpers.Make(h.GetSubjectAssignment))
		// r.Post("/{id}/assignments/all", helpers.Make(h.GetAllSubjectAssignments))
		// r.With(middleware.Auth("authenticate")).Post("/{id}/assignments", helpers.Make(h.AddSubjectAssignment))
		// r.With(middleware.Auth("authenticate")).Put("/{id}/assignments/{assignmentId}", helpers.Make(h.UpdateSubjectAssignment))
		// r.With(middleware.Auth("authenticate")).Delete("/{id}/assignments/{assignmentId}", helpers.Make(h.DeleteSubjectAssignment))

		// //subject faqs
		// r.Get("/{id}/faqs/{faqId}", helpers.Make(h.GetSubjectFAQ))
		// r.Post("/{id}/faqs/all", helpers.Make(h.GetAllSubjectFAQs))
		// r.With(middleware.Auth("authenticate")).Post("/{id}/faqs", helpers.Make(h.AddSubjectFAQ))
		// r.With(middleware.Auth("authenticate")).Put("/{id}/faqs/{faqId}", helpers.Make(h.UpdateSubjectFAQ))
		// r.With(middleware.Auth("authenticate")).Delete("/{id}/faqs/{faqId}", helpers.Make(h.DeleteSubjectFAQ))
	})
}

func (h *SubjectHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	subjectId := chi.URLParam(r, "id")

	subject, err := h.subjectsvcs.GetById(ctx, subjectId)

	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, subject)
}

func (h *SubjectHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		return err
	}

	result, err := h.subjectsvcs.Get(ctx, skip, limit, &query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *SubjectHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var query query.Conditions
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		return err
	}

	result, err := h.subjectsvcs.GetAll(ctx, &query)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *SubjectHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.SubjectDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	subject, err := h.subjectsvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, subject)
}

func (h *SubjectHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	subjectId := chi.URLParam(r, "id")

	var data *models.SubjectDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	subject, err := h.subjectsvcs.Update(ctx, subjectId, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, subject)
}

func (h *SubjectHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	subjectId := chi.URLParam(r, "id")

	err := h.subjectsvcs.Delete(ctx, subjectId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}

// // subject study note
// func (h *SubjectHandler) GetAllSubjectStudyNotes(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var query query.Conditions
// 	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
// 		return err
// 	}

// 	studyNotes, err := h.subjectsvcs.GetAllSubjectStudyNotes(ctx, subjectId, &query)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, studyNotes)
// }

// func (h *SubjectHandler) GetSubjectStudyNote(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	studyNoteId := chi.URLParam(r, "studyNoteId")

// 	studyNote, err := h.subjectsvcs.GetSubjectStudyNote(ctx, subjectId, studyNoteId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, studyNote)
// }

// func (h *SubjectHandler) AddSubjectStudyNote(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var data *models.SubjectStudyNoteDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	result, err := h.subjectsvcs.AddSubjectStudyNote(ctx, subjectId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
// }

// func (h *SubjectHandler) UpdateSubjectStudyNote(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	studyNoteId := chi.URLParam(r, "studyNoteId")

// 	var data *models.SubjectStudyNoteDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	result, err := h.subjectsvcs.UpdateSubjectStudyNote(ctx, subjectId, studyNoteId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
// }

// func (h *SubjectHandler) DeleteSubjectStudyNote(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	studyNoteId := chi.URLParam(r, "studyNoteId")

// 	err := h.subjectsvcs.DeleteSubjectStudyNote(ctx, subjectId, studyNoteId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }

// // subject flashcard handlers
// func (h *SubjectHandler) GetAllSubjectFlashcards(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var query query.Conditions
// 	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
// 		return err
// 	}

// 	flashcards, err := h.subjectsvcs.GetAllSubjectFlashcards(ctx, subjectId, &query)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, flashcards)
// }

// func (h *SubjectHandler) GetSubjectFlashcard(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	flashcardId := chi.URLParam(r, "flashcardId")

// 	flashcard, err := h.subjectsvcs.GetSubjectFlashcard(ctx, subjectId, flashcardId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, flashcard)
// }

// func (h *SubjectHandler) AddSubjectFlashcard(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var data *models.SubjectFlashcardDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	flashcard, err := h.subjectsvcs.AddSubjectFlashcard(ctx, subjectId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, flashcard)
// }

// func (h *SubjectHandler) UpdateSubjectFlashcard(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	flashcardId := chi.URLParam(r, "flashcardId")

// 	var data *models.SubjectFlashcardDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	flashcard, err := h.subjectsvcs.UpdateSubjectFlashcard(ctx, subjectId, flashcardId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, flashcard)
// }

// func (h *SubjectHandler) DeleteSubjectFlashcard(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	flashcardId := chi.URLParam(r, "flashcardId")

// 	err := h.subjectsvcs.DeleteSubjectFlashcard(ctx, subjectId, flashcardId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }

// // subject flashcard category handlers
// func (h *SubjectHandler) GetAllSubjectFlashcardCategories(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var query query.Conditions
// 	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
// 		return err
// 	}

// 	categories, err := h.subjectsvcs.GetAllSubjectFlashcardCategories(ctx, subjectId, &query)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, categories)
// }

// func (h *SubjectHandler) AddSubjectFlashcardCategory(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var data *models.SubjectFlashcardCategoryDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	category, err := h.subjectsvcs.AddSubjectFlashcardCategory(ctx, subjectId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, category)
// }

// func (h *SubjectHandler) UpdateSubjectFlashcardCategory(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	categoryId := chi.URLParam(r, "categoryId")

// 	var data *models.SubjectFlashcardCategoryDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	category, err := h.subjectsvcs.UpdateSubjectFlashcardCategory(ctx, subjectId, categoryId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, category)
// }

// func (h *SubjectHandler) DeleteSubjectFlashcardCategory(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	categoryId := chi.URLParam(r, "categoryId")

// 	err := h.subjectsvcs.DeleteSubjectFlashcardCategory(ctx, subjectId, categoryId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }

// // subject study note category handlers
// func (h *SubjectHandler) GetAllSubjectStudyNoteCategories(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var query query.Conditions
// 	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
// 		return err
// 	}

// 	categories, err := h.subjectsvcs.GetAllSubjectStudyNoteCategories(ctx, subjectId, &query)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, categories)
// }

// func (h *SubjectHandler) AddSubjectStudyNoteCategory(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var data *models.SubjectStudyNoteCategoryDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	category, err := h.subjectsvcs.AddSubjectStudyNoteCategory(ctx, subjectId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, category)
// }

// func (h *SubjectHandler) UpdateSubjectStudyNoteCategory(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	categoryId := chi.URLParam(r, "categoryId")

// 	var data *models.SubjectStudyNoteCategoryDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	category, err := h.subjectsvcs.UpdateSubjectStudyNoteCategory(ctx, subjectId, categoryId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, category)
// }

// func (h *SubjectHandler) DeleteSubjectStudyNoteCategory(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	categoryId := chi.URLParam(r, "categoryId")

// 	err := h.subjectsvcs.DeleteSubjectStudyNoteCategory(ctx, subjectId, categoryId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }

// // Subject Assignment Handlers
// func (h *SubjectHandler) GetAllSubjectAssignments(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var query query.Conditions
// 	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
// 		return err
// 	}

// 	assignments, err := h.subjectsvcs.GetAllSubjectAssignments(ctx, subjectId, &query)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, assignments)
// }

// func (h *SubjectHandler) GetSubjectAssignment(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	assignmentId := chi.URLParam(r, "assignmentId")

// 	assignment, err := h.subjectsvcs.GetSubjectAssignment(ctx, subjectId, assignmentId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, assignment)
// }

// func (h *SubjectHandler) AddSubjectAssignment(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var data *models.SubjectAssignmentDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	assignment, err := h.subjectsvcs.AddSubjectAssignment(ctx, subjectId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, assignment)
// }

// func (h *SubjectHandler) UpdateSubjectAssignment(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	assignmentId := chi.URLParam(r, "assignmentId")

// 	var data *models.SubjectAssignmentDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	assignment, err := h.subjectsvcs.UpdateSubjectAssignment(ctx, subjectId, assignmentId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, assignment)
// }

// func (h *SubjectHandler) DeleteSubjectAssignment(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	assignmentId := chi.URLParam(r, "assignmentId")

// 	err := h.subjectsvcs.DeleteSubjectAssignment(ctx, subjectId, assignmentId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }

// // Subject FAQ Handlers
// func (h *SubjectHandler) GetAllSubjectFAQs(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var query query.Conditions
// 	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
// 		return err
// 	}

// 	faqs, err := h.subjectsvcs.GetAllSubjectFAQs(ctx, subjectId, &query)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, faqs)
// }

// func (h *SubjectHandler) GetSubjectFAQ(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	faqId := chi.URLParam(r, "faqId")

// 	faq, err := h.subjectsvcs.GetSubjectFAQ(ctx, subjectId, faqId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, faq)
// }

// func (h *SubjectHandler) AddSubjectFAQ(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")

// 	var data *models.SubjectFAQDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	faq, err := h.subjectsvcs.AddSubjectFAQ(ctx, subjectId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, faq)
// }

// func (h *SubjectHandler) UpdateSubjectFAQ(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	faqId := chi.URLParam(r, "faqId")

// 	var data *models.SubjectFAQDto
// 	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
// 		return err
// 	}

// 	faq, err := h.subjectsvcs.UpdateSubjectFAQ(ctx, subjectId, faqId, data)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, faq)
// }

// func (h *SubjectHandler) DeleteSubjectFAQ(w http.ResponseWriter, r *http.Request) error {
// 	ctx, _ := util.AddCtxAppCfg(r)

// 	subjectId := chi.URLParam(r, "id")
// 	faqId := chi.URLParam(r, "faqId")

// 	err := h.subjectsvcs.DeleteSubjectFAQ(ctx, subjectId, faqId)
// 	if err != nil {
// 		return err
// 	}

// 	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
// }
