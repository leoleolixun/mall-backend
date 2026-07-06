# Go 商城项目真实开发准则

## 目标

这个项目仍然是学习项目，但开发方式要尽量贴近真实后端团队的日常标准。

真实开发不是只把接口跑通，而是每个功能都要回答清楚：

- 接口契约是什么
- 数据归属是谁
- 错误如何表达
- 是否需要鉴权
- 是否会产生脏数据
- 是否可重复提交
- 是否可以被测试
- 日志是否能帮助排查问题
- 配置和密钥是否安全

## 每个接口的标准开发顺序

以后每写一个接口，都按这个顺序做：

```text
1. 明确接口契约
2. 定义 DTO
3. 定义或确认 Model
4. 写 Repository
5. 写 Service
6. 写 Handler
7. 注册 Router
8. 补充错误处理
9. 写最小测试或手工验收脚本
10. 更新当天文档
11. gofmt + go test ./...
12. 检查 git diff 和敏感信息
13. 按规范提交
```

不要一上来就在 Handler 里堆业务代码。

## 接口契约

每个接口开发前先写清楚：

```text
Method
Path
是否需要 JWT
请求参数
响应结构
常见错误
数据隔离规则
```

示例：

```http
POST /api/v1/cart/items
Authorization: Bearer <token>
```

请求：

```json
{
  "sku_id": 1,
  "quantity": 2
}
```

响应：

```json
{
  "code": 0,
  "message": "ok",
  "data": {
    "sku_id": 1,
    "quantity": 2
  }
}
```

错误：

```text
未登录
参数错误
SKU 不存在
商品已下架
库存不足
```

## 分层职责

### Handler

只做：

- 解析 path/query/body/header
- 读取当前用户
- 调用 Service
- 返回统一响应

不要做：

- SQL 查询
- Redis key 拼装
- 复杂业务判断
- 事务

### Service

只做：

- 业务规则
- 参数二次校验
- 数据组合
- 事务
- 幂等
- 状态流转
- 调用 Repository 和 Redis

### Repository

只做：

- MySQL 查询
- MySQL 写入
- SQL 条件封装

不要 import：

```text
gin
response
handler
service
```

### Model

只表达数据库结构，不塞业务逻辑。

### DTO

只表达接口输入和输出，不直接复用数据库 model。

## 鉴权和数据隔离

买家侧需要登录的接口必须从 JWT 中拿当前用户：

```text
middleware.CurrentUserID(c)
```

所有用户私有数据都必须按 `user_id` 过滤：

```text
addresses
cart
orders
```

错误示例：

```sql
SELECT * FROM addresses WHERE id = ?
```

正确示例：

```sql
SELECT * FROM addresses WHERE id = ? AND user_id = ?
```

这是避免越权访问的基本要求。

## 错误处理

当前项目先用统一响应：

```json
{
  "code": 40000,
  "message": "参数错误"
}
```

但 Service 不应该直接返回 HTTP 响应。

真实开发中更好的方向是：

```text
Service 返回业务错误
Handler 把业务错误映射成 HTTP 状态码和业务 code
```

后续可以增加：

```text
pkg/errors
```

用于定义：

```text
参数错误
未登录
无权限
资源不存在
库存不足
状态不允许
重复提交
```

## 配置和密钥

真实配置文件：

```text
config.yaml
```

必须保持 ignored。

提交示例配置：

```text
config.example.yaml
```

提交前检查：

```bash
git status --short --ignored
git diff --cached | rg "password|secret|token|your-real-host|your-real-redis-password"
```

注意：如果只是代码里的 key 名，如 `login_fail:password`，不是泄露；如果是真实密码、真实 host、真实 token，就不能提交。

## Redis 使用规范

Redis key 必须有项目和场景前缀：

```text
mall:auth:refresh:{refresh_token}
mall:auth:login_fail:password:{username}
mall:cart:{user_id}
mall:product:detail:{product_id}
mall:order:idempotency:{user_id}:{token}
```

原则：

- 临时数据必须设置 TTL
- 用户私有数据 key 必须带 user_id
- 不要在业务代码里使用 `FLUSHALL`
- refresh token 使用一次后应该失效

## 数据库和事务

只有读接口通常不需要事务。

这些场景必须考虑事务：

- 设置默认地址
- 创建订单
- 取消订单并恢复库存
- 支付状态变更

事务中的所有数据库操作必须使用同一个 `tx`。

订单创建必须保证：

```text
创建订单
创建订单明细
扣减库存
清理购物车
```

要么全部成功，要么全部回滚。

## 幂等性

这些接口必须考虑重复提交：

```text
创建订单
支付回调
取消订单
刷新 token
```

第一版订单创建使用 Redis 幂等 token：

```text
mall:order:idempotency:{user_id}:{token}
```

## 测试和验收

每个模块至少要有手工验收记录。

关键模块要补最小测试：

```text
JWT 中间件
密码校验
购物车数量边界
订单创建成功
库存不足
重复提交
状态非法流转
```

提交前必须执行：

```bash
gofmt -w <changed go files>
go test ./...
git diff --check
```

## 日志

开发阶段可以先使用 Gin logger 和 zap。

真实排查时至少要能看到：

- 服务启动配置
- MySQL/Redis 初始化成功或失败
- 请求路径和状态码
- 关键业务失败原因

不要在日志中输出：

```text
密码
完整 token
真实密钥
个人敏感信息
```

## 提交规范

提交前确认：

```bash
git status --short --ignored
go test ./...
git diff --cached --check
```

提交信息遵守：

```text
<type>(<scope>): <subject>
```

示例：

```text
feat(cart): 增加 Redis 购物车接口
fix(auth): 修复微信登录路由不一致
docs(backend): 更新真实开发准则
test(order): 增加订单重复提交测试
```
