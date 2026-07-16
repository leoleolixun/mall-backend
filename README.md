# go-mall backend

Go 模块化商城后端，当前用于学习和实现 PC/H5/小程序商城。项目采用分层结构，已包含多商户目录、购物车、交易拆单、交易级支付分配、订单履约、售后退款和商户结算账本，并接入支付宝 PC/H5 支付能力。

## 技术栈

- Go 1.25.12+
- Gin
- GORM
- MySQL
- Redis
- JWT
- Swagger / OpenAPI
- GitHub Actions
- Alipay SDK
- Tencent COS / Qiniu Kodo SDK

## 目录结构

```text
cmd/
  server/      # HTTP 服务入口
  migrate/     # 数据库迁移入口
internal/
  bootstrap/   # MySQL、Redis、迁移、种子数据
  config/      # 配置结构
  dto/         # 请求和响应 DTO
  handler/     # HTTP Handler
  middleware/  # 中间件
  model/       # GORM Model
  repository/  # 数据访问层
  router/      # 路由注册
  service/     # 业务逻辑层
  storage/     # 腾讯云 COS、七牛 Kodo 对象存储适配
pkg/
  jwt/
  logger/
  password/
  response/
docs/
  openapi.yaml # OpenAPI 文档
```

## 本地启动

复制示例配置：

```bash
cp config.example.yaml config.yaml
```

修改 `config.yaml` 中的 MySQL、Redis、JWT、支付宝等配置。

生产环境必须保持以下开发开关为 `false`：

```yaml
auth:
  unsafe_wechat_open_id_login_enabled: false
payment:
  mock_enabled: false
```

当前微信 `open_id` 直登只用于本地联调，`release` 模式不会注册该路由。正式小程序登录需要由前端提交 `wx.login` 返回的 `code`，后端通过微信 `code2session` 换取 `openid`。

服务启动时会校验生产配置：买家和商家 JWT 密钥必须不同、长度至少 32 字节，且不能使用 `change_me` 等示例值。`release` 模式还会拒绝自动迁移、种子数据、模拟支付、不安全微信登录以及支付宝沙箱配置。服务器应设置：

```yaml
server:
  mode: release
app:
  auto_migrate: false
  seed_data: false
```

环境变量 `GIN_MODE` 会覆盖 `server.mode`；当前 systemd 使用 `GIN_MODE=release` 时同样会触发上述校验。

只检查配置而不连接 MySQL、Redis，可以运行：

```bash
GIN_MODE=release go run ./cmd/check-config
```

部署 workflow 会先在服务器执行同样的检查，校验通过后才替换正式二进制并执行数据库迁移。

安装依赖并检查：

```bash
go mod tidy
go test ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.6.0 ./...
```

执行数据库迁移：

```bash
go run ./cmd/migrate
```

启动服务：

```bash
go run ./cmd/server
```

健康检查：

```bash
curl http://127.0.0.1:8080/health
```

Swagger：

```text
http://127.0.0.1:8080/swagger/index.html
```

OpenAPI：

```text
http://127.0.0.1:8080/docs/openapi.yaml
```

## 配置说明

真实配置文件 `config.yaml` 不提交到 Git。仓库只保留 `config.example.yaml`。

敏感文件也不提交：

```text
config.yaml
internal/config/alipay/**
```

支付宝普通公钥模式需要：

```yaml
payment:
  alipay:
    enabled: true
    sandbox: false
    app_id: "your_alipay_app_id"
    app_private_key_path: "internal/config/alipay/app_private_key.pem"
    alipay_public_key_path: "internal/config/alipay/alipay_public_key.pem"
    notify_url: "https://your-domain.com/api/v1/payments/alipay/notify"
    page:
      enabled: true
      return_url: "https://your-domain.com/payment/result"
      product_code: "FAST_INSTANT_TRADE_PAY"
      timeout_express: "15m"
```

`app_private_key.pem` 是应用私钥，自己生成并保存；`alipay_public_key.pem` 是支付宝公钥，用于验签。

