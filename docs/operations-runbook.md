# go-mall 运行、监控与回滚手册

本文用于 M6 发布门禁和线上值守。对象存储、支付宝真实支付与真实退款仍按独立验收计划执行，不能由本手册中的 Mock 测试替代。

## 1. 生产配置门禁

发布前在服务器运行：

```bash
cd /opt/mall/backend
GIN_MODE=release ./bin/go-mall-check-config
```

生产配置至少满足：

- `server.mode: release`，`log.format: json`。
- `app.auto_migrate`、`app.seed_data`、`payment.mock_enabled` 和不安全微信登录均为 `false`。
- M8 首次迁移和结算 dry-run 完成前保持 `settlement.enabled: false`；开启前确认 `hold_days` 和 `batch_size`。
- 买家与商家 JWT 密钥不同、长度至少 32 字节，且不是示例值。
- `server.trusted_proxies` 只包含实际反向代理；同域部署时 CORS 可为空，跨域时只填写精确 HTTPS Origin。
- 支付宝生产环境不得使用沙箱网关；真实密钥只在服务器保存。

## 2. 发布门禁

普通推送不执行部署 job。只有提交信息包含 `[deploy]` 或在 GitHub Actions 手动运行时，workflow 才会执行：

```text
go test ./...
go test -race ./internal/pricing ./internal/middleware ./pkg/jwt
go vet ./...
govulncheck ./...
Linux 二进制构建
服务器配置预检
旧二进制备份
数据库迁移
systemd 定时器与服务重启
发布后 smoke test
```

常规 workflow 只自动执行既有 GORM 基线迁移。M8 版本化 schema 和历史回填命令会被打入发布包，但不会自动执行，必须按第 3 节前的 M8 操作门禁人工完成。

验收脚本会修改数据，必须使用隔离测试库、Redis DB 或统一测试前缀，并显式设置：

```bash
export ALLOW_TEST_MUTATIONS=1
```

买家、商家、售后、支付和 RBAC 脚本分别位于 `scripts/verify-*.sh`。支付脚本还要求启用 Mock，禁止在生产环境运行。

### M8 版本化迁移与历史回填

禁止直接在生产主库首次执行。先把同一份生产备份恢复到 MySQL 8.4 副本，并确认应用写流量没有连接该副本。推荐顺序：

```bash
cd /opt/mall/backend

./bin/go-mall-schema-migrate -command status
./bin/go-mall-schema-migrate -command verify

MALL_ALLOW_SCHEMA_MIGRATION=1 \
  ./bin/go-mall-schema-migrate -command up

./bin/go-mall-backfill-trades -command source

MALL_ALLOW_M8_BACKFILL=1 \
  ./bin/go-mall-backfill-trades -command backfill -batch-size 100

./bin/go-mall-backfill-trades -command verify
./bin/go-mall-schema-migrate -command verify
```

执行前记录：备份时间、数据库版本、行数、所有 migration checksum、DDL 耗时和回填批次数。`source` 预检必须全部为 `ok`；回填后的 `verify` 必须全部为 `ok`，并核对：

- 每张历史订单都有一张 `LEGACY...` 单商户交易，交易金额等于订单金额。
- 每张历史支付都有一条支付分配，分配金额等于支付金额和订单实付。
- 历史成功退款累计值等于分配的 `refunded_amount`，且不超过分配金额。
- 待支付单的 `active_trade_id` 与 `trade_id` 一致；终态支付不保留有效键。
- 没有缺失商户、孤儿订单/支付/退款、跨用户或跨商户引用。

历史数据没有可靠的佣金配置快照，M8 回填固定使用 `commission_rate_bps=0`、`commission_amount=0`、`settlement_amount=payable_amount`。启用真实佣金前必须另行形成从生效时间开始的商户费率配置，不能倒推修改历史账。

MySQL DDL 会隐式提交。运行器在 DDL 前写入 `dirty=true`；如果失败，不要直接删除 `schema_migrations` 记录或重跑，先依据实际 schema 和 SQL 人工核对。已经执行的 migration 文件不可修改，否则 checksum 校验会拒绝启动操作。

`MALL_ALLOW_SCHEMA_ROLLBACK=1 ./bin/go-mall-schema-migrate -command down` 仅用于副本或临时库演练。生产完成回填后，不执行 schema down；旧版本代码可以忽略新增可空字段和新表，先回退二进制、停止新交易写入并对账。

### M8-D 结算 dry-run 与人工生成

`go-mall-generate-settlement` 已包含在发布包中，但 workflow 不会自动运行，也不会安装自动结算 timer。先在生产备份副本中把 `settlement.enabled` 临时设为 `true`，使用明确的左闭右开周期执行：

```bash
./bin/go-mall-generate-settlement \
  -config config.yaml \
  -merchant-id 1 \
  -period-start 2026-07-01T00:00:00+08:00 \
  -period-end 2026-08-01T00:00:00+08:00
```

命令先循环补记所有缺失的销售、佣金、历史成功退款和佣金退回流水，再生成结算单。验收输出和数据库时核对：

- `gross - commission - refund + adjustment = net`。
- 每条结算流水只归属一张结算单，原流水金额未被覆盖。
- 已完成订单的销售/佣金流水齐全，成功退款的退款/佣金退回流水齐全。
- 相同商户、开始时间和结束时间重复执行返回同一张结算单，不重复归集。
- 观察期内订单或退款不提前进入结算单。

