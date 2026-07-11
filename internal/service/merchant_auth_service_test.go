package service

import (
	"context"
	"testing"
	"time"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/pkg/jwt"
	"go-mall/pkg/password"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeMerchantAuthRepository struct {
	account         model.MerchantAccount
	merchant        model.Merchant
	lastLoginUpdate *time.Time
}

func (r *fakeMerchantAuthRepository) Create(_ context.Context, account *model.MerchantAccount) error {
	r.account = *account
	return nil
}

func (r *fakeMerchantAuthRepository) FindByUsername(_ context.Context, username string) (*model.MerchantAccount, error) {
	if r.account.Username != username {
		return nil, redis.Nil
	}
	copy := r.account
	return &copy, nil
}

func (r *fakeMerchantAuthRepository) FindByID(_ context.Context, accountID int64) (*model.MerchantAccount, error) {
	if r.account.ID != accountID {
		return nil, redis.Nil
	}
	copy := r.account
	return &copy, nil
}

func (r *fakeMerchantAuthRepository) FindByIDAndMerchantID(_ context.Context, accountID int64, merchantID int64) (*model.MerchantAccount, error) {
	if r.account.ID != accountID || r.account.MerchantID != merchantID {
		return nil, redis.Nil
	}
	copy := r.account
	return &copy, nil
}

func (r *fakeMerchantAuthRepository) FindMerchantByID(_ context.Context, merchantID int64) (*model.Merchant, error) {
	if r.merchant.ID != merchantID {
		return nil, redis.Nil
	}
	copy := r.merchant
	return &copy, nil
}

func (r *fakeMerchantAuthRepository) UpdateLastLoginAt(_ context.Context, _ int64, lastLoginAt time.Time) error {
	r.lastLoginUpdate = &lastLoginAt
	return nil
}

func newMerchantAuthServiceForTest(t *testing.T) (MerchantAuthService, *fakeMerchantAuthRepository, *miniredis.Miniredis) {
	t.Helper()
	passwordHash, err := password.HashPassword("merchant-password")
	if err != nil {
		t.Fatalf("hash password: %v", err)
	}
	repo := &fakeMerchantAuthRepository{
		account: model.MerchantAccount{
			ID:           11,
			MerchantID:   3,
			Username:     "merchant_admin",
			PasswordHash: passwordHash,
			Nickname:     "店铺管理员",
			Role:         model.MerchantRoleOwner,
			Status:       model.StatusEnabled,
		},
		merchant: model.Merchant{ID: 3, Name: "测试商户", Status: model.StatusEnabled},
	}
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	service := NewMerchantAuthService(repo, redisClient, config.JWTConfig{
		MerchantAccessSecret:     "merchant-secret",
		MerchantAccessTTLMinutes: 30,
		MerchantRefreshTTLHours:  24,
	})
	return service, repo, redisServer
}

func TestMerchantLoginAndRefresh(t *testing.T) {
	service, repo, redisServer := newMerchantAuthServiceForTest(t)
	ctx := context.Background()

	login, err := service.Login(ctx, dto.MerchantLoginRequest{
		Username: "merchant_admin",
		Password: "merchant-password",
	})
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}
	if repo.lastLoginUpdate == nil || login.User.MerchantID != 3 || login.User.Role != model.MerchantRoleOwner || len(login.User.Permissions) == 0 {
		t.Fatalf("unexpected login response: %+v", login)
	}
	claims, err := jwt.ParseMerchantAccessToken(login.AccessToken, "merchant-secret")
	if err != nil || claims.AccountID != 11 || claims.MerchantID != 3 {
		t.Fatalf("unexpected access token claims: %+v, err=%v", claims, err)
	}
	if !redisServer.Exists(merchantRefreshTokenKey(login.RefreshToken)) {
		t.Fatal("refresh token was not stored")
	}

	refreshed, err := service.Refresh(ctx, login.RefreshToken)
	if err != nil {
		t.Fatalf("refresh returned error: %v", err)
	}
	if redisServer.Exists(merchantRefreshTokenKey(login.RefreshToken)) {
		t.Fatal("old refresh token was not deleted")
	}
	if refreshed.RefreshToken == login.RefreshToken || !redisServer.Exists(merchantRefreshTokenKey(refreshed.RefreshToken)) {
		t.Fatal("refresh token was not rotated")
	}

	if err := service.Logout(ctx, 11, refreshed.RefreshToken); err != nil {
		t.Fatalf("logout returned error: %v", err)
	}
	if redisServer.Exists(merchantRefreshTokenKey(refreshed.RefreshToken)) {
		t.Fatal("refresh token was not deleted on logout")
	}
}

func TestMerchantLoginTracksInvalidPassword(t *testing.T) {
	service, _, redisServer := newMerchantAuthServiceForTest(t)

	_, err := service.Login(context.Background(), dto.MerchantLoginRequest{
		Username: "merchant_admin",
		Password: "wrong-password",
	})
	if err == nil {
		t.Fatal("expected login error")
	}
	if count, err := redisServer.Get(merchantLoginFailKey("merchant_admin")); err != nil || count != "1" {
		t.Fatalf("unexpected login failure count %q, err=%v", count, err)
	}
}

func TestMerchantLogoutRejectsAnotherAccountToken(t *testing.T) {
	service, _, redisServer := newMerchantAuthServiceForTest(t)
	login, err := service.Login(context.Background(), dto.MerchantLoginRequest{
		Username: "merchant_admin",
		Password: "merchant-password",
	})
	if err != nil {
		t.Fatalf("login returned error: %v", err)
	}

	if err := service.Logout(context.Background(), 99, login.RefreshToken); err == nil {
		t.Fatal("expected account mismatch error")
	}
	if !redisServer.Exists(merchantRefreshTokenKey(login.RefreshToken)) {
		t.Fatal("another account deleted the refresh token")
	}
}
