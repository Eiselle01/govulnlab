# 架构设计

## 1. 设计目标与约束

| 目标 | 具体落地 |
| --- | --- |
| 安全需求可落地 | 每个安全要求都能在代码里指到具体实现，而不是停留在文档 |
| 结构可讲清 | 分层清晰、依赖单向，面试时能画出一张图讲完 |
| 单二进制部署 | 模板、CSS、SQL 迁移全部 `go:embed`，编译产物可直接拷走 |
| 不引入重型框架 | 只用 `net/http` + Go 1.22 的 ServeMux 方法路由，降低理解成本 |
| 可测 | service 层不依赖 HTTP，store 层不依赖全局变量，关键策略有表驱动测试 |

约束：只支持 MySQL 8 + Redis 7；不追求高并发极限，但要求并发安全（事务、原子限流、会话一致性）。

---

## 2. 分层与依赖方向

```
                    ┌───────────────────────────┐
                    │        cmd/govulnlab      │  组装依赖、解析子命令、优雅关闭
                    └────────────┬──────────────┘
                                 │
        ┌────────────────────────▼────────────────────────┐
        │                internal/handler                 │  只做 HTTP 语义：解析、渲染、状态码
        │        （页面 handler + JSON API handler）       │
        └───────┬────────────────────────────────┬────────┘
                │                                │
   ┌────────────▼─────────────┐      ┌──────────▼─────────────┐
   │     internal/middleware  │      │     internal/lab       │  漏洞靶场（独立挂载）
   │ ID→IP→Recover→Log→Header │      │  guard + 5 个模块      │
   │ →BodyLimit→Session→Limit │      └──────────┬─────────────┘
   └────────────┬─────────────┘                 │
                │                               │
        ┌───────▼───────────────────────────────▼────────┐
        │               internal/service                 │  业务规则：策略、校验、编排
        │  AuthService / OrderService / Guestbook / Audit │
        └───────────────────┬────────────────────────────┘
                            │
        ┌───────────────────▼────────────────────────────┐
        │                internal/store                  │  数据访问：SQL、事务、Redis
        │  user / order / guestbook / audit / lab         │
        └───────────────────┬────────────────────────────┘
                            │
     ┌──────────────────────▼─────────────────────────────┐
     │  基础能力（被各层使用，不反向依赖业务）             │
     │  config  logger  apperr  reqctx  session  httpx     │
     │  webui（模板渲染）  migrations（embed SQL）         │
     └────────────────────────────────────────────────────┘
```

依赖规则（也是 code review 的检查项）：

1. **handler 不写 SQL、不做业务判断**：它只把 `request` 变成 `service 调用`，把结果变成 `response`。
2. **service 不碰 `http.ResponseWriter`**：错误通过 `apperr` 向上抛，由 handler 决定 302 还是 JSON。
3. **store 不 import service**：数据层不知道业务规则，只负责“按条件读写”。
4. **基础包不 import 业务包**：`reqctx` 不知道什么是订单，`session` 不知道什么是角色权限。
5. **依赖通过 `Deps` 显式注入**，没有包级全局变量（除了 `slog` 默认 logger 这种框架级约定）。

---

## 3. 一次请求的完整生命周期

中间件链在 `cmd/govulnlab/main.go` 里按从外到内的顺序组装：

```
RequestID → ClientIP → Recover → AccessLog → SecurityHeaders
          → BodyLimit → LoadSession → GlobalRateLimiter → ServeMux
```

每条顺序都有理由，换个位置就会出问题：

