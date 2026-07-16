# M8 多商户领域与迁移设计

> 设计基线：2026-07-16
>
> 状态：领域决策已确定，M8-A 公共目录、M8-B 版本化迁移/历史回填和 M8-C 跨商户交易已完成工程实现；M8-B/M8-C 已通过隔离 MySQL 验收
> 约束：对象存储与支付宝真实资金验收继续暂缓，但多商户资金模型不能用单订单支付的临时方案替代

## 1. 目标与边界

M8 的目标不是给更多表补 `merchant_id`，而是完成以下真实闭环：

```text
多商户商品浏览
  -> 跨商户购物车
  -> 一次预览
  -> 一张交易单 + 多张商户子订单
  -> 一次合并支付 + 逐子订单金额分配
  -> 商户独立发货和售后
  -> 退款冲减原支付分配
  -> 平台佣金和商户结算台账
```

本阶段继续不处理：

- 平台优惠券和平台补贴；只有商户券，且每张商户子订单最多使用一张。
- 自动打款给商户；先形成不可变结算台账，再接具备合规分账能力的支付产品。
- 商户入驻审批和平台运营 UI，归入 M9。
- 跨境、多币种、税费和发票。

## 2. 已核对的当前事实

代码与远程 MySQL schema 已于 2026-07-16 核对：

- `merchants` 当前只有商户 `1`，状态启用。
- `categories`、`products`、`product_skus`、`orders`、`payments`、`after_sales`、`refunds`、`coupons` 和 `user_coupons` 已有 `merchant_id`。
- 买家侧公共商户、商品、商户分类、收藏和购物车已不再固定 `merchant_id = 1`；旧 `/categories` 为兼容现有客户端仍指向默认商户，但会校验商户启用状态。
- `GET /coupons` 可按 `merchant_id` 查询启用商户的可领券；旧订单预览/创建继续使用 `defaultMerchantID = 1`，仅作为单商户兼容入口。跨商户结算统一使用 `/trades`。
- Redis 购物车为 `mall:cart:{user_id}` Hash，field 是全局唯一 `sku_id`，该结构本身可以保存跨商户 SKU。
- `orders` 已可作为商户子订单，但没有上级交易单。
- `payments` 强制 `order_id NOT NULL`，并以 `active_order_id` 保证一张订单只有一张待支付单；无法表达一次支付覆盖多张子订单。
- `refunds` 已关联 `payment_id`、`order_id` 和 `merchant_id`，但没有支付分配关联。
- 当前没有独立 SQL migration 历史，生产结构主要由 GORM AutoMigrate 形成；M8 起必须引入版本化 SQL migration。
- `order_items` 的真实物理字段仍为 `sk_uid` 和 `sk_uimage`。M8 不顺便重命名，避免把无关兼容风险混入交易迁移。

当前索引不足以支撑市场级列表：

- 商品缺少 `(merchant_id, status, deleted_at, id)` 组合索引。
- SKU 缺少 `(merchant_id, product_id, status, deleted_at)` 组合索引。
- 分类缺少 `(merchant_id, status, deleted_at, sort, id)` 组合索引。
- 订单现有商户复合索引可继续使用，但新增交易查询需要独立索引。

## 3. 已确定的业务决策

### 3.1 购物车和提交原子性

- 一个买家购物车允许同时包含多个商户的商品。
- 列表按商户分组，失效商品保留并显示原因。
- 一次结算可以选择多个商户分组。
- 任一 SKU 不存在、下架、库存不足或商户停用时，整次创建交易失败，不创建部分子订单。
- MySQL 事务成功后再清理 Redis 中本次提交的 SKU；清理失败只会残留购物车，不影响交易幂等。

### 3.2 交易单和子订单

- `trades` 表示买家一次结算和一次支付边界。
- 现有 `orders` 继续表示商户独立履约的子订单，并新增 `trade_id`。
- 同一交易中每个商户只能生成一张子订单。
- 收货地址快照写入每张子订单，后续可为不同商户扩展不同配送规则。
- 商户停用后不能产生新交易，但历史订单、发货、售后、退款和结算仍可处理。

交易状态只表达资金聚合状态：

```text
1 pending_payment
2 paid
3 closed
4 partially_refunded
5 refunded
```

履约状态继续由每张 `orders.status` 独立表达，不从子订单强行聚合成一个“已发货”状态。

### 3.3 取消规则

