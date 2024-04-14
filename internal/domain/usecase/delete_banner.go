package usecase

import (
	"context"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
)

type deleteBannerUsecase struct {
	bannerService BannerService
}

func NewDeleteBannerUsecase(bannerService BannerService) *deleteBannerUsecase {
	return &deleteBannerUsecase{bannerService}
}

func (u *deleteBannerUsecase) DeleteBanner(ctx context.Context, dto entity.DeleteBannerDTO) error {
	return u.bannerService.DeleteBanner(ctx, dto)
}
