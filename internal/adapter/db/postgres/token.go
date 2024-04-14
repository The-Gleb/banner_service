package db

import (
	"context"
	stdErrors "errors"
	"log/slog"

	"github.com/The-Gleb/banner_service/internal/domain/service"
	"github.com/The-Gleb/banner_service/internal/errors"
	"github.com/The-Gleb/banner_service/pkg/client/postgresql"
	"github.com/jackc/pgx/v5"
)

var _ service.TokenStorage = new(tokenStorage)

type tokenStorage struct {
	client postgresql.Client
}

func NewTokenStorage(c postgresql.Client) *tokenStorage {
	return &tokenStorage{c}
}

func (s *tokenStorage) CheckToken(ctx context.Context, token string) (bool, error) {
	row := s.client.QueryRow(
		ctx,
		`SELECT is_admin
		FROM tokens
		WHERE "token" = $1;`,
		token,
	)

	var isAdmin bool
	err := row.Scan(&isAdmin)
	if err != nil {
		slog.Error("error scanning row", "error", err)
		if stdErrors.Is(err, pgx.ErrNoRows) {
			return false, errors.NewDomainError(errors.ErrUnauthorized, "")
		}
		return false, errors.NewDomainError(errors.ErrDB, "")
	}

	return isAdmin, nil
}
