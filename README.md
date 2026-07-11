# go-mall backend

Go 单商户商城后端，当前用于学习和实现小程序/PC 商城 MVP。项目采用分层结构，已包含商品、用户、地址、购物车、订单、支付等基础模块，并接入支付宝 PC 网页支付。

## 技术栈

- Go 1.25+
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

安装依赖并检查：

```bash
go mod tidy
go test ./...
go vet ./...
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
| `owner` / `admin` | 全部商家后台权限 |
| `operator` | 经营概览、订单、发货、商品、库存和上传 |
| `sales` | 经营概览、订单查看、商品只读 |
| `warehouse` | 订单查看、发货、商品只读、库存查看与调整 |

登录、刷新和 `/api/v1/merchant/me` 会返回 `permissions`。前端使用它控制菜单和按钮；后端权限中间件会对越权请求返回 HTTP `403`。

已实现接口：

```text
POST /api/v1/merchant/auth/login
POST /api/v1/merchant/auth/refresh
POST /api/v1/merchant/auth/logout
GET  /api/v1/merchant/me
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

当前 MVP 的买家订单流程停在“已发货”，暂不开放物流轨迹查询和确认收货接口。商家订单详情仍会返回配送类型、发货时间以及普通快递的公司和运单号，方便后台管理。

## 支付流程

创建支付单：

```http
POST /api/v1/payments
Authorization: Bearer <access_token>
Content-Type: application/json
```

```json
{
  "order_id": 1,
  "pay_channel": "alipay",
  "pay_scene": "page"
}
```

响应中的 `data.pay_params.pay_url` 是支付宝 PC 网页支付跳转地址。

支付宝异步通知接口：

```text
POST /api/v1/payments/alipay/notify
```

该接口不使用 JWT。后端会通过支付宝签名验签、`app_id`、支付单号和金额校验后，在事务中更新支付单与订单状态。

支付完成页可以主动同步支付宝状态，避免只依赖异步通知：

```http
POST /api/v1/payments/{payment_no}/sync
Authorization: Bearer <access_token>
```

只有当前用户自己的支付宝支付单可以同步。后端会调用 `alipay.trade.query`，校验支付单号和金额后更新支付单与订单。

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

## 数据库迁移

本项目使用 GORM AutoMigrate。生产部署建议显式执行迁移命令：

```bash
go run ./cmd/migrate
```

CI/CD 中会构建 `go-mall-migrate`，部署时执行：

```bash
./bin/go-mall-migrate
```

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
go test
go vet
build server/migrate/create-merchant-account/cancel-expired-orders
上传服务器
执行数据库迁移
重启 systemd 服务
检查 /health
```

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
