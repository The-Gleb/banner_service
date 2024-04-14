package usecase

import (
	"context"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
)

type updateBannerUsecase struct {
	bannerService BannerService
}

func NewUpdateBannerUsecase(bannerService BannerService) *updateBannerUsecase {
	return &updateBannerUsecase{bannerService}
}

func (u *updateBannerUsecase) UpdateBanner(ctx context.Context, dto entity.UpdateBannerDTO) error {
	return u.bannerService.UpdateBanner(ctx, dto)
}
