package v1

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	v1 "github.com/The-Gleb/banner_service/internal/controller/http/v1/middleware"
	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/go-chi/chi/v5"
)

const (
	getUserBannerURL = "/user_banner"
)

type GetUserBannerUsecase interface {
	GetUserBanner(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error)
}

type getUserBannerHandler struct {
	middlewares []func(http.Handler) http.Handler
	usecase     GetUserBannerUsecase
}

func NewGetUserBannerHandler(usecase GetUserBannerUsecase) *getUserBannerHandler {
	return &getUserBannerHandler{
		usecase:     usecase,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

func (h *getUserBannerHandler) AddToRouter(r *chi.Mux) {
	var handler http.Handler
	handler = h
	for _, md := range h.middlewares {
		handler = md(h)
	}

	r.Get(getUserBannerURL, handler.ServeHTTP)
}

func (h *getUserBannerHandler) Middlewares(md ...func(http.Handler) http.Handler) *getUserBannerHandler {
	h.middlewares = append(h.middlewares, md...)
	return h
}

func (h *getUserBannerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	strTagID := r.URL.Query().Get("tag_id")
	strFeatureID := r.URL.Query().Get("feature_id")
	strUseLastRevision := r.URL.Query().Get("use_last_revision")
	slog.Debug("query", "tag_id", strTagID, "feature_id", strFeatureID, "use_last_revision", strUseLastRevision)

	tagID, err := strconv.ParseInt(strTagID, 10, 64)
	if err != nil || tagID < 1 {
		http.Error(w, "invalid tag ID", http.StatusBadRequest)
		return
	}
	featureID, err := strconv.ParseInt(strFeatureID, 10, 64)
	if err != nil || featureID < 1 {
		http.Error(w, "invalid feature ID", http.StatusBadRequest)
		return
	}
	useLastRevision, err := strconv.ParseBool(strUseLastRevision)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	isAdmin, ok := r.Context().Value(v1.Key("isAdmin")).(bool)
	if !ok {
		slog.Error("unable to conver isAdmin context value to bool", "value", r.Context().Value("isAdmin"))
		http.Error(w, "", http.StatusInternalServerError)
		return
	}

	content, err := h.usecase.GetUserBanner(r.Context(), entity.GetUserBannerDTO{
		TagID:           tagID,
		FeatureID:       featureID,
		UseLastRevision: useLastRevision,
		IsAdmin:         isAdmin,
	})
	if err != nil {
		switch errors.Code(err) {
		case errors.ErrForbidden:
			http.Error(w, err.Error(), http.StatusForbidden)
			return
		case errors.ErrNoDataFound:
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		default:
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
	}

	body, err := json.Marshal(content)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Write(body)

}