- 未支付的多商户交易只能整笔取消，统一恢复所有库存和商户券。
- 多商户待支付子订单调用旧 `/orders/:id/cancel` 时返回冲突错误，并提示取消交易。
- 单商户交易可继续兼容旧订单取消入口，但内部仍按交易取消处理。
- 支付成功后不能使用取消接口，必须走各子订单售后。

### 3.4 优惠规则

- M8 只支持商户券；券的 `merchant_id` 必须等于对应子订单商户。
- 每个商户子订单最多选择一个 `user_coupon_id`。
- 商户券只在本商户商品小计内分摊，不能抵扣其他商户商品或运费。
- `merchant_id = 0` 的平台券在 M8 API 中明确拒绝，等 M9 定义补贴承担方和预算台账后再开放。
- 每个订单明细继续保存 `discount_amount` 和 `payable_amount` 快照。

### 3.5 支付和金额分配

- 一张 `payments` 关联一张 `trade`，金额等于所有子订单应付金额之和。
- `payment_allocations` 为每张支付单、每张子订单保存不可变金额分配。
- 支付回调只更新交易资金状态和支付分配，不直接改变其他交易。
- 相同交易只能存在一张有效待支付单，使用可空 `active_trade_id` 唯一索引保证并发兜底。
- 新代码不能把第一张子订单 ID 填入 `payments.order_id` 伪装成合并支付。

必须满足：

```text
payment.amount = trade.payable_amount
sum(payment_allocations.amount) = payment.amount
payment_allocation.amount = order.payable_amount
trade.payable_amount = sum(orders.payable_amount)
```

支付宝或其他渠道要进入真实多商户生产环境，必须先确认平台收款、分账和商户结算的产品资质。内部台账正确不等于资金合规。

### 3.6 售后、退款和结算

- 售后仍按 `order_item` 发起，商户只能处理自己的子订单。
- `refunds` 新增 `payment_allocation_id`，退款金额只能冲减该子订单剩余可退分配金额。
- 一个子订单退款不会改变其他子订单状态或支付分配。
- 支付总状态根据累计退款金额变为部分退款或全部退款。
- 子订单完成并超过可配置售后期后才生成可结算余额。
- 结算使用不可变流水：销售入账、平台佣金、退款冲正和人工调整分别记账，不能直接覆盖余额。

## 4. 目标数据模型

### 4.1 `trades`

```text
id                    bigint PK
trade_no              varchar(64) UNIQUE
user_id               bigint NOT NULL
status                tinyint NOT NULL
goods_amount          bigint NOT NULL
freight_amount        bigint NOT NULL
discount_amount       bigint NOT NULL
payable_amount        bigint NOT NULL
idempotency_key       varchar(64) NOT NULL
paid_at               datetime NULL
closed_at             datetime NULL
created_at/updated_at datetime

UNIQUE (user_id, idempotency_key)
INDEX (user_id, status, created_at, id)
INDEX (status, created_at, id)
```

数据库唯一键是最终幂等兜底；Redis 预览令牌只负责短期校验，不能作为唯一防重机制。

### 4.2 `orders` 增量字段

```text
trade_id                 bigint NULL -> 回填后 NOT NULL
merchant_name            varchar(100) NOT NULL  商户名称快照
commission_rate_bps      int NOT NULL            佣金万分比快照
commission_amount        bigint NOT NULL
settlement_amount        bigint NOT NULL
```

增加：

```text
UNIQUE (trade_id, merchant_id)
INDEX (trade_id, status, id)
```

### 4.3 `payments` 兼容演进

第一阶段只增加：

```text
trade_id         bigint NULL
active_trade_id  bigint NULL UNIQUE
```

回填完成并切换所有读写后：

- 将历史每张订单补为一张单商户交易。
- 将 `payments.trade_id` 回填为订单对应交易。
- 为每张历史支付创建一条 `payment_allocations`。
- 新代码只使用 `trade_id` 和 `active_trade_id`。
- `order_id`、`order_no`、`active_order_id` 保留一个兼容版本后才允许改为可空并停止写入，不能同一版本直接删除。

### 4.4 `payment_allocations`

```text
id                    bigint PK
payment_id            bigint NOT NULL
trade_id              bigint NOT NULL
order_id              bigint NOT NULL
merchant_id           bigint NOT NULL
amount                bigint NOT NULL
refunded_amount       bigint NOT NULL DEFAULT 0
created_at/updated_at datetime

UNIQUE (payment_id, order_id)
INDEX (order_id)
INDEX (merchant_id, created_at, id)
```