## 图片上传

统一图片上传接口：

```http
POST /api/v1/uploads
Authorization: Bearer <access_token>
Content-Type: multipart/form-data
```

表单字段：`file` 为图片文件，`scene` 为可选业务场景，例如 `avatar`、`product`、`after_sale`。接口默认限制 10 MB，仅接受 JPEG、PNG、WebP 和 GIF，并返回 `url`、`key`、`content_type`、`size` 和 `provider`。

在 `config.yaml` 的 `storage.provider` 中选择 `cos` 或 `qiniu`，然后填写 [config.example.yaml](config.example.yaml) 中对应配置。真实 SecretKey 只保存在服务器配置中，不提交到 Git。上传头像时，先调用上传接口取得 `data.url`，再将该 URL 传给 `PUT /api/v1/me` 的 `avatar` 字段。

## 商家后台认证

买家账号与商家账号使用不同的数据表、JWT 密钥和 Redis Refresh Token 命名空间。启用商家后台前，先在 `config.yaml` 设置：

```yaml
jwt:
  merchant_access_secret: replace_with_a_long_random_secret
  merchant_access_ttl_minutes: 120
  merchant_refresh_ttl_hours: 168
```

执行迁移后，通过命令行创建第一个商家账号。密码从环境变量读取，不作为命令参数传递：

```bash
read -s MERCHANT_ACCOUNT_PASSWORD
export MERCHANT_ACCOUNT_PASSWORD
./bin/go-mall-create-merchant-account \
  -merchant-id 1 \
  -username merchant_admin \
  -nickname 店铺管理员 \
  -role owner
unset MERCHANT_ACCOUNT_PASSWORD
```

本地开发可以把命令替换为：

```bash
go run ./cmd/create-merchant-account -merchant-id 1 -username merchant_admin -nickname 店铺管理员 -role owner
```

角色支持 `owner`、`admin`、`operator`、`sales` 和 `warehouse`。例如创建销售与库管账号：

```bash
read -s MERCHANT_ACCOUNT_PASSWORD
export MERCHANT_ACCOUNT_PASSWORD
go run ./cmd/create-merchant-account -merchant-id 1 -username sales01 -nickname 销售一组 -role sales
go run ./cmd/create-merchant-account -merchant-id 1 -username warehouse01 -nickname 一号仓库 -role warehouse
unset MERCHANT_ACCOUNT_PASSWORD
```

权限矩阵：

| 角色 | 主要权限 |
| --- | --- |
| `owner` | 全部商家后台权限，可查看本商户结算，可管理所有员工角色 |
| `admin` | 全部业务权限和本商户结算只读权限，可管理运营、销售和库管账号，不能管理店主或其他管理员 |
| `operator` | 经营概览、订单、发货、商品、库存、上传和顾客查看 |
| `sales` | 经营概览、订单查看、商品只读和顾客查看 |
| `warehouse` | 订单查看、发货、商品只读、库存查看与调整 |

登录、刷新和 `/api/v1/merchant/me` 会返回 `permissions`。前端使用它控制菜单和按钮；后端权限中间件会对越权请求返回 HTTP `403`。

员工账号支持后台创建、编辑、启停和密码重置。账号被停用、角色被调整或密码被重置后，旧访问令牌和刷新令牌会立即失效；系统会阻止停用当前登录账号以及移除最后一个启用的店主账号。

已实现接口：

