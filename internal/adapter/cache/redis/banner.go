package cache

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/redis/go-redis/v9"
)

type redisCache struct {
	client *redis.Client
}

func NewRedisCache(client *redis.Client) *redisCache {
	return &redisCache{client: client}
}

func (c *redisCache) Set(ctx context.Context, dto entity.UpdateCacheDTO) error {
	bannerID := fmt.Sprint(dto.BannerID)
	featureID := fmt.Sprintf("feature:%d", dto.FeatureID)
	tagID := fmt.Sprintf("tag:%d", dto.TagID)

	err := c.client.HSet(ctx, bannerID, dto.Content).Err()
	if err != nil {
		slog.Error("error updating banner in redis", "error", err)
		return err
	}
	err = c.client.Expire(ctx, bannerID, 5*time.Minute).Err()
	if err != nil {
		slog.Error("error setting expiry in redis", "error", err)
		return err
	}

	err = c.client.SAdd(ctx, featureID, bannerID).Err()
	if err != nil {
		slog.Error("error updating banner feature in redis", "error", err)
		return err
	}
	err = c.client.Expire(ctx, featureID, 5*time.Minute).Err()
	if err != nil {
		slog.Error("error setting expiry in redis", "error", err)
		return err
	}

	err = c.client.SAdd(ctx, tagID, bannerID).Err()
	if err != nil {
		slog.Error("error updating tags in redis", "error", err)
		return err
	}
	err = c.client.Expire(ctx, tagID, 5*time.Minute).Err()
	if err != nil {
		slog.Error("error setting expiry in redis", "error", err)
		return err
	}

	return nil

}

func (c *redisCache) Get(ctx context.Context, dto entity.GetUserBannerDTO) (entity.BannerContent, error) {
	tagKey := fmt.Sprintf("tags:%d", dto.TagID)
	featureKey := fmt.Sprintf("features:%d", dto.FeatureID)
	bannerIDs, err := c.client.SInter(ctx, tagKey, featureKey).Result()
	if err != nil {
		slog.Error("error getting bannerIDs from redis", "error", err)
		return entity.BannerContent{}, err
	}

	slog.Debug("bannerIDs", "ids", bannerIDs)

	jsonContent, err := c.client.Get(ctx, bannerIDs[0]).Result()
	if err != nil {
		slog.Error("error getting banner content from redis", "error", err)
		return entity.BannerContent{}, err
	}

	var content entity.BannerContent
	err = json.Unmarshal([]byte(jsonContent), &content)
	if err != nil {
		slog.Error("error unmarshalling result from redis", "error", err)
		return entity.BannerContent{}, err
	}

	slog.Debug("got banner content", "content", content)

	return content, nil

}
