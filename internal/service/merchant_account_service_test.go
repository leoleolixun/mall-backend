package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go-mall/internal/authorization"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"go-mall/pkg/password"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
)

type fakeMerchantAccountRepository struct {
	accounts map[int64]model.MerchantAccount
	nextID   int64
}

func (r *fakeMerchantAccountRepository) Transaction(_ context.Context, fn func(repository.MerchantAccountRepository) error) error {
	return fn(r)
}

func (r *fakeMerchantAccountRepository) List(_ context.Context, merchantID int64, offset int, limit int, role string, status *int, keyword string) ([]model.MerchantAccount, int64, error) {
	result := make([]model.MerchantAccount, 0)
	for _, account := range r.accounts {
		if account.MerchantID != merchantID || role != "" && account.Role != role || status != nil && account.Status != *status {
			continue
		}
		if keyword != "" && account.Username != keyword && account.Nickname != keyword {
			continue
		}
		result = append(result, account)
	}
	total := int64(len(result))
	if offset >= len(result) {
		return []model.MerchantAccount{}, total, nil
	}
	end := offset + limit
	if end > len(result) {
		end = len(result)
	}
	return result[offset:end], total, nil
}

func (r *fakeMerchantAccountRepository) FindForUpdate(_ context.Context, merchantID int64, accountID int64) (*model.MerchantAccount, error) {
	account, ok := r.accounts[accountID]
	if !ok || account.MerchantID != merchantID {
		return nil, fmt.Errorf("not found")
	}
	return &account, nil
}

func (r *fakeMerchantAccountRepository) UsernameExists(_ context.Context, username string) (bool, error) {
	for _, account := range r.accounts {
		if account.Username == username {
			return true, nil
		}
	}
	return false, nil
}

func (r *fakeMerchantAccountRepository) Create(_ context.Context, account *model.MerchantAccount) error {
	r.nextID++
	account.ID = r.nextID
	account.CreatedAt = time.Now()
	account.UpdatedAt = account.CreatedAt
	r.accounts[account.ID] = *account
	return nil
}

func (r *fakeMerchantAccountRepository) Update(_ context.Context, account *model.MerchantAccount) error {
	r.accounts[account.ID] = *account
	return nil
}

func (r *fakeMerchantAccountRepository) UpdatePassword(_ context.Context, merchantID int64, accountID int64, passwordHash string) error {
	account, ok := r.accounts[accountID]
	if !ok || account.MerchantID != merchantID {
		return fmt.Errorf("not found")
	}
	account.PasswordHash = passwordHash
	r.accounts[accountID] = account
	return nil
}

func (r *fakeMerchantAccountRepository) CountEnabledOwnersForUpdate(_ context.Context, merchantID int64) (int64, error) {
	var count int64
	for _, account := range r.accounts {
		if account.MerchantID == merchantID && account.Role == model.MerchantRoleOwner && account.Status == model.StatusEnabled {
			count++
		}
	}
	return count, nil
}

func (r *fakeMerchantAccountRepository) CountByRole(_ context.Context, merchantID int64) (map[string]int64, error) {
	counts := map[string]int64{}
	for _, account := range r.accounts {
		if account.MerchantID == merchantID {
			counts[account.Role]++
		}
	}
	return counts, nil
}

func newMerchantAccountServiceForTest(t *testing.T) (MerchantAccountService, *fakeMerchantAccountRepository, *miniredis.Miniredis) {
	t.Helper()
	repo := &fakeMerchantAccountRepository{
		accounts: map[int64]model.MerchantAccount{
			1: {ID: 1, MerchantID: 10, Username: "owner", Nickname: "店主", Role: model.MerchantRoleOwner, Status: model.StatusEnabled},
			2: {ID: 2, MerchantID: 10, Username: "sales", Nickname: "销售", Role: model.MerchantRoleSales, Status: model.StatusEnabled},
		},
		nextID: 2,
	}
	redisServer := miniredis.RunT(t)
	redisClient := redis.NewClient(&redis.Options{Addr: redisServer.Addr()})
	t.Cleanup(func() { _ = redisClient.Close() })
	return NewMerchantAccountService(repo, redisClient), repo, redisServer
}

