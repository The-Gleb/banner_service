package v1

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/The-Gleb/banner_service/internal/errors"
)

type Key string

type CheckTokenUsecase interface {
	CheckToken(ctx context.Context, token string) (bool, error)
}

type authMiddleWare struct {
	usecase CheckTokenUsecase
}

func NewAuthMiddleware(usecase CheckTokenUsecase) *authMiddleWare {
	return &authMiddleWare{usecase}
}

func (m *authMiddleWare) Do(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		slog.Debug("auth middleware working")

		token := r.Header.Get("token")
		if token == "" {
			http.Error(w, string(errors.ErrUnauthorized), http.StatusUnauthorized)
			return
		}

		isAdmin, err := m.usecase.CheckToken(r.Context(), token)
		if err != nil {
			http.Error(w, string(errors.ErrUnauthorized), http.StatusUnauthorized)
			return
		}

		slog.Debug("url", "path", r.URL.Path)
		if r.URL.Path != "/user_banner" && !isAdmin {
			http.Error(w, string(errors.ErrForbidden), http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), Key("isAdmin"), isAdmin)

		r = r.WithContext(ctx)

		next.ServeHTTP(w, r)
	})
}
