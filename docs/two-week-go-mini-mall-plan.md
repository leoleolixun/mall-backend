# Go 小程序商城后端两周学习与开发计划

## 目标定位

两周内完成一个可运行、可联调、结构规范的 Go 商城后端第一版。

真实开发准则见：[real-development-guidelines.md](/Users/leo/GoWorkSpace/mall/mall-backend/docs/real-development-guidelines.md)

当前执行计划见：[current-backend-plan.md](/Users/leo/GoWorkSpace/mall/mall-backend/docs/current-backend-plan.md)

第一版目标不是完整商业商城，而是：

- 掌握 Go 后端核心开发方式
- 完成单商户商城 MVP
- 支持账号密码登录和微信登录
- 从第一天开始接入 Redis
- 数据模型预留后续多商户能力
- 能通过 Docker Compose 本地启动 MySQL、Redis 和后端服务
- 能用 Swagger、Apifox 或 Postman 完整跑通核心接口

第一版完成标准：

- 商品分类、商品、SKU 可查询
- 用户可通过账号密码或微信方式登录
- 登录后可维护收货地址
- 购物车使用 Redis
- 可创建订单、查询订单、取消订单、模拟支付
- 创建订单时正确使用 MySQL 事务
- 订单保留商品、SKU、价格、地址快照
- 所有买家侧核心接口都能跑通
- 项目结构清晰，方便继续扩展多商户

工程质量完成标准：

- 每个功能有清晰接口契约
- 用户私有数据按 `user_id` 做隔离
- Service 不依赖 Gin，Repository 不处理 HTTP
- 写操作考虑事务、幂等或重复提交风险
- 提交前通过 `gofmt`、`go test ./...`、`git diff --check`
- 真实配置和密钥不进入 Git

## 技术栈

- Go 1.24+
- Gin：HTTP API
- GORM：MySQL ORM
- MySQL：核心业务数据
- Redis：购物车、登录态、限流、幂等 token、缓存
- JWT：访问令牌
- bcrypt：密码哈希
- Viper 或 cleanenv：配置管理
- zap 或 slog：日志
- Swagger：接口文档
- Docker Compose：本地开发环境

## 项目结构

```text
mall-backend/
├── cmd/
│   └── server/
│       └── main.go
├── internal/
│   ├── config/
│   ├── handler/
│   ├── service/
│   ├── repository/
│   ├── model/
│   ├── middleware/
│   ├── router/
│   ├── dto/
│   └── bootstrap/
├── pkg/
│   ├── response/
│   ├── jwt/
│   ├── password/
│   ├── logger/
│   └── errors/
├── migrations/
├── docs/
├── scripts/
├── config.yaml
├── docker-compose.yml
├── Dockerfile
└── go.mod
```

分层约定：

```text
Handler     处理 HTTP 参数、鉴权上下文、统一响应
Service     处理业务规则、事务、状态流转
Repository  处理数据库读写
Model       定义数据库模型
DTO         定义请求和响应结构
Middleware  处理 JWT、日志、限流、恢复 panic
```

依赖方向：

```text
handler -> service -> repository -> model
```

不要让 repository 依赖 handler，也不要在 handler 里直接写复杂业务。

## 第一版功能边界

### 必做

- 健康检查
- 配置加载
- MySQL 初始化
- Redis 初始化
- 统一响应结构
- 统一错误处理
- 商品分类列表
- 商品列表
- 商品详情
- SKU 查询
- 账号密码注册
- 账号密码登录
- 微信小程序登录
- JWT 鉴权中间件
- 获取当前用户信息
- 收货地址增删改查
- Redis 购物车增删改查
- 订单预览
- 创建订单
- 订单列表
- 订单详情
- 取消订单
- 模拟支付
- Swagger 文档
- Docker Compose 本地启动

### 暂不做

- 真实微信支付
- 优惠券
- 秒杀
- 拼团
- 分销
- 售后退款
- 物流轨迹
- 商家后台完整权限系统
- 微服务拆分
- 消息队列
- Elasticsearch

## 多商户预留策略

第一版业务仍按单商户运行，但数据模型提前加入商户维度。

核心原则：

