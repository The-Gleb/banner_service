package usecase

import "context"

type TokenService interface {
	CheckToken(ctx context.Context, token string) (bool, error)
}

type checkTokenUsecase struct {
	tokenService TokenService
}

func NewCheckTokenUsecase(tokenService TokenService) *checkTokenUsecase {
	return &checkTokenUsecase{tokenService}
}

func (u *checkTokenUsecase) CheckToken(ctx context.Context, token string) (bool, error) {
	return u.tokenService.CheckToken(ctx, token)
}
