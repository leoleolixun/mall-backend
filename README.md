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
build server/migrate
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
