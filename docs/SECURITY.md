# 安全设计与验证

本文档回答三个问题：**这个系统假设攻击者能做什么**、**每个漏洞是怎么修的**、**怎么证明它真的修好了**。

---

## 1. 威胁模型

| 维度 | 假设 |
| --- | --- |
| 攻击者能力 | 能匿名访问所有公开接口；能注册普通账号；能构造任意 HTTP 请求（含跨站请求）；能控制自己的输入（表单、JSON、URL 参数） |
| 攻击者不能做 | 读取服务端内存/文件系统；拿到数据库直连；拿到其他用户的会话 Cookie |
| 保护资产 | 用户口令、会话凭证、订单数据（尤其他人订单）、审计日志完整性 |
| 信任边界 | 公网 → Nginx（TLS 终止、边缘限流）→ 应用（鉴权、CSRF、限流）→ MySQL/Redis（仅内网） |
| 不在范围 | 主机层提权、供应链投毒、DDoS 流量清洗（属于基础设施职责） |

一个明确的设计前提：**所有输入都是恶意的**。因此校验发生在 service 层（业务规则）与 store 层（参数化 SQL）两处，而不是只在页面上加个 `maxlength`。

---

## 2. 五个漏洞：成因 → 修复 → 验证

### 2.1 SQL 注入

**成因**（靶场 `internal/lab/sqli.go`，真实缺陷来自原始项目 `handler/loginbad.go`）：

```go
query := fmt.Sprintf("SELECT id, username, password, role FROM lab_users WHERE username = '%s' AND password = '%s'", username, password)
```

输入 `admin' OR '1'='1' -- ` 后，`WHERE` 条件被逻辑短路，`--` 注释掉后半段，任意口令都能登录。

**修复**（`internal/store/user.go`）：

```go
row := s.DB.QueryRowContext(ctx, `SELECT id, username, password_hash, role, status FROM users WHERE username = ?`, username)
```

参数化查询让输入永远被当作**数据**，而不是 SQL 语法的一部分。它之所以有效，不是因为“过滤了引号”，而是因为驱动把 SQL 结构与参数值分两次传给数据库。

**额外措施**：

- 全项目只有 `store.LabQueryUserRaw` 一处允许拼接（靶场专用，函数注释里写明“禁止在其他地方模仿”）；
- 数据库连接**不开** `multiStatements`，即使某天漏了一处拼接，危害也被限制在单条语句内；
- 所有查询走 `database/sql` 的 `QueryContext`，超时与取消由 ctx 控制。

**验证**：

```bash
# 靶场：注入成功
curl -s -X POST http://127.0.0.1:8080/lab/sqli --data-urlencode "username=admin' OR '1'='1' -- " -d "password=x" | grep 注入成功

# 业务接口：同样输入只会得到“用户名或密码错误”，且不返回 SQL 报错
curl -s -X POST http://127.0.0.1:8080/auth/login --data-urlencode "username=admin' OR '1'='1' -- " -d "password=x"
```

### 2.2 存储型 XSS

**成因**（靶场 `internal/lab/xss.go`）：把用户提交的内容标记成 `template.HTML` 直接输出，`<script>` 会被浏览器执行；即使换成 `text/template`，只要不转义也是同样的结果。原始项目里是 `fmt.Fprintf(w, "<p>%s</p>", content)` 直接拼 HTML。

**修复**：

1. 业务页面统一用 `html/template`（`internal/webui/render.go`），模板里的 `{{ .Content }}` 会按上下文自动转义；
2. 留言内容额外做长度限制与去空白；
3. 纵深防御：CSP 使用**每请求随机 nonce**，注入的 `<script>` 没有 nonce 不会执行（`internal/middleware/headers.go`）。

**验证**：

```bash
# 靶场：脚本原样输出（可执行）
curl -s -X POST http://127.0.0.1:8080/lab/xss -d 'content=<script>alert(1)</script>' | grep -F '<script>'

# 业务留言板：脚本被转义成实体
curl -s -b cookies -X POST http://127.0.0.1:8080/guestbook -d "csrf_token=$TOKEN" --data-urlencode 'content=<script>alert(1)</script>'
# 返回的页面里应是 &lt;script&gt;alert(1)&lt;/script&gt;
```