- 商品、分类、SKU、订单都带 `merchant_id`
- 第一版用默认商户 `merchant_id = 1`
- 买家接口默认只查上架商品
- 后续商家后台通过登录商户身份决定可管理的 `merchant_id`
- 后续多商户订单可以从“单订单单商户”开始，不急着做“一个订单跨多个商户”

推荐第一版只支持：

```text
一个订单只属于一个 merchant_id
```

后续如果购物车里有多个商户商品，下单时再拆成多个订单。

## 核心数据表

### users

买家用户主表。

关键字段：

```text
id
nickname
avatar
mobile
status
created_at
updated_at
deleted_at
```

### user_auths

用户登录方式表，用于同时支持账号密码、微信登录，并预留更多登录方式。

关键字段：

```text
id
user_id
provider        // password, wechat_mini_program
provider_uid    // username 或 openid
credential      // 密码 hash，微信登录为空
created_at
updated_at
```

唯一约束：

```text
provider + provider_uid
```

### merchants

商户表。第一版只初始化一个默认商户。

关键字段：

```text
id
name
logo
status
created_at
updated_at
```

### categories

商品分类表。

关键字段：

```text
id
merchant_id
parent_id
name
sort
status
created_at
updated_at
deleted_at
```

### products

商品 SPU 表。

关键字段：

```text
id
merchant_id
category_id
name
cover
description
status          // draft, on_sale, off_sale
created_at
updated_at
deleted_at
```

### product_skus

商品 SKU 表。

关键字段：

```text
id
merchant_id
product_id
name
image
price
stock
status
created_at
updated_at
deleted_at
```

库存第一版直接放在 SKU 表里即可。后续需要库存流水时再加 `inventory_logs`。

### addresses

收货地址表。

关键字段：

```text
id
user_id
receiver_name
receiver_phone
province
city
district
detail
is_default
created_at
updated_at
deleted_at
```

### orders

订单主表。

关键字段：

```text
id
order_no
user_id
merchant_id
status
total_amount
pay_amount
address_snapshot
paid_at
cancelled_at
created_at
updated_at
deleted_at
```

订单状态：

```go
type OrderStatus int

const (
    OrderStatusPendingPayment OrderStatus = 1
    OrderStatusPaid           OrderStatus = 2
    OrderStatusShipped        OrderStatus = 3
    OrderStatusCompleted      OrderStatus = 4
    OrderStatusCancelled      OrderStatus = 5
)
```

### order_items

订单明细表。

关键字段：

```text
id
order_id
user_id
merchant_id
product_id
sku_id
product_name
sku_name
sku_image
price
quantity
total_amount
created_at
updated_at
```

商品名称、SKU 名称、价格必须保存快照，不要只依赖商品表。

## Redis 设计

### 购物车

```text
key: mall:cart:{user_id}
type: hash
field: sku_id
value: quantity
```

常用操作：

```text
HSET mall:cart:1001 2005 3
HINCRBY mall:cart:1001 2005 1
HGETALL mall:cart:1001
HDEL mall:cart:1001 2005
```

### Refresh Token

```text
key: mall:auth:refresh:{refresh_token}
value: user_id
ttl: 7d 或 30d
```

### 登录失败限流

```text
key: mall:auth:login_fail:{provider}:{provider_uid}
value: fail_count
ttl: 15m
```

### 商品详情缓存

```text
key: mall:product:detail:{product_id}
value: json
ttl: 5m
```

商品修改或上下架时删除缓存。

### 订单幂等 token

```text
key: mall:order:idempotency:{user_id}:{token}
value: processing 或 order_no
ttl: 10m
```

创建订单前先校验 token，避免重复提交。

## API 规划

### 基础

```http
GET /health
```

### 认证

```http
POST /api/v1/auth/register
POST /api/v1/auth/login/password
POST /api/v1/auth/login/wechat
POST /api/v1/auth/refresh
POST /api/v1/auth/logout
GET  /api/v1/me
```

第一版接口命名为微信小程序登录，入参先使用测试 `open_id` 跑通流程。后续接入微信 `code2session` 时，仍然复用同一个接口：

```http
POST /api/v1/auth/login/wechat
```

