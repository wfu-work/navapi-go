# navapi-go 规划

`navapi-go` 是参考 `new-api` 能力重建的 AI API 网关后端。项目只做后台服务，不内置前端页面；系统底座使用 `github.com/wfu-work/nav-common-go-lib`，启动、配置、数据库、日志、鉴权、基础用户/角色/权限等公共能力按 `vpn-server` 的接入方式实现。

## 目标

- 对齐 `new-api` 的核心后端功能：模型网关、上游服务商管理、令牌鉴权、额度计费、日志统计、任务转发、系统配置。
- 保持 OpenAI 兼容接口优先可用，逐步扩展 Claude、Gemini、图像、音频、Rerank、异步任务等协议。
- 复用 `nav-common-go-lib` 的工程底座，避免重复实现通用后台能力。
- 后端接口保持清晰边界，便于未来接入独立 Web 前端、移动端或第三方管理台。

## 参考项目

- `new-api`：功能参考，重点参考路由分组、上游服务商模型、令牌模型、转发链路、计费与日志设计。
- `vpn-server`：基础框架接入参考，重点参考 `SysInit` 生命周期、业务表注册、路由注册、定时任务注册方式。
- `nav-common-go-lib`：项目基础框架，提供 Gin、Gorm、Viper、Zap、JWT、Casbin、默认系统模块、公共响应、定时任务等能力。

## 范围定义

### 本项目负责

- 业务数据表与迁移注册。
- 业务 API：上游服务商、令牌、模型、日志、额度、任务、系统选项。
- Relay API：OpenAI 兼容转发与后续多协议适配。
- 上游服务商选择、模型匹配、失败重试、自动禁用/恢复。
- 额度预扣、结算、退款、消费日志。
- 管理端所需的纯后端接口。
- 定时任务：上游服务商检测、余额刷新、任务轮询、日志清理、配置刷新。

### 本项目不负责

- 不内置 React/Vue/Angular 前端。
- 不照搬 `new-api` 的 Web 静态资源和页面路由。
- 不在第一阶段实现全部支付、OAuth、部署市场、复杂多语言页面配置。
- 不直接复制 `new-api` 代码结构，按 `nav-common-go-lib` 的工程风格重组。

## 技术底座

### 启动方式

按 `vpn-server` 的方式组织：

```text
main.go
  -> inits.Init()
       -> SysInit.OnTableInit(registerTables)
       -> SysInit.OnRouterInit(registerRouters)
       -> SysInit.OnScheInit(registerSchedules)
       -> SysInit.OnOtherInit(registerCachesAndOptions)
       -> SysInit.Init()
```

### 推荐目录

```text
.
├── apis/             HTTP handler
├── configs/          navapi 业务配置
├── constants/        上游服务商类型、状态、上下文 key、relay mode
├── domains/          Gorm 数据模型
├── dto/              上下游请求响应结构
├── inits/            业务初始化入口
├── middlewares/      token auth、relay context、rate limit
├── relay/            协议转发核心
│   ├── adapters/     OpenAI、Claude、Gemini 等适配器
│   └── formats/      请求/响应格式转换
├── routers/          路由注册
├── scheduleds/       定时任务
├── services/         业务服务
├── settings/         动态配置加载与缓存
└── vos/              管理接口 VO
```

## 功能模块

### 1. 基础系统

依赖 `nav-common-go-lib` 提供：

- 配置加载与热更新。
- Gin 服务启动。
- Gorm 数据库连接。
- Zap 日志。
- JWT 登录态。
- Casbin 权限。
- 用户、角色、企业、文件、系统配置等默认模块。
- `/api/health`、Swagger 等公共接口。

`navapi-go` 只扩展 AI 网关业务表和业务路由。

### 2. 用户与权限

第一阶段优先复用 `nav-common-go-lib` 的用户体系，不重新实现 `new-api` 的完整用户表。

需要补充：

- 用户额度账户。
- 用户可用分组。
- 用户 token 归属。
- 管理员、普通用户、根用户的业务权限映射。
- Token 访问时从业务 token 解析到基础用户。

### 3. 上游服务商管理

核心对象：`VendorMeta`。

字段规划：

- 基础信息：服务商标识、展示名称、类型、启用状态、排序、备注。
- 上游信息：Base URL、API Key、多 Key、组织 ID、自定义 Header。
- 模型能力：支持模型列表、模型映射、模型覆盖和模型白名单。
- 余额信息：余额接口模板、授权方式、响应路径和单位换算。
- 扩展配置：Header 覆盖、Query 参数覆盖、备注说明。

接口规划：

