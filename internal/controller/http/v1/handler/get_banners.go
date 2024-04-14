package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/go-chi/chi/v5"
)

const (
	getBannersURL = "/banner"
)

type GetBannerUsecase interface {
	GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.Banner, error)
}

type getBannersHandler struct {
	middlewares []func(http.Handler) http.Handler
	usecase     GetBannerUsecase
}

func NewGetBannersHandler(usecase GetBannerUsecase) *getBannersHandler {
	return &getBannersHandler{
		usecase:     usecase,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

func (h *getBannersHandler) AddToRouter(r *chi.Mux) {
	var handler http.Handler
	handler = h
	for _, md := range h.middlewares {
		handler = md(h)
	}

	r.Get(getBannersURL, handler.ServeHTTP)
}

func (h *getBannersHandler) Middlewares(md ...func(http.Handler) http.Handler) *getBannersHandler {
	h.middlewares = append(h.middlewares, md...)
	return h
}

func (h *getBannersHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	strTagID := r.URL.Query().Get("tag_id")
	strFeatureID := r.URL.Query().Get("feature_id")
	strLimit := r.URL.Query().Get("limit")
	strOffset := r.URL.Query().Get("offset")

	slog.Debug("query", "tag_id", strTagID, "feature_id", strFeatureID)

	filters := make(map[string]int64)

	var tagID int64
	var err error
	if strTagID != "" {
		tagID, err = strconv.ParseInt(strTagID, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		filters["tag"] = tagID
	}

	var featureID int64
	if strFeatureID != "" {
		featureID, err = strconv.ParseInt(strFeatureID, 10, 64)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		filters["feature"] = featureID
	}

	if len(filters) < 1 {
		http.Error(w, "there must be at least one filter", http.StatusBadRequest)
		return
	}

	limit, err := strconv.Atoi(strLimit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	offset, err := strconv.Atoi(strOffset)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	banners, err := h.usecase.GetBanners(r.Context(), entity.GetBannersDTO{
		Filters: filters,
		Limit:   limit,
		Offset:  offset,
	})
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	err = json.NewEncoder(w).Encode(banners)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

}