| 顺序 | 中间件 | 为什么必须在这个位置 |
| --- | --- | --- |
| 1 | `RequestID` | 后面所有日志、审计、错误响应都要带它，必须最先注入 ctx |
| 2 | `ClientIP` | 限流、审计、登录计数都按 IP 记账，必须在它们之前算好；`trust_proxy=false` 时不信任 `X-Forwarded-For`，防止伪造绕过限流 |
| 3 | `Recover` | 要在业务之前“包住”panic，但又在日志之前，才能让日志记录到 500 而不是连接被重置 |
| 4 | `AccessLog` | 需要拿到最终状态码，所以包在业务外层；用 `statusRecorder` 包装 ResponseWriter |
| 5 | `SecurityHeaders` | 给每个响应（包括错误页）都带上安全头与 CSP nonce；nonce 写入 ctx 供模板使用 |
| 6 | `BodyLimit` | 在读 body 之前限制大小，避免 `ParseForm` 把内存吃光 |
| 7 | `LoadSession` | 从 Redis 读会话写入 ctx；必须早于鉴权、CSRF、handler |
| 8 | `GlobalRateLimiter` | 放在会话之后、路由之前：合法用户也要限流，但它不该拖慢健康检查（探针单独不受限） |

请求进入 `ServeMux` 后：路由匹配 → 路由级中间件（`RequireAuth` / `RequireAdmin` / `CSRFGuard`）→ handler → service → store。

**双模式中间件**：`RequireAuth(ModePage)` 未登录时 302 到 `/login?next=...`，`RequireAuth(ModeAPI)` 返回 401 JSON。同一套鉴权逻辑，两种表现，避免前端脚本拿到一个登录页 HTML。

---

## 4. 数据模型

```
users(id, username UK, password_hash, role, status, created_at, updated_at)
  └─ orders(id, order_no UK, user_id FK, goods_name, amount_cent, status, remark, ...)
  │     KEY idx_user_created(user_id, created_at)
  └─ guestbook(id, user_id FK, content, created_at)
audit_log(id, user_id, action, detail, ip, ua, request_id, created_at)
  │  KEY idx_user_time(user_id, created_at) / idx_action_time(action, created_at)
  └─ 注意：user_id 故意不建外键（登录失败事件没有合法用户）

-- 靶场独立表，与业务表零耦合
lab_users(id, username, password, role)          -- 明文，仅演示
lab_orders(id, user_id, order_no, goods_name, amount_cent, status)
lab_notes(id, author, content, created_at)       -- 存储型 XSS 用
schema_migrations(version PK, applied_at)        -- 迁移版本记录
```

设计说明：

- **金额 `amount_cent BIGINT`**：货币绝不能用 float。`0.1 + 0.2 = 0.30000000000000004` 在账务上是事故。解析层 `ParseAmount` 自己按字符串拆整数/小数部分，拒绝 `1e3`、拒绝超过两位小数、拒绝负数与 0。
- **业务号 vs 自增 id**：`order_no` 是对外展示号（时间 + 随机），自增 id 只在内部用，减少被顺序遍历的信息泄露。靶场里故意暴露自增 id，用来演示 IDOR。
- **`status TINYINT` 而非字符串**：状态机只有三种取值，用常量 `OrderStatusPending/Paid/Canceled` 收敛，模板通过 `StatusText()` 翻译，避免 UI 层散落魔法数字。
- **靶场表独立**：靶场口令是明文、查询是拼接的。如果混进 `users`，一次误配置就可能把真实账号体系拖下水。物理隔离让“关掉靶场”变成真正的 `lab.enabled=false`。

---

## 5. 关键取舍（面试常被追问的部分）

### 5.1 为什么不用 ORM？

- 需要精确控制 SQL：索引、`LIMIT/OFFSET`、事务边界、`WHERE user_id = ?` 把归属判断下推到数据库；
- 需要向面试官展示“我知道 SQL 注入发生在哪一行、参数化为什么能防住”，ORM 会把这段黑盒化；
- 数据量小、查询简单，ORM 的收益不足以抵消可读性损失。

### 5.2 为什么不用 JWT，而用 Redis 会话？

| 维度 | JWT | Redis 会话（本项目） |
| --- | --- | --- |
| 撤销 | 需要黑名单，等于又引入中心存储 | 删除 key 即失效 |
| 改密踢下线 | 难（旧 token 仍然有效直到过期） | 改密时 `Destroy` 旧会话，立刻失效 |
| 会话固定防护 | 需要额外设计 | 登录时换新 ID 天然解决 |
| 水平扩展 | 无状态，天然好 | 依赖共享 Redis（本项目已共享） |

