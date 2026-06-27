# 2026-06-27 详细教程：Go 后端项目骨架、MySQL、Redis

## 今天你要完成什么

今天的目标是把 `mall-backend` 变成一个最小可运行的 Go 后端项目。

完成后，你应该能做到：

- 启动一个 Gin HTTP 服务
- 访问 `/health` 得到统一 JSON 响应
- 后端启动时能读取配置文件
- 后端启动时能连接 MySQL
- 后端启动时能连接 Redis
- 本地能通过 Docker Compose 启动 MySQL 和 Redis

今天不要急着写商品、登录、订单。第一天只做工程底座。

## 推荐学习顺序

先学 1 小时，再写代码。

建议顺序：

```text
1. Go module 和 package
2. struct、method、pointer
3. error 显式处理
4. Gin 最小服务
5. GORM 连接 MySQL
6. go-redis 连接 Redis
7. Docker Compose 基础
```

## 第 0 步：确认环境

在 `mall-backend` 目录下操作：

```bash
cd /Users/leo/GoWorkSpace/mall/mall-backend
```

检查 Go：

```bash
go version
```

建议 Go 版本：

```text
Go 1.24+
```

检查 Docker：

```bash
docker version
docker compose version
```

检查 Redis 和 MySQL 端口是否可能冲突：

```bash
lsof -i :3306
lsof -i :6379
```

如果你本地已经有 MySQL 或 Redis，可以继续使用本地环境，也可以让 Docker Compose 使用不同端口。

## 第 1 步：初始化 Go module

在 `mall-backend` 下执行：

```bash
go mod init go-mini-mall
```

这里的 module 名可以先用 `go-mini-mall`。后续如果要发布到 GitHub，可以再改成类似：

```text
github.com/yourname/go-mini-mall
```

初始化后你应该看到：

```text
go.mod
```

此时先理解两件事：

- `go.mod` 类似 Python 项目里的依赖声明文件
- Go 的 import 路径会基于 module 名组织

## 第 2 步：创建目录结构

今天先创建这些目录：

```text
cmd/server
internal/config
internal/bootstrap
internal/router
internal/handler
internal/middleware
internal/model
internal/repository
internal/service
internal/dto
pkg/response
pkg/logger
pkg/errors
```

建议你手动创建，顺便熟悉 Go 项目的目录习惯。

目录职责：

```text
cmd/server        程序入口
internal/config   配置结构和加载逻辑
internal/bootstrap 初始化 MySQL、Redis、Logger 等组件
internal/router   路由注册
internal/handler  HTTP Handler
internal/service  业务逻辑
internal/repository 数据库访问
internal/model    数据库模型
internal/dto      请求和响应结构
pkg/response      统一响应
pkg/logger        日志封装
pkg/errors        业务错误封装
```

今天不会全部写满，但先建好，后面每天都往里面加。

## 第 3 步：安装今天需要的依赖

今天建议先安装这些：

```bash
go get github.com/gin-gonic/gin
go get gorm.io/gorm
go get gorm.io/driver/mysql
go get github.com/redis/go-redis/v9
go get github.com/spf13/viper
go get go.uber.org/zap
```

依赖说明：

```text
gin              HTTP 框架
gorm             ORM
gorm mysql driver MySQL 驱动
go-redis         Redis 客户端
viper            配置读取
zap              结构化日志
```

安装后执行：

```bash
go mod tidy
```

## 第 4 步：编写配置文件

创建：

```text
config.yaml
```

建议内容：

```yaml
server:
  port: 8080
  mode: debug

mysql:
  host: 127.0.0.1
  port: 3306
  user: mall
  password: mall123456
  database: mall
  charset: utf8mb4
  parse_time: true
  loc: Local

redis:
  addr: 127.0.0.1:6379
  password: ""
  db: 0

log:
  level: debug
```

注意：

- `parse_time: true` 是为了让 MySQL 时间字段能正确映射到 Go 的 `time.Time`
- `charset: utf8mb4` 是为了支持中文和 emoji
- 今天先不做多环境配置，先保证本地跑通

## 第 5 步：定义配置结构

创建：

```text
internal/config/config.go
```

你需要定义一个总配置结构，例如：

```go
type Config struct {
    Server ServerConfig `mapstructure:"server"`
    MySQL  MySQLConfig  `mapstructure:"mysql"`
    Redis  RedisConfig  `mapstructure:"redis"`
    Log    LogConfig    `mapstructure:"log"`
}
```

再定义：

```go
type ServerConfig struct {
    Port int    `mapstructure:"port"`
    Mode string `mapstructure:"mode"`
}

type MySQLConfig struct {
    Host      string `mapstructure:"host"`
    Port      int    `mapstructure:"port"`
    User      string `mapstructure:"user"`
    Password  string `mapstructure:"password"`
    Database  string `mapstructure:"database"`
    Charset   string `mapstructure:"charset"`
    ParseTime bool   `mapstructure:"parse_time"`
    Loc       string `mapstructure:"loc"`
}

type RedisConfig struct {
    Addr     string `mapstructure:"addr"`
    Password string `mapstructure:"password"`
    DB       int    `mapstructure:"db"`
}

type LogConfig struct {
    Level string `mapstructure:"level"`
}
```