```text
POST /api/v1/merchant/auth/login
POST /api/v1/merchant/auth/refresh
POST /api/v1/merchant/auth/logout
GET  /api/v1/merchant/me
GET  /api/v1/merchant/accounts
POST /api/v1/merchant/accounts
PUT  /api/v1/merchant/accounts/{id}
PUT  /api/v1/merchant/accounts/{id}/password
GET  /api/v1/merchant/roles
GET  /api/v1/merchant/customers/overview
GET  /api/v1/merchant/customers
GET  /api/v1/merchant/customers/{id}
POST /api/v1/merchant/uploads
GET  /api/v1/merchant/orders
GET  /api/v1/merchant/orders/{id}
POST /api/v1/merchant/orders/{id}/ship
GET  /api/v1/merchant/categories
POST /api/v1/merchant/categories
PUT  /api/v1/merchant/categories/{id}
DELETE /api/v1/merchant/categories/{id}
GET  /api/v1/merchant/products
POST /api/v1/merchant/products
GET  /api/v1/merchant/products/{id}
PUT  /api/v1/merchant/products/{id}
DELETE /api/v1/merchant/products/{id}
POST /api/v1/merchant/products/{id}/on-sale
POST /api/v1/merchant/products/{id}/off-sale
POST /api/v1/merchant/products/{id}/skus
PUT  /api/v1/merchant/products/{id}/skus/{sku_id}
DELETE /api/v1/merchant/products/{id}/skus/{sku_id}
GET  /api/v1/merchant/inventory-logs
GET  /api/v1/merchant/inventory-alerts
PUT  /api/v1/merchant/inventory/skus/{sku_id}/stock
GET  /api/v1/merchant/dashboard/overview
GET  /api/v1/merchant/dashboard/analytics
GET  /api/v1/merchant/settlement-entries
GET  /api/v1/merchant/settlements
GET  /api/v1/merchant/settlements/{id}
```

商品创建后默认为草稿。至少创建一个价格大于 0 的启用 SKU 后才能上架；上架商品不能删除或禁用最后一个可售 SKU。分类、商品和 SKU 都使用软删除，删除商品前必须先下架。

库存变更记录覆盖下单扣减、用户取消、超时取消、商家初始化 SKU 和商家调整库存。流水记录变更前后库存、有符号变更量、业务来源和操作人，并与库存更新处于同一数据库事务。查询示例：

```http
GET /api/v1/merchant/inventory-logs?page=1&page_size=10&sku_id=1&change_type=merchant_adjustment
Authorization: Bearer <merchant_access_token>
```

每个 SKU 可以配置 `low_stock_threshold`。值为 `0` 时关闭预警；值大于 `0` 且当前库存小于等于阈值时，会出现在预警列表中：

```http
GET /api/v1/merchant/inventory-alerts?page=1&page_size=10&keyword=iPhone
Authorization: Bearer <merchant_access_token>
```

库存为 `0` 的记录返回 `severity: out_of_stock`，其他命中阈值的记录返回 `severity: low_stock`。

库管使用独立库存调整接口，不能通过该接口修改 SKU 价格、名称或状态：

```http
PUT /api/v1/merchant/inventory/skus/1/stock
Authorization: Bearer <merchant_access_token>
Content-Type: application/json

{"stock":20,"low_stock_threshold":5,"remark":"仓库盘点调整"}
```

商家后台经营概览一次返回商品、库存、订单和成交统计。金额字段单位均为分：

```http
GET /api/v1/merchant/dashboard/overview
Authorization: Bearer <merchant_access_token>
```

销售分析接口返回连续日期的成交趋势和热销商品排行，默认统计包含今天在内的近 7 日：

```http
GET /api/v1/merchant/dashboard/analytics?days=7&top_limit=10
Authorization: Bearer <merchant_access_token>
```

`days` 支持 1 到 30，`top_limit` 支持 1 到 20。销量使用订单商品数量汇总，销售额使用订单商品小计汇总。

商家接口可重复验收脚本会创建临时分类、商品和 SKU，验证完整状态流转后软删除这些业务数据。库存流水属于不可删除的审计记录，因此验收环境会保留对应流水，不要在生产环境频繁执行：

```bash
export BASE_URL=http://127.0.0.1:8080/api/v1
export MERCHANT_USERNAME=merchant_admin
read -s MERCHANT_PASSWORD
export MERCHANT_PASSWORD
./scripts/verify-merchant-api.sh
unset MERCHANT_PASSWORD
```