`refunded_amount` 只允许在退款结果确定成功后原子递增，并校验不超过 `amount`。

### 4.5 结算台账

```text
merchant_settlements
  id, settlement_no, merchant_id, period_start, period_end,
  gross_amount, commission_amount, refund_amount, net_amount,
  status, confirmed_at, paid_at, created_at, updated_at

settlement_entries
  id, entry_no, merchant_id, order_id, refund_id, entry_type,
  amount, available_at, settlement_id, created_at
```

`entry_no` 为全局唯一幂等键。`settlement_entries` 为不可变流水，退款通过新增负向冲正记录处理，不修改原销售流水。

## 5. API 设计方向

### 5.1 第一批兼容接口

```text
GET /api/v1/merchants
GET /api/v1/merchants/{id}
GET /api/v1/merchants/{id}/categories
GET /api/v1/merchants/{id}/products
GET /api/v1/products?merchant_id={id}
```

现有商品 DTO 增加 `merchant_name` 和 `merchant_logo`。现有调用不传 `merchant_id` 时返回所有启用商户的在售商品；只有一个商户时响应行为与当前一致。

购物车第一批保持数组响应兼容，但每项增加：

```text
merchant_id
merchant_name
merchant_logo
```

前端可立即按商户分组。交易接口完成后再把聚合金额放入新的预览响应，不破坏旧客户端。

### 5.2 多商户交易接口

```text
POST /api/v1/trades/preview
POST /api/v1/trades
GET  /api/v1/trades
GET  /api/v1/trades/{id}
POST /api/v1/trades/{id}/cancel
POST /api/v1/payments        请求改为 trade_id
```

预览和创建请求示意：

```json
{
  "address_id": 4,
  "merchant_coupons": [
    {"merchant_id": 1, "user_coupon_id": 8}
  ],
  "items": [
    {"sku_id": 3, "quantity": 2},
    {"sku_id": 21, "quantity": 1}
  ]
}
```

响应按 `merchant_groups` 返回每张子订单预览和总交易金额。创建时还必须提交最新 `idempotency_token`。

## 6. 向前兼容迁移顺序

每一步使用独立版本化 SQL，先在生产副本演练：

1. 建立 `schema_migrations` 和 migration 命令，记录 checksum，禁止同版本 SQL 被修改。
2. 创建 `trades`、`payment_allocations`、`merchant_settlements` 和 `settlement_entries`。
3. 给 `orders`、`payments` 和 `refunds` 增加可空兼容字段及索引。
4. 部署能够同时读取旧订单和新交易结构的代码，但仍只写旧结构。
5. 分批给历史订单创建单商户交易并回填订单、支付和支付分配，记录进度和异常。
6. 校验金额等式、孤儿记录、重复待支付单和退款累计金额。
7. 切换为双写并观察；双写必须在同一个 MySQL 事务内。
8. 切换新交易读路径和多商户创建入口。
9. 稳定一个版本后停止旧字段写入；收紧非空约束和删除旧字段另行立项。

禁止直接用 GORM AutoMigrate 承担历史回填、字段改空和兼容切换。

## 7. 事务和并发不变量

创建交易必须在一个 MySQL 事务中完成：

```text
锁定所有 SKU（按 sku_id 升序，避免死锁）
  -> 锁定所选用户券（按 id 升序）
  -> 校验启用商户、商品、价格和库存
  -> 创建 trade
  -> 按 merchant_id 升序创建 orders
  -> 创建 order_items 和金额快照
  -> 扣库存并写库存流水
  -> 核销商户券
  -> commit
  -> 清理 Redis 购物车
```

必须自动化验证：

- 100 个并发相同幂等键只创建一张交易单。
- 100 个并发不同用户购买同一 SKU 不超卖。
- 任一商户库存不足时没有任何子订单、库存扣减或券核销残留。
- 锁顺序固定，不因请求中商品顺序不同产生循环等待。
- 支付分配总和始终等于支付金额。
- 并发退款不能让任一分配的累计退款超过可退金额。
- 商户账号不能查询、发货、退款或结算其他商户子订单。

## 8. 分阶段实现与验收

### M8-A：公共商户与买家目录