结论：这是一个“用户能改密、管理员能封号”的系统，**可撤销性比无状态更重要**。JWT 更适合服务间鉴权、短生命周期凭证。

### 5.3 为什么迁移器自己拆 SQL，而不是开 `multiStatements=true`？

`multiStatements=true` 会让一次 `Exec` 可以执行多条语句，**这恰好放大了 SQL 注入的危害**（注入点从“改一个查询”变成“执行任意语句”）。
本项目坚持关闭该选项，在 `internal/store/migrate.go` 里用 `splitStatements()` 按分号拆分（跳过 `--` 注释、忽略字符串字面量里的分号），迁移脚本因此可以正常多语句执行，而运行期连接仍然是“一条语句一次执行”。

### 5.4 为什么模板和 SQL 要 `go:embed`？

- 部署简单：一个二进制 + 一个配置文件，不需要同步模板目录；
- 不会漂移：模板改了必须重新编译，避免“线上模板是三个月前的版本”；
- 启动即校验：`webui.New()` 解析全部模板，模板语法错误会在启动时暴露（并有 `TestTemplatesParse` 兜底）。

### 5.5 为什么手写中间件而不是引入 gin/echo？

- 中间件顺序、`statusRecorder`、`http.Server` 超时、优雅关闭这些细节，用标准库写一遍才真的懂；
- 依赖少 = 供应链风险小、构建快、安全审计面小；
- Go 1.22 的 `POST /orders/{id}/cancel` 路由已经够用，不需要第三方路由树。

---

## 6. 并发与一致性

| 场景 | 风险 | 处理 |
| --- | --- | --- |
| 下单 + 审计 | 订单写成功、审计写失败 → 审计缺口 | 同一事务提交，任一失败整体回滚（`store.CreateOrder`） |
| 取消订单 | “先查归属再更新”存在 TOCTOU，两个请求可能都通过检查 | 更新语句带 `AND status = ?`，用 `RowsAffected()==0` 判断状态已变；归属校验在同一事务内完成 |
| 限流 | 读-改-写计数在并发下会超发 | Redis Lua 脚本一次性完成“取令牌 + 回填时间戳”，原子执行 |
| 登录失败计数 | 并发失败可能少记 | `INCR` 本身原子；首次设置 TTL 用 `n==1` 判断 |
| 会话续期 | 每次请求都写 Redis 会放大写压力 | `Renew` 只在剩余 TTL 低于阈值时续期（滑动过期） |
| 优雅关闭 | 直接 kill 会截断在途请求 | `signal.NotifyContext` + `srv.Shutdown(10s)`，先停止接收新连接再等在途完成 |

---

## 7. 可观测性

- **request_id**：入口生成（或透传上游），贯穿访问日志、业务日志、审计表 `audit_log.request_id`，出问题可以用一个 ID 把“访问日志 → 业务日志 → 审计记录”串起来。
- **结构化日志**：`slog` 输出 `time/level/msg/key=value`，生产切 `json` 可直接被 Loki/ELK 采集。
- **访问日志字段**：method、path、status、bytes、cost_ms、ip、ua、request_id。
- **审计与访问日志分离**：访问日志是“运维视角”（谁访问了什么），审计是“合规视角”（谁做了什么敏感操作），后者落库、可查询、不可被日志轮转丢掉。
- **探针分层**：`/healthz` 只表示进程活着（不查依赖，避免依赖抖动导致容器被误杀）；`/readyz` 检查 MySQL + Redis（依赖不可用就不接流量）。

---

## 8. 扩展方向（当前未实现）

1. **审计异步化**：写队列（Redis Stream / Kafka）→ 批量落库 + 按月分区，解决写热点。
2. **多租户/多角色**：`role` 目前只有 user/admin，可扩展为 RBAC 表（角色-权限-资源）。
3. **可观测指标**：`/metrics` 暴露 QPS、P99、限流拒绝数、登录失败数。
4. **分布式限流的一致性**：当前按单 IP 令牌桶，多实例共享同一 Redis key，天然一致；若要按用户维度限流需引入用户 ID 作为 key 维度。
5. **密钥管理**：数据库口令目前走环境变量，生产可接 Vault/KMS。
