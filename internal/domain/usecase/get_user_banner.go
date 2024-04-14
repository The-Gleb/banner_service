package usecase

import (
	"context"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
)

type getUserBannerUsecase struct {
	bannerService BannerService
}

func NewGetUserBannerUsecase(bannerService BannerService) *getUserBannerUsecase {
	return &getUserBannerUsecase{bannerService}
}

func (u *getUserBannerUsecase) GetUserBanner(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error) {
	return u.bannerService.GetUserBanner(ctx, dto)
}