### 商品

```http
GET /api/v1/categories
GET /api/v1/products
GET /api/v1/products/:id
GET /api/v1/products/:id/skus
```

### 地址

```http
GET    /api/v1/addresses
POST   /api/v1/addresses
PUT    /api/v1/addresses/:id
DELETE /api/v1/addresses/:id
PUT    /api/v1/addresses/:id/default
```

### 购物车

```http
GET    /api/v1/cart
POST   /api/v1/cart/items
PUT    /api/v1/cart/items/:sku_id
DELETE /api/v1/cart/items/:sku_id
DELETE /api/v1/cart
```

### 订单

```http
POST /api/v1/orders/preview
POST /api/v1/orders
GET  /api/v1/orders
GET  /api/v1/orders/:id
POST /api/v1/orders/:id/cancel
POST /api/v1/orders/:id/pay
```

## 一周压缩执行计划

如果目标是 7 天内完成第一版，需要每天投入 6 到 8 小时，并且严格压缩范围。

当前每日计划只保留今天及之后的文件。今天之前的每日计划已经清理，旧内容需要时从 Git 历史查看。

```text
docs/daily-plans/07-06.md
docs/daily-plans/07-07.md
docs/daily-plans/07-08.md
```

一周版只追求后端主流程跑通：

- 不接真实微信 `code2session`，微信小程序登录接口先使用测试 `open_id` 跑通流程
- 不做商家后台，只预留 `merchant_id`
- 不做复杂权限，只做买家 JWT 鉴权
- 不做真实支付，只做模拟支付
- 不做复杂测试覆盖，只覆盖订单、登录、鉴权几个关键点
- Swagger 可以先覆盖核心接口，不追求完整注释

### 第 1 天：Go 基础、项目骨架、MySQL、Redis

学习重点：

- Go module 和 package
- struct、method、pointer
- error 显式处理
- Gin 路由
- GORM 初始化
- Redis 客户端初始化

必须完成：

- 初始化 `go.mod`
- 创建基础目录结构
- 实现 `cmd/server/main.go`
- 实现配置加载
- 实现 logger
- 实现 `/health`
- 接入 MySQL
- 接入 Redis
- 编写 `docker-compose.yml`
- 定义统一响应结构
- 定义统一错误返回

当天验收：

- `go run ./cmd/server` 能启动
- `/health` 正常返回
- 服务启动时能连接 MySQL 和 Redis
- Docker Compose 能启动 MySQL 和 Redis

### 第 2 天：数据模型、商户预留、商品系统

学习重点：

- GORM model
- AutoMigrate 或 migration
- Repository、Service、Handler 分层
- 分页查询
- `context.Context` 在数据库查询中的传递

必须完成：

- 定义 `merchants`
- 定义 `categories`
- 定义 `products`
- 定义 `product_skus`
- 初始化默认商户 `merchant_id = 1`
- 初始化几条分类、商品、SKU 测试数据
- 实现分类列表接口
- 实现商品列表接口
- 实现商品详情接口
- 商品详情接入 Redis 缓存

当天验收：

- 可查询分类
- 可分页查询商品
- 可查询商品详情和 SKU
- 商品、分类、SKU 都有 `merchant_id`
- 商品详情第二次查询能命中 Redis 缓存

### 第 3 天：用户体系、账号密码登录、微信小程序登录

学习重点：

- interface 设计
- bcrypt 密码哈希
- JWT 签发和校验
- Gin middleware
- Redis 登录态和限流

必须完成：

- 定义 `users`
- 定义 `user_auths`
- 实现账号密码注册
- 实现账号密码登录
- 实现微信小程序登录
- 登录成功签发 access token
- refresh token 写入 Redis
- 实现 JWT 中间件
- 实现 `/api/v1/me`
- 账号密码登录失败接入 Redis 限流

当天验收：

- 用户可以注册
- 用户可以账号密码登录
- 用户可以微信小程序登录
- 两种登录方式最终都映射到 `users.id`
- 带 JWT 可以访问 `/api/v1/me`
- 不带 JWT 访问受保护接口会被拒绝

### 第 4 天：地址和 Redis 购物车

