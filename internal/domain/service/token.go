package service

import "context"

type TokenStorage interface {
	CheckToken(ctx context.Context, token string) (bool, error)
}

type tokenService struct {
	storage TokenStorage
}

func NewTokenService(storage TokenStorage) *tokenService {
	return &tokenService{storage: storage}
}

func (service *tokenService) CheckToken(ctx context.Context, token string) (bool, error) {
	return service.storage.CheckToken(ctx, token)
}
