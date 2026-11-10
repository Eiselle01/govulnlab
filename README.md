# GoVulnLab

> 一个用 Go 标准库写的 Web 安全练习靶场：**同一套业务，提供「安全实现」与「漏洞实现」两套对照代码**，
> 并把这套代码按生产工程标准交付——分层架构、MySQL 迁移、Redis 会话、CSRF/限流/审计、Docker 部署、CI 与端到端测试。

<p>
  <a href="https://github.com/Eiselle01/govulnlab/actions/workflows/ci.yml"><img alt="CI" src="https://github.com/Eiselle01/govulnlab/actions/workflows/ci.yml/badge.svg"></a>
  <img alt="Go" src="https://img.shields.io/badge/Go-1.26%2B-00ADD8?logo=go&logoColor=white">
  <img alt="MySQL" src="https://img.shields.io/badge/MySQL-8.0-4479A1?logo=mysql&logoColor=white">
  <img alt="Redis" src="https://img.shields.io/badge/Redis-7-DC382D?logo=redis&logoColor=white">
  <img alt="Docker" src="https://img.shields.io/badge/Docker-compose-2496ED?logo=docker&logoColor=white">
  <img alt="Tests" src="https://img.shields.io/badge/go%20test-passing-brightgreen">
</p>

---

## 1. 这个项目解决什么问题

学 Web 安全最容易卡在两个地方：

1. **只会背概念**：知道 SQL 注入是拼接字符串，但没见过它在真实业务代码里长什么样、怎么被修掉、修掉之后怎么证明真的修掉了。
2. **只会写靶场 demo**：一个飘着 `fmt.Println` 的漏洞页，没有任何工程约束——没有事务、没有审计、没有会话管理，学完还是不会写生产代码。

GoVulnLab 把这两件事合到一起：

- **业务是真的**：注册、登录、改密、订单、留言板、审计日志、管理后台，全部走标准分层，SQL 全部参数化；
- **漏洞是可控的**：5 类经典漏洞放在独立的 `lab_*` 表和独立路由下，默认只允许本机访问，一条配置就能整体关掉；
- **对比是显式的**：每个漏洞都能在代码里找到「不安全实现」和「安全实现」两段，注释里写清楚差在哪、为什么这么改；
- **工程是完整的**：迁移、种子数据、探针、优雅关闭、结构化日志、Dockerfile、compose、systemd、Nginx、CI、端到端冒烟脚本。

一句话概括：**这不是一个漏洞演示页，而是一个「安全需求如何落地成工程」的最小完整样板。**

---

## 2. 功能地图

### 业务侧（安全实现）

| 模块 | 能力 | 关键实现 |
| --- | --- | --- |
| 账号 | 注册、登录、登出、改密 | bcrypt cost=12、密码策略、登录失败锁定、会话轮换 |
| 订单 | 下单、列表、详情、取消、JSON API | 金额用「分」、归属校验、状态机、下单与审计同事务 |
| 留言板 | 发布、列表 | `html/template` 自动转义，长度校验 |
| 审计 | 我的审计 / 全站审计 | 记录登录成功失败、下单、取消、改密，含 request_id 与来源 IP |
| 后台 | 全站订单、全站审计 | 需 `admin` 角色，页面与 API 分别返回 302/401/403 |

### 安全侧（纵深防御）

