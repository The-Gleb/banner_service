package service

import (
	"context"
	"log/slog"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/domain/usecase"
)

var _ usecase.BannerService = new(bannerService)
var _ usecase.TokenService = new(tokenService)

type BannerStorage interface {
	CreateBanner(ctx context.Context, dto entity.CreateBannerDTO) (int64, error)
	DeleteBanner(ctx context.Context, dto entity.DeleteBannerDTO) error
	GetUserBanner(ctx context.Context, dto entity.GetUserBannerDTO) (entity.UpdateCacheDTO, error)
	GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.Banner, error)
	UpdateBanner(ctx context.Context, dto entity.UpdateBannerDTO) error
}

type BannerCache interface {
	Set(ctx context.Context, dto entity.UpdateCacheDTO) error
	Get(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error)
}

type bannerService struct {
	storage BannerStorage
	cache   BannerCache
}

func NewBannerService(storage BannerStorage, cache BannerCache) *bannerService {
	return &bannerService{
		storage: storage,
		cache:   cache,
	}
}

func (service *bannerService) CreateBanner(ctx context.Context, dto entity.CreateBannerDTO) (int64, error) {
	return service.storage.CreateBanner(ctx, dto)
}

func (service *bannerService) DeleteBanner(ctx context.Context, dto entity.DeleteBannerDTO) error {
	return service.storage.DeleteBanner(ctx, dto)
}

func (service *bannerService) GetUserBanner(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error) {

	if dto.UseLastRevision {
		banner, err := service.storage.GetUserBanner(ctx, dto)
		if err != nil {
			return entity.BannerContent{}, err
		}

		err = service.cache.Set(ctx, banner)
		if err != nil {
			return entity.BannerContent{}, err
		}

		return banner.Content, err
	}

	content, err := service.cache.Get(ctx, dto)
	if err == nil {
		content.Title += " from cache" // added for tests
		slog.Debug("banner found in cache")
		return content, nil
	}

	updateCacheDTO, err := service.storage.GetUserBanner(ctx, dto)
	if err != nil {
		return entity.BannerContent{}, err
	}

	err = service.cache.Set(ctx, updateCacheDTO)
	if err != nil {
		return entity.BannerContent{}, err
	}

	return updateCacheDTO.Content, nil
}

func (service *bannerService) GetBanners(ctx context.Context, dto entity.GetBannersDTO) ([]entity.Banner, error) {
	return service.storage.GetBanners(ctx, dto)
}

func (service *bannerService) UpdateBanner(ctx context.Context, dto entity.UpdateBannerDTO) error {
	return service.storage.UpdateBanner(ctx, dto)
}
