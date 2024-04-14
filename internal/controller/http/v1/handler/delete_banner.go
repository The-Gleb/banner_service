package v1

import (
	"context"
	"net/http"
	"strconv"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/go-chi/chi/v5"
)

const (
	deleteBannerURL = "/banner/{id}"
)

type DeleteBannerUsecase interface {
	DeleteBanner(ctx context.Context, dto entity.DeleteBannerDTO) error
}

type deleteBannerHandler struct {
	middlewares []func(http.Handler) http.Handler
	usecase     DeleteBannerUsecase
}

func NewDeleteBannerHandler(usecase DeleteBannerUsecase) *deleteBannerHandler {
	return &deleteBannerHandler{
		usecase:     usecase,
		middlewares: make([]func(http.Handler) http.Handler, 0),
	}
}

func (h *deleteBannerHandler) AddToRouter(r *chi.Mux) {
	var handler http.Handler
	handler = h
	for _, md := range h.middlewares {
		handler = md(h)
	}

	r.Delete(deleteBannerURL, handler.ServeHTTP)
}

func (h *deleteBannerHandler) Middlewares(md ...func(http.Handler) http.Handler) *deleteBannerHandler {
	h.middlewares = append(h.middlewares, md...)
	return h
}

func (h *deleteBannerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {

	strID := chi.URLParam(r, "id")

	ID, err := strconv.ParseInt(strID, 10, 64)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	err = h.usecase.DeleteBanner(r.Context(), entity.DeleteBannerDTO{BannerID: ID})
	if err != nil {
		switch errors.Code(err) {
		case errors.ErrNoDataFound:
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		default:
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

	}
	w.WriteHeader(204)

}