学习重点：

- 登录用户上下文
- 参数校验
- Redis Hash
- MySQL 数据和 Redis 数据组合查询
- 用户数据隔离

必须完成：

- 定义 `addresses`
- 实现地址列表
- 实现新增地址
- 实现修改地址
- 实现删除地址
- 实现设置默认地址
- 实现购物车添加商品
- 实现购物车修改数量
- 实现购物车删除商品
- 实现购物车列表
- 购物车使用 Redis Hash

当天验收：

- 用户只能查看和修改自己的地址
- 购物车数据写入 Redis
- 购物车列表能返回商品名、SKU、价格、数量、小计
- 商品下架、SKU 不存在、库存不足时有明确错误

### 第 5 天：订单预览、创建订单、事务

学习重点：

- MySQL 事务
- 库存扣减
- 订单快照
- 幂等 token
- 状态字段设计

必须完成：

- 定义 `orders`
- 定义 `order_items`
- 实现订单预览
- 订单预览返回幂等 token
- 幂等 token 写入 Redis
- 实现创建订单
- 创建订单时保存地址快照
- 创建订单时保存商品和 SKU 快照
- 创建订单时扣减库存
- 创建订单后清理购物车
- 创建订单整体放入事务

当天验收：

- 库存不足时不能创建订单
- 地址不属于当前用户时不能创建订单
- 创建订单后库存减少
- 创建订单后购物车对应商品被清理
- 重复提交不会生成重复订单

### 第 6 天：订单查询、取消、模拟支付、核心测试

学习重点：

- 订单状态机
- 状态流转校验
- Go testing
- httptest 或 service 层测试

必须完成：

- 实现订单列表
- 实现订单详情
- 实现取消订单
- 取消订单恢复库存
- 实现模拟支付
- 只有待支付订单可以支付
- 只有待支付订单可以取消
- 增加 JWT 中间件测试
- 增加登录测试
- 增加创建订单测试
- 增加库存不足测试
- 增加重复提交测试

当天验收：

- 用户只能查看自己的订单
- 支付后订单状态变为已支付
- 已支付订单不能取消
- 已取消订单不能支付
- 取消订单后库存恢复
- 核心测试通过

### 第 7 天：接口文档、Docker、全流程验收、复盘

学习重点：

- Swagger
- Dockerfile
- Docker Compose
- README 写法
- 项目复盘和技术债管理

必须完成：

- 接入 Swagger
- 补充核心接口文档
- 编写 Dockerfile
- Docker Compose 增加后端服务
- 编写 README 启动说明
- 整理接口调用顺序
- 整理数据库表说明
- 整理多商户二期计划
- 手工跑通完整主流程

当天验收：

- 一条命令能启动 MySQL、Redis、后端
- Swagger 可访问
- README 能指导从零启动项目
- 完整跑通：注册或登录 -> 商品详情 -> 加购物车 -> 地址 -> 订单预览 -> 创建订单 -> 模拟支付 -> 查询订单

### 一周版每日时间分配

建议每天按下面比例执行：

```text
学习 Go 基础和相关库：1.5 小时
写业务代码：4 到 5 小时
接口自测和修 bug：1 小时
当天复盘和记录：0.5 小时
```

每天结束时必须记录：

- 今天学会了什么 Go 知识
- 今天完成了哪些接口
- 哪些接口已经自测通过
- 明天最优先解决什么问题

### 一周版风险控制

如果当天进度落后，按下面优先级砍范围：

1. 先砍 Swagger 完整度，只保留核心接口
2. 再砍商品详情缓存，只保留 Redis 购物车和 token
3. 再砍登录失败限流
4. 再砍地址的默认地址逻辑
5. 最后才能砍测试，但订单创建事务必须手工测通

不能砍：

- 账号密码登录
- 微信小程序登录
- JWT 鉴权
- Redis 购物车
- 创建订单事务
- 库存扣减
- 订单快照
- `merchant_id` 预留

## 两周执行计划

每天建议投入 3 到 5 小时。两周内要完成第一版，需要严格控制范围。

### 第 1 天：Go 项目初始化

学习重点：

