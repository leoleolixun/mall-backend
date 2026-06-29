# 2026-06-29 Auth 模块实现教程

## 目标

今天完成统一用户体系和登录认证。

完成后需要跑通：

```http
POST /api/v1/auth/register
POST /api/v1/auth/login/password
POST /api/v1/auth/login/wechat
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/me
```

两种登录方式都要映射到同一个用户体系：

```text
users
user_auths
```

不要做两套用户表。

当前项目命名约定：

```text
DTO:      WechatMiniProgramLoginRequest
Service:  WechatMiniProgramLogin
Route:    POST /api/v1/auth/login/wechat
Provider: wechat_mini_program
JSON:     open_id
```

## 第 1 步：扩展配置

打开：

```text
config.yaml
config.example.yaml
internal/config/config.go
```

在配置中增加 JWT：

```yaml
jwt:
  access_secret: "change_me_access_secret"
  access_ttl_minutes: 120
  refresh_ttl_hours: 168
```

`config.example.yaml` 也要加，但只能放占位值。

在 `internal/config/config.go` 增加：

```go
type Config struct {
	Server ServerConfig `mapstructure:"server"`
	MySQL  MySQLConfig  `mapstructure:"mysql"`
	Redis  RedisConfig  `mapstructure:"redis"`
	Log    LogConfig    `mapstructure:"log"`
	JWT    JWTConfig    `mapstructure:"jwt"`
}

type JWTConfig struct {
	AccessSecret     string `mapstructure:"access_secret"`
	AccessTTLMinutes int    `mapstructure:"access_ttl_minutes"`
	RefreshTTLHours  int    `mapstructure:"refresh_ttl_hours"`
}
```

## 第 2 步：定义用户模型

创建：

```text
internal/model/user.go
```

建议：

```go
package model

import (
	"time"

	"gorm.io/gorm"
)

type User struct {
	ID        int64          `gorm:"primaryKey" json:"id"`
	Nickname  string         `gorm:"type:varchar(100)" json:"nickname"`
	Avatar    string         `gorm:"type:varchar(255)" json:"avatar"`
	Mobile    string         `gorm:"type:varchar(20);index" json:"mobile"`
	Status    int            `gorm:"not null;default:1;index" json:"status"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"-"`
}
```

创建：

```text
internal/model/user_auth.go
```

建议：

```go
package model

import "time"

const (
	AuthProviderPassword         = "password"
	AuthProviderWechatMiniProgram = "wechat_mini_program"
)

type UserAuth struct {
	ID          int64     `gorm:"primaryKey" json:"id"`
	UserID      int64     `gorm:"not null;index" json:"user_id"`
	Provider    string    `gorm:"type:varchar(50);not null;uniqueIndex:uk_provider_uid" json:"provider"`
	ProviderUID string    `gorm:"type:varchar(100);not null;uniqueIndex:uk_provider_uid" json:"provider_uid"`
	Credential  string    `gorm:"type:varchar(255)" json:"-"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
```

说明：

- 账号密码登录：`provider=password`，`provider_uid=username`
- 微信小程序登录：`provider=wechat_mini_program`，`provider_uid=open_id`
- `credential` 存 bcrypt hash 后的密码

## 第 3 步：更新 AutoMigrate

打开：

```text
internal/bootstrap/migrate.go
```

加入：

```go
&model.User{},
&model.UserAuth{},
```

顺序建议：

```go
return db.AutoMigrate(
	&model.Merchant{},
	&model.Category{},
	&model.Product{},
	&model.ProductSKU{},
	&model.User{},
	&model.UserAuth{},
)
```

## 第 4 步：定义认证 DTO

创建：

```text
internal/dto/auth.go
```

建议：

```go
package dto

type RegisterRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
	Nickname string `json:"nickname"`
}

type PasswordLoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type WechatMiniProgramLoginRequest struct {
	OpenID   string `json:"open_id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type AuthResponse struct {
	AccessToken  string       `json:"access_token"`
	RefreshToken string       `json:"refresh_token"`
	User          UserResponse `json:"user"`
}

type UserResponse struct {
	ID       int64  `json:"id"`
	Nickname string `json:"nickname"`
	Avatar   string `json:"avatar"`
	Mobile   string `json:"mobile"`
}
```

## 第 5 步：实现密码工具

创建：

```text
pkg/password/password.go
```

写：

```go
package password

import "golang.org/x/crypto/bcrypt"

func HashPassword(raw string) (string, error) {
	hashed, err := bcrypt.GenerateFromPassword([]byte(raw), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hashed), nil
}

func CheckPassword(hash string, raw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(raw)) == nil
}
```

安装依赖：

```bash
go get golang.org/x/crypto/bcrypt
```

## 第 6 步：实现 JWT 工具

创建：

```text
pkg/jwt/jwt.go
```

建议使用：

```bash
go get github.com/golang-jwt/jwt/v5
```

实现：

```go
package jwt

import (
	"fmt"
	"time"

	gojwt "github.com/golang-jwt/jwt/v5"
)

type Claims struct {
	UserID int64 `json:"user_id"`
	gojwt.RegisteredClaims
}

func GenerateAccessToken(userID int64, secret string, ttl time.Duration) (string, error) {
	claims := Claims{
		UserID: userID,
		RegisteredClaims: gojwt.RegisteredClaims{
			ExpiresAt: gojwt.NewNumericDate(time.Now().Add(ttl)),
			IssuedAt:  gojwt.NewNumericDate(time.Now()),
		},
	}

	token := gojwt.NewWithClaims(gojwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(secret))
}

func ParseAccessToken(tokenString string, secret string) (int64, error) {
	token, err := gojwt.ParseWithClaims(tokenString, &Claims{}, func(token *gojwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*gojwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method")
		}
		return []byte(secret), nil
	})
	if err != nil {
		return 0, err
	}

	claims, ok := token.Claims.(*Claims)
	if !ok || !token.Valid {
		return 0, fmt.Errorf("invalid token")
	}

	return claims.UserID, nil
}
```

## 第 7 步：实现 AuthRepository

创建：

```text
internal/repository/auth_repository.go
```

接口建议：

```go
package repository

import (
	"context"

	"go-mall/internal/model"

	"gorm.io/gorm"
)

type AuthRepository interface {
	CreateUser(ctx context.Context, user *model.User) error
	CreateUserAuth(ctx context.Context, auth *model.UserAuth) error
	FindAuthByProvider(ctx context.Context, provider string, providerUID string) (*model.UserAuth, error)
	FindUserByID(ctx context.Context, userID int64) (*model.User, error)
}

type authRepository struct {
	db *gorm.DB
}

func NewAuthRepository(db *gorm.DB) AuthRepository {
	return &authRepository{db: db}
}
```

实现：

```go
func (r *authRepository) CreateUser(ctx context.Context, user *model.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

func (r *authRepository) CreateUserAuth(ctx context.Context, auth *model.UserAuth) error {
	return r.db.WithContext(ctx).Create(auth).Error
}

func (r *authRepository) FindAuthByProvider(ctx context.Context, provider string, providerUID string) (*model.UserAuth, error) {
	var auth model.UserAuth
	err := r.db.WithContext(ctx).
		Where("provider = ? AND provider_uid = ?", provider, providerUID).
		First(&auth).Error
	if err != nil {
		return nil, err
	}
	return &auth, nil
}

func (r *authRepository) FindUserByID(ctx context.Context, userID int64) (*model.User, error) {
	var user model.User
	err := r.db.WithContext(ctx).
		Where("id = ? AND status = ?", userID, model.StatusEnabled).
		First(&user).Error
	if err != nil {
		return nil, err
	}
	return &user, nil
}
```

第一版先不做事务也能跑通。更严谨的做法是注册和首次微信登录时把 `users`、`user_auths` 放在同一个事务里，后续可以补。

## 第 8 步：实现 AuthService 基础结构

创建：

```text
internal/service/auth_service.go
```

接口建议：

```go
package service

import (
	"context"

	"go-mall/internal/config"
	"go-mall/internal/dto"
	"go-mall/internal/repository"

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
```

## 第 9 步：实现 token 生成辅助方法

在 `auth_service.go` 中加私有方法：

```go
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

	key := fmt.Sprintf("mall:auth:refresh:%s", refreshToken)
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
```

需要：

```bash
go get github.com/google/uuid
```

imports 会用到：

```go
import (
	"fmt"
	"time"

	"go-mall/internal/model"
	"go-mall/pkg/jwt"

	"github.com/google/uuid"
)
```

## 第 10 步：实现 Register

流程：

```text
校验 username/password
检查 username 是否已存在
hash 密码
创建 user
创建 user_auth
返回 token
```

示例：

```go
func (s *authService) Register(ctx context.Context, req dto.RegisterRequest) (*dto.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	req.Nickname = strings.TrimSpace(req.Nickname)

	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("username and password required")
	}
	if len(req.Password) < 6 {
		return nil, fmt.Errorf("password too short")
	}

	if _, err := s.authRepo.FindAuthByProvider(ctx, model.AuthProviderPassword, req.Username); err == nil {
		return nil, fmt.Errorf("username already exists")
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
```

说明：这里如果 `CreateUserAuth` 失败，`users` 会留下孤立数据。第一版先接受，之后可以把注册放进事务优化。

## 第 11 步：实现 PasswordLogin

流程：

```text
读取失败次数
失败过多则拒绝
查 user_auth
校验 bcrypt 密码
失败则 Redis 计数 +1
成功则清理失败计数并返回 token
```

Redis key：

```go
key := fmt.Sprintf("mall:auth:login_fail:password:%s", req.Username)
```

示例核心逻辑：

```go
func (s *authService) PasswordLogin(ctx context.Context, req dto.PasswordLoginRequest) (*dto.AuthResponse, error) {
	req.Username = strings.TrimSpace(req.Username)
	req.Password = strings.TrimSpace(req.Password)
	if req.Username == "" || req.Password == "" {
		return nil, fmt.Errorf("username and password required")
	}

	failKey := fmt.Sprintf("mall:auth:login_fail:password:%s", req.Username)
	failCount, _ := s.redis.Get(ctx, failKey).Int()
	if failCount >= 5 {
		return nil, fmt.Errorf("too many login failures")
	}

	auth, err := s.authRepo.FindAuthByProvider(ctx, model.AuthProviderPassword, req.Username)
	if err != nil || !password.CheckPassword(auth.Credential, req.Password) {
		_ = s.redis.Incr(ctx, failKey).Err()
		_ = s.redis.Expire(ctx, failKey, 15*time.Minute).Err()
		return nil, fmt.Errorf("invalid username or password")
	}

	_ = s.redis.Del(ctx, failKey).Err()

	user, err := s.authRepo.FindUserByID(ctx, auth.UserID)
	if err != nil {
		return nil, err
	}

	return s.buildAuthResponse(ctx, user)
}
```

## 第 12 步：实现 WechatMiniProgramLogin

流程：

```text
接收 openid
查 user_auth
存在则返回老用户 token
不存在则创建用户和 user_auth
```

示例：

```go
func (s *authService) WechatMiniProgramLogin(ctx context.Context, req dto.WechatMiniProgramLoginRequest) (*dto.AuthResponse, error) {
	req.OpenID = strings.TrimSpace(req.OpenID)
	if req.OpenID == "" {
		return nil, fmt.Errorf("openid required")
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
```

## 第 13 步：实现 Me、Refresh、Logout

`Me`：

```go
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
```

`RefreshTokenRequest` 只需要 refresh token，不需要客户端额外传 `user_id`：

```go
type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token"`
}
```

对应 Redis key 设计：

```text
key:   mall:auth:refresh:{refresh_token}
value: user_id
ttl:   7d 或配置中的 refresh_ttl_hours
```

这样服务端可以通过 refresh token 直接拿到 `user_id`，不需要遍历 Redis，也不需要信任客户端传来的 `user_id`。

实现：

```go
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
```

对应接口也把 `Refresh` 方法签名调整为：

```go
Refresh(ctx context.Context, refreshToken string) (*dto.AuthResponse, error)
```

`Logout`：

```go
func (s *authService) Logout(ctx context.Context, refreshToken string) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return nil
	}
	key := fmt.Sprintf("mall:auth:refresh:%s", refreshToken)
	return s.redis.Del(ctx, key).Err()
}
```

这里采用 refresh token rotation：刷新时删除旧 refresh token，并由 `buildAuthResponse` 签发新的 refresh token。

## 第 14 步：实现 JWT 中间件

创建：

```text
internal/middleware/auth.go
```

建议：

```go
package middleware

