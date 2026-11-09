# 完整全流程讲解

从「敲下启动命令」到「最后一个请求处理完」，把整个链路走一遍。每一步都标了对应文件，方便对照代码看。

---

## 0. 全景图

```
进程启动
  ├─ 解析参数（-config / -cmd / seed 相关）
  ├─ config.Load      → 默认值 → config.yaml → 环境变量 → Validate()
  ├─ logger.New       → slog（text/json）
  ├─ store.New        → MySQL 连接池 + Redis 客户端 + Ping（5s 超时）
  └─ 子命令分发
       ├─ migrate → 读 embed SQL → 查 schema_migrations → 逐条事务执行
       ├─ seed    → migrate + 幂等写入管理员/演示用户/靶场账号/靶场订单
       └─ serve   → 组装 service/handler → 建中间件链 → http.Server → 等信号

单个请求
  RequestID → ClientIP → Recover → AccessLog → SecurityHeaders
            → BodyLimit → LoadSession → GlobalRateLimiter
            → ServeMux 路由 → [RequireAuth / RequireAdmin / CSRFGuard]
            → handler → service → store(MySQL/Redis) → 渲染 or JSON
```

---

## 1. 启动流程（`cmd/govulnlab/main.go`）

### 1.1 参数与配置

```
flag: -config（默认取 GVL_CONFIG，否则 config.yaml）
      -cmd=serve|migrate|seed
      -admin-user / -admin-password / -demo-user / -demo-password（仅 seed 用）
```

`config.Load()` 的合并顺序是 **默认值 → 文件 → 环境变量**。所有键都有默认值，所以删掉 `config.yaml` 也能起（但连不上库）。

`Validate()` 在启动阶段就拦住错误配置：端口范围、超时为正、`login_max_fail > 0`、`password_min_len >= 8`、`global_rate_per_sec > 0`。**配置错误在启动时爆炸，而不是等第一个用户登录时才炸。**

日志里打印的是 `SafeDSN()`（`root@tcp(...)/govulnlab`），口令永远不出现在日志里。

### 1.2 依赖初始化

`store.New(ctx, cfg)` 做三件事：

1. `sql.Open("mysql", cfg.DSN())` 建连接池，设置 `MaxOpenConns/MaxIdleConns/ConnMaxLifetime/ConnMaxIdleTime`；
2. 建 `redis.Client`，设置连接池大小；
3. 5 秒超时的 `Ping` 验证两端可用——**快速失败**，不要带着坏连接启动然后再慢慢报错。

数据库连接串里带 `parseTime=true&loc=Local&charset=utf8mb4`，否则 `DATETIME` 扫不进 `time.Time`、中文会乱码。

### 1.3 serve 分支的组装

```
session.Manager   ← Redis + cookie 配置
AuthService       ← store + logger + security 配置（含 dummyHash）
OrderService / GuestbookService / AuditService
handler.Deps      ← 以上全部 + 模板渲染器
handler.NewRouter ← 注册路由，挂靶场
middleware.Chain  ← 8 层中间件
http.Server       ← ReadHeaderTimeout=5s / Read/Write/Idle 超时 / ErrorLog 接 slog
```

`ReadHeaderTimeout=5s` 专门防「Slowloris 慢速头发送」：攻击者用极慢速度发 header 占住连接，没有这个超时几十个连接就能拖垮服务。

### 1.4 优雅关闭

```
signal.NotifyContext(SIGINT, SIGTERM)
  ├─ 正常服务：ListenAndServe 阻塞
  └─ 收到信号：srv.Shutdown(10s 超时)
       → 停止接受新连接
       → 等在途请求处理完
       → 超时则强制退出并记 Error 日志
       → defer st.Close() 关闭数据库连接池
```

为什么不是 `os.Exit`：直接退出会让正在提交的订单事务被数据库判为连接中断，用户看到失败但数据可能已部分写入（虽然事务会回滚，但客户端拿不到明确的响应）。

---

## 2. 单个请求的旅程

### 2.1 中间件逐层

