package v1

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/go-chi/chi/v5"
)

const (
	createBannerURL = "/banner"
)

type CreateBannerUsecase interface {
	CreateBanner(ctx context.Context, dto entity.CreateBannerDTO) (int64, error)
}

type createBannerHandler struct {
	middlewares []func(http.Handler) http.Handler
	usecase     CreateBannerUsecase
}

func NewCreateBannerHandler(usecase CreateBannerUsecase) *createBannerHandler {
	return &createBannerHandler{
		usecase:     usecase,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

func (h *createBannerHandler) AddToRouter(r *chi.Mux) {
	var handler http.Handler
	handler = h
	for _, md := range h.middlewares {
		handler = md(h)
	}

	r.Post(createBannerURL, handler.ServeHTTP)
}

func (h *createBannerHandler) Middlewares(md ...func(http.Handler) http.Handler) *createBannerHandler {

	h.middlewares = append(h.middlewares, md...)
	return h
}

func (h *createBannerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	var dto entity.CreateBannerDTO

	err := json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, "error decoding json request body", http.StatusBadRequest)
		return
	}

	// TODO: check if dto is valid

	id, err := h.usecase.CreateBanner(r.Context(), dto)
	if err != nil {
		switch errors.Code(err) {
		case errors.ErrAlreadyExists, errors.ErrFeatureNotFound, errors.ErrTagNotFound:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	b, err := json.Marshal(struct {
		BannerID int64 `json:"banner_id"`
	}{BannerID: id})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(b)

	w.WriteHeader(http.StatusCreated)

}
