package service

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"go-mall/internal/authorization"
	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"go-mall/pkg/jwt"
	"go-mall/pkg/password"

	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
)

type MerchantAuthService interface {
	Login(ctx context.Context, req dto.MerchantLoginRequest) (*dto.MerchantAuthResponse, error)
	Refresh(ctx context.Context, refreshToken string) (*dto.MerchantAuthResponse, error)
	Logout(ctx context.Context, accountID int64, refreshToken string) error
	Me(ctx context.Context, accountID int64, merchantID int64) (*dto.MerchantAccountResponse, error)
}

type merchantAuthService struct {
	repo   repository.MerchantAuthRepository
	redis  *redis.Client
	jwtCfg config.JWTConfig
}

func NewMerchantAuthService(
	repo repository.MerchantAuthRepository,
	redisClient *redis.Client,
	jwtCfg config.JWTConfig,
) MerchantAuthService {
	return &merchantAuthService{repo: repo, redis: redisClient, jwtCfg: jwtCfg}
}

func merchantRefreshTokenKey(token string) string {
	return fmt.Sprintf("mall:merchant:auth:refresh:%s", token)
}

func merchantLoginFailKey(username string) string {
	return fmt.Sprintf("mall:merchant:auth:login_fail:%s", username)
}

func merchantAccountSessionVersionKey(accountID int64) string {
	return authorization.MerchantAccountSessionVersionKey(accountID)
}

func parseMerchantRefreshTokenValue(value string) (int64, int64, error) {
	parts := strings.SplitN(value, ":", 2)
	accountID, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil || accountID <= 0 {
		return 0, 0, fmt.Errorf("refresh token 数据不合法")
	}
	if len(parts) == 1 {
		return accountID, 0, nil
	}
	version, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || version < 0 {
		return 0, 0, fmt.Errorf("refresh token 数据不合法")
	}
	return accountID, version, nil
}

func toMerchantAccountResponse(account *model.MerchantAccount, merchant *model.Merchant) dto.MerchantAccountResponse {
	return dto.MerchantAccountResponse{
		ID:           account.ID,
		MerchantID:   account.MerchantID,
		MerchantName: merchant.Name,
		Username:     account.Username,
		Nickname:     account.Nickname,
		Role:         account.Role,
		Permissions:  authorization.MerchantPermissionNames(account.Role),
	}
}

func (s *merchantAuthService) loadActiveAccount(
	ctx context.Context,
	accountID int64,
	merchantID int64,
) (*model.MerchantAccount, *model.Merchant, error) {
	account, err := s.repo.FindByIDAndMerchantID(ctx, accountID, merchantID)
	if err != nil || account.Status != model.StatusEnabled {
		return nil, nil, fmt.Errorf("商家账号不可用")
	}
	if !model.IsValidMerchantRole(account.Role) {
		return nil, nil, fmt.Errorf("商家账号角色不合法")
	}
	merchant, err := s.repo.FindMerchantByID(ctx, merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return nil, nil, fmt.Errorf("商户不可用")
	}
	return account, merchant, nil
}

func (s *merchantAuthService) buildAuthResponse(
	ctx context.Context,
	account *model.MerchantAccount,
	merchant *model.Merchant,
) (*dto.MerchantAuthResponse, error) {
	sessionVersion, err := s.redis.Get(ctx, merchantAccountSessionVersionKey(account.ID)).Int64()
	if err == redis.Nil {
		sessionVersion = 0
	} else if err != nil {
		return nil, err
	}
	accessTTL := time.Duration(s.jwtCfg.MerchantAccessTTLMinutes) * time.Minute
	if accessTTL <= 0 {
		accessTTL = 2 * time.Hour
	}
	accessToken, err := jwt.GenerateMerchantAccessToken(
		account.ID,
		account.MerchantID,
		account.Role,
		sessionVersion,
		s.jwtCfg.MerchantAccessSecret,
		accessTTL,
	)
	if err != nil {
		return nil, err
	}

	refreshTTL := time.Duration(s.jwtCfg.MerchantRefreshTTLHours) * time.Hour
	if refreshTTL <= 0 {
		refreshTTL = 7 * 24 * time.Hour
	}
	refreshToken := uuid.NewString()
	refreshValue := fmt.Sprintf("%d:%d", account.ID, sessionVersion)
	if err := s.redis.Set(ctx, merchantRefreshTokenKey(refreshToken), refreshValue, refreshTTL).Err(); err != nil {
		return nil, err
	}

	return &dto.MerchantAuthResponse{
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		User:         toMerchantAccountResponse(account, merchant),
	}, nil
}