然后实现一个 `Load` 函数：

```go
func Load(path string) (*Config, error) {
    // 使用 viper 读取 config.yaml
}
```

你要练习的重点不是复制代码，而是理解：

- 为什么配置要用结构体接收
- 为什么函数要返回 `(*Config, error)`
- 为什么错误要向上返回，而不是直接 panic

建议今天规则：

- 配置文件不存在，可以返回 error
- 配置解析失败，可以返回 error
- 不要在 `config.Load` 里启动服务或连接数据库

## 第 6 步：定义统一响应

创建：

```text
pkg/response/response.go
```

建议统一响应格式：

```json
{
  "code": 0,
  "message": "ok",
  "data": {}
}
```

建议定义：

```go
type Response struct {
    Code    int         `json:"code"`
    Message string      `json:"message"`
    Data    interface{} `json:"data,omitempty"`
}
```

再提供两个函数：

```go
func Success(c *gin.Context, data interface{})
func Error(c *gin.Context, httpStatus int, code int, message string)
```

今天先不要设计复杂错误码，可以先约定：

```text
0       成功
40000   参数错误
50000   服务内部错误
```

为什么第一天就做统一响应：

- 后面所有接口风格一致
- 前端联调更简单
- 错误处理更清晰

## 第 7 步：初始化日志

创建：

```text
pkg/logger/logger.go
```

今天可以简单封装 zap。

你要完成：

- 根据配置创建 logger
- 提供 `Info`、`Error` 等基础方法，或者直接返回 `*zap.Logger`
- 服务启动失败时能打出清晰错误

第一天不要做：

- 日志文件切割
- 链路追踪
- 请求 ID
- Sentry

这些后续再加。

## 第 8 步：准备 Docker Compose

创建：

```text
docker-compose.yml
```

建议服务：

```yaml
services:
  mysql:
    image: mysql:8.0
    container_name: mall-mysql
    environment:
      MYSQL_ROOT_PASSWORD: root123456
      MYSQL_DATABASE: mall
      MYSQL_USER: mall
      MYSQL_PASSWORD: mall123456
    ports:
      - "3306:3306"
    volumes:
      - mysql_data:/var/lib/mysql
    command:
      - --character-set-server=utf8mb4
      - --collation-server=utf8mb4_unicode_ci

  redis:
    image: redis:7
    container_name: mall-redis
    ports:
      - "6379:6379"

volumes:
  mysql_data:
```

启动依赖：

```bash
docker compose up -d mysql redis
```

查看容器：

```bash
docker compose ps
```

停止：

```bash
docker compose down
```

如果你的本机 3306 或 6379 已被占用，可以改成：

```yaml
ports:
  - "13306:3306"
```

然后 `config.yaml` 里 MySQL 端口也改成 `13306`。

## 第 9 步：初始化 MySQL

创建：

```text
internal/bootstrap/mysql.go
```

目标：

```go
func InitMySQL(cfg config.MySQLConfig) (*gorm.DB, error) {
    // 拼接 DSN
    // gorm.Open(mysql.Open(dsn), &gorm.Config{})
    // 获取底层 sqlDB
    // Ping
    // 设置连接池
    // 返回 *gorm.DB
}
```

DSN 大概长这样：

```text
mall:mall123456@tcp(127.0.0.1:3306)/mall?charset=utf8mb4&parseTime=True&loc=Local
```

连接池可以先设置：

```go
sqlDB.SetMaxIdleConns(10)
sqlDB.SetMaxOpenConns(100)
sqlDB.SetConnMaxLifetime(time.Hour)
```

今天要理解：

- `gorm.DB` 是 ORM 对象
- `sql.DB` 是底层连接池
- 启动时 `Ping` 可以尽早发现配置错误

## 第 10 步：初始化 Redis

创建：

```text
internal/bootstrap/redis.go
```

目标：

```go
func InitRedis(cfg config.RedisConfig) (*redis.Client, error) {
    // 创建 redis.Client
    // Ping
    // 返回 client
}
```

Redis v9 基本都会使用 context：

```go
ctx := context.Background()
err := client.Ping(ctx).Err()
```

今天要理解：

- Go 里很多 IO 操作都会接收 `context.Context`
- 后面 HTTP 请求进来后，要尽量把请求的 context 往下传
- Redis 连接失败应该让服务启动失败，而不是静默忽略

## 第 11 步：实现健康检查 Handler

创建：

```text
internal/handler/health_handler.go
```

建议：

```go
type HealthHandler struct {
    db    *gorm.DB
    redis *redis.Client
}
```

今天可以做简单版本：

- 返回 `status: ok`

更好的版本：

- 检查 MySQL ping
- 检查 Redis ping
- 返回依赖状态

接口：

```http
GET /health
```