- `GET /api/provider/list`
- `GET /api/provider/:guid`
- `POST /api/provider/`
- `PUT /api/provider/`
- `DELETE /api/provider/:guid`
- `GET /api/provider/:guid/key`
- `PUT /api/provider/:guid/key`
- `POST /api/provider/test`
- `POST /api/provider/fetch_models`

### 4. Token 管理

核心对象：`ApiToken`。

能力：

- 生成、更新、删除 token。
- token 额度、过期时间、启停状态。
- 模型限制、IP 限制、分组限制。
- token key 脱敏展示。
- token 使用统计。

接口规划：

- `GET /api/token/`
- `GET /api/token/:id`
- `POST /api/token/`
- `PUT /api/token/`
- `DELETE /api/token/:id`
- `POST /api/token/:id/key`
- `GET /api/usage/token/`

### 5. Relay 网关

第一阶段优先实现 OpenAI 兼容接口：

- `GET /v1/models`
- `GET /v1/models/:model`
- `POST /v1/chat/completions`
- `POST /v1/completions`
- `POST /v1/embeddings`
- `POST /v1/images/generations`
- `POST /v1/audio/transcriptions`
- `POST /v1/audio/translations`

后续扩展：

- `POST /v1/responses`
- `GET /v1/realtime`
- `POST /v1/messages` Claude Messages。
- `/v1beta/models/*path` Gemini native。
- `/mj/*` Midjourney 任务。
- `/suno/*` Suno 任务。
- Rerank、Moderations、文件接口。

Relay 链路：

```text
request
  -> CORS / recover / request id / stats
  -> TokenAuth
  -> ModelRequestRateLimit
  -> Select upstream provider
  -> Relay format parser
  -> Adapter convert request
  -> upstream request
  -> Adapter convert response / stream passthrough
  -> quota settlement
  -> usage log
```

### 6. 上游服务商选择

选择策略：

- 按 token 分组过滤上游服务商。
- 按请求模型匹配服务商模型列表。
- 支持模型映射。
- 支持权重随机、优先级排序、响应时间参考。
- 支持 auto 分组。
- 支持失败重试与跨分组重试。
- 支持上游亲和缓存，连续请求优先命中稳定服务商。

第一阶段实现：

- 分组 + 模型 + 状态过滤。
- 权重随机。
- 失败重试。

第二阶段增强：

- 上游亲和。
- 自动禁用。
- 多 Key 轮询与单 Key 禁用。
- 状态码映射。

### 7. 模型与价格

核心对象：

- `ModelMeta`：模型展示、owner、能力、上下文、排序。
- `VendorMeta`：供应商元信息。
- `Pricing`：模型倍率、分组倍率、缓存倍率、图片/音频/补全倍率。

能力：

- 从上游服务商同步模型。
- 手动维护模型元数据。
- 不同分组独立倍率。
- 公开价格查询接口。
- 后台价格配置接口。

### 8. 额度与计费

计费流程：

```text
before relay:
  estimate prompt / image / audio / task quota
  pre-consume token quota

after relay:
  parse upstream usage
  calculate final quota
  refund or supplement delta
  write consume log
  update user / token usage quota
```

第一阶段：

- 文本 token 计费。
- streaming 最终 usage 解析。
- 失败退款。
- 消费日志。

第二阶段：

- 图片、音频、缓存命中、工具调用、分层计费。
- 异步任务预扣与完成后差额结算。

### 9. 日志与统计

核心对象：`UsageLog`。

字段：

- 用户、token、上游服务商、模型。
- prompt tokens、completion tokens、quota。
- stream 标记、耗时、IP、request id、upstream request id。
- 错误内容、扩展 JSON。

接口：

- `GET /api/usage/list`
- `GET /api/usage/self/list`
- `GET /api/usage/stat`
- `GET /api/usage/self/stat`
- `GET /api/usage/summary`
- `GET /api/usage/self/summary`
- `GET /api/data/list`
- `GET /api/data/self/list`

### 10. 动态配置

配置来源分两层：

- 静态配置：`config.yaml`，由 `nav-common-go-lib` 加载。
- 动态配置：业务 `Option` 表，运行中刷新缓存。

动态配置包括：

- 系统名称、公告、关于信息。
- 是否开启注册、日志、任务、绘图。
- 默认额度、新用户额度、提醒阈值。
- 模型倍率、分组倍率。
- 上游服务商自动禁用策略。
- 敏感词和请求限制。

### 11. 异步任务

核心对象：`Task`。

支持场景：

- Midjourney。
- Suno。
- 视频生成。
- 其他异步模型任务。

第一阶段只保留任务模型和查询接口，Relay 异步任务放第二阶段。

### 12. 安全与风控

需要实现：

