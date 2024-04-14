package usecase

import (
	"context"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
)

type getBannersUsecase struct {
	bannerService BannerService
}

func NewGetBannersUsecase(bannerService BannerService) *getBannersUsecase {
	return &getBannersUsecase{bannerService}
}

func (u *getBannersUsecase) GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.Banner, error) {
	return u.bannerService.GetBanners(ctx, dto)
}
