package service

import (
	"context"
	"fmt"
	"regexp"
	"strings"
	"time"
	"unicode/utf8"

	"go-mall/internal/authorization"
	"go-mall/internal/dto"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"go-mall/pkg/password"

	"github.com/redis/go-redis/v9"
)

var merchantUsernamePattern = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9._@-]{2,99}$`)

type MerchantAccountService interface {
	List(ctx context.Context, merchantID int64, req dto.MerchantAccountListRequest) (*dto.PageResponse[dto.MerchantAccountListItem], error)
	Create(ctx context.Context, actorRole string, merchantID int64, req dto.MerchantAccountCreateRequest) (*dto.MerchantAccountListItem, error)
	Update(ctx context.Context, actorID int64, actorRole string, merchantID int64, accountID int64, req dto.MerchantAccountUpdateRequest) (*dto.MerchantAccountListItem, error)
	ResetPassword(ctx context.Context, actorID int64, actorRole string, merchantID int64, accountID int64, req dto.MerchantAccountPasswordRequest) error
	Roles(ctx context.Context, merchantID int64) ([]dto.MerchantRoleResponse, error)
}

type merchantAccountService struct {
	repo  repository.MerchantAccountRepository
	redis *redis.Client
}

func NewMerchantAccountService(repo repository.MerchantAccountRepository, redisClient *redis.Client) MerchantAccountService {
	return &merchantAccountService{repo: repo, redis: redisClient}
}

func merchantRoleName(role string) string {
	switch role {
	case model.MerchantRoleOwner:
		return "店主"
	case model.MerchantRoleAdmin:
		return "管理员"
	case model.MerchantRoleOperator:
		return "运营人员"
	case model.MerchantRoleSales:
		return "销售人员"
	case model.MerchantRoleWarehouse:
		return "库管人员"
	default:
		return role
	}
}

func merchantRoleDescription(role string) string {
	switch role {
	case model.MerchantRoleOwner:
		return "拥有商户后台全部权限，包括员工账号管理"
	case model.MerchantRoleAdmin:
		return "拥有商户后台全部业务权限，可管理普通员工"
	case model.MerchantRoleOperator:
		return "负责商品、订单、库存和店铺运营，不可管理员工账号"
	case model.MerchantRoleSales:
		return "可查看经营数据、订单和商品"
	case model.MerchantRoleWarehouse:
		return "可查看订单、执行发货并管理库存"
	default:
		return ""
	}
}

func toMerchantAccountListItem(account model.MerchantAccount) dto.MerchantAccountListItem {
	var lastLoginAt *string
	if account.LastLoginAt != nil {
		value := account.LastLoginAt.Format(time.RFC3339)
		lastLoginAt = &value
	}
	return dto.MerchantAccountListItem{
		ID:          account.ID,
		MerchantID:  account.MerchantID,
		Username:    account.Username,
		Nickname:    account.Nickname,
		Role:        account.Role,
		RoleName:    merchantRoleName(account.Role),
		Status:      account.Status,
		LastLoginAt: lastLoginAt,
		CreatedAt:   account.CreatedAt.Format(time.RFC3339),
		UpdatedAt:   account.UpdatedAt.Format(time.RFC3339),
	}
}

func validateMerchantAccountStatus(status int) error {
	if status != model.StatusDisabled && status != model.StatusEnabled {
		return fmt.Errorf("账号状态不合法")
	}
	return nil
}

func validateMerchantNickname(nickname string) (string, error) {
	nickname = strings.TrimSpace(nickname)
	if nickname == "" {
		return "", fmt.Errorf("员工昵称不能为空")
	}
	if utf8.RuneCountInString(nickname) > 100 {
		return "", fmt.Errorf("员工昵称不能超过 100 个字符")
	}
	return nickname, nil
}

func validateMerchantPassword(raw string) error {
	if len(raw) < 8 {
		return fmt.Errorf("密码不能少于 8 个字符")
	}
	if len([]byte(raw)) > 72 {
		return fmt.Errorf("密码不能超过 72 个字节")
	}
	return nil
}

func canManageMerchantRole(actorRole string, targetRole string) bool {
	if actorRole == model.MerchantRoleOwner {
		return true
	}
	if actorRole == model.MerchantRoleAdmin {
		return targetRole != model.MerchantRoleOwner && targetRole != model.MerchantRoleAdmin
	}
	return false
}

func (s *merchantAccountService) List(
	ctx context.Context,
	merchantID int64,
	req dto.MerchantAccountListRequest,
) (*dto.PageResponse[dto.MerchantAccountListItem], error) {
	if req.Role != "" && !model.IsValidMerchantRole(req.Role) {
		return nil, fmt.Errorf("角色参数不合法")
	}
	if req.Status != nil {
		if err := validateMerchantAccountStatus(*req.Status); err != nil {
			return nil, err
		}
	}
	page, pageSize := normalizeMerchantProductPage(req.Page, req.PageSize)
	accounts, total, err := s.repo.List(ctx, merchantID, (page-1)*pageSize, pageSize, req.Role, req.Status, strings.TrimSpace(req.Keyword))
	if err != nil {
		return nil, err
	}
	list := make([]dto.MerchantAccountListItem, 0, len(accounts))
	for _, account := range accounts {
		list = append(list, toMerchantAccountListItem(account))
	}
	return &dto.PageResponse[dto.MerchantAccountListItem]{List: list, Page: page, PageSize: pageSize, Total: total}, nil
}

func (s *merchantAccountService) Create(
	ctx context.Context,
	actorRole string,
	merchantID int64,
	req dto.MerchantAccountCreateRequest,
) (*dto.MerchantAccountListItem, error) {
	username := strings.TrimSpace(req.Username)
	if !merchantUsernamePattern.MatchString(username) {
		return nil, fmt.Errorf("用户名需为 3-100 位字母、数字或 . _ @ -")
	}
	nickname, err := validateMerchantNickname(req.Nickname)
	if err != nil {
		return nil, err
	}
	if !model.IsValidMerchantRole(req.Role) {
		return nil, fmt.Errorf("员工角色不合法")
	}
	if !canManageMerchantRole(actorRole, req.Role) {
		return nil, fmt.Errorf("当前角色无权创建该角色账号")
	}
	if err := validateMerchantPassword(req.Password); err != nil {
		return nil, err
	}
	status := model.StatusEnabled
	if req.Status != nil {
		status = *req.Status
	}
	if err := validateMerchantAccountStatus(status); err != nil {
		return nil, err
	}
	exists, err := s.repo.UsernameExists(ctx, username)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, fmt.Errorf("用户名已存在")
	}
	passwordHash, err := password.HashPassword(req.Password)
	if err != nil {
		return nil, err
	}
	account := &model.MerchantAccount{
		MerchantID:   merchantID,
		Username:     username,
		PasswordHash: passwordHash,
		Nickname:     nickname,
		Role:         req.Role,
		Status:       status,
	}
	if err := s.repo.Create(ctx, account); err != nil {
		return nil, err
	}
	result := toMerchantAccountListItem(*account)
	return &result, nil
}

func (s *merchantAccountService) Update(
	ctx context.Context,
	actorID int64,
	actorRole string,
	merchantID int64,
	accountID int64,
	req dto.MerchantAccountUpdateRequest,
) (*dto.MerchantAccountListItem, error) {
	nickname, err := validateMerchantNickname(req.Nickname)
	if err != nil {
		return nil, err
	}
	if !model.IsValidMerchantRole(req.Role) {
		return nil, fmt.Errorf("员工角色不合法")
	}
	if req.Status == nil {
		return nil, fmt.Errorf("账号状态不能为空")
	}
	if err := validateMerchantAccountStatus(*req.Status); err != nil {
		return nil, err
	}
	var updated *model.MerchantAccount
	err = s.repo.Transaction(ctx, func(txRepo repository.MerchantAccountRepository) error {
		account, findErr := txRepo.FindForUpdate(ctx, merchantID, accountID)
		if findErr != nil {
			return fmt.Errorf("员工账号不存在")
		}
		if !canManageMerchantRole(actorRole, account.Role) || !canManageMerchantRole(actorRole, req.Role) {
			return fmt.Errorf("当前角色无权修改该账号")
		}
		if account.ID == actorID && *req.Status == model.StatusDisabled {
			return fmt.Errorf("不能停用当前登录账号")
		}
		if account.Role == model.MerchantRoleOwner && (req.Role != model.MerchantRoleOwner || *req.Status == model.StatusDisabled) {
			ownerCount, countErr := txRepo.CountEnabledOwnersForUpdate(ctx, merchantID)
			if countErr != nil {
				return countErr
			}
			if account.Status == model.StatusEnabled && ownerCount <= 1 {
				return fmt.Errorf("商户至少需要保留一个启用的店主账号")
			}
		}
		account.Nickname = nickname
		account.Role = req.Role
		account.Status = *req.Status
		if updateErr := txRepo.Update(ctx, account); updateErr != nil {
			return updateErr
		}
		if _, redisErr := s.redis.Incr(ctx, authorization.MerchantAccountSessionVersionKey(account.ID)).Result(); redisErr != nil {
			return redisErr
		}
		updated = account
		return nil
	})
	if err != nil {
		return nil, err
	}
	result := toMerchantAccountListItem(*updated)
	return &result, nil
}

func (s *merchantAccountService) ResetPassword(
	ctx context.Context,
	actorID int64,
	actorRole string,
	merchantID int64,
	accountID int64,
	req dto.MerchantAccountPasswordRequest,
) error {
	if err := validateMerchantPassword(req.Password); err != nil {
		return err
	}
	passwordHash, err := password.HashPassword(req.Password)
	if err != nil {
		return err
	}
	return s.repo.Transaction(ctx, func(txRepo repository.MerchantAccountRepository) error {
		account, findErr := txRepo.FindForUpdate(ctx, merchantID, accountID)
		if findErr != nil {
			return fmt.Errorf("员工账号不存在")
		}
		if account.ID != actorID && !canManageMerchantRole(actorRole, account.Role) {
			return fmt.Errorf("当前角色无权重置该账号密码")
		}
		if updateErr := txRepo.UpdatePassword(ctx, merchantID, accountID, passwordHash); updateErr != nil {
			return updateErr
		}
		_, redisErr := s.redis.Incr(ctx, authorization.MerchantAccountSessionVersionKey(account.ID)).Result()
		return redisErr
	})
}

func (s *merchantAccountService) Roles(ctx context.Context, merchantID int64) ([]dto.MerchantRoleResponse, error) {
	counts, err := s.repo.CountByRole(ctx, merchantID)
	if err != nil {
		return nil, err
	}
	roles := []string{
		model.MerchantRoleOwner,
		model.MerchantRoleAdmin,
		model.MerchantRoleOperator,
		model.MerchantRoleSales,
		model.MerchantRoleWarehouse,
	}
	result := make([]dto.MerchantRoleResponse, 0, len(roles))
	for _, role := range roles {
		result = append(result, dto.MerchantRoleResponse{
			Role:        role,
			Name:        merchantRoleName(role),
			Description: merchantRoleDescription(role),
			Permissions: authorization.MerchantPermissionNames(role),
			MemberCount: counts[role],
		})
	}
	return result, nil
}