发货请求示例：

普通快递：

```json
{
  "delivery_type": "express",
  "logistics_company": "顺丰速运",
  "tracking_no": "SF1234567890"
}
```

商家自行配送：

```json
{
  "delivery_type": "self_delivery"
}
```

买家可以查询订单的基础物流信息，并主动确认收货。普通快递返回物流公司和运单号，商家自行配送返回配送类型和发货时间；当前不接入第三方实时快递轨迹。

## 支付流程

创建支付单：

```http
POST /api/v1/payments
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "trade_id": 1001,
  "pay_channel": "alipay",
  "pay_scene": "page"
}
```

`order_id` 和 `trade_id` 必须且只能填写一个。新结算流程统一使用 `trade_id`，后端为整张交易创建一张支付单，并为每张商户子订单创建不可变支付分配；`order_id` 只兼容历史单订单交易。响应中的 `data.pay_params.pay_url` 是支付宝 PC 网页支付跳转地址，H5 使用 `pay_scene: wap`。

支付宝异步通知接口：

```text
POST /api/v1/payments/alipay/notify
```

该接口不使用 JWT。后端会通过支付宝签名验签、`app_id`、支付单号和金额校验后，在事务中更新支付单、交易和全部子订单状态。

支付完成页可以主动同步支付宝状态，避免只依赖异步通知：

```http
POST /api/v1/payments/{payment_no}/sync
Authorization: Bearer <access_token>
```

只有当前用户自己的支付宝支付单可以同步。后端会调用 `alipay.trade.query`，校验支付单号和金额后更新支付单、交易与子订单。

## 超时订单取消

配置默认关闭超时取消，首次部署后先确认支付宝密钥和任务日志正确：

```yaml
order:
  cancel_expired_enabled: false
  pending_payment_timeout_minutes: 15
  cancel_batch_size: 100
```

启用前可以手动执行验证：

```bash
./bin/go-mall-cancel-expired-orders
```

任务处理顺序：主动查询支付宝状态、关闭未支付支付宝交易、锁定订单、恢复库存、关闭本地支付单、取消订单。支付宝查询或关闭失败时会保留订单，不会冒险取消。

部署 workflow 会安装并启动 `go-mall-cancel-expired-orders.timer`，每分钟执行一次。配置保持 `cancel_expired_enabled: false` 时任务只输出禁用提示，不修改数据。确认后改为 `true`：

```bash
sudo systemctl list-timers go-mall-cancel-expired-orders.timer
sudo journalctl -u go-mall-cancel-expired-orders.service -n 100 --no-pager
```

## 发货订单自动完成

买家可以调用 `POST /api/v1/orders/:id/confirm` 主动确认收货。未主动确认的订单可由独立任务按发货时间自动完成：

```yaml
order:
  auto_complete_enabled: false
  shipped_auto_complete_days: 10
  complete_batch_size: 100
```

手动验证和查看定时任务：

```bash
./bin/go-mall-complete-shipped-orders
sudo systemctl list-timers go-mall-complete-shipped-orders.timer
sudo journalctl -u go-mall-complete-shipped-orders.service -n 100 --no-pager
```

任务只处理仍处于“已发货”的订单，并同时写入订单完成时间和物流签收时间。首次部署保持关闭，验证查询范围后再开启。

## 退款主动查询与对账

支付宝退款只有明确返回资金已发生变化时才标记成功。网络超时、限流或渠道状态不明确时，退款单保持“退款结果确认中”，不会直接标记失败。后续查询和重试始终复用原 `refund_no` 作为支付宝 `out_request_no`，避免重复退款。

首次部署保持自动对账关闭：

```yaml
payment:
  refund:
    reconcile_enabled: false
    retry_interval_minutes: 5
    reconcile_batch_size: 100
```

商家可以手动同步单笔退款：

```http
POST /api/v1/merchant/after-sales/:id/refund/sync
Authorization: Bearer <merchant_access_token>
```