- Go module
- package 组织
- Gin 基础
- 配置加载
- 统一响应

开发任务：

- 初始化 `go.mod`
- 搭建 `cmd/server/main.go`
- 加入 Gin
- 加入配置文件 `config.yaml`
- 实现 `/health`
- 实现统一响应结构
- 实现统一错误结构
- 准备 Docker Compose 中的 MySQL 和 Redis

验收标准：

- 本地能启动后端
- `/health` 返回正常
- MySQL 和 Redis 容器能启动

### 第 2 天：数据库、Redis 和基础工程

学习重点：

- GORM 初始化
- Redis 客户端初始化
- context 传递
- 日志

开发任务：

- 初始化 MySQL 连接
- 初始化 Redis 连接
- 封装 bootstrap
- 接入 logger
- 定义基础 Model
- 准备 migrations 或 AutoMigrate
- 创建默认 merchant 数据

验收标准：

- 服务启动时能连接 MySQL 和 Redis
- 数据库中有基础表和默认商户

### 第 3 天：商品分类和商品模型

学习重点：

- GORM model
- Repository 模式
- Service 模式
- 分页查询

开发任务：

- 实现 `merchants`
- 实现 `categories`
- 实现 `products`
- 实现 `product_skus`
- 初始化测试商品数据
- 实现分类列表接口
- 实现商品列表接口
- 实现商品详情接口

验收标准：

- 可查询分类
- 可分页查询商品
- 可查询商品详情和 SKU
- 查询条件包含 `merchant_id = 1`

### 第 4 天：商品缓存和接口文档

学习重点：

- Redis 缓存
- 缓存穿透基础防护
- Swagger 注释

开发任务：

- 商品详情接入 Redis 缓存
- 商品不存在时设置短 TTL 空值缓存
- 增加 Swagger
- 补充商品接口文档
- 整理商品模块 DTO

验收标准：

- 商品详情第二次查询命中缓存
- Swagger 能看到商品相关接口

### 第 5 天：账号密码登录

学习重点：

- bcrypt
- JWT
- Gin middleware
- 用户上下文

开发任务：

- 实现 `users`
- 实现 `user_auths`
- 实现账号密码注册
- 实现账号密码登录
- 密码使用 bcrypt 存储
- 登录成功签发 access token 和 refresh token
- refresh token 写入 Redis
- 实现 JWT 鉴权中间件
- 实现 `/api/v1/me`

验收标准：

- 用户能注册
- 用户能通过账号密码登录
- 带 JWT 能访问 `/api/v1/me`
- refresh token 存在 Redis

### 第 6 天：微信小程序登录和登录限流

学习重点：

- 多登录方式统一用户体系
- Redis 计数器
- 登录安全基础

开发任务：

- 实现微信小程序登录
- 微信登录写入 `user_auths`
- provider 使用 `wechat_mini_program`
- provider_uid 模拟为 openid
- 账号密码登录失败接入 Redis 限流
- 登录失败超过阈值后短时间拒绝登录

验收标准：

- 同一个 openid 重复登录返回同一个用户
- 账号密码和微信登录都签发同一种 JWT
- 登录失败限流生效

### 第 7 天：收货地址

学习重点：

- 登录态接口
- 参数校验
- 用户数据隔离

开发任务：

- 实现地址增删改查
- 实现默认地址设置
- 地址查询必须按当前 user_id 过滤
- 增加地址接口 Swagger

验收标准：

- 用户只能看到自己的地址
- 默认地址设置正确

### 第 8 天：Redis 购物车

学习重点：

- Redis Hash
- 缓存数据与数据库数据组合查询
- SKU 库存校验

开发任务：

- 实现购物车添加商品
- 实现购物车修改数量
- 实现购物车删除商品
- 实现清空购物车
- 实现购物车列表
- 购物车列表从 Redis 取数量，从 MySQL 取商品和 SKU 信息

验收标准：

- 购物车数据存在 Redis
- 商品下架或 SKU 不存在时能正确提示
- 购物车列表包含商品名称、SKU、价格、数量、小计

### 第 9 天：订单预览

学习重点：

- 金额计算
- 快照概念
- 下单前校验

开发任务：