生产首次运行必须按商户逐个执行并保存输出。M8-D 只生成待确认结算单，不做确认或实际打款；不得通过手工 SQL 修改为已打款。

## 3. 发布后烟雾检查

```bash
BASE_URL=https://mall.leedu.ac.cn CHECK_METRICS=false ./scripts/post-deploy-smoke.sh

sudo systemctl is-active go-mall
sudo systemctl list-timers --all | grep go-mall
sudo journalctl -u go-mall -n 100 --no-pager
sudo journalctl -u go-mall-reconcile-refunds.service -n 100 --no-pager
```

公网 Nginx 不应代理 `/metrics`。Prometheus 在服务器内网或本机采集：

```yaml
scrape_configs:
  - job_name: go-mall
    static_configs:
      - targets: ["127.0.0.1:8080"]
```

检查关键指标：

```bash
curl -fsS http://127.0.0.1:8080/metrics | grep '^go_mall_'
```

告警规则模板位于 `deploy/monitoring/prometheus-rules.yml`。其中 Nginx、MySQL、Redis、systemd、磁盘和证书告警分别依赖对应 exporter、Node Exporter 与 Blackbox Exporter；部署后必须在 Prometheus 中执行 `promtool check rules`。

## 4. 日志和 request_id

后端每个响应都返回 `X-Request-ID`。客户端报错时先记录该值，再查询 JSON 日志：

```bash
REQUEST_ID=00000000-0000-0000-0000-000000000000
sudo journalctl -u go-mall --since "30 minutes ago" -o cat \
  | jq -c --arg id "$REQUEST_ID" 'select(.request_id == $id)'
```

日志允许记录用户、商户、订单、支付单、售后单和退款单标识，不记录密码、Authorization、Refresh Token 或密钥。

服务使用 journald。先查看当前限制：

```bash
journalctl --disk-usage
systemd-analyze cat-config systemd/journald.conf
```

在 `/etc/systemd/journald.conf.d/99-go-mall.conf` 配置服务器级轮转上限，例如：

```ini
[Journal]
SystemMaxUse=1G
RuntimeMaxUse=256M
MaxRetentionSec=14day
Compress=yes
```

修改后运行 `sudo systemctl restart systemd-journald`，并确认 Node Exporter 的磁盘告警已生效。该配置影响服务器全部 journald 日志，应按实际磁盘容量调整。

## 5. 性能基线

仅在固定测试服务器和固定数据集运行：

```bash
BASE_URL=https://test-mall.example.com/api/v1 \
TEST_PRODUCT_ID=1 \
./scripts/benchmark-api.sh
```

订单压测必须为每个请求准备不同幂等令牌，并使用隔离测试数据：

```bash
AUTH_TOKEN=... \
ORDER_BODIES_DIR=/tmp/go-mall-order-bodies \
./scripts/benchmark-api.sh
```

记录 CPU、内存、MySQL/Redis 位置、商品/订单数据量、并发数、总请求数、P50/P95/P99、5xx 和测试提交。门槛为商品读接口 P95 < 500ms、普通接口 P95 < 800ms、创建订单 P95 < 1.5s；50 并发读无 5xx，10 并发下单不超卖且不重复。

## 6. 故障处理

### 服务或 5xx 告警

```bash
sudo systemctl status go-mall --no-pager
sudo journalctl -u go-mall --since "15 minutes ago" --no-pager
sudo nginx -t
sudo ss -ltnp | grep ':8080'
```

同时检查 MySQL、Redis、Nginx 和磁盘。不要在未确认原因时连续重启，避免掩盖崩溃日志。

### 支付回调失败

按 `payment_no` 和 `request_id` 查日志，核对签名、`app_id`、订单号、金额和交易状态。返回页不是支付成功依据；最终状态只来自验签后的异步通知或主动查询。

### 未知退款积压

```bash
sudo systemctl status go-mall-reconcile-refunds.timer --no-pager
sudo journalctl -u go-mall-reconcile-refunds.service -n 100 --no-pager
```

不要人工把未知状态直接改成成功。恢复渠道查询后复用原 `refund_no` 对账。

### 结算生成失败或金额不平

停止继续生成后续商户周期，保存命令输出和数据库快照。先检查订单佣金快照、支付分配、成功退款累计和账本唯一键；不要删除账本流水、修改原流水金额或手工重绑结算单。修复应使用幂等补记或新增冲正流水，并在隔离副本重跑相同周期确认结果。

## 7. 回滚

workflow 在 `/opt/mall/backend/rollback/<commit-sha>/bin` 保留最近 5 个版本。发布失败会自动恢复上一套二进制；人工回滚示例：

```bash
cd /opt/mall/backend
sudo systemctl stop go-mall
sudo cp -a rollback/<previous-sha>/bin/. bin/
GIN_MODE=release ./bin/go-mall-check-config
sudo systemctl start go-mall
BASE_URL=http://127.0.0.1:8080 CHECK_METRICS=true ./scripts/post-deploy-smoke.sh
```

数据库迁移必须向前兼容，不能依赖自动回滚表结构。若已发生支付、库存或退款数据差异，应先停止相关写操作并对账，不能只回滚二进制。
