# M8-D 交易支付与结算工程验收记录

> 验收日期：2026-07-16
>
> 范围：`mall-backend`、`mall-pc-web`、`mall-admin`、`mall-uniapp`
> 结论：工程实现与隔离环境验收通过；生产迁移、真实支付宝资金、结算打款和小程序真机不在本结论内。

## 1. 本次完成范围

- 一张交易对应一张聚合支付单，按子订单生成不可变支付分配。
- 支付回调和主动同步原子更新支付单、交易及全部子订单。
- 子订单售后退款绑定原支付分配，限制累计可退金额并维护交易退款状态。
- 商户费率、订单佣金和应结算金额在下单时固化快照。
- 销售、佣金、退款和佣金退回写入不可变结算账本。
- 按商户和周期生成幂等结算单，支持历史成功退款及缺失流水修复。
- 商家店主/管理员只读查询结算单、明细和账本。
- PC Web 与 UniApp 使用 `trade_id` 发起聚合支付；旧单商户订单入口继续兼容。

本次明确不包含：对象存储真实联调、支付宝真实付款/退款、微信支付、结算确认和实际打款、平台后台、小程序真机。

## 2. 数据库与兼容性

新增并验收：

- `000011_make_payment_order_reference_nullable`：交易支付可不再写旧 `order_id/order_no/merchant_id`；SQL 会先检查旧字段是否存在。
- `000012_add_merchant_commission_rate`：增加商户费率和订单佣金、应结算金额快照。
- GORM 模型对 M8 版本化字段使用 `-:migration`，避免 `AutoMigrate` 越权创建字段或索引。
- 历史回填继续把无可靠费率快照的旧订单按 `0 bps` 处理，不倒推修改历史佣金。
- 新调用旧 `POST /orders` 时，在同一事务内创建单订单交易并写入当前费率快照；旧 `order_id` 支付只映射单订单交易，多商户子订单仍要求 `trade_id`。

## 3. 核心不变量

```text
payment.amount = trade.payable_amount
sum(payment_allocations.amount) = payment.amount
payment_allocation.amount = child_order.payable_amount
allocation.refunded_amount <= allocation.amount
order.commission_amount + order.settlement_amount = order.payable_amount
settlement.net = gross - commission - refund + adjustment
```

- 同一交易最多存在一个有效待支付意图。
- 支付分配创建后不修改原始金额，只累计 `refunded_amount`。
- 退款只能引用原交易、原支付单、原子订单和原支付分配。
- 退款成功通过新账本流水冲正，不覆盖销售或佣金原流水。
- 结算单只归集达到可结算时间且尚未归集的流水；同商户同周期重复生成幂等。

## 4. 接口与命令

买家侧：

```http
POST /api/v1/payments              # order_id 或 trade_id 二选一
GET  /api/v1/payments/{payment_no}
POST /api/v1/payments/{payment_no}/sync
POST /api/v1/payments/alipay/notify
```

商家只读结算：

```http
GET /api/v1/merchant/settlement-entries
GET /api/v1/merchant/settlements
GET /api/v1/merchant/settlements/{id}
```

周期结算命令：

```bash
./bin/go-mall-generate-settlement \
  -config config.yaml \
  -merchant-id 1 \
  -period-start 2026-07-01T00:00:00+08:00 \
  -period-end 2026-08-01T00:00:00+08:00
```

命令会先循环补记所有已完成订单和历史成功退款的遗漏账本，再生成结算单。当前不安装自动结算 timer，首次生产运行必须人工核对。

## 5. 自动化结果

后端：

- `go test ./...`：通过。
- `go vet ./...`：通过。
- MySQL 8.4 migration 000001-000012 全量升级、全量降级和再次升级：通过。
- 历史交易/支付/退款回填及重复回填：通过。
- 100 次同一幂等提交只生成一张交易：通过。
- 100 个用户竞争 10 件库存，无超卖：通过。
- 50 次并发创建支付意图，只保留一张支付单和两条分配：通过。
- 两个商户分别退款、佣金按比例冲正、历史退款流水修复及周期结算：通过。

商家管理端：

- TypeScript 类型检查、3 条组件单测、生产构建：通过。
- Playwright 4 条流程：商品、库存/发货、销售权限、店主结算单/账本/明细全部通过。
- `sales/warehouse/operator` 不显示结算导航，直接接口仍由后端 RBAC 拒绝。

PC Web：

- TypeScript 类型检查、4 条单测、生产构建：通过。
- Playwright 跨商户结算流程验证支付请求体为 `trade_id`，通过。
- 构建存在约 505 kB 主包警告，不影响功能，后续通过路由懒加载处理。

UniApp：

- 48 个接口契约、订单最新预览令牌和 19 个页面路由：通过。
- Chromium、WebKit、微信 H5 UA 共 12 条 E2E：通过。
- H5 和微信小程序生产编译：通过。
- H5 使用交易级支付宝支付；小程序在微信支付未实现前不展示支付操作。

## 6. 生产前剩余门禁

1. 在生产备份恢复出的 MySQL 8.4 副本执行 migration、历史回填、校验和结算 dry-run，保存 checksum、行数、耗时和对账结果。
2. 使用隔离账号和隔离 Redis 前缀执行 HTTP 写验收，确认部署二进制与数据库版本一致。
3. 配置 `settlement.enabled: true` 前核对费率生效时间、观察期和商户结算周期。
4. 真实支付宝付款、异步通知、主动查询及退款仍按独立资金门禁执行。
5. 结算确认、实际打款、平台复核和审计由 M9 或合规分账产品实现。
6. H5 正式部署、小程序 AppID/合法域名和真机流程仍需补证。

任何一项生产门禁未完成，都只能宣称“工程验收通过”，不能宣称多商户资金链路已经上线。