| 层 | 做什么 | 失败时 |
| --- | --- | --- |
| RequestID | 取 `X-Request-Id`（校验格式）或生成 UUID，写 ctx 与响应头 | —— |
| ClientIP | `trust_proxy` 为真才解析 `X-Forwarded-For` 第一段，否则用 `RemoteAddr` | —— |
| Recover | `defer recover()`，把 panic 转成 500 + 日志（含堆栈） | 500 |
| AccessLog | 包 `statusRecorder`，请求结束后记 method/path/status/bytes/cost_ms/ip/ua/request_id | —— |
| SecurityHeaders | 生成 CSP nonce → 写 ctx → 下发 CSP 等头 | —— |
| BodyLimit | `http.MaxBytesReader` 限制 1MiB | 解析 body 时报错 → 400 |
| LoadSession | 读 Cookie → 查 Redis → 反序列化 → 写 ctx | 无会话则 ctx 为空，不阻断 |
| GlobalRateLimiter | Redis Lua 令牌桶按 IP 取令牌 | 429 |

### 2.2 路由级鉴权

- `RequireAuth(ModePage)`：无会话 → `302 /login?next=<原路径>`；
- `RequireAuth(ModeAPI)`：无会话 → `401 {"code":"unauthorized"}`；
- `RequireAdmin`：角色不是 `admin` → 页面 403 / API 403；
- `CSRFGuard`：只对写方法生效，先查 Origin，再比对会话里的 Token（表单 `csrf_token` 或头 `X-CSRF-Token`）。

`next` 参数会经过 `sanitizeNext()`：只接受以 `/` 开头且不是 `//` 的站内路径，否则回落到 `/profile`——防开放重定向。

---

## 3. 登录流程（最值得讲清楚的一条）

```
POST /auth/login  (username, password)
 │
 ├─ 登录限流器：按 IP 取令牌，桶空 → 429
 ├─ handler.loginSubmit：ParseForm → sanitizeNext
 ├─ AuthService.Login(ctx, ip, username, password)
 │    ├─ 空值校验 → 400
 │    ├─ Redis 查失败计数 gvl:login:fail:<ip>:<sha256(username)[:8]>
 │    │     ≥ 5 → 429「登录失败次数过多，请 15 分钟后再试」
 │    ├─ 查用户
 │    │     不存在 → 跑一次 dummyHash 的 bcrypt 比较（等时）→ 计数 +1 → 401 统一话术
 │    ├─ 状态检查：status != 1 → 计数 +1 → 401
 │    ├─ bcrypt.CompareHashAndPassword
 │    │     失败 → 计数 +1 → 401 统一话术
 │    └─ 成功 → DEL 失败计数 → 返回 user
 ├─ 会话固定防护：若请求已带旧会话 → Destroy 旧 SID
 ├─ Sess.Create(user) → 新 SID（32 字节随机，base64url）
 ├─ SetCookie：Path=/ HttpOnly SameSite=Lax MaxAge=2h（生产加 Secure）
 ├─ 审计：action=login，含 ip/ua/request_id
 └─ 302 → next（默认 /profile）
```

**三个容易讲错的细节**：

1. **为什么要 dummyHash**：如果"用户不存在"直接返回，bcrypt 根本不执行，响应会明显更快；攻击者用时间差就能判断账号是否存在。给不存在的用户也跑一次同 cost 的比较，把两条路径的耗时拉平。
2. **为什么失败计数 key 要哈希用户名**：Redis key 常被 `--scan` 导出到监控面板或被人 `MONITOR`，明文用户名会泄露"谁在被撞库"。
3. **为什么登录成功后要删计数**：否则用户偶发输错几次，之后每次登录都被"历史失败"拖着，体验和安全双输。

---

## 4. 注册流程

```
POST /auth/register
 ├─ 登录限流器（与登录共用一个桶，防批量注册）
 ├─ ValidateUsername：^[a-zA-Z0-9_]{3,32}$（ASCII，避免同形字/大小写折叠绕过）
 ├─ ValidatePassword：长度 ≥ 10、含字母与数字、不能单字符重复、≤ 72 字节（bcrypt 上限）
 ├─ bcrypt.GenerateFromPassword(cost=12)（约 100~300ms，故意慢）
 ├─ store.CreateUser → 唯一索引冲突 → ErrDuplicate → 409「用户名已存在」
 └─ 302 → /login?registered=1
```