- 公共商户列表/详情。
- 商品列表可按商户筛选，商品响应带商户快照。
- 商户分类和商户商品入口。
- 购物车查询不再固定商户 1，并返回商户信息。
- 收藏记录使用商品真实商户。

验收：停用商户不出现在公共接口；跨商户 SKU 可同时存在购物车；现有单商户前端不回归。

完成记录（2026-07-16）：

- 已实现 `GET /api/v1/merchants`、详情、商户分类和商户商品四个公开接口。
- `GET /api/v1/products` 支持可选 `merchant_id`，不传时查询所有启用商户；列表和详情返回商户名称与 Logo。
- 商品详情缓存命中时仍重新校验商户状态，并用当前商户资料补齐旧缓存字段，停用商户不能通过缓存继续曝光。
- Redis 购物车继续使用全局唯一 `sku_id` 作为 Hash field，可同时保存跨商户 SKU；响应增加商户信息并按商户、SKU 稳定排序。
- 购物车新增和更新会拒绝停用商户；已经存在的条目保留并返回明确不可用原因。
- 收藏新增使用商品真实 `merchant_id`，收藏列表跨商户查询并隐藏停用商户商品。
- UniApp 首页已增加公开商户入口，商品卡与详情展示商户信息，购物车按商户分组。M8-A 阶段曾临时阻止跨商户结算，该门禁已由 M8-C 的交易接口替代。
- OpenAPI 已同步新增路由、查询参数、404 响应以及分类、商品、购物车商户字段。
- 服务测试覆盖公开商户过滤、分类停用校验、跨商户商品、缓存停用校验、跨商户购物车和收藏真实商户。
- `go test ./...`、`go vet ./...`、路由/OpenAPI 契约测试和 `git diff --check` 通过；当时 UniApp 43 个接口契约、H5/小程序构建和 3 个浏览器环境共 12 个 E2E 用例通过，M8-C 后接口契约已增至 48 个。
- 使用远程 MySQL 执行 `TestPublicCatalogReadOnlyIntegration`，公开商户、分类、商品列表和详情查询通过，全程只读。

M8-A 没有新增或修改业务表。当前线上数据仍只有一个启用商户，因此“两个真实商户同时出现在购物车”的运行环境证据由自动化测试提供；部署到隔离环境补第二商户后再保存 HTTP 验收证据。

### M8-B：交易模型和历史回填

- 版本化 SQL migration runner。
- 新表、兼容字段和回填命令。
- 金额与引用一致性校验器。

验收：生产副本全量回填无孤儿记录，所有历史订单形成一对一交易和支付分配。

工程完成记录（2026-07-16）：

- 新增十组带 up/down 的 SQL migration，覆盖交易、订单/支付/退款兼容字段、支付分配、结算台账和公共目录索引。
- `go-mall-schema-migrate` 将 SQL 嵌入二进制，使用 MySQL 会话锁、dirty 标记和 SHA-256 checksum；历史必须是当前二进制的严格有序前缀。
- schema `up` 和 `down` 都需要独立显式环境开关；发布 workflow 只打包命令，不自动执行 M8 DDL。
- `go-mall-backfill-trades` 先检查旧订单、支付和退款引用及金额，再以固定顺序按批事务回填单商户交易、支付分配和退款链接；重复运行不会重复生成记录。
- 回填后的校验器覆盖交易/子订单金额、支付分配总额、退款累计、有效待支付键、商户/用户引用和孤儿记录。
- 历史佣金无可靠配置来源，明确回填为 0 bps，历史结算金额等于订单实付；后续佣金只从配置生效时间开始记录快照。
- 在临时 MySQL 8.4.10 中完成十个 migration 的首次升级、重复升级零变更、全量回滚、再升级；带部分退款、待支付和已取消数据的三张历史订单以 batch size 2 回填通过，重复回填零变更。
- 验收测试故意将一条支付分配金额加 1，校验器同时发现支付分配总额和订单快照不一致；恢复后所有检查为零。

生产副本和生产主库尚未执行这些迁移。正式切换前仍必须按 `operations-runbook.md` 在真实备份副本全量演练并保存行数、耗时和校验报告；本地构造数据通过不能替代该环境证据。

### M8-C：跨商户预览与创建

- 新预览/创建/列表/详情/取消接口。
- 原子库存、券和幂等处理。
- PC Web、H5/小程序购物车和结算分组。

验收：正常、库存不足、券不匹配、重复提交、并发和事务故障注入全部通过。