- TokenAuth：兼容 `Authorization: Bearer sk-xxx`。
- 管理接口 JWT + Casbin 权限。
- IP 限制。
- 模型限制。
- 请求频率限制。
- 请求体大小限制。
- SSRF 防护：图片、音频、文件 URL 获取必须限制内网地址。
- API Key 加密存储或至少避免普通接口明文返回。

## 数据表初稿

业务表建议：

- `nav_api_tokens`
- `nav_api_model_meta`
- `nav_api_vendor_meta`
- `nav_api_options`
- `nav_api_usage_logs`
- `nav_api_quota_dates`
- `nav_api_tasks`
- `nav_api_redemptions`
- `nav_api_subscriptions`，后续阶段。
- `nav_api_payments`，后续阶段。

所有业务表通过 `inits.registerTables()` 注册到 `global.NAV_DB.AutoMigrate()`。

## 路由规划

### 管理 API

管理 API 挂载在 `nav-common-go-lib` 的 `system.router-prefix` 下，默认 `/api`。

```text
GET    /api/status
GET    /api/models
GET    /api/pricing

POST   /api/user/register
POST   /api/user/login
GET    /api/user/self
GET    /api/user/models

GET    /api/provider/list
GET    /api/provider/:guid
POST   /api/provider/
PUT    /api/provider/
DELETE /api/provider/:guid

GET    /api/token/
POST   /api/token/
PUT    /api/token/
DELETE /api/token/:id

GET    /api/usage/list
GET    /api/usage/self/list

GET    /api/task/
GET    /api/task/self

GET    /api/option/
PUT    /api/option/
```

### Relay API

Relay API 保持在根路径，兼容 OpenAI 客户端默认 base URL。

```text
GET    /v1/models
GET    /v1/models/:model
POST   /v1/chat/completions
POST   /v1/completions
POST   /v1/embeddings
POST   /v1/images/generations
POST   /v1/audio/transcriptions
POST   /v1/audio/translations
POST   /v1/responses
POST   /v1/messages
POST   /v1beta/models/*path
```

## 实施阶段

### Phase 0：工程骨架

- 清理 GoLand 示例 `main.go`。
- 引入 `nav-common-go-lib`。
- 建立 `inits/routers/apis/services/domains` 目录。
- 增加最小 `config.yaml` 示例。
- 通过 `SysInit` 启动服务。
- 注册空业务路由和健康检查验证。

验收：

- `go test ./...` 通过。
- 服务可启动。
- `/api/health` 正常。

### Phase 1：最小可用网关

- `Provider`、`ApiToken`、`UsageLog` 表。
- TokenAuth 中间件。
- OpenAI `/v1/models` 和 `/v1/chat/completions`。
- OpenAI adapter。
- 上游服务商选择：状态 + 分组 + 模型 + 权重。
- 基础额度扣减和消费日志。
- 管理接口：上游服务商 CRUD、token CRUD、日志列表。

验收：

- 使用 `Authorization: Bearer sk-xxx` 可以调用 `/v1/chat/completions`。
- 能配置至少一个 OpenAI 兼容上游服务商。
- 请求成功后 token、用户、上游服务商用量增加。
- 上游失败时返回 OpenAI 风格错误。

### Phase 2：协议与计费补齐

- Embeddings、images、audio。
- Responses API。
- Claude Messages。
- Gemini native。
- streaming 使用量结算。
- 模型倍率、分组倍率、缓存倍率。
- 上游服务商测试、余额刷新、自动禁用。
- 定时任务和配置缓存刷新。

验收：

- 主流 OpenAI SDK 可直接使用。
- 管理接口可维护倍率和上游服务商。
- streaming 与非 streaming 计费一致。

### Phase 3：高级能力

- Midjourney、Suno、视频等异步任务。
- 多 Key 管理。
- 上游亲和。
- 敏感词、SSRF、模型限流。
- 兑换码、订阅、支付接口。
- OAuth/第三方登录按实际前端需求接入。

验收：

- 异步任务可提交、轮询、计费、退款。
- 高并发下上游服务商选择和额度扣减无明显竞态。
- 管理端可覆盖 `new-api` 的主要运营能力。

## 优先级

P0：

- 工程骨架。
- 上游服务商、token、日志、OpenAI chat relay。
- 基础额度结算。

P1：

- 模型管理、价格倍率、streaming 计费。
- embeddings、images、audio、responses。
- 上游服务商测试与自动禁用。

P2：

- Claude、Gemini。
- 异步任务。
- 多 Key。
- 敏感词、SSRF、限流。

P3：

- 支付、订阅、OAuth、统计看板扩展接口。
- 部署市场、复杂供应商扩展。

## 关键设计决策