- 实现订单预览接口
- 校验地址是否属于当前用户
- 校验 SKU 是否存在、上架、有库存
- 计算商品总价和应付金额
- 返回订单幂等 token
- 幂等 token 写入 Redis

验收标准：

- 订单预览返回商品明细、地址、金额
- Redis 中存在订单幂等 token

### 第 10 天：创建订单和事务

学习重点：

- MySQL 事务
- 行锁或条件扣库存
- 事务回滚
- 幂等处理

开发任务：

- 实现 `orders`
- 实现 `order_items`
- 创建订单时校验幂等 token
- 创建订单时保存地址快照
- 创建订单时保存商品快照
- 创建订单明细
- 扣减 SKU 库存
- 清理购物车中已下单商品
- 整个流程放在事务中

验收标准：

- 库存不足时创建订单失败
- 任一步失败时事务回滚
- 重复提交不会创建多笔订单

### 第 11 天：订单查询、取消和模拟支付

学习重点：

- 订单状态机
- 状态流转校验
- 用户数据隔离

开发任务：

- 实现订单列表
- 实现订单详情
- 实现取消订单
- 取消订单时恢复库存
- 实现模拟支付
- 只有待支付订单可以支付
- 只有待支付订单可以取消

验收标准：

- 用户只能看到自己的订单
- 状态非法流转会被拒绝
- 支付后订单状态变为已支付
- 取消后库存恢复

### 第 12 天：测试和错误处理补强

学习重点：

- Go 单元测试
- httptest
- 业务边界测试

开发任务：

- 补充商品列表测试
- 补充账号密码登录测试
- 补充 JWT 中间件测试
- 补充创建订单测试
- 补充库存不足测试
- 补充重复提交测试
- 梳理错误码

验收标准：

- 核心 service 有测试
- 核心接口可通过测试或 Postman 集合跑通

### 第 13 天：Docker、Swagger 和联调准备

学习重点：

- Dockerfile
- Docker Compose
- 环境配置
- 接口文档整理

开发任务：

- 完善 Dockerfile
- 完善 docker-compose.yml
- 后端服务加入 compose
- 整理 Swagger 文档
- 增加 README 启动说明
- 准备 Postman 或 Apifox 接口集合

验收标准：

- 一条命令能启动 MySQL、Redis、后端
- Swagger 文档可访问
- 核心接口调用顺序清晰

### 第 14 天：收尾、复盘和多商户二期设计

学习重点：

- 项目复盘
- 技术债识别
- 多商户演进设计

开发任务：

- 全流程手工验收
- 整理 README
- 整理数据库表说明
- 整理 API 清单
- 整理多商户二期计划
- 标记暂不实现的能力

验收标准：

- 可完整跑通：注册或登录 -> 商品详情 -> 加购物车 -> 订单预览 -> 创建订单 -> 模拟支付 -> 查询订单
- 文档能指导自己从零启动项目
- 多商户后续改造点清晰

## 每天固定学习任务

每天开发前 30 到 45 分钟学习 Go。

建议顺序：

```text
第 1 天：module、package、struct、method
第 2 天：pointer、slice、map、error
第 3 天：interface、依赖倒置
第 4 天：context、defer
第 5 天：Gin middleware
第 6 天：JWT、bcrypt
第 7 天：GORM 查询和关联
第 8 天：Redis 数据结构
第 9 天：事务
第 10 天：并发安全和库存扣减
第 11 天：状态机
第 12 天：testing、httptest
第 13 天：Docker
第 14 天：性能、日志、工程化复盘
```

## 第一版关键难点

### 1. 多登录方式不要写成两套用户系统

错误方向：

```text
password_users
wechat_users
```

推荐方向：

```text
users
user_auths
```

所有登录方式最终映射到同一个 `users.id`。

### 2. 订单不能只引用商品当前价格

创建订单时必须保存快照：

- 商品名
- SKU 名
- SKU 图片
- 下单价格
- 购买数量
- 地址信息

否则商品改价、改名、地址修改后，历史订单会被污染。

### 3. 多商户不要一开始做完整平台

第一版只需要：

- 有 `merchants` 表
- 商品和订单有 `merchant_id`
- 查询和写入时带默认 `merchant_id = 1`