工程完成记录（2026-07-16）：

- 新增交易预览、创建、列表、详情和整笔取消五个接口；预览令牌绑定用户、地址、商品报价、数量和各商户券选择。
- 创建交易在一个 MySQL 事务中按稳定顺序锁定 SKU 和用户券，原子创建一张交易单及每商户一张子订单，并在提交后清理本次购物车条目。
- 商户券按真实 `merchant_id` 查询、领取和核销；每个商户分组最多选择一张券，券不能跨商户抵扣。
- 未支付交易只允许整笔取消，统一恢复库存和用户券；交易下的子订单不能再通过旧订单取消接口局部取消。
- PC Web 和 UniApp 均完成购物车按商户分组、逐商户选券、汇总预览和一次提交交易；M8-C 验收时交易支付入口保持关闭，已在 M8-D 改为交易级支付。
- 后端 `go test ./...`、`go vet ./...` 通过；PC Web 类型检查、单元测试和跨商户浏览器 E2E 通过；UniApp 48 个接口契约、19 个路由、H5 构建和 3 个浏览器项目共 12 个 E2E 用例通过。
- 临时 MySQL 8.4 环境验证两商户拆单、券不匹配、报价变化、故障注入全回滚、100 次相同幂等提交只生成一张交易，以及 100 个用户争抢 10 件库存不超卖。

详细证据见 `m8-c-trade-acceptance-2026-07-16.md`。生产备份副本迁移和生产 HTTP 写验收仍未执行，不能据此宣称 M8-C 已上线。

### M8-D：支付、退款和结算

- 合并支付与分配。
- 子订单售后退款冲正。
- 结算流水、周期汇总和商家只读查询。

验收：支付、子订单金额、退款、佣金和商户净额可逐笔对账；真实渠道资金验收仍按支付服务独立门禁执行。

工程完成记录（2026-07-16）：

- 新增 migration 000011/000012，使交易级支付不再依赖旧订单字段，并为商户费率与订单佣金、结算金额增加快照字段；迁移支持旧 schema 条件升级和隔离环境降级演练。
- 一张交易只创建一张支付单；每张子订单生成一条不可变 `payment_allocations`，支付单金额、分配总额和交易应付必须相等。
- 支付成功在同一事务内更新支付单、交易和全部子订单；重复通知幂等，金额或引用不一致时拒绝收敛。
- 售后退款绑定原支付分配并校验累计可退金额；退款成功新增退款和佣金退回流水，同时更新分配、支付单和交易的部分/全部退款状态。
- 订单创建时固化商户名称、费率、佣金和应结算金额；旧 `/orders` 兼容入口也在事务中创建单订单交易，旧 `order_id` 支付会映射到该交易。
- `settlement_entries` 作为不可变账本记录销售、佣金、退款、佣金退回和调账；历史成功退款或缺失佣金流水可通过归集命令幂等补记。
- `go-mall-generate-settlement` 先排空待补记账本，再按商户和周期生成结算单；相同周期重复执行返回原结算单。结算确认和实际打款不在 M8-D 范围内。
- 商家 API 提供结算单、结算明细和账本只读查询，仅 `owner/admin` 拥有 `settlement:read`；管理端已接入真实页面。
- PC Web 和 UniApp 均使用 `trade_id` 发起聚合支付；UniApp 小程序端在微信支付未实现前隐藏支付操作，H5 保留支付宝入口。
- 临时 MySQL 8.4 已通过 12 个 migration 往返、历史回填、100 次幂等提交、100 用户库存竞争、50 次并发支付意图、两商户退款冲正和结算金额验收。

详细证据见 `m8-d-payment-settlement-acceptance-2026-07-16.md`。生产备份副本、真实支付宝资金、结算打款和小程序真机仍是独立门禁，不能由本地 Mock 或隔离库结果替代。

## 9. 第一批编码准入条件

M8-A 已按“不新增业务表”的准入条件完成。查询只使用以下已核实字段：

```text
merchants: id, name, logo, status
products: id, merchant_id, category_id, name, cover, description, status, deleted_at
product_skus: id, merchant_id, product_id, name, image, price, stock, status, deleted_at
categories: id, merchant_id, parent_id, name, sort, status, deleted_at
```

远程生产 schema 在生产备份副本完成 migration、全量回填、校验和回滚演练前不得切换；隔离环境通过不替代生产变更审批与证据。
