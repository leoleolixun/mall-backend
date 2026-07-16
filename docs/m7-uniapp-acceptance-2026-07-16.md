# M7 UniApp 工程验收记录

> 验收日期：2026-07-16
>
> 涉及仓库：`mall-frontend/mall-uniapp`、`mall-backend/docs/openapi.yaml`
> 当前结论：工程验收通过，真实环境与真机验收待补证

## 1. 本轮范围

已完成账号密码体系下的移动买家端闭环：

- H5 和微信小程序共用 UniApp Vue 3 CLI 工程。
- 注册、登录、Token 刷新合并、退出和个人资料。
- 分类、搜索、商品详情、SKU、收藏和购物车。
- 地址、逐商户优惠券、跨商户交易预览、最新预览令牌和原子拆单。
- 订单列表、详情、取消、配送信息、确认收货。
- 售后申请、列表、详情和取消。
- H5 支付宝支付意图、跳转以及支付详情/主动同步；M8-D 已把入口升级为 `trade_id` 聚合支付。
- 应用从后台恢复时只刷新当前可见页面。

按当前要求不纳入本轮：

- 对象存储、头像上传和售后图片凭证。
- 支付宝真实资金支付与退款。
- 微信登录、微信支付和客户端直接提交 `open_id` 的任何临时方案。
- 第三方快递实时轨迹。

## 2. 契约和安全约束

- API 客户端当前覆盖 48 个已使用接口（M7 原 39 个，M8-A 新增 4 个公开商户接口，M8-C 新增 5 个交易接口），不再暴露旧 `/orders/:id/pay`。
- 生产源码和构建产物不包含 `/payments/:id/mock-complete`、模拟支付、Stripe 或伪微信一键登录。
- 401 自动刷新只执行一次；并发请求共享同一个刷新 Promise，失败后才清理登录态。
- 创建交易前强制重新预览，只提交最新 `idempotency_token` 和各商户对应的 `user_coupon_id`。
- H5/小程序可以创建一张交易和多张商户子订单；H5 使用交易级支付，小程序在微信支付未实现前隐藏支付操作。
- `MALL_SITE_ORIGIN` 可让 H5 开发代理和小程序构建安全切换到隔离后端，默认值仍为正式商城域名。
- OpenAPI 已补齐预览令牌、订单优惠券 ID、优惠券领取标记、支付状态文本和售后显示字段。
- 写入 API E2E 默认拒绝执行；生产域名还需要第二个显式风险开关。

## 3. 自动化证据

### 快速契约门禁

```bash
cd mall-frontend/mall-uniapp
npm test
```

结果：通过。

```text
API contract OK: 48 endpoints
Order flow contract OK: latest trade preview token is required
Page contract OK: 19 routes and 5 tabs
```

页面契约会验证：

- `pages.json` 可解析且没有重复路由。
- 19 个注册路由与 19 个 Vue 页面文件完全一致。
- 5 个 Tab 均指向已注册页面。
- 页面适配器路由与 `pages.json` 一致。
- 旧订单支付和模拟支付接口不能回归。

### H5 浏览器验收

```bash
npm run build:h5
npm run test:e2e:h5
```

结果：3 个浏览器项目、12 个用例全部通过。M8-C 用例验证跨商户购物车分组、逐商户优惠券、最新交易预览和一次创建交易，且不会调用旧订单预览/创建。

```text
mobile-chromium: passed
mobile-webkit: passed
wechat-h5: passed
total: 12 passed
```

覆盖：

- 登录后多个接口同时 401 只刷新一次。
- 收藏、跨商户购物车、地址、逐商户优惠券和最新交易预览令牌下单。
- 多商户交易创建成功后展示子订单；H5 支付请求只提交 `trade_id`，不把任一子订单伪装成合并支付入口。
- 地址创建、默认、编辑和删除。
- 商家自配送、确认收货、售后申请和取消。
- 接口错误展示服务端消息，点击重试后恢复。

### 平台编译

```bash
npm run build:h5
npm run build:mp-weixin
```

结果：H5 和微信小程序生产编译均通过。小程序产物位于 `dist/build/mp-weixin`，尚未导入绑定真实 AppID 的微信开发者工具。

### 真实后端只读检查

```bash
npm run test:live
```

目标：`https://mall.leedu.ac.cn/api/v1`。

结果：分类、商品列表和商品详情 3 项通过；由于未提供专用测试账号，登录态只读接口跳过。直接跨域 `OPTIONS` 返回 404，被记录为诊断告警；正式 H5 使用同源 `/api`，微信小程序不依赖浏览器 CORS，因此当前不阻断。

### OpenAPI 和依赖

```bash
cd mall-backend
go test ./internal/router -run TestImplementedRoutesExistInOpenAPI -count=1
```

结果：通过。

```bash
cd mall-frontend/mall-uniapp
npm audit --omit=dev --audit-level=high
```

结果：部署运行依赖 0 个漏洞。直接依赖 Rollup 已从 `4.14.3` 升级到修复版本 `4.62.2`。DCloud 当前编译批次的完整开发依赖树仍报告 11 个高危告警；Vite 被插件精确要求为 `5.2.8`，未执行会破坏编译器组合的强制升级。本地开发服务器已限制为 `127.0.0.1` 并关闭跨源访问。

## 4. CI/CD 和发布方式

`.github/workflows/deploy-uniapp-h5.yml` 只在手动运行或相关推送提交信息含 `[deploy]` 时执行：

1. 安装锁定依赖。
2. 契约测试和生产依赖审计。
3. H5、小程序构建和 12 个浏览器用例。
4. 上传小程序 Artifact。
5. H5 原子发布到 `/opt/mall/mobile/releases`。
6. 更新 `/opt/mall/mobile/dist`，失败自动回切，保留最近 5 个版本。

正式部署前，Nginx 必须先配置 `/mobile/` 指向 `/opt/mall/mobile/dist/`。完整配置和 GitHub Secrets 见 `mall-frontend/mall-uniapp/README.md`。

## 5. 仍需外部环境补证

以下项目没有足够证据，不能标记为通过：

1. 使用专用账号执行真实后端登录态只读检查。
2. 在隔离 MySQL schema、Redis DB 和测试商户上运行 `npm run test:e2e:api`，验证真实写接口及清理过程。
3. 填写微信小程序 AppID、配置 request 合法域名并导入微信开发者工具。
4. 在至少一台 iOS 和一台 Android 真机验收弱网、切后台、Token 过期和深层路由。
5. 配置服务器 `/mobile/` 后执行实际部署、HTTPS 静态资源和回滚检查。2026-07-16 的只读检查显示该路径仍返回 PC Web 的 `Mall PC Web` 和 `id="root"`，尚未配置移动端 location；当前 workflow 的 `id="app"` 烟雾检查会拒绝该发布并自动回切。
6. 对象存储、支付宝真实支付/退款和微信登录/支付继续按明确决定暂缓。

## 6. 外部验收命令

专用账号只读检查：

```bash
MALL_TEST_USERNAME=test-user \
MALL_TEST_PASSWORD=test-password \
npm run test:live
```

隔离环境写入验收：

```bash
MALL_API_BASE=https://test-mall.example.com/api/v1 \
MALL_ALLOW_MUTATION_TESTS=1 \
npm run test:e2e:api
```

发布后检查：

```bash
curl -fsS https://mall.leedu.ac.cn/mobile/ | grep 'id="app"'
curl -fsS https://mall.leedu.ac.cn/api/v1/categories >/dev/null
```

外部验收记录需补充测试账号、设备型号、系统版本、微信版本、request_id、部署提交和结论，不记录密码、Token 或私钥。