| 风险 | 措施 | 代码位置 |
| --- | --- | --- |
| SQL 注入 | 全参数化查询；唯一拼接点在靶场且被隔离 | `internal/store/*.go`、`internal/store/lab.go` |
| 存储型 XSS | `html/template` 上下文转义 + CSP nonce | `internal/webui/render.go`、`internal/middleware/headers.go` |
| CSRF | 会话内同步 Token + `subtle.ConstantTimeCompare` + Origin 校验 | `internal/middleware/csrf.go` |
| 水平越权 | 归属校验下推到 SQL 条件；越权统一 404 | `internal/store/order.go`、`internal/service/order.go` |
| 暴力破解 | Redis 失败计数 + 15 分钟锁定 + 等时 bcrypt | `internal/service/auth.go` |
| 会话固定 | 登录/改密时销毁旧会话、签发新会话 | `internal/handler/pages.go` |
| 会话伪造 | 服务端会话（Redis），Cookie 仅存 256bit 随机 ID | `internal/session/session.go` |
| 账号枚举 | 用户不存在与密码错误同一话术 + 等时比较 | `internal/service/auth.go` |
| 拒绝服务 | Redis Lua 令牌桶（全局 + 登录）、BodyLimit、超时 | `internal/middleware/ratelimit.go`、`bodylimit.go` |
| 信息泄露 | 统一错误响应、`SafeDSN()` 脱敏、审计不存口令 | `internal/apperr`、`internal/config` |
| 点击劫持 | `X-Frame-Options: DENY` + `frame-ancestors 'none'` | `internal/middleware/headers.go` |
| 开放重定向 | `sanitizeNext()` 只允许站内相对路径 | `internal/handler/helpers.go` |

### 靶场侧（故意不安全，默认仅本机）

| 靶场 | URL | 漏洞实现 | 对应修复 |
| --- | --- | --- | --- |
| SQL 注入 | `/lab/sqli` | `fmt.Sprintf` 拼接 `WHERE username='%s'` | `store.GetUserByUsername` 参数化 |
| 存储型 XSS | `/lab/xss` | `template.HTML` 直接输出未转义内容 | `html/template` 默认转义 |
| CSRF | `/lab/csrf` | 改昵称只认 Cookie，不校验 Token/Origin | `CSRFGuard` 中间件 |
| 水平越权 | `/lab/idor` | 只按 `id` 查订单，不看归属 | `OrderService.Detail` 校验归属 |
| 暴力破解 | `/lab/bruteforce` | 无计数、无锁定、无延迟 | `AuthService.Login` 计数 + 锁定 |

---

## 3. 快速开始

### 方式 A：Docker Compose（推荐，一条命令起全套）

```bash
cp .env.example .env          # 修改 MYSQL_ROOT_PASSWORD 等
docker compose up -d --build
docker compose --profile tools run --rm seed    # 迁移 + 演示数据
docker compose logs -f app
# 打开 http://127.0.0.1:8080 ，用 admin / Admin@123456 登录
```

### 方式 B：本地进程（已有 MySQL / Redis）

```bash
# 1. 建库（只需一次）
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS govulnlab DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"

# 2. 用环境变量传口令，避免写进配置文件
export GVL_DB_PASSWORD='你的数据库口令'      # Windows PowerShell: $env:GVL_DB_PASSWORD="..."
go run ./cmd/govulnlab -cmd=seed             # 迁移 + 演示数据
go run ./cmd/govulnlab -cmd=serve            # 启动，默认 :8080
```

Windows 上跑端到端冒烟：

```powershell
pwsh -File scripts/smoke.ps1
```

### 演示账号

| 账号 | 口令 | 角色 | 说明 |
| --- | --- | --- | --- |
| `admin` | `Admin@123456` | admin | 业务管理员，可看全站订单/审计 |
| `demo` | `Demo@123456` | user | 普通用户 |
| `labadmin` | `Lab@123456` | admin | **仅靶场表**，明文存储，用于注入演示 |
| `labuser` | `Lab@123456` | user | **仅靶场表**，明文存储，用于暴破演示 |

> 口令都可以通过 `GVL_SEED_ADMIN_PASSWORD` / `GVL_SEED_DEMO_PASSWORD` 覆盖；首次跑完请立即改掉。
> 靶场账号与业务账号**不同表、不同校验逻辑**，不会污染真实账号体系。

---

## 4. 目录结构