func TestMerchantAccountCreateSales(t *testing.T) {
	accountService, repo, _ := newMerchantAccountServiceForTest(t)
	status := model.StatusEnabled
	created, err := accountService.Create(context.Background(), model.MerchantRoleAdmin, 10, dto.MerchantAccountCreateRequest{
		Username: "warehouse_1", Password: "secure-password", Nickname: "一号库管", Role: model.MerchantRoleWarehouse, Status: &status,
	})
	if err != nil {
		t.Fatalf("create account: %v", err)
	}
	stored := repo.accounts[created.ID]
	if created.Role != model.MerchantRoleWarehouse || !password.CheckPassword(stored.PasswordHash, "secure-password") {
		t.Fatalf("unexpected created account: %+v", created)
	}
}

func TestMerchantAdminCannotCreateAdmin(t *testing.T) {
	accountService, _, _ := newMerchantAccountServiceForTest(t)
	_, err := accountService.Create(context.Background(), model.MerchantRoleAdmin, 10, dto.MerchantAccountCreateRequest{
		Username: "another_admin", Password: "secure-password", Nickname: "管理员", Role: model.MerchantRoleAdmin,
	})
	if err == nil {
		t.Fatal("expected admin privilege escalation to be rejected")
	}
}

func TestMerchantAccountKeepsLastEnabledOwner(t *testing.T) {
	accountService, _, _ := newMerchantAccountServiceForTest(t)
	disabled := model.StatusDisabled
	_, err := accountService.Update(context.Background(), 2, model.MerchantRoleOwner, 10, 1, dto.MerchantAccountUpdateRequest{
		Nickname: "店主", Role: model.MerchantRoleOwner, Status: &disabled,
	})
	if err == nil {
		t.Fatal("expected last enabled owner to be protected")
	}
}

func TestMerchantAccountUpdateInvalidatesSession(t *testing.T) {
	accountService, repo, redisServer := newMerchantAccountServiceForTest(t)
	enabled := model.StatusEnabled
	updated, err := accountService.Update(context.Background(), 1, model.MerchantRoleOwner, 10, 2, dto.MerchantAccountUpdateRequest{
		Nickname: "仓库主管", Role: model.MerchantRoleWarehouse, Status: &enabled,
	})
	if err != nil {
		t.Fatalf("update account: %v", err)
	}
	if updated.Role != model.MerchantRoleWarehouse || repo.accounts[2].Role != model.MerchantRoleWarehouse {
		t.Fatalf("unexpected updated account: %+v", updated)
	}
	version, err := redisServer.Get(authorization.MerchantAccountSessionVersionKey(2))
	if err != nil || version != "1" {
		t.Fatalf("unexpected session version %q: %v", version, err)
	}
}

func TestMerchantAccountResetPasswordInvalidatesSession(t *testing.T) {
	accountService, repo, redisServer := newMerchantAccountServiceForTest(t)
	if err := accountService.ResetPassword(context.Background(), 1, model.MerchantRoleOwner, 10, 2, dto.MerchantAccountPasswordRequest{
		Password: "new-secure-password",
	}); err != nil {
		t.Fatalf("reset password: %v", err)
	}
	if !password.CheckPassword(repo.accounts[2].PasswordHash, "new-secure-password") {
		t.Fatal("password hash was not updated")
	}
	if version, _ := redisServer.Get(authorization.MerchantAccountSessionVersionKey(2)); version != "1" {
		t.Fatalf("unexpected session version: %s", version)
	}
}

func TestMerchantAccountCannotUpdateAnotherMerchantAccount(t *testing.T) {
	accountService, repo, _ := newMerchantAccountServiceForTest(t)
	repo.accounts[3] = model.MerchantAccount{
		ID:         3,
		MerchantID: 20,
		Username:   "other_merchant_sales",
		Nickname:   "其他商户销售",
		Role:       model.MerchantRoleSales,
		Status:     model.StatusEnabled,
	}
	enabled := model.StatusEnabled
	_, err := accountService.Update(context.Background(), 1, model.MerchantRoleOwner, 10, 3, dto.MerchantAccountUpdateRequest{
		Nickname: "越权修改",
		Role:     model.MerchantRoleWarehouse,
		Status:   &enabled,
	})
	if err == nil {
		t.Fatal("expected another merchant account update to be rejected")
	}
	if repo.accounts[3].Nickname != "其他商户销售" || repo.accounts[3].Role != model.MerchantRoleSales {
		t.Fatalf("another merchant account was modified: %+v", repo.accounts[3])
	}
}