二期再做：

- 商家账号
- 商家后台
- 商家权限
- 多商户购物车分组
- 多商户订单拆单
- 商户结算

### 4. Redis 购物车不是订单依据

Redis 购物车只保存用户选择。

创建订单时仍然必须从 MySQL 重新查询：

- SKU 是否存在
- 商品是否上架
- 当前价格
- 当前库存

### 5. 不要强行使用 goroutine

第一版核心链路不需要强行并发。

后续适合加入 goroutine 或消息队列的地方：

- 异步记录操作日志
- 异步发送通知
- 定时取消超时订单
- 支付回调后异步处理后续任务

## 二期多商户计划

二期目标：从单商户 MVP 演进为基础多商户平台。

### 商户账号和权限

新增表：

```text
merchant_users
merchant_user_roles
roles
permissions
```

功能：

- 商户管理员登录
- 商户员工管理
- 商户角色权限
- 商户只能管理自己的商品、订单

### 商家后台接口

新增接口分组：

```http
/api/v1/merchant/*
```

示例：

```http
GET  /api/v1/merchant/products
POST /api/v1/merchant/products
PUT  /api/v1/merchant/products/:id
POST /api/v1/merchant/products/:id/on-sale
POST /api/v1/merchant/products/:id/off-sale
GET  /api/v1/merchant/orders
GET  /api/v1/merchant/orders/:id
POST /api/v1/merchant/orders/:id/ship
```

### 多商户购物车

Redis 购物车可以继续使用同一个 key，但列表返回时按商户分组：

```text
mall:cart:{user_id}
```

返回结构：

```json
{
  "merchants": [
    {
      "merchant_id": 1,
      "merchant_name": "默认商户",
      "items": []
    }
  ]
}
```

### 多商户下单拆单

推荐二期仍然坚持：

```text
一个订单只属于一个商户
```

如果用户同时购买多个商户商品：

```text
一次提交 -> 按 merchant_id 拆成多笔 orders
```

可以新增父级交易单：

```text
trade_orders
orders
order_items
```

第一版暂不需要 `trade_orders`。

### 商户结算

后续新增：

```text
merchant_settlements
merchant_account_logs
```

第一版不涉及真实支付和结算。

## 建议提交节奏

按功能小步提交，提交信息遵守当前仓库规则。

示例：

```text
chore(backend): 初始化 Go 后端项目结构
feat(backend): 增加 MySQL 和 Redis 初始化
feat(product): 增加商品分类和商品查询接口
feat(auth): 增加账号密码登录
feat(auth): 增加微信小程序登录
feat(cart): 增加 Redis 购物车
feat(order): 增加订单创建事务
test(order): 增加库存不足和重复提交测试
docs(backend): 增加后端启动说明
```

## 两周内的取舍

必须保证：

- 主流程跑通
- 事务正确
- 登录体系可扩展
- Redis 用在真实场景
- 数据模型预留多商户
- 文档能指导启动和测试

可以延期：

- 后台管理界面
- 真实微信支付
- 复杂权限
- 自动取消订单
- 消息队列
- 高并发优化
- 完整 CI/CD

## 最终验收清单

- [ ] `docker-compose up` 能启动依赖和后端
- [ ] `/health` 正常
- [ ] Swagger 可访问
- [ ] 可注册账号
- [ ] 可账号密码登录
- [ ] 可微信小程序登录
- [ ] JWT 鉴权生效
- [ ] 可查询商品分类
- [ ] 可查询商品列表
- [ ] 可查询商品详情
- [ ] 商品详情缓存生效
- [ ] 可维护收货地址
- [ ] 购物车写入 Redis
- [ ] 可订单预览
- [ ] 可创建订单
- [ ] 创建订单扣减库存
- [ ] 创建订单保存商品和地址快照
- [ ] 重复提交不会重复创建订单
- [ ] 可查询订单列表和详情
- [ ] 可取消订单并恢复库存
- [ ] 可模拟支付
- [ ] 用户数据隔离正确
- [ ] 表结构预留 `merchant_id`
- [ ] README 有启动和接口调用说明
