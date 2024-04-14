package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	cache "github.com/The-Gleb/banner_service/internal/adapter/cache/redis"
	db "github.com/The-Gleb/banner_service/internal/adapter/db/postgres"
	v1 "github.com/The-Gleb/banner_service/internal/controller/http/v1/middleware"
	"github.com/The-Gleb/banner_service/internal/domain/entity"
	"github.com/The-Gleb/banner_service/internal/domain/service"
	"github.com/The-Gleb/banner_service/internal/domain/usecase"
	"github.com/The-Gleb/banner_service/pkg/client/postgresql"
	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5"
	"github.com/ory/dockertest"
	"github.com/ory/dockertest/docker"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	code, err := createTestPostgresContainer(m)
	if err != nil {
		log.Fatal(err)
	}
	os.Exit(code)
}

var dsn string = "postgres://test_db:test_db@:5434/tcp/test_db?sslmode=disable"
var redisAddr string

func cleanTables(t *testing.T, dsn string, tableNames ...string) {
	client, err := postgresql.NewClient(context.Background(), dsn)
	require.NoError(t, err)
	for _, name := range tableNames {
		query := fmt.Sprintf("TRUNCATE TABLE \"%s\" CASCADE", name)
		_, err := client.Exec(
			context.Background(),
			query,
		)
		require.NoError(t, err)
	}
}

func startRedis() (string, func()) {

	pool, err := dockertest.NewPool("")
	if err != nil {
		log.Fatalf("Failed to start Dockertest: %+v", err)
	}

	resource, err := pool.Run("redis", "5-alpine", nil)
	if err != nil {
		log.Fatalf("Failed to start redis: %+v", err)
	}

	// determine the port the container is listening on
	addr := net.JoinHostPort("localhost", resource.GetPort("6379/tcp"))

	// wait for the container to be ready
	err = pool.Retry(func() error {
		var e error
		client := redis.NewClient(&redis.Options{Addr: addr})
		defer client.Close()

		_, e = client.Ping(context.Background()).Result()
		return e
	})

	if err != nil {
		log.Fatalf("Failed to ping Redis: %+v", err)
	}

	destroyFunc := func() {
		pool.Purge(resource)
	}

	return addr, destroyFunc
}

