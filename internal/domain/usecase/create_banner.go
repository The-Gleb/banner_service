package usecase

import (
	"context"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
)

type BannerService interface {
	CreateBanner(ctx context.Context, dto entity.CreateBannerDTO) (int64, error)
	DeleteBanner(ctx context.Context, dto entity.DeleteBannerDTO) error
	GetUserBanner(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error)
	GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.Banner, error)
	UpdateBanner(ctx context.Context, dto entity.UpdateBannerDTO) error
}

type createBannerUsecase struct {
	bannerService BannerService
}

func NewCreateBannerUsecase(bannerService BannerService) *createBannerUsecase {
	return &createBannerUsecase{bannerService}
}

func (u *createBannerUsecase) CreateBanner(ctx context.Context, dto entity.CreateBannerDTO) (int64, error) {
	return u.bannerService.CreateBanner(ctx, dto)
}