```
cmd/govulnlab/           唯一入口：serve / migrate / seed 三个子命令
internal/
  config/                配置加载 + 校验 + DSN 拼接（viper，GVL_ 前缀环境变量）
  logger/                slog 初始化（text / json）
  apperr/                业务错误类型（Code + HTTP Status + 对外文案）
  reqctx/                请求级上下文：request_id / client_ip / session / csp nonce
  session/               Redis 会话管理（Create/Get/Renew/Destroy + Cookie 属性）
  httpx/                 统一 JSON 信封、分页解析、严格 JSON 解码
  store/                 数据访问：MySQL + Redis，事务与哨兵错误
  service/               业务规则：认证策略、订单、留言、审计、金额解析
  middleware/            中间件：request_id、真实 IP、recover、访问日志、安全头、
                         体积限制、会话加载、鉴权、CSRF、限流
  handler/               路由与 HTTP 语义（页面 + JSON API）
  lab/                   漏洞靶场（独立埋点，带总开关与来源限制）
  webui/                 模板渲染器（模板函数、Page 上下文、nonce 注入）
  seed/                  幂等种子数据
migrations/              0001_init.sql / 0002_lab.sql，go:embed 进二进制
web/                     templates/*.html + static/app.css，go:embed 进二进制
scripts/                 smoke.ps1 / smoke.sh / lab-bruteforce.ps1
deploy/                  systemd unit、env 模板、nginx 配置
docs/                    架构、安全、部署、全流程、简历材料
```

**为什么模板和 SQL 用 `go:embed`？** 单二进制交付，容器镜像不需要额外挂载文件，也不会出现「部署机少拷了一个模板」这种低级事故。

---

## 5. 配置说明

优先级：**默认值 < `config.yaml` < 环境变量**（前缀 `GVL_`，层级用下划线）。

| 配置项 | 环境变量 | 默认 | 说明 |
| --- | --- | --- | --- |
| `app.addr` | `GVL_APP_ADDR` | `:8080` | 监听地址 |
| `app.body_limit_bytes` | `GVL_APP_BODY_LIMIT_BYTES` | 1 MiB | 请求体上限 |
| `app.auto_migrate` | `GVL_APP_AUTO_MIGRATE` | `true` | 启动时自动迁移（生产可关掉，交给独立 job） |
| `log.level` / `log.format` | `GVL_LOG_LEVEL` / `GVL_LOG_FORMAT` | `info` / `text` | 生产建议 `json` |
| `db.*` | `GVL_DB_HOST` 等 | 本机 3306 | `GVL_DB_PASSWORD` 只走环境变量 |
| `redis.addr` / `redis.db` | `GVL_REDIS_ADDR` / `GVL_REDIS_DB` | `127.0.0.1:6379` / 0 | 会话、限流、登录计数 |
| `session.ttl_minutes` | `GVL_SESSION_TTL_MINUTES` | 120 | 会话有效期 |
| `session.secure` | `GVL_SESSION_SECURE` | `false` | 生产 HTTPS 必须 `true` |
| `security.login_max_fail` | `GVL_SECURITY_LOGIN_MAX_FAIL` | 5 | 失败几次锁定 |
| `security.login_block_minutes` | `GVL_SECURITY_LOGIN_BLOCK_MINUTES` | 15 | 锁定时长 |
| `security.password_min_len` | `GVL_SECURITY_PASSWORD_MIN_LEN` | 10 | 密码长度下限 |
| `security.global_rate_per_sec` / `global_burst` | 同名 | 20 / 40 | 全局限流 |
| `security.trust_proxy` | `GVL_SECURITY_TRUST_PROXY` | `false` | 在 Nginx 后面才开 |
| `lab.enabled` | `GVL_LAB_ENABLED` | `true` | **生产必须 false** |
| `lab.allow_remote` | `GVL_LAB_ALLOW_REMOTE` | `false` | 非本机 IP 访问靶场直接 403 |

启动时 `config.Validate()` 会做区间校验（端口、超时、限流参数、密码长度等），配置写错在启动阶段就失败，而不是运行到一半才炸。

---

## 6. 路由一览

