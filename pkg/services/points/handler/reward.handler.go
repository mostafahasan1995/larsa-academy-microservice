package handler

import (
	"errors"
	"io"
	"larsa-academy-microservice/pkg/helpers"
	"larsa-academy-microservice/pkg/middleware"
	"larsa-academy-microservice/pkg/query"
	"larsa-academy-microservice/pkg/services/contact"
	"larsa-academy-microservice/pkg/services/points"
	"larsa-academy-microservice/pkg/services/points/models"
	"larsa-academy-microservice/pkg/util"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-playground/validator/v10"
	"github.com/goccy/go-json"
	"github.com/samber/do"
)

type RewardHandler struct {
	rewardSvcs         points.RewardSvcs
	studentSvcs        contact.StudentSvcs
	validationInstance *validator.Validate
}

func NewRewardHandler(i *do.Injector, r *chi.Mux) {
	h := &RewardHandler{
		rewardSvcs:         do.MustInvoke[points.RewardSvcs](i),
		studentSvcs:        do.MustInvoke[contact.StudentSvcs](i),
		validationInstance: do.MustInvoke[*validator.Validate](i),
	}

	r.Route("/rewards", func(r chi.Router) {
		r.Get("/{id}", helpers.Make(h.GetById))
		r.With(middleware.Auth("authenticate")).Post("/list", helpers.Make(h.Get))
		r.With(middleware.Auth("authenticate")).Post("/all", helpers.Make(h.GetAll))
		r.With(middleware.Auth("authenticate")).Post("/", helpers.Make(h.Add))
		r.With(middleware.Auth("authenticate")).Put("/{id}", helpers.Make(h.Update))
		r.With(middleware.Auth("authenticate")).Delete("/{id}", helpers.Make(h.Delete))
	})

	// Student-facing rewards listing - automatically filters by student's grade/class
	r.With(middleware.Auth("authenticate")).Post("/api/students/{studentId}/rewards/list", helpers.Make(h.GetStudentRewards))
}

func (h *RewardHandler) GetById(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	result, err := h.rewardSvcs.GetById(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RewardHandler) Get(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		return err
	}
	result, err := h.rewardSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

// GetStudentRewards returns rewards filtered by the student's grade/class
func (h *RewardHandler) GetStudentRewards(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)

	studentId := chi.URLParam(r, "studentId")

	skip, limit, err := util.Paginate(r)
	if err != nil {
		return err
	}

	// Decode optional query conditions (e.g., type, status filters)
	var conditions *query.Conditions
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &conditions); err != nil {
		if !errors.Is(err, io.EOF) {
			return err
		}
		conditions = nil
	}

	// Load student to derive grade/class
	student, err := h.studentSvcs.GetOne(ctx, studentId)
	if err != nil {
		return err
	}

	studentClass := student.AcademicInfo.Class
	if studentClass != "" {
		if conditions == nil {
			conditions = &query.Conditions{}
		}
		conditions.Columns = append(conditions.Columns, query.Column{
			Name:  "class",
			Exp:   query.In,
			Value: []string{studentClass},
		})
	}

	result, err := h.rewardSvcs.Get(ctx, skip, limit, conditions)
	if err != nil {
		return err
	}

	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RewardHandler) GetAll(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var filter map[string]interface{}
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &filter); err != nil {
		return err
	}
	result, err := h.rewardSvcs.GetAll(ctx, filter)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RewardHandler) Add(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	var data *models.Reward
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	if err := h.validationInstance.Struct(data); err != nil {
		return err
	}
	result, err := h.rewardSvcs.Add(ctx, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RewardHandler) Update(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	var data *models.Reward
	if err := json.NewDecoder(r.Body).DecodeContext(ctx, &data); err != nil {
		return err
	}
	result, err := h.rewardSvcs.Update(ctx, id, data)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, result)
}

func (h *RewardHandler) Delete(w http.ResponseWriter, r *http.Request) error {
	ctx, _ := util.AddCtxAppCfg(r)
	id := chi.URLParam(r, "id")
	err := h.rewardSvcs.Delete(ctx, id)
	if err != nil {
		return err
	}
	return helpers.WriteJsonCtx(ctx, w, http.StatusOK, "ok")
}
