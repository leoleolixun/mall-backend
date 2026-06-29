package service

import (
	"context"
	"fmt"
	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"go-mall/pkg/jwt"
	"go-mall/pkg/password"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type AuthService interface {
	Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error)
	PasswordLogin(ctx context.Context, req dto.PasswordLoginRequest) (*dto.AuthResponse, error)
	WechatMiniProgramLogin(ctx context.Context, req dto.WechatMiniProgramLoginRequest) (*dto.AuthResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*dto.AuthResponse, error)
	Logout(ctx context.Context, refreshToken string) error
	Me(ctx context.Context, userID int64) (*dto.UserResponse, error)
}

type authService struct {
	authRepo repository.AuthRepository
	redis    *redis.Client
	jwtCfg   config.JWTConfig
}

func NewAuthService(authRepo repository.AuthRepository, redis *redis.Client, jwtCfg config.JWTConfig) AuthService {
	return &authService{
		authRepo: authRepo,
		redis:    redis,
		jwtCfg:   jwtCfg,
	}
}

func refreshTokenKey(refreshToken string) string {
	return fmt.Sprintf("mall:auth:refresh:%s", refreshToken)
}

func (s *authService) buildAuthResponse(ctx context.Context, user *model.User) (*dto.AuthResponse, error) {
	accessTTL := time.Duration(s.jwtCfg.AccessTTLMinutes) * time.Minute
	if accessTTL <= 0 {
		accessTTL = 2 * time.Hour
	}

	accessToken, err := jwt.GenerateAccessToken(user.ID, s.jwtCfg.AccessSecret, accessTTL)
	if err != nil {
		return nil, err
	}

	refreshToken := uuid.NewString()
	refreshTTL := time.Duration(s.jwtCfg.RefreshTTLHours) * time.Hour
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}

	key := refreshTokenKey(refreshToken)
	if err := s.redis.Set(ctx, key, user.ID, refreshTTL).Err(); err != nil {
		return nil, err
	}
	return &dto.AuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User: dto.UserResponse{
			ID:       user.ID,
			Nickname: user.Nickname,
			Avatar:   user.Avatar,
			Mobile:   user.Mobile,
		},
	}, nil
}

func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Nickname = strings.TrimSpace(req.Nickname)

	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("用户名和密码不能为空")
	}
	if len(req.Password) < 6 {
		return nil, fmt.Errorf("密码长度不能小于6位")
	}

	if _, err := s.authRepo.FindAuthByProvider(ctx, model.AuthProviderPassword, req.Username); err == nil {
		return nil, fmt.Errorf("用户名已存在")
	}

	hashed, err := password.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}

	nickname := req.Nickname
	if nickname == "" {
		nickname = req.Username
	}

	user := &model.User{
		Nickname: nickname,
		Status:   model.StatusEnabled,
	}
	if err := s.authRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	auth := &model.UserAuth{
		UserID:      user.ID,
		Provider:    model.AuthProviderPassword,
		ProviderUID: req.Username,
		Credential:  hashed,
	}
	if err := s.authRepo.CreateUserAuth(ctx, auth); err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user)
}

func (s *authService) PasswordLogin(ctx context.Context, req dto.PasswordLoginRequest) (*dto.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("用户名和密码不能为空")
	}

	failKey := fmt.Sprintf("mall:auth:login_fail:password:%s", req.Username)
	failCount, _ := s.redis.Get(ctx, failKey).Int()
	if failCount >= 5 {
		return nil, fmt.Errorf("登录失败次数过多，请稍后再试")
	}

	auth, err := s.authRepo.FindAuthByProvider(ctx, model.AuthProviderPassword, req.Username)
	if err != nil || !password.CheckPassword(auth.Credential, req.Password) {
		_ = s.redis.Incr(ctx, failKey).Err()
		_ = s.redis.Expire(ctx, failKey, 15*time.Minute).Err()
		return nil, fmt.Errorf("用户名或密码错误")
	}

	_ = s.redis.Del(ctx, failKey).Err()

	user, err := s.authRepo.FindUserByID(ctx, auth.UserID)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user)
}

func (s *authService) WechatMiniProgramLogin(ctx context.Context, req dto.WechatMiniProgramLoginRequest) (*dto.AuthResponse, error) {
	req.OpenID = strings.TrimSpace(req.OpenID)
	if req.OpenID == "" {
		return nil, fmt.Errorf("OpenID不能为空")
	}

	auth, err := s.authRepo.FindAuthByProvider(ctx, model.AuthProviderWechatMiniProgram, req.OpenID)
	if err == nil {
		user, err := s.authRepo.FindUserByID(ctx, auth.UserID)
		if err != nil {
			return nil, err
		}
		return s.buildAuthResponse(ctx, user)
	}

	nickname := strings.TrimSpace(req.Nickname)
	if nickname == "" {
		nickname = "微信用户"
	}

	user := &model.User{
		Nickname: nickname,
		Avatar:   req.Avatar,
		Status:   model.StatusEnabled,
	}
	if err := s.authRepo.CreateUser(ctx, user); err != nil {
		return nil, err
	}

	newAuth := &model.UserAuth{
		UserID:      user.ID,
		Provider:    model.AuthProviderWechatMiniProgram,
		ProviderUID: req.OpenID,
	}
	if err := s.authRepo.CreateUserAuth(ctx, newAuth); err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user)
}

func (s *authService) Refresh(ctx context.Context, refreshToken string) (*dto.AuthResponse, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token required")
	}

	key := fmt.Sprintf("mall:auth:refresh:%s", refreshToken)
	userID, err := s.redis.Get(ctx, key).Int64()
	if err != nil {
		return nil, fmt.Errorf("invalid refresh token")
	}

	_ = s.redis.Del(ctx, key).Err()

	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.buildAuthResponse(ctx, user)
}

func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	key := fmt.Sprintf("mall:auth:refresh:%s", refreshToken)
	return s.redis.Del(ctx, key).Err()
}

func (s *authService) Me(ctx context.Context, userID int64) (*dto.UserResponse, error) {
	user, err := s.authRepo.FindUserByID(ctx, userID)
	if err != nil {
		return nil, err
	}

	return &dto.UserResponse{
		ID:       user.ID,
		Nickname: user.Nickname,
		Avatar:   user.Avatar,
		Mobile:   user.Mobile,
	}, nil
}
