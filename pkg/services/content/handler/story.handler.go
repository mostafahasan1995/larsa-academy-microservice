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
	"go.mongodb.org/mongo-driver/bson/primitive"
)

type StoryHandler struct {
	storySvcs          content.StorySvcs
	validationInstance *validator.Validate
}

func NewStoryHandler(i *do.Injector, r *chi.Mux) {
	h := &StoryHandler{
		storySvcs:          do.MustInvoke[content.StorySvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/stories", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

// GetById godoc
// @Summary Get story by ID
// @Description Get story details by ID
// @Tags Story
// @Accept json
// @Produce json
// @Param id path string true "Story ID"
// @Success 200 {object} models.Story
// @Failure 400 {object} helpers.ErrorResponse
// @Failure 404 {object} helpers.ErrorResponse
// @Router /stories/{id} [get]
func (h *StoryHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.storySvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Get godoc
// @Summary Get stories with pagination
// @Description Get a list of stories with pagination and filtering
// @Tags Story
// @Accept json
// @Produce json
// @Param skip query int false "Skip records"
// @Param limit query int false "Limit records per page"
// @Param query query string false "Query conditions in JSON format"
// @Success 200 {object} models.StoryPagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /stories/list [post]
func (h *StoryHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, cfg := util.AddCtxAppCfg(r)

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	var query *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &query); err != nil {
		return err
	}

	var requesterStudentId *primitive.ObjectID
	if cfg != nil && cfg.User != nil && !cfg.User.Id.IsZero() {
		requesterStudentId = &cfg.User.Id
	}

	result, err := h.storySvcs.Get(ctx, skip, limit, query, requesterStudentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Add godoc
// @Summary Create a new story
// @Description Create a new story with the provided data
// @Tags Story
// @Accept json
// @Produce json
// @Param story body models.StoryDto true "Story data"
// @Success 200 {object} models.Story
// @Failure 400 {object} helpers.ErrorResponse
// @Router /stories [post]
func (h *StoryHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.StoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.storySvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Update godoc
// @Summary Update a story
// @Description Update a story with the provided data
// @Tags Story
// @Accept json
// @Produce json
// @Param id path string true "Story ID"
// @Param story body models.StoryDto true "Story data"
// @Success 200 {object} models.Story
// @Failure 400 {object} helpers.ErrorResponse
// @Failure 404 {object} helpers.ErrorResponse
// @Router /stories/{id} [put]
func (h *StoryHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.StoryDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.storySvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Delete godoc
// @Summary Delete a story
// @Description Delete a story by ID
// @Tags Story
// @Accept json
// @Produce json
// @Param id path string true "Story ID"
// @Success 200 {object} helpers.SuccessResponse
// @Failure 400 {object} helpers.ErrorResponse
// @Failure 404 {object} helpers.ErrorResponse
// @Router /stories/{id} [delete]
func (h *StoryHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.storySvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, map[string]string{"message": "Story deleted successfully"})
}