func createTestPostgresContainer(m *testing.M) (int, error) {
	pool, err := dockertest.NewPool("")
	if err != nil {
		return 0, err
	}

	pg, err := pool.RunWithOptions(

		&dockertest.RunOptions{
			Repository: "postgres",
			Tag:        "alpine",
			Name:       "migrations-integration-tests",
			Env: []string{
				"POSTGRES_USER=postgres",
				"POSTGRES_PASSWORD=postgres",
			},
			ExposedPorts: []string{"5432"},
		},
		func(config *docker.HostConfig) {
			config.AutoRemove = true
			config.RestartPolicy = docker.RestartPolicy{Name: "no"}
		},
	)
	if err != nil {
		return 0, err
	}

	defer func() {
		if err := pool.Purge(pg); err != nil {
			slog.Error("failed to purge the postgres container: %v", err)
		}
	}()

	dsn = fmt.Sprintf("postgres://postgres:postgres@%s/postgres?sslmode=disable", pg.GetHostPort("5432/tcp"))
	slog.Info(dsn)

	pool.MaxWait = 2 * time.Second
	var conn *pgx.Conn

	err = pool.Retry(func() error {
		conn, err = pgx.Connect(context.Background(), dsn)
		if err != nil {
			return fmt.Errorf("failed to connect to the DB: %w", err)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			slog.Error("failed to correctly close the connection: %v", err)
		}
	}()

	addr, stopRedis := startRedis()
	defer stopRedis()
	redisAddr = addr

	code := m.Run()

	return code, nil
}

func testRequest(
	t *testing.T, ts *httptest.Server,
	method, path string, body []byte, token string,
) (*http.Response, string) {
	t.Helper()

	req, err := http.NewRequest(method, ts.URL+path, bytes.NewReader(body))
	require.NoError(t, err)

	if token != "" {
		req.Header.Set("token", token)
	}

	resp, err := ts.Client().Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	require.NoError(t, err)

	return resp, string(respBody)
}

func Test_getUserBannerHandler_ServeHTTP(t *testing.T) {

	c, err := postgresql.NewClient(context.Background(), dsn)
	require.NoError(t, err)

	err = db.RunMigrations(dsn)
	require.NoError(t, err)

	cleanTables(
		t, dsn,
		"banners", "banner_tag", "banner_feature",
	)

	_, err = c.Exec(
		context.Background(),
		`INSERT INTO tags (id)
		VALUES (1),(2),(3),(4),(5);

		INSERT INTO features (id)
		VALUES (1),(2),(3),(4),(5);
		
		INSERT INTO banners
		(id, title, text, url, is_active, created_at)
		VALUES
			(1, 'title1', 'text1', 'url1', true, NOW()),
			(2, 'title2', 'text2', 'url2', false, NOW()),
			(3, 'title3', 'text3', 'url3', false, NOW());
		
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
	require.NoError(t, err)

	bannerStorage := db.NewBannerStorage(c)

	redisClient := redis.NewClient(&redis.Options{
		Addr:     redisAddr,
		Password: "",
		DB:       0,
	})
	bannerCache := cache.NewRedisCache(redisClient, 3600)
	bannerService := service.NewBannerService(bannerStorage, bannerCache)
	getUserBannerUsecase := usecase.NewGetUserBannerUsecase(bannerService)
	getUserBannerHandler := NewGetUserBannerHandler(getUserBannerUsecase)

	tokenStorage := db.NewTokenStorage(c)
	tokenService := service.NewTokenService(tokenStorage)
	checkTokenUsecase := usecase.NewCheckTokenUsecase(tokenService)
	checkTokenHandler := v1.NewAuthMiddleware(checkTokenUsecase)

	r := chi.NewRouter()
	getUserBannerHandler.Middlewares(checkTokenHandler.Do).AddToRouter(r)
	s := httptest.NewServer(r)

	type want struct {
		code    int
		content entity.BannerContent
	}
	tests := []struct {
		name            string
		tagID           int64
		featureID       int64
		useLastRevision bool
		token           string
		sleepDur        int
		want            want
	}{
		{
			name:            "positive, last revision, active, admin",
			tagID:           1,
			featureID:       1,
			useLastRevision: true,
			token:           "admin_token",
			sleepDur:        1,
			want: want{
				code: 200,
				content: entity.BannerContent{
					Title: "title1",
					Text:  "text1",
					URL:   "url1",
				},
			},
		},
		{
			name:            "positive, from cache, active, admin",
			tagID:           1,
			featureID:       1,
			useLastRevision: false,
			token:           "admin_token",
			sleepDur:        0,
			want: want{
				code: 200,
				content: entity.BannerContent{
					Title: "title1 from cache",
					Text:  "text1",
					URL:   "url1",
				},
			},
		},
		{
			name:            "positive, from db, not active, admin",
			tagID:           4,
			featureID:       3,
			useLastRevision: true,
			token:           "admin_token",
			sleepDur:        0,
			want: want{
				code: 200,
				content: entity.BannerContent{
					Title: "title2",
					Text:  "text2",
					URL:   "url2",
				},
			},
		},
		{
			name:            "negative, from db, not active, not admin",
			tagID:           4,
			featureID:       3,
			useLastRevision: true,
			token:           "user_token",
			sleepDur:        0,
			want: want{
				code: 403,
			},
		},
		{
			name:            "positive, from cache, not active, admin",
			tagID:           4,
			featureID:       3,
			useLastRevision: false,
			token:           "admin_token",
			sleepDur:        0,
			want: want{
				code: 200,
				content: entity.BannerContent{
					Title: "title2 from cache",
					Text:  "text2",
					URL:   "url2",
				},
			},
		},
		{
			name:            "negative, from db, not active, not admin",
			tagID:           4,
			featureID:       3,
			useLastRevision: false,
			token:           "user_token",
			sleepDur:        0,
			want: want{
				code: 403,
			},
		},
		{
			name:      "negative, bad request",
			tagID:     0,
			featureID: -1,
			token:     "user_token",
			want: want{
				code: 400,
			},
		},
		{
			name:      "negative, not found",
			tagID:     5,
			featureID: 10,
			token:     "user_token",
			want: want{
				code: 404,
			},
		},
		{
			name:      "negative, unregistered token",
			tagID:     1,
			featureID: 1,
			token:     "some_token",
			want: want{
				code: 401,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := fmt.Sprintf(
				"/user_banner?tag_id=%d&feature_id=%d&use_last_revision=%t",
				tt.tagID, tt.featureID, tt.useLastRevision,
			)
			// r, err := http.NewRequest("GET", url, nil)
			// require.NoError(t, err)
			// r.Header.Set("token", tt.token)

			// if tt.token == "admin_token" {
			// 	r = r.WithContext(context.WithValue(context.Background(), "isAdmin", true))
			// } else {
			// 	r = r.WithContext(context.WithValue(context.Background(), "isAdmin", false))
			// }

			// rr := httptest.NewRecorder()

			// getUserBannerHandler.ServeHTTP(rr, r)

			resp, body := testRequest(t, s, "GET", path, nil, tt.token)
			require.NoError(t, err)

			require.Equal(t, tt.want.code, resp.StatusCode)
			if tt.want.code != 200 {
				return
			}

			// rb := rr.Body.Bytes()

			slog.Info("body", "buf", body)

			var content entity.BannerContent
			err = json.Unmarshal([]byte(body), &content)
			require.NoError(t, err)

			require.EqualValues(t, tt.want.content, content)

		})
	}
}
