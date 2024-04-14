package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/go-chi/chi/v5"
)

const (
	updateBannerURL = "/banner/{id}"
)

type UpdateBannerUsecase interface {
	UpdateBanner(ctx context.Context, dto entity.UpdateBannerDTO) error
}

type updateBannerHandler struct {
	middlewares []func(http.Handler) http.Handler
	usecase     UpdateBannerUsecase
}

func NewUpdateBannerHandler(usecase UpdateBannerUsecase) *updateBannerHandler {
	return &updateBannerHandler{
		usecase:     usecase,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

func (h *updateBannerHandler) AddToRouter(r *chi.Mux) {
	var handler http.Handler
	handler = h
	for _, md := range h.middlewares {
		handler = md(h)
	}

	r.Patch(updateBannerURL, handler.ServeHTTP)
}

func (h *updateBannerHandler) Middlewares(md ...func(http.Handler) http.Handler) *updateBannerHandler {
	h.middlewares = append(h.middlewares, md...)
	return h
}

func (h *updateBannerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	strID := chi.URLParam(r, "id")

	ID, err := strconv.ParseInt(strID, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	var dto entity.UpdateBannerDTO

	err = json.NewDecoder(r.Body).Decode(&dto)
	if err != nil {
		http.Error(w, "error decoding json request body", http.StatusBadRequest)
		return
	}

	dto.BannerID = ID

	// is_active will updated to false until otherwise stated
	if dto.BannerID == 0 || dto.FeatureID == 0 || len(dto.TagIDs) == 0 ||
		dto.Content.Title == "" || dto.Content.Text == "" || dto.Content.URL == "" {
		slog.Debug("bad request", "error", "banner id must be specified")
		http.Error(w, "banner id must be specified", http.StatusBadRequest)
		return
	}

	err = h.usecase.UpdateBanner(r.Context(), dto)
	if err != nil {
		switch errors.Code(err) {
		case errors.ErrNoDataFound:
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		case errors.ErrAlreadyExists, errors.ErrFeatureNotFound, errors.ErrTagNotFound:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusOK)

}