func (s *merchantAuthService) Login(
	ctx context.Context,
	req dto.MerchantLoginRequest,
) (*dto.MerchantAuthResponse, error) {
	username := strings.TrimSpace(req.Username)
	passwordText := strings.TrimSpace(req.Password)
	if username == "" || passwordText == "" {
		return nil, fmt.Errorf("用户名和密码不能为空")
	}

	failKey := merchantLoginFailKey(username)
	failCount, _ := s.redis.Get(ctx, failKey).Int()
	if failCount >= 5 {
		return nil, fmt.Errorf("登录失败次数过多，请稍后再试")
	}

	account, err := s.repo.FindByUsername(ctx, username)
	if err != nil || !password.CheckPassword(account.PasswordHash, passwordText) {
		_ = s.redis.Incr(ctx, failKey).Err()
		_ = s.redis.Expire(ctx, failKey, 15*time.Minute).Err()
		return nil, fmt.Errorf("用户名或密码错误")
	}
	if account.Status != model.StatusEnabled {
		return nil, fmt.Errorf("商家账号已禁用")
	}
	if !model.IsValidMerchantRole(account.Role) {
		return nil, fmt.Errorf("商家账号角色不合法")
	}

	merchant, err := s.repo.FindMerchantByID(ctx, account.MerchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		return nil, fmt.Errorf("商户不可用")
	}

	_ = s.redis.Del(ctx, failKey).Err()
	now := time.Now()
	if err := s.repo.UpdateLastLoginAt(ctx, account.ID, now); err != nil {
		return nil, err
	}
	account.LastLoginAt = &now

	return s.buildAuthResponse(ctx, account, merchant)
}

func (s *merchantAuthService) Refresh(ctx context.Context, refreshToken string) (*dto.MerchantAuthResponse, error) {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil, fmt.Errorf("refresh token 不能为空")
	}

	key := merchantRefreshTokenKey(refreshToken)
	refreshValue, err := s.redis.Get(ctx, key).Result()
	if err != nil {
		return nil, fmt.Errorf("refresh token 无效或已过期")
	}
	accountID, tokenVersion, err := parseMerchantRefreshTokenValue(refreshValue)
	if err != nil {
		return nil, fmt.Errorf("refresh token 无效或已过期")
	}
	currentVersion, err := s.redis.Get(ctx, merchantAccountSessionVersionKey(accountID)).Int64()
	if err == redis.Nil {
		currentVersion = 0
	} else if err != nil {
		return nil, err
	}
	if tokenVersion != currentVersion {
		_ = s.redis.Del(ctx, key).Err()
		return nil, fmt.Errorf("refresh token 已失效，请重新登录")
	}
	account, err := s.repo.FindByID(ctx, accountID)
	if err != nil {
		return nil, fmt.Errorf("商家账号不可用")
	}
	account, merchant, err := s.loadActiveAccount(ctx, account.ID, account.MerchantID)
	if err != nil {
		return nil, err
	}

	if err := s.redis.Del(ctx, key).Err(); err != nil {
		return nil, err
	}
	return s.buildAuthResponse(ctx, account, merchant)
}

func (s *merchantAuthService) Logout(ctx context.Context, accountID int64, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}

	key := merchantRefreshTokenKey(refreshToken)
	refreshValue, err := s.redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		return err
	}
	storedAccountID, _, parseErr := parseMerchantRefreshTokenValue(refreshValue)
	if parseErr != nil || storedAccountID != accountID {
		return fmt.Errorf("refresh token 不属于当前商家账号")
	}
	return s.redis.Del(ctx, key).Err()
}

func (s *merchantAuthService) Me(
	ctx context.Context,
	accountID int64,
	merchantID int64,
) (*dto.MerchantAccountResponse, error) {
	account, merchant, err := s.loadActiveAccount(ctx, accountID, merchantID)
	if err != nil {
		return nil, err
	}
	response := toMerchantAccountResponse(account, merchant)
	return &response, nil
}