### 2.3 CSRF

**成因**（靶场 `internal/lab/csrf.go`）：改昵称接口只依赖会话 Cookie，不校验任何 Token；浏览器对跨站 POST 会自动带 Cookie，攻击者页面因此可以替受害者改数据。

**修复**（`internal/middleware/csrf.go`）：

- **会话内同步 Token**：登录时生成随机 Token 存进会话，渲染页面时写入隐藏字段 `csrf_token`，提交时服务端比对；
- 比较用 `crypto/subtle.ConstantTimeCompare`，长度不同直接拒绝，避免通过响应时间猜 Token；
- AJAX 支持 `X-CSRF-Token` 头；
- **Origin 校验**：请求带 `Origin` 头时比对 `Host`，不匹配直接 403（纵深防御，不依赖浏览器一定发 Origin）；
- **Cookie 属性**：`HttpOnly`（XSS 拿不到会话）、`SameSite=Lax`（跨站 POST 不携带）、`Secure`（生产开 HTTPS 后开启）、不设 `Domain`（避免子域共享）。

**为什么不用「双提交 Cookie」**：Token 存在会话里，攻击者既读不到 Cookie（HttpOnly）也读不到会话内容，比“Cookie 与表单各存一份”更抗子域注入。

**验证**：

```bash
# 正确：带 Token → 302 成功
# 错误：不带 Token → 403，且响应体是明确的拒绝提示
curl -s -o /dev/null -w "%{http_code}\n" -b cookies -X POST http://127.0.0.1:8080/orders -d 'goodsname=x&amount=1.00'
# 期望 403
```

### 2.4 水平越权（IDOR）

**成因**（靶场 `internal/lab/idor.go`）：`SELECT ... FROM lab_orders WHERE id = ?`，只按 id 查，不问“这个订单是不是你的”。原始项目 `OrderDetailHandler` 是同样的毛病。

**修复**（`internal/service/order.go` + `internal/store/order.go`）：

- 业务查询把归属条件写进 SQL：`WHERE o.id = ? AND o.user_id = ?`；
- 取消订单在事务内先校验归属与状态，再 `UPDATE ... WHERE id=? AND status=?`，用 `RowsAffected()==0` 判断并发状态变化（避免 TOCTOU）；
- **越权统一返回 404**，而不是 403：403 等于告诉攻击者“这个 id 存在但不属于你”，本身就是信息泄露。

**验证**：

```bash
# 用 admin 登录后访问 demo 的订单 id → 期望 404
curl -s -o /dev/null -w "%{http_code}\n" -b admin_cookies http://127.0.0.1:8080/api/orders/<demo的订单id>
```

### 2.5 暴力破解 / 撞库

**成因**（靶场 `internal/lab/bruteforce.go`）：失败不计数、不锁定、无延迟、错误信息还能区分“用户不存在/密码错误”。

**修复**（`internal/service/auth.go`）：

1. Redis 计数：key 为 `gvl:login:fail:<ip>:<username的SHA256前8字节>`（不把用户名明文写进 key，因为 key 可能被导出到监控面板）；
2. 达到 `login_max_fail`（默认 5）后返回 429，锁定 `login_block_minutes`（默认 15 分钟），登录成功即清零；
3. **等时比较**：用户不存在时也跑一次 bcrypt 比较（`dummyHash`），让“账号不存在”和“密码错误”的响应时间接近，抗时序侧信道；
4. **统一话术**：“用户名或密码错误”，不区分具体原因，抗账号枚举；
5. **双层限流**：登录接口单独令牌桶（约 5 次/分钟），全局限流器兜底，防止换账号名把 bcrypt 与 Redis 打满；
6. 所有登录尝试（成功/失败）写审计，带 IP、UA、request_id。