import (
	"net/http"
	"strings"

	"go-mall/internal/config"
	"go-mall/pkg/jwt"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

const ContextUserIDKey = "user_id"

func Auth(jwtCfg config.JWTConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, "未登录")
			c.Abort()
			return
		}

		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || !strings.EqualFold(parts[0], "Bearer") {
			response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, "无效 token")
			c.Abort()
			return
		}

		userID, err := jwt.ParseAccessToken(parts[1], jwtCfg.AccessSecret)
		if err != nil {
			response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, "无效 token")
			c.Abort()
			return
		}

		c.Set(ContextUserIDKey, userID)
		c.Next()
	}
}

func CurrentUserID(c *gin.Context) (int64, bool) {
	value, exists := c.Get(ContextUserIDKey)
	if !exists {
		return 0, false
	}
	userID, ok := value.(int64)
	return userID, ok
}
```

后面 Handler 里用 `middleware.CurrentUserID(c)` 取当前用户。

## 第 15 步：实现 AuthHandler

创建：

```text
internal/handler/auth_handler.go
```

基础结构：

```go
package handler

import (
	"net/http"

	"go-mall/internal/dto"
	"go-mall/internal/middleware"
	"go-mall/internal/service"
	"go-mall/pkg/response"

	"github.com/gin-gonic/gin"
)