bcrypt cost=12 的代价是每次注册/登录 100ms 级；这正是对抗离线爆破的关键——**让攻击者的每次尝试都变贵**。

---

## 5. 下单流程（事务与审计）

```
POST /orders (goodsname, amount, csrf_token)
 ├─ RequireAuth + CSRFGuard
 ├─ OrderService.Create
 │    ├─ 商品名去空白、1~128 字符（按 rune 计，中文算 1 个）
 │    ├─ ParseAmount("99.99") → 9999 分
 │    │     拒绝：空、0、负数、科学计数法、超过两位小数、> 1 亿分（100 万元）
 │    └─ 生成 order_no（时间 + 随机），构造 Order{Status: pending}
 ├─ store.CreateOrder（同一事务）
 │    ├─ INSERT orders(...)
 │    ├─ INSERT audit_log(action=create_order, request_id, ip, ua)
 │    └─ COMMIT（任一步失败 → ROLLBACK）
 └─ 302 → /orders
```

JSON API 走同一套 service，只是返回 `201 + {"code":"created","data":{...}}`。

**为什么审计必须同事务**：如果订单成功、审计失败，事后追查"谁在什么时候下了什么单"就出现空洞。安全审计的价值在于**完整性**，一条丢失的审计记录比没有审计更糟——因为它会让人误以为"没有发生"。

### 5.1 取消订单（TOCTOU 处理）

```
CancelOrder(userID, orderID) 事务内：
  SELECT user_id, status WHERE id=?
     ├─ 无记录 → ErrNotFound
     ├─ owner != userID → ErrNotOwner
     ├─ status != pending → ErrInvalidState
  UPDATE orders SET status=canceled WHERE id=? AND status=pending
     └─ RowsAffected()==0 → 状态已被并发改动 → ErrInvalidState
  INSERT audit_log(action=cancel_order)
  COMMIT
```

handler 把 `ErrNotFound / ErrNotOwner` 统一映射成 **404**（不泄露"存在但不属于你"），`ErrInvalidState` 映射 409。

---

## 6. 订单详情与越权防护

```
GET /orders/{id}
 ├─ RequireAuth
 ├─ parseID：非正整数 → 404（不是 400，避免探测）
 ├─ OrderService.Detail(userID, id)
 │    └─ store.GetOrderByIDAndUser：WHERE id=? AND user_id=?
 └─ 查不到 → 404；查到 → 渲染详情
```

关键点：**归属条件写进 SQL**，而不是"先查出来再在 Go 里 if 判断"。后者在多副本、读写分离场景下容易漏，且代码 review 时不容易一眼看出。

---

## 7. 改密流程（会话轮换）

```
POST /account/password (old, new, csrf_token)
 ├─ RequireAuth + CSRFGuard
 ├─ ChangePassword
 │    ├─ 新旧非空、新密码过策略、新旧不能相同
 │    ├─ 查当前用户 → bcrypt 校验旧口令 → 401/400
 │    ├─ bcrypt 新口令 → UPDATE users SET password_hash=?
 │    └─ 返回成功
 ├─ 审计：action=change_password
 ├─ Destroy 当前会话（旧 SID 立刻失效）
 ├─ Create 新会话 + SetCookie
 └─ 302 → /login?changed=1（强制重新登录，让用户确认新口令可用）
```

**为什么必须换会话**：口令泄露往往伴随会话泄露。改密后如果旧会话仍有效，攻击者手里的 Cookie 还能继续用 2 小时。销毁 + 重建让"改密"真正成为一次「踢下线」。

---

## 8. 留言板

```
POST /guestbook (content, csrf_token)
 ├─ RequireAuth + CSRFGuard
 ├─ 去空白、1~1000 字符
 ├─ INSERT guestbook(user_id, content)
 └─ 302 → /guestbook

GET /guestbook
 └─ 查询列表（JOIN users 取用户名）→ html/template 渲染
      {{ .Content }} 由模板自动转义：<script> 变成 &lt;script&gt;
```