**验证**（实测记录）：

```
attempt 1: HTTP 401  用户名或密码错误
attempt 2: HTTP 401  用户名或密码错误
attempt 3: HTTP 401  用户名或密码错误
attempt 4: HTTP 401  用户名或密码错误
attempt 5: HTTP 401  用户名或密码错误
attempt 6: HTTP 429  登录失败次数过多，请 15 分钟后再试
attempt 7: HTTP 429  登录失败次数过多，请 15 分钟后再试
```

---

## 3. 纵深防御清单

| 层面 | 措施 | 代码 |
| --- | --- | --- |
| 传输 | HTTPS 由 Nginx 终止；生产 `session.secure=true` | `deploy/nginx/govulnlab.conf` |
| 会话 | 256bit 随机 ID、`gvl:session:` 前缀、TTL 120 分钟、登录/改密轮换 | `internal/session/session.go` |
| 授权 | 页面与 API 分离的 302/401/403 语义；admin 角色单独校验 | `internal/middleware/auth.go` |
| 输入 | 用户名/口令/金额/长度全部在 service 层校验；JSON 用 `DisallowUnknownFields` | `internal/service`、`internal/httpx` |
| 输出 | `html/template` 转义；JSON 统一信封；错误信息不暴露内部细节 | `internal/webui`、`internal/httpx` |
| 资源 | BodyLimit 1MiB；`http.Server` 读写/空闲超时；`ReadHeaderTimeout=5s`（慢速攻击） | `cmd/govulnlab/main.go` |
| 限流 | Redis Lua 令牌桶（原子），全局 + 登录双桶 | `internal/middleware/ratelimit.go` |
| 响应头 | CSP(nonce)、nosniff、X-Frame-Options=DENY、Referrer-Policy=no-referrer、COOP | `internal/middleware/headers.go` |
| 可用性 | panic 恢复、优雅关闭、`/healthz` 与 `/readyz` 分层探针 | `middleware/recover.go`、`main.go` |
| 审计 | 敏感操作落库，含 request_id 与来源 IP，与访问日志可对账 | `internal/service/audit.go`、`audit_log` 表 |
| 配置 | 启动即校验；`SafeDSN()` 脱敏日志；口令只走环境变量 | `internal/config/config.go` |
| 依赖 | 依赖极少（mysql driver、redis、viper、uuid、x/crypto），审计面小 | `go.mod` |

---

## 4. 靶场隔离（防止“教学后门”变成真后门）

`internal/lab/lab.go` 里的守卫是一个全局前置检查：

```
请求 /lab/**  →  lab.enabled == false ?  → 404（不是 403，不暴露内部结构）
              →  lab.allow_remote == false 且 IP 非 loopback ? → 403
              →  通过
```

- 默认 `allow_remote=false`：只有本机（127.0.0.1/::1）能打开靶场，扫描器从外部访问得到 403；
- 生产配置模板（`deploy/systemd/govulnlab.env`）里直接 `GVL_LAB_ENABLED=false`；
- 靶场数据在独立 `lab_*` 表，明文口令不会污染业务账号；
- 靶场的“不安全代码”集中在一个包内，`grep -r "Sprintf.*SELECT"` 只会命中这一处，便于 code review 时确认没有扩散。

---

## 5. 剩余风险与上线检查表

| 项 | 现状 | 上线前动作 |
| --- | --- | --- |
| 靶场 | 默认开启（本机） | 生产 `GVL_LAB_ENABLED=false` |
| 会话 Secure | 本地 `false` | HTTPS 后设 `GVL_SESSION_SECURE=true` |
| 数据库口令 | 环境变量 | 接 Vault/KMS，限制文件权限 |
| 审计写库 | 同步写 | 高并发下改异步 + 分区 |
| 日志脱敏 | 不记录口令 | 确认新增日志不含敏感字段 |
| 依赖漏洞 | 未接 SCA | CI 增加 `govulncheck` |
| 备份 | 未实现 | MySQL 定时备份 + 恢复演练 |
