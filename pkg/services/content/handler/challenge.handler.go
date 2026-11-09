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

type ChallengeHandler struct {
	challengeSvcs      content.ChallengeSvcs
	validationInstance *validator.Validate
}

func NewChallengeHandler(i *do.Injector, r *chi.Mux) {
	h := &ChallengeHandler{
		challengeSvcs:      do.MustInvoke[content.ChallengeSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/challenges", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})
}

// GetById godoc
// @Summary Get challenge by ID
// @Description Get a challenge by its ID
// @Tags Challenge
// @Accept json
// @Produce json
// @Param id path string true "Challenge ID"
// @Success 200 {object} models.Challenge
// @Failure 400 {object} helpers.ErrorResponse
// @Failure 404 {object} helpers.ErrorResponse
// @Router /challenges/{id} [get]
func (h *ChallengeHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	result, err := h.challengeSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Get godoc
// @Summary Get challenges with pagination
// @Description Get a list of challenges with pagination
// @Tags Challenge
// @Accept json
// @Produce json
// @Param skip query int false "Skip"
// @Param limit query int false "Limit"
// @Param query query string false "Query conditions"
// @Success 200 {object} models.ChallengePagination
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/list [post]
func (h *ChallengeHandler) Get(w http.ResponseWriter, r *http.Request) error {
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

	result, err := h.challengeSvcs.Get(ctx, skip, limit, query, requesterStudentId)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Add godoc
// @Summary Create a new challenge
// @Description Create a new challenge
// @Tags Challenge
// @Accept json
// @Produce json
// @Param challenge body models.ChallengeDto true "Challenge Data"
// @Success 200 {object} models.Challenge
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges [post]
func (h *ChallengeHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	var data *models.ChallengeDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	if err := data.Validate(h.validationInstance); err != nil {
		return err
	}

	result, err := h.challengeSvcs.Add(ctx, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Update godoc
// @Summary Update a challenge
// @Description Update a challenge by ID
// @Tags Challenge
// @Accept json
// @Produce json
// @Param id path string true "Challenge ID"
// @Param challenge body models.ChallengeDto true "Challenge Data"
// @Success 200 {object} models.Challenge
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/{id} [put]
func (h *ChallengeHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	var data *models.ChallengeDto
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}

	result, err := h.challengeSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// Delete godoc
// @Summary Delete a challenge
// @Description Delete a challenge by ID
// @Tags Challenge
// @Accept json
// @Produce json
// @Param id path string true "Challenge ID"
// @Success 200 {object} helpers.SuccessResponse
// @Failure 400 {object} helpers.ErrorResponse
// @Router /challenges/{id} [delete]
func (h *ChallengeHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	id := chi.URLParam(r, "id")

	err := h.challengeSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