也可以在服务器手动运行批量对账：

```bash
./bin/go-mall-reconcile-refunds
```

部署 workflow 会安装 `go-mall-reconcile-refunds.timer`，每五分钟检查一次到期的待确认退款。确认支付宝退款权限和日志正常后，再把 `reconcile_enabled` 改为 `true`：

```bash
sudo systemctl list-timers go-mall-reconcile-refunds.timer
sudo journalctl -u go-mall-reconcile-refunds.service -n 100 --no-pager
```

## 数据库迁移

M0-M7 的既有模型仍由 GORM AutoMigrate 维护：

```bash
go run ./cmd/migrate
```

CI/CD 中会构建 `go-mall-migrate`，部署时执行：

```bash
./bin/go-mall-migrate
```

从 M8 开始，交易、支付分配和结算结构使用带 checksum 的版本化 SQL，不允许继续交给 AutoMigrate。查看和校验迁移历史：

```bash
./bin/go-mall-schema-migrate -command status
./bin/go-mall-schema-migrate -command verify
```

只允许先在备份恢复出的同版本副本执行。确认备份、DDL 时间和回滚策略后，显式开启 schema 写入：

```bash
MALL_ALLOW_SCHEMA_MIGRATION=1 \
  ./bin/go-mall-schema-migrate -command up

./bin/go-mall-backfill-trades -command source

MALL_ALLOW_M8_BACKFILL=1 \
  ./bin/go-mall-backfill-trades -command backfill -batch-size 100

./bin/go-mall-backfill-trades -command verify
```

迁移运行器使用 MySQL 会话锁、`dirty` 标记和 SHA-256 checksum。已执行的 SQL 文件禁止修改。`down` 只用于隔离环境演练，并且需要 `MALL_ALLOW_SCHEMA_ROLLBACK=1`；生产回填后采用向前兼容回退旧二进制，不删除新表或兼容字段。

CI/CD 会打包 `go-mall-schema-migrate`、`go-mall-backfill-trades` 和 `go-mall-generate-settlement`，但不会自动执行 M8 schema、历史回填或周期结算。操作顺序、核对项和故障处理见 [运行、监控与回滚手册](docs/operations-runbook.md)。

M8-D 已实现一张交易对应一张支付单、按子订单生成不可变支付分配、退款冲正和商户结算账本。首次结算必须人工执行，并先在生产备份副本 dry-run：

```bash
./bin/go-mall-generate-settlement \
  -config config.yaml \
  -merchant-id 1 \
  -period-start 2026-07-01T00:00:00+08:00 \
  -period-end 2026-08-01T00:00:00+08:00
```

只有 `settlement.enabled: true` 时命令才会运行。它会先幂等补记已完成订单、历史成功退款和遗漏佣金流水，再生成周期结算单；当前不会自动确认或实际打款。工程验收证据见 [M8-D 交易支付与结算验收记录](docs/m8-d-payment-settlement-acceptance-2026-07-16.md)。

## CI/CD

后端部署 workflow：

```text
.github/workflows/deploy-backend.yml
```

只有以下两种情况会部署到服务器：

```text
1. commit message 包含 [deploy]
2. 在 GitHub Actions 页面手动 Run workflow
```

部署时才会执行测试、构建和发布：

```text
go test ./...
go test -race ./internal/pricing ./internal/middleware ./pkg/jwt
go vet
govulncheck（可达漏洞必须为 0）
build server/check-config/migrate/schema-migrate/backfill-trades/create-merchant-account/cancel-expired-orders/complete-shipped-orders/reconcile-refunds/generate-settlement
上传服务器
执行数据库迁移
重启 systemd 服务
检查 /health
检查 Swagger、OpenAPI、认证保护、request_id 和 /metrics
```

完整的生产配置、监控、日志轮转、性能基线、故障处理和回滚步骤见 [运行、监控与回滚手册](docs/operations-runbook.md)。Prometheus 告警规则模板位于 `deploy/monitoring/prometheus-rules.yml`。