响应示例：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "status": "ok",
    "mysql": "ok",
    "redis": "ok"
  }
}
```

## 第 12 步：注册路由

创建：

```text
internal/router/router.go
```

目标：

```go
func NewRouter(healthHandler *handler.HealthHandler) *gin.Engine {
    r := gin.New()
    r.Use(gin.Logger())
    r.Use(gin.Recovery())

    r.GET("/health", healthHandler.Health)

    return r
}
```

今天先用 Gin 自带的 Logger 和 Recovery。

后续再替换为：

- 自定义日志中间件
- 请求 ID
- 统一 panic 处理

## 第 13 步：组装 main.go

创建：

```text
cmd/server/main.go
```

主流程应该清楚：

```text
1. 加载配置
2. 初始化 logger
3. 初始化 MySQL
4. 初始化 Redis
5. 初始化 handler
6. 初始化 router
7. 启动 HTTP 服务
```

你可以把 main 写成非常直白的流程。第一天不要过度抽象。

伪代码：

```go
func main() {
    cfg, err := config.Load("config.yaml")
    if err != nil {
        panic(err)
    }

    log, err := logger.New(cfg.Log)
    if err != nil {
        panic(err)
    }

    db, err := bootstrap.InitMySQL(cfg.MySQL)
    if err != nil {
        log.Fatal("init mysql failed", zap.Error(err))
    }

    rdb, err := bootstrap.InitRedis(cfg.Redis)
    if err != nil {
        log.Fatal("init redis failed", zap.Error(err))
    }

    healthHandler := handler.NewHealthHandler(db, rdb)
    r := router.NewRouter(healthHandler)

    addr := fmt.Sprintf(":%d", cfg.Server.Port)
    if err := r.Run(addr); err != nil {
        log.Fatal("server stopped", zap.Error(err))
    }
}
```

你要注意：

- `main` 可以处理启动失败
- 业务层代码不要随便 `panic`
- 初始化失败直接退出是合理的

## 第 14 步：运行和调试

启动依赖：

```bash
docker compose up -d mysql redis
```

整理依赖：

```bash
go mod tidy
```

启动后端：

```bash
go run ./cmd/server
```

访问健康检查：

```bash
curl http://127.0.0.1:8080/health
```

期望看到：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "status": "ok"
  }
}
```

如果你做了依赖检查，期望看到：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "status": "ok",
    "mysql": "ok",
    "redis": "ok"
  }
}
```

## 常见错误和排查方式

### 1. MySQL 连接失败

常见原因：

- 容器没启动
- 端口被占用
- 用户名或密码不一致
- 数据库名不一致

排查：

```bash
docker compose ps
docker logs mall-mysql
```

确认 `config.yaml`：

```yaml
user: mall
password: mall123456
database: mall
```

### 2. Redis 连接失败

排查：

```bash
docker compose ps
docker logs mall-redis
```

确认：

```yaml
addr: 127.0.0.1:6379
```

### 3. import 路径错误

如果 module 是：

```text
go-mini-mall
```

那么内部包 import 应该类似：

```go
import "go-mini-mall/internal/config"
```

不要写成文件系统绝对路径。

### 4. Gin 服务启动了但接口 404

检查：

- 是否注册了 `GET /health`
- 是否启动的是正确的 router
- 是否请求了正确端口

### 5. 修改代码后依赖异常

执行：

```bash
go mod tidy
```

再运行：

```bash
go run ./cmd/server
```

## 今天的代码质量要求

今天不要追求高级架构，但要做到：

- 每个包职责清楚
- 错误要返回，不要吞掉
- 配置不要硬编码在业务代码里
- MySQL 和 Redis 初始化失败要让服务启动失败
- `/health` 返回统一响应格式
- import 路径清晰

## 今天不要做的事情

- 不写商品业务
- 不写登录业务
- 不写 JWT
- 不写订单
- 不做真实微信登录
- 不做复杂依赖注入框架
- 不做微服务
- 不做过度封装

## 今日最终验收清单

完成后逐项打勾：

- [ ] `mall-backend/go.mod` 已存在
- [ ] `cmd/server/main.go` 已存在
- [ ] `config.yaml` 已存在
- [ ] `docker-compose.yml` 已存在
- [ ] `docker compose up -d mysql redis` 可以成功
- [ ] `go run ./cmd/server` 可以启动
- [ ] 服务启动时 MySQL 连接成功
- [ ] 服务启动时 Redis 连接成功
- [ ] `curl http://127.0.0.1:8080/health` 返回统一 JSON
- [ ] 响应格式包含 `code`、`message`、`data`
- [ ] 代码可以通过 `go mod tidy`

## 今日复盘模板

完成后在 [06-27.md](/Users/leo/GoWorkSpace/mall/mall-backend/docs/daily-plans/06-27.md) 里记录。

建议记录：

```text
今天学会的 Go 知识：
- 

今天完成的接口：
- 

今天遇到的问题：
- 

明天优先处理：
- 
```

## 明天预告

明天会进入商品系统：

- `merchants`
- `categories`
- `products`
- `product_skus`
- 商品分类接口
- 商品列表接口
- 商品详情接口
- 商品详情 Redis 缓存

所以今天必须保证：

- 项目能启动
- MySQL 能连
- Redis 能连
- 基础目录结构清楚

