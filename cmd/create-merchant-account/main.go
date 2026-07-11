package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"go-mall/internal/bootstrap"
	"go-mall/internal/config"
	"go-mall/internal/model"
	"go-mall/internal/repository"
	"go-mall/pkg/password"

	"gorm.io/gorm"
)

func main() {
	merchantID := flag.Int64("merchant-id", 0, "merchant ID")
	username := flag.String("username", "", "login username")
	nickname := flag.String("nickname", "", "display name")
	role := flag.String("role", model.MerchantRoleOwner, "owner, admin, operator, sales or warehouse")
	configPath := flag.String("config", "config.yaml", "config file path")
	flag.Parse()

	*username = strings.TrimSpace(*username)
	*nickname = strings.TrimSpace(*nickname)
	*role = strings.ToLower(strings.TrimSpace(*role))
	rawPassword := strings.TrimSpace(os.Getenv("MERCHANT_ACCOUNT_PASSWORD"))

	if *merchantID <= 0 || *username == "" || rawPassword == "" {
		panic("merchant-id、username 和环境变量 MERCHANT_ACCOUNT_PASSWORD 不能为空")
	}
	if len(rawPassword) < 8 {
		panic("商家账号密码不能少于 8 位")
	}
	if *nickname == "" {
		*nickname = *username
	}
	if !model.IsValidMerchantRole(*role) {
		panic("role 只支持 owner、admin、operator、sales 或 warehouse")
	}

	cfg, err := config.Load(*configPath)
	if err != nil {
		panic(fmt.Errorf("load config failed: %w", err))
	}
	db, err := bootstrap.InitMySQL(cfg.MySQL)
	if err != nil {
		panic(fmt.Errorf("init mysql failed: %w", err))
	}

	repo := repository.NewMerchantAuthRepository(db)
	ctx := context.Background()
	merchant, err := repo.FindMerchantByID(ctx, *merchantID)
	if err != nil || merchant.Status != model.StatusEnabled {
		panic("商户不存在或已禁用")
	}
	if _, err := repo.FindByUsername(ctx, *username); err == nil {
		panic("商家账号用户名已存在")
	} else if !errors.Is(err, gorm.ErrRecordNotFound) {
		panic(fmt.Errorf("check username failed: %w", err))
	}

	passwordHash, err := password.HashPassword(rawPassword)
	if err != nil {
		panic(fmt.Errorf("hash password failed: %w", err))
	}
	account := &model.MerchantAccount{
		MerchantID:   *merchantID,
		Username:     *username,
		PasswordHash: passwordHash,
		Nickname:     *nickname,
		Role:         *role,
		Status:       model.StatusEnabled,
	}
	if err := repo.Create(ctx, account); err != nil {
		panic(fmt.Errorf("create merchant account failed: %w", err))
	}

	fmt.Printf("merchant account created: id=%d merchant_id=%d username=%s role=%s\n", account.ID, account.MerchantID, account.Username, account.Role)
}