当前只读性能基线及尚未在生产执行的订单写压测边界见 [2026-07-16 性能基线](docs/performance-baseline-2026-07-16.md)。

## 可观察性与接口安全

每个 HTTP 响应都会返回 `X-Request-ID`，结构化日志会记录同一个 `request_id`、标准化路由、状态码、耗时、用户/商户以及白名单内的订单或支付标识。日志不会记录密码、Token 或密钥。

本机指标地址：

```text
http://127.0.0.1:8080/metrics
```

指标包含请求量与 P95 所需耗时直方图、5xx、并发请求、支付回调失败、待支付订单和未知退款积压。`/metrics` 只供内网 Prometheus 采集，不应通过 Nginx 暴露到公网。

认证接口同时按客户端 IP 限流。超限返回 HTTP `429`、业务码 `42900`、`Retry-After` 和剩余额度响应头；Redis 故障时认证入口返回 `503`，避免在限流失效时放开暴力尝试。正式环境只信任配置中的反向代理，并仅允许精确配置的跨域 Origin。

触发部署示例：

```bash
git commit -m "feat(order): 增加商家发货接口 [deploy]"
git push
```

只提交代码或文档但不部署：

```bash
git commit -m "docs(readme): 更新部署说明"
git push
```

## 提交与部署约定

提交信息遵循：

```text
<type>(<scope>): <subject>
```

示例：

```bash
git commit -m "feat(order): 增加商家发货接口"
git commit -m "docs(readme): 更新部署说明"
git commit -m "chore(ci): 调整后端部署流程"
```

后续让 Codex 代为提交和推送时，默认规则：

```text
普通提交：不添加 [deploy]，只推送代码，不触发部署。
部署提交：提交信息末尾添加 [deploy]，推送后触发 GitHub Actions 部署。
```

例如：

```bash
git commit -m "feat(payment): 优化支付宝回调处理 [deploy]"
git push
```

如果只说“提交并推送”，默认不部署。

如果需要部署，明确说：

```text
提交并推送，触发部署
```

或：

```text
提交并推送，加 [deploy]
```

GitHub Secrets：

```text
SERVER_HOST
SERVER_PORT
SERVER_USER
SERVER_SSH_KEY
```

运行时配置不经过 GitHub Actions。服务器需要长期保存：

```text
/opt/mall/backend/config.yaml
/opt/mall/backend/internal/config/alipay/app_private_key.pem
/opt/mall/backend/internal/config/alipay/alipay_public_key.pem
```

CI/CD 只上传二进制和 OpenAPI 文档，不覆盖服务器上的 `config.yaml` 和支付宝密钥。第一次部署前需要手动创建这些文件：

```bash
sudo mkdir -p /opt/mall/backend/internal/config/alipay
sudo cp config.yaml /opt/mall/backend/config.yaml
sudo cp internal/config/alipay/app_private_key.pem /opt/mall/backend/internal/config/alipay/app_private_key.pem
sudo cp internal/config/alipay/alipay_public_key.pem /opt/mall/backend/internal/config/alipay/alipay_public_key.pem
sudo chmod 600 /opt/mall/backend/config.yaml /opt/mall/backend/internal/config/alipay/*.pem
```

服务器默认部署目录：

```text
/opt/mall/backend
```

systemd 服务示例：

```ini
[Unit]
Description=go-mall backend
After=network.target

[Service]
Type=simple
WorkingDirectory=/opt/mall/backend
ExecStart=/opt/mall/backend/bin/go-mall-server
Restart=always
RestartSec=3
Environment=GIN_MODE=release

[Install]
WantedBy=multi-user.target
```

## 常用命令

```bash
go test ./...
go vet ./...
go run ./cmd/migrate
go run ./cmd/server
```

构建 Linux 二进制：

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/go-mall-server ./cmd/server
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o dist/go-mall-migrate ./cmd/migrate
```