- 管理用户体系优先复用 `nav-common-go-lib`，不平移 `new-api.User`。
- Relay token 使用独立业务 token 表，避免和后台登录 JWT 混在一起。
- Relay API 不挂 `/api` 前缀，保持客户端兼容。
- 上游协议通过 adapter 接口隔离，每个供应商只处理 URL、Header、请求转换、响应转换。
- 额度扣减必须服务层事务化，不能散落在 handler 中。
- 日志写入允许异步，但额度扣减必须同步确认。
- API Key 默认不在列表接口返回明文，只通过受保护接口查看。

## 风险与注意事项

- `new-api` 功能面很大，应先实现最小闭环，再逐步扩协议。
- 流式响应的 usage 解析和错误处理要单独测试。
- 多 Key 轮询、自动禁用、失败重试会引入并发状态，需要加锁或数据库原子更新。
- 额度预扣和退款必须防重复，尤其是客户端断连、上游超时、异步任务轮询场景。
- 图片/音频 URL 获取必须做 SSRF 防护。
- 不应把前端展示配置和后端核心配置耦合太深。

## 下一步任务

1. 初始化工程骨架并接入 `nav-common-go-lib`。
2. 增加 `config.yaml` 示例和本地 SQLite 默认配置。
3. 定义 P0 数据模型并注册 AutoMigrate。
4. 实现 TokenAuth、ProviderService、TokenService。
5. 实现 OpenAI chat relay 最小闭环。
6. 增加单元测试和一个 curl 调用示例。

## 当前实现状态

已完成 P0 最小闭环：

- 接入 `nav-common-go-lib` 启动、配置、数据库、日志、JWT 后台鉴权。
- 新增业务表：上游服务商、API Token、用户额度账户、模型元数据、价格倍率、使用日志、动态选项、任务、兑换码、额度日期。
- 新增后台接口：`/api/provider/*`、`/api/token/*`、`/api/quota/*`、`/api/usage/*`、`/api/models/*`、`/api/pricing/*`、`/api/option/*`、`/api/task/*`、`/api/redemption/*`。
- 一般列表接口统一使用 `/list` 前缀，例如 `/api/provider/list`、`/api/token/list`、`/api/usage/self/list`。
- 上游服务商管理支持上游模型拉取、连通性测试、模型映射和请求覆盖配置。
- 价格模块支持公开 `/api/pricing` 查询和后台倍率维护，Relay 额度结算会应用模型/分组倍率、缓存命中倍率。
- Token 管理支持 `/api/usage/token/` 使用统计，日志模块支持 `/api/data/list` 和 `/api/data/self/list` 近 N 天用量聚合。
- 用户额度账户支持 `/api/quota/self` 查询和后台维护，token 创建/更新会校验用户可用分组。
- 新增 OpenAI 兼容接口：`GET /v1/models`、`POST /v1/chat/completions`、`POST /v1/completions`、`POST /v1/embeddings`、`POST /v1/moderations`、`POST /v1/rerank`、`POST /v1/images/generations`、`POST /v1/audio/*`、`POST /v1/responses`。
- 新增 Claude Messages、Gemini native、异步任务基础入口：`POST /v1/messages`、`POST /v1beta/models/*path`、`POST /mj/*path`、`POST /suno/*path`。
- Relay 链路已包含：`sk-` token 鉴权、模型限制、服务商选择、失败重试、上游亲和缓存、多 Key 轮询、模型映射、Header/Query 覆盖、上游转发、非流式/流式 usage 解析、Responses usage 兼容、倍率计费、额度扣减、消费日志。
- 上游自动禁用已支持基础场景：上游返回 401/403 时记录禁用原因并停用该服务商。
- 安全与风控已支持：Token IP 白名单、请求体大小限制、token+model 级限流、敏感词拦截、图片/音频/文件 URL 的私网地址拦截。
- 动态配置支持启动加载和每分钟定时刷新；风控配置可通过 Option 表调整。
- 兑换码支持在事务中核销并给当前用户指定 API Token 增加剩余额度。
- 任务模块支持后台/用户维度的创建、查询、更新、删除；MJ/Suno 基础入口可提交任务并返回 task_id。

本地启动：

```bash
go run .
```

默认配置见 `config.yaml`，服务端口为 `8888`，管理 API 前缀为 `/api`，Relay API 保持根路径 `/v1`。

最小调用流程：

1. 通过 `nav-common-go-lib` 内置注册/登录接口创建后台用户并取得 JWT。
2. 使用 JWT 调用 `POST /api/provider/` 创建 OpenAI 兼容上游服务商。
3. 使用 JWT 调用 `POST /api/token/` 创建 API Token，返回 `sk-...`。
4. 使用 `Authorization: Bearer sk-...` 调用 `POST /v1/chat/completions`。