type AuthHandler struct {
	authService service.AuthService
}

func NewAuthHandler(authService service.AuthService) *AuthHandler {
	return &AuthHandler{authService: authService}
}
```

注册：

```go
func (h *AuthHandler) Register(c *gin.Context) {
	var req dto.RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}
	result, err := h.authService.Register(c.Request.Context(), req)
	if err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
```

账号密码登录、微信小程序登录同理：

```go
func (h *AuthHandler) PasswordLogin(c *gin.Context)
func (h *AuthHandler) WechatMiniProgramLogin(c *gin.Context)
```

`Me`：

```go
func (h *AuthHandler) Me(c *gin.Context) {
	userID, ok := middleware.CurrentUserID(c)
	if !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, "未登录")
		return
	}

	user, err := h.authService.Me(c.Request.Context(), userID)
	if err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "查询用户失败")
		return
	}

	response.Success(c, user)
}
```

`Refresh`：

```go
func (h *AuthHandler) Refresh(c *gin.Context) {
	var req dto.RefreshTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		response.Error(c, http.StatusBadRequest, response.CodeBadRequest, "参数错误")
		return
	}
	result, err := h.authService.Refresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, err.Error())
		return
	}
	response.Success(c, result)
}
```

`Logout`：

```go
func (h *AuthHandler) Logout(c *gin.Context) {
	if _, ok := middleware.CurrentUserID(c); !ok {
		response.Error(c, http.StatusUnauthorized, response.CodeBadRequest, "未登录")
		return
	}

	var req dto.RefreshTokenRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.authService.Logout(c.Request.Context(), req.RefreshToken); err != nil {
		response.Error(c, http.StatusInternalServerError, response.CodeInternalError, "退出失败")
		return
	}

	response.Success(c, gin.H{"status": "ok"})
}
```

## 第 16 步：更新 Router

打开：

```text
internal/router/router.go
```

让 `NewRouter` 多接收：

```go
authHandler *handler.AuthHandler,
jwtCfg config.JWTConfig,
```

所以需要 import：

```go
"go-mall/internal/config"
"go-mall/internal/middleware"
```

注册路由：

```go
api := r.Group("/api/v1")
{
	api.GET("/categories", categoryHandler.List)
	api.GET("/products", productHandler.List)
	api.GET("/products/:id", productHandler.Detail)
	api.GET("/products/:id/skus", productHandler.SKUs)

	auth := api.Group("/auth")
	{
		auth.POST("/register", authHandler.Register)
		auth.POST("/login/password", authHandler.PasswordLogin)
		auth.POST("/login/wechat", authHandler.WechatMiniProgramLogin)
		auth.POST("/refresh", authHandler.Refresh)
	}

	protected := api.Group("")
	protected.Use(middleware.Auth(jwtCfg))
	{
		protected.GET("/me", authHandler.Me)
		protected.POST("/auth/logout", authHandler.Logout)
	}
}
```

## 第 17 步：更新 main.go

打开：

```text
cmd/server/main.go
```

初始化 repository：

```go
authRepo := repository.NewAuthRepository(db)
```

初始化 service：

```go
authService := service.NewAuthService(authRepo, rdb, cfg.JWT)
```

初始化 handler：

```go
authHandler := handler.NewAuthHandler(authService)
```

router：

```go
r := router.NewRouter(healthHandler, categoryHandler, productHandler, authHandler, cfg.JWT)
```

## 第 18 步：格式化和编译

执行：

```bash
gofmt -w internal/model/user.go \
  internal/model/user_auth.go \
  internal/dto/auth.go \
  internal/repository/auth_repository.go \
  internal/service/auth_service.go \
  internal/middleware/auth.go \
  internal/handler/auth_handler.go \
  internal/router/router.go \
  cmd/server/main.go \
  pkg/password/password.go \
  pkg/jwt/jwt.go \
  internal/config/config.go
