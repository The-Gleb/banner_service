package v1

import (
	"context"
	"net/http"

	handlers "github.com/The-Gleb/banner_service/internal/controller/http/v1/handler"
	middleware "github.com/The-Gleb/banner_service/internal/controller/http/v1/middleware"
	"github.com/go-chi/chi/v5"
)

type httpServer struct {
	server *http.Server
}

func NewServer(
	address string,
	createBannerUsecase handlers.CreateBannerUsecase,
	deleteBannerUsecase handlers.DeleteBannerUsecase,
	getBannerUsecase handlers.GetBannerUsecase,
	getUserBannerUsecase handlers.GetUserBannerUsecase,
	updateBannerUsecase handlers.UpdateBannerUsecase,
	checkTokenUsecase middleware.CheckTokenUsecase,
) (*httpServer, error) {

	createBannerHandler := handlers.NewCreateBannerHandler(createBannerUsecase)
	deleteBannerHandler := handlers.NewDeleteBannerHandler(deleteBannerUsecase)
	getBannerHandler := handlers.NewGetBannersHandler(getBannerUsecase)
	getUserBannerHandler := handlers.NewGetUserBannerHandler(getUserBannerUsecase)
	updateBannerHandler := handlers.NewUpdateBannerHandler(updateBannerUsecase)

	checkTokenMiddleware := middleware.NewAuthMiddleware(checkTokenUsecase)

	r := chi.NewMux()
	r.Use(checkTokenMiddleware.Do)

	createBannerHandler.AddToRouter(r)
	deleteBannerHandler.AddToRouter(r)
	getBannerHandler.AddToRouter(r)
	getUserBannerHandler.AddToRouter(r)
	updateBannerHandler.AddToRouter(r)

	server := &http.Server{
		Addr:    address,
		Handler: r,
	}

	return &httpServer{server: server}, nil
}

func (s *httpServer) Start() error {
	return s.server.ListenAndServe()
}

func (s *httpServer) Stop(ctx context.Context) error {
	return s.server.Shutdown(ctx)
}