| 方法 | 路径 | 鉴权 | 说明 |
| --- | --- | --- | --- |
| GET | `/` | - | 首页 |
| GET | `/login` `/register` | - | 登录 / 注册页 |
| POST | `/auth/login` | 限流 | 登录 |
| POST | `/auth/register` | 限流 | 注册 |
| POST | `/auth/logout` | 会话 + CSRF | 登出 |
| GET | `/profile` | 会话 | 个人中心 |
| GET/POST | `/account/password` | 会话 + CSRF | 改密 |
| GET/POST | `/orders` | 会话 (+CSRF) | 订单列表 / 下单 |
| GET | `/orders/{id}` | 会话 | 订单详情（越权返回 404） |
| POST | `/orders/{id}/cancel` | 会话 + CSRF | 取消订单 |
| GET/POST | `/api/orders` | 会话 (+CSRF) | JSON 列表 / 下单 |
| GET | `/api/orders/{id}` | 会话 | JSON 详情 |
| POST | `/api/orders/{id}/cancel` | 会话 + CSRF | JSON 取消 |
| GET/POST | `/guestbook` | 会话 (+CSRF) | 留言板 |
| GET | `/audit` | 会话 | 我的审计 |
| GET | `/admin/orders` `/admin/audit` | 会话 + admin | 管理后台 |
| GET | `/healthz` | - | 存活探针 |
| GET | `/readyz` | - | 就绪探针（MySQL + Redis 都通才 200） |
| GET | `/static/` | - | 静态资源 |
| — | `/lab/**` | 会话 + 靶场守卫 | 5 个漏洞模块 |

---

## 7. 质量门禁

```bash
make vet        # go vet
make test       # 单元测试
make test-race  # 竞态检测
bash scripts/smoke.sh          # 端到端（Linux/macOS/CI）
pwsh -File scripts/smoke.ps1   # 端到端（Windows）
```

本项目已在本机真实环境跑通：

- `go build ./...`、`go vet ./...`、`go test ./...` 全绿；
- 真实 MySQL 8.0.45 + Redis 7 上完成 `migrate` → `seed` → `serve`；
- 端到端冒烟 **26 项全部通过**（登录、会话、CSRF 正反例、下单、JSON API、越权 404、后台、5 个靶场、注销）；
- 登录锁定实测：连续 5 次失败后返回 **429 + 15 分钟锁定**。

---

## 8. 文档

| 文档 | 内容 |
| --- | --- |
| [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) | 分层设计、依赖方向、请求生命周期、数据模型、关键取舍 |
| [docs/SECURITY.md](docs/SECURITY.md) | 威胁模型、每个漏洞的「成因 → 修复 → 验证」三段式、纵深防御清单 |
| [docs/DEPLOY.md](docs/DEPLOY.md) | 本地 / Compose / 裸机 systemd+Nginx 三套部署、迁移策略、回滚、运维手册 |
| [docs/FLOW.md](docs/FLOW.md) | 完整全流程讲解：启动 → 登录 → 下单 → 审计 → 靶场 → 关闭 |
| [docs/RESUME.md](docs/RESUME.md) | 简历话术：项目描述、亮点、量化、面试追问准备 |

---

## 9. 已知限制（诚实清单）

- 靶场是**教学用途**，`lab_*` 表故意明文存口令；生产部署必须 `GVL_LAB_ENABLED=false`，否则等于自己开了个后门。
- 单实例会话存 Redis，没有做多活/会话同步；横向扩容依赖 Redis 共享（本设计天然支持，但未做压测）。
- 审计日志同步写库，高并发下会成为写热点；生产建议改异步队列 + 分区表（文档里给了方案，代码未实现）。
- 没有接入完整可观测体系（Prometheus metrics / tracing），目前是结构化日志 + request_id 串联。
- 未实现邮箱验证、找回密码、双因素；这些属于业务扩展，不影响安全骨架的完整性。

## 10. 免责声明

本项目仅用于**本地学习与安全自测**。其中的漏洞实现是故意保留的教学样本，请勿部署到公网。
任何未经授权对他人系统进行渗透测试的行为都是违法的。
