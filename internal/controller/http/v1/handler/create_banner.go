package v1

import (
	"context"
	"net/http"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
)

type createBannerUsecase interface {
	CreateBanner(ctx context.Context, entity.CreateBannerDTO) error
}

type createBannerHandler struct {
	usecase createBannerUsecase
}

func NewCreateBannerHandler(usecase createBannerUsecase) *createBannerHandler {
	return &createBannerHandler{usecase}
}

func (h *createBannerHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	 
}