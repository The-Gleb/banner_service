package cache

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client *redis.Client
	expiry time.Duration
}

func NewRedisCache(client *redis.Client, expirySeconds int) *redisCache {
	return &redisCache{
		client: client,
		expiry: time.Duration(expirySeconds) * time.Second,
	}
}

func (c *redisCache) Set(ctx context.Context, dto entity.UpdateCacheDTO) error {
	bannerID := fmt.Sprint(dto.BannerID)
	featureID := fmt.Sprintf("features:%d", dto.FeatureID)
	tagID := fmt.Sprintf("tags:%d", dto.TagID)

	err := c.client.HSet(ctx, bannerID, "content", dto.Content, "isActive", dto.IsActive).Err()
	if err != nil {
		slog.Error("error updating banner in redis", "error", err)
		return err
	}
	err = c.client.Expire(ctx, bannerID, c.expiry).Err()
	if err != nil {
		slog.Error("error setting expiry in redis", "error", err)
		return err
	}

	err = c.client.SAdd(ctx, featureID, bannerID).Err()
	if err != nil {
		slog.Error("error updating banner feature in redis", "error", err)
		return err
	}
	err = c.client.Expire(ctx, featureID, c.expiry).Err()
	if err != nil {
		slog.Error("error setting expiry in redis", "error", err)
		return err
	}

	err = c.client.SAdd(ctx, tagID, bannerID).Err()
	if err != nil {
		slog.Error("error updating tags in redis", "error", err)
		return err
	}
	err = c.client.Expire(ctx, tagID, c.expiry).Err()
	if err != nil {
		slog.Error("error setting expiry in redis", "error", err)
		return err
	}

	return nil

}

func (c *redisCache) Get(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error) {
	tagKey := fmt.Sprintf("tags:%d", dto.TagID)
	featureKey := fmt.Sprintf("features:%d", dto.FeatureID)

	slog.Debug("keys", "tag", tagKey, "feature", featureKey)

	bannerIDs, err := c.client.SInter(ctx, featureKey, tagKey).Result()
	if err != nil {
		slog.Error("error getting bannerIDs from redis", "error", err)
		return entity.BannerContent{}, err
	}

	if len(bannerIDs) < 1 {
		slog.Error("banner with that tag not found cache", "tag_id", dto.TagID)
		return entity.BannerContent{}, errors.NewDomainError(errors.ErrNotCached, "")
	}

	slog.Debug("bannerIDs", "ids", bannerIDs)

	strIsActive, err := c.client.HGet(ctx, bannerIDs[0], "isActive").Result()
	if err != nil {
		slog.Error("error getting banner content from redis", "error", err)
		return entity.BannerContent{}, err
	}
	slog.Debug(strIsActive)

	slog.Debug("check permission", "is_active", strIsActive, "is_admin", dto.IsAdmin)
	if strIsActive == "0" && !dto.IsAdmin {
		slog.Error("banner is not active and user is not admin")
		return entity.BannerContent{}, errors.NewDomainError(errors.ErrForbidden, "")
	}

	jsonContent, err := c.client.HGet(ctx, bannerIDs[0], "content").Result()
	if err != nil {
		slog.Error("error getting banner content from redis", "error", err)
		return entity.BannerContent{}, err
	}

	slog.Debug(jsonContent)
	content := entity.BannerContent{}
	err = content.UnmarshalBinary([]byte(jsonContent))
	if err != nil {
		slog.Error("error unmarshalling result from redis", "error", err)
		return entity.BannerContent{}, err
	}

	slog.Debug("got banner content from cache", "content", content)

	return content, nil

}