靶场 `/lab/xss` 用同一份输入，但输出时走了 `template.HTML`（不转义），于是脚本能执行——这就是两者唯一的差别，也是面试时最容易被追问的点。

---

## 9. 审计查询

- `GET /audit`：只查当前用户（`WHERE user_id = ?`），分页；
- `GET /admin/audit`：全站，按时间倒序，展示 action / user / ip / request_id；
- 用 `request_id` 可以和访问日志对账：`grep <request_id> server.log`。

审计记录字段的取舍：不记口令、不记完整 UA（截断 255）、`detail` 只放必要上下文（如订单 id），避免审计表本身成为新的泄露源。

---

## 10. 靶场流程

### 10.0 守卫（所有 `/lab/**` 共用）

```
if !cfg.Lab.Enabled            → 404
if !cfg.Lab.AllowRemote && !isLoopback(ip) → 403
```

用 404 而不是 403 表示"功能关闭"，是为了不暴露"这里本来有个靶场"。

### 10.1 SQL 注入 `/lab/sqli`

```
输入 username/password → LabQueryUserRaw 拼接 SQL → 命中 → 页面显示「注入成功」+ 返回的用户行
```

讲的时候要强调：这不是"我们的登录接口有洞"，而是**演示区**；业务登录走参数化。冒烟脚本同时验证两边：靶场注入成功、业务接口注入失败。

### 10.2 存储型 XSS `/lab/xss`

```
提交 content → 存 lab_notes → 列表渲染时 template.HTML(content)
```

配合 CSP：即使脚本执行了，`alert(1)` 这种内联脚本在没有 nonce 的情况下也会被 CSP 拦下——用来讲"纵深防御"：转义是主防线，CSP 是兜底。

### 10.3 CSRF `/lab/csrf` + `/lab/csrf/attacker`

```
受害者登录 → 打开攻击者页面（自动提交表单到 /lab/csrf/profile）→ 昵称被改
```

对比业务改密接口：同样跨站提交，缺 Token 直接 403。

### 10.4 水平越权 `/lab/idor`

```
?id=1 → 直接查 lab_orders → 显示「读到订单」，包括不属于当前用户的
```

业务订单接口同样构造 → 404。

### 10.5 暴力破解 `/lab/bruteforce`

```
每次提交 attempts+1 → 用户名口令比对 lab_users（明文）→ 命中显示「命中口令」
```

无计数、无锁定、无延迟。可用 `scripts/lab-bruteforce.ps1` 演示字典攻击；对照 `/auth/login` 的 5 次锁定。

---

## 11. 关闭流程

```
Ctrl+C / systemctl stop
 → ctx Done
 → 打日志「收到退出信号，开始优雅关闭」
 → srv.Shutdown(10s)：拒绝新连接、等在途请求
 → 关闭 DB 连接池与 Redis
 → 进程退出，systemd 不再重启（exit 0）
```

如果是 `kill -9` 或 OOM：事务未提交的会回滚，Redis 里未过期的会话仍在（用户无感），但访问日志可能丢最后几条——这就是为什么**审计要落库**、不能只写日志。

---

## 12. 从零复现（面试现场版）

```bash
# 1. 起依赖（Docker 方式最稳）
docker compose up -d mysql redis

# 2. 建库 + 迁移 + 种子
export GVL_DB_PASSWORD='***'
go run ./cmd/govulnlab -cmd=seed

# 3. 起服务
go run ./cmd/govulnlab -cmd=serve

# 4. 验证（另一个终端）
bash scripts/smoke.sh          # 期望 26 通过 / 0 失败

# 5. 打开浏览器：http://127.0.0.1:8080
#    业务：admin / Admin@123456
#    靶场：/lab → 五个模块逐个点
```

现场演示建议顺序：**首页 → 登录 → 下单 → 详情 → 越权 404 → 靶场注入 → 靶场 XSS → 锁定 429 → 审计页 → 关靶场开关**。这条动线能在 3 分钟内把"安全实现"和"漏洞对照"都讲完。
