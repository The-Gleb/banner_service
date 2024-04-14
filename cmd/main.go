package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	cache "github.com/The-Gleb/banner_service/internal/adapter/cache/redis"
	db "github.com/The-Gleb/banner_service/internal/adapter/db/postgres"
	"github.com/The-Gleb/banner_service/internal/config"
	v1 "github.com/The-Gleb/banner_service/internal/controller/http/v1/server"
	"github.com/The-Gleb/banner_service/internal/domain/service"
	"github.com/The-Gleb/banner_service/internal/domain/usecase"
	"github.com/The-Gleb/banner_service/internal/logger"
	"github.com/The-Gleb/banner_service/pkg/client/postgresql"
	"github.com/redis/go-redis/v9"
)

func main() {
	if err := run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

func run(ctx context.Context) error {
	ctx, cancel := signal.NotifyContext(ctx, syscall.SIGINT, syscall.SIGTERM, syscall.SIGQUIT)
	defer cancel()

	configFile := os.Getenv("CONFIG_FILE")
	cfg := config.MustBuild(configFile)

	logger.Initialize(cfg.LogLevel)
	slog.Info("config is built", "struct", cfg)

	dsn := fmt.Sprintf("postgres://%s:%s@%s:%d/%s?sslmode=disable", cfg.DB.Username, cfg.DB.Password, cfg.DB.Host, cfg.DB.Port, cfg.DB.DbName)
	postgresClient, err := postgresql.NewClient(context.Background(), dsn)
	if err != nil {
		return err
	}
	redisClient := redis.NewClient(&redis.Options{
		Addr:     cfg.RedisURL,
		Password: "",
		DB:       0,
	})

	err = db.RunMigrations(dsn)
	if err != nil {
		return err
	}

	// err = prepairDB(postgresClient)
	// if err != nil {
	// 	return err
	// }

	bannerStorage := db.NewBannerStorage(postgresClient)
	bannerCache := cache.NewRedisCache(redisClient, cfg.CacheExpiry)
	tokenStorage := db.NewTokenStorage(postgresClient)

	bannerService := service.NewBannerService(bannerStorage, bannerCache)
	tokenService := service.NewTokenService(tokenStorage)

	createBannerUsecase := usecase.NewCreateBannerUsecase(bannerService)
	deleteBannerUsecase := usecase.NewDeleteBannerUsecase(bannerService)
	getBannerUsecase := usecase.NewGetBannersUsecase(bannerService)
	getUserBannerUsecase := usecase.NewGetUserBannerUsecase(bannerService)
	updateBannerUsecase := usecase.NewUpdateBannerUsecase(bannerService)
	checkTokenUsecase := usecase.NewCheckTokenUsecase(tokenService)

	s, err := v1.NewServer(
		cfg.RunAddress,
		createBannerUsecase,
		deleteBannerUsecase,
		getBannerUsecase,
		getUserBannerUsecase,
		updateBannerUsecase,
		checkTokenUsecase,
	)
	if err != nil {
		return err
	}

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()

		<-ctx.Done()

		ctxShutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		err := s.Stop(ctxShutdown)
		if err != nil {
			panic(err)
		}
		slog.Info("server was successfuly shutdown")
	}()

	slog.Info("starting server")
	if err := s.Start(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		slog.Error("server error", "error", err)
	}

	return nil
}

func prepairDB(c postgresql.Client) error {
	_, err := c.Exec(
		context.Background(),
		`
		INSERT INTO tags (id)
		VALUES (1),(2),(3),(4),(5);

		INSERT INTO features (id)
		VALUES (1),(2),(3),(4),(5);

		INSERT INTO banners
		(title, text, url, is_active, created_at)
		VALUES
			('title1', 'text1', 'url1', true, NOW()),
			('title2', 'text2', 'url2', false, NOW()),
			('title3', 'text3', 'url3', false, NOW());

		INSERT INTO banner_tag (banner_id, tag_id)
		VALUES
			(1, 1), (1,2), (1,3),
			(2, 4), (3, 5), (3, 2);

		INSERT INTO banner_feature (banner_id, feature_id)
		VALUES
			(1, 1), (2, 3), (3, 3);


		INSERT INTO tokens (token, is_admin, created_at)
		VALUES
			('admin_token', true, NOW()),
			('user_token', false, NOW());`,
	)
	return err
}