```

再执行：

```bash
go mod tidy
go test ./...
```

## 第 19 步：启动和 curl 验收

启动：

```bash
go run ./cmd/server
```

注册：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/register \
  -H 'Content-Type: application/json' \
  -d '{"username":"leo","password":"123456","nickname":"Leo"}'
```

账号密码登录：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/login/password \
  -H 'Content-Type: application/json' \
  -d '{"username":"leo","password":"123456"}'
```

微信小程序登录：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/login/wechat \
  -H 'Content-Type: application/json' \
  -d '{"open_id":"openid_001","nickname":"微信测试用户","avatar":""}'
```

把登录返回的 `access_token` 放入：

```bash
TOKEN="你的 access_token"
```

查询当前用户：

```bash
curl -sS http://127.0.0.1:8080/api/v1/me \
  -H "Authorization: Bearer $TOKEN"
```

刷新 token：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/refresh \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"你的 refresh_token"}'
```

退出：

```bash
curl -sS -X POST http://127.0.0.1:8080/api/v1/auth/logout \
  -H "Authorization: Bearer $TOKEN" \
  -H 'Content-Type: application/json' \
  -d '{"refresh_token":"你的 refresh_token"}'
```

## 第 20 步：检查 Redis

如果没有 `redis-cli`，可以先跳过；接口能跑通即可。

有 `redis-cli` 时：

```bash
redis-cli -n 5 keys "mall:auth:refresh:*"
redis-cli -n 5 keys "mall:auth:login_fail:*"
```

DB 号按你的 `config.yaml` 为准。

## 今日验收清单

- [ ] `users` 表已创建
- [ ] `user_auths` 表已创建
- [ ] 账号密码注册成功
- [ ] 密码以 bcrypt hash 形式保存
- [ ] 账号密码登录成功
- [ ] 微信小程序登录成功
- [ ] 同一个 openid 重复登录返回同一个用户
- [ ] 登录返回 access token 和 refresh token
- [ ] refresh token 写入 Redis
- [ ] JWT 中间件能保护 `/api/v1/me`
- [ ] 不带 token 访问 `/api/v1/me` 会失败
- [ ] `go test ./...` 通过

## 今天可以暂时接受的不足

- 注册和首次微信登录还没放入事务
- 错误类型还比较粗糙
- refresh token 使用 `mall:auth:refresh:{refresh_token}` 定位用户，不需要客户端传 `user_id`
- 登录失败限流只做账号密码登录
- 没有真实微信 `code2session`

这些后续再优化，今天先把统一用户体系和登录主流程跑通。
