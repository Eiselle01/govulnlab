# 部署手册

本项目提供三套部署方式，按场景选：

| 场景 | 方式 | 适用 |
| --- | --- | --- |
| 本地开发 / 快速演示 | 本地进程（已有 MySQL/Redis） | 面试现场演示、写代码 |
| 一键起全套 | Docker Compose | 换机器复现、给别人跑 |
| 长期运行 | 裸机 + systemd + Nginx | 有公网域名、要 HTTPS |

---

## 0. 前置条件

| 组件 | 版本 | 说明 |
| --- | --- | --- |
| Go | 1.26+ | `go.mod` 里声明的版本优先 |
| MySQL | 8.0+ | 需要 `utf8mb4`，驱动要求 `parseTime=true` |
| Redis | 6.0+ | 会话、限流、登录计数 |
| Docker | 24+（可选） | 仅 Compose 方式需要 |

数据库字符集统一 `utf8mb4 / utf8mb4_general_ci`，否则中文商品名、留言会变成问号。

---

## 1. 方式一：本地进程

### 1.1 建库

```bash
mysql -uroot -p -e "CREATE DATABASE IF NOT EXISTS govulnlab DEFAULT CHARACTER SET utf8mb4 COLLATE utf8mb4_general_ci;"
```

### 1.2 准备配置

`config.yaml` 里不要写口令，用环境变量覆盖：

```bash
# Linux/macOS
export GVL_DB_PASSWORD='***'
# Windows PowerShell
$env:GVL_DB_PASSWORD = '***'
```

### 1.3 迁移 + 种子

```bash
go run ./cmd/govulnlab -cmd=migrate   # 只迁移
go run ./cmd/govulnlab -cmd=seed      # 迁移 + 演示数据（幂等，可重复跑）
```

迁移机制：

- 脚本在 `migrations/*.sql`，通过 `go:embed` 编译进二进制；
- 执行前查 `schema_migrations`，已应用的版本跳过；
- 每个脚本在**独立事务**里逐条执行（`splitStatements` 拆分），失败整体回滚，不会留下半套表；
- 版本号就是文件名，新增迁移只需加一个 `0003_xxx.sql`，不要改历史文件。

### 1.4 启动

```bash
go run ./cmd/govulnlab -cmd=serve
# 或先编译
go build -o bin/govulnlab ./cmd/govulnlab && ./bin/govulnlab -cmd=serve
```

验证：

```bash
curl -s http://127.0.0.1:8080/healthz   # ok
curl -s http://127.0.0.1:8080/readyz    # ready（MySQL + Redis 都通）
```

### 1.5 端到端冒烟

```bash
bash scripts/smoke.sh          # Linux/macOS
pwsh -File scripts/smoke.ps1   # Windows
```

期望输出结尾：`结果：26 通过 / 0 失败`。

---

## 2. 方式二：Docker Compose

```bash
cp .env.example .env
# 编辑 .env：MYSQL_ROOT_PASSWORD 换成强口令
docker compose up -d --build
docker compose ps             # 等 mysql/redis healthy、app running
docker compose --profile tools run --rm seed
docker compose logs -f app
```

要点：

- `mysql` 与 `redis` 端口只绑 `127.0.0.1`，不给公网；
- `app` 通过 `depends_on: condition: service_healthy` 等依赖真的可用再起，避免启动即崩；
- 应用容器内 `GVL_LAB_ALLOW_REMOTE=false`，靶场只在容器网络内可访问；需要外部演示时再显式打开并配合端口映射；
- 数据在 `mysql-data` / `redis-data` 两个 volume，`docker compose down` 不删数据，`down -v` 才会删。

常用运维命令：

```bash
docker compose restart app            # 重启应用
docker compose exec mysql mysql -uroot -p -e "SHOW TABLES FROM govulnlab"
docker compose exec redis redis-cli keys 'gvl:*'
docker compose --profile tools run --rm seed   # 重新灌种子（幂等）
```

---

## 3. 方式三：裸机 systemd + Nginx

### 3.1 安装二进制与配置

```bash
sudo useradd --system --no-create-home --shell /usr/sbin/nologin govulnlab
sudo install -d -o govulnlab -g govulnlab /etc/govulnlab
sudo install -m 0755 bin/govulnlab /usr/local/bin/govulnlab
sudo install -m 0640 -o govulnlab -g govulnlab config.yaml /etc/govulnlab/config.yaml
sudo install -m 0640 -o govulnlab -g govulnlab deploy/systemd/govulnlab.env /etc/govulnlab/govulnlab.env
sudo vi /etc/govulnlab/govulnlab.env      # 填入口令，改 GVL_LAB_ENABLED=false
```

### 3.2 首次迁移

```bash
sudo -u govulnlab env $(sudo cat /etc/govulnlab/govulnlab.env | xargs) \
  /usr/local/bin/govulnlab -config /etc/govulnlab/config.yaml -cmd=migrate
```

或者临时把 `app.auto_migrate` 设为 `true`（默认就是 true），首次启动会自动迁移，跑完再改回 `false`，把迁移权收回到发布流程。

### 3.3 启动服务

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now govulnlab
sudo systemctl status govulnlab
sudo journalctl -u govulnlab -f
```

单元文件里已经做了加固：非 root 运行、`ProtectSystem=strict`、`NoNewPrivileges`、`PrivateTmp`、地址族白名单、`ReadWritePaths` 只放开日志目录。

### 3.4 Nginx + HTTPS

```bash
sudo cp deploy/nginx/govulnlab.conf /etc/nginx/sites-available/govulnlab.conf
sudo ln -s /etc/nginx/sites-available/govulnlab.conf /etc/nginx/sites-enabled/
sudo nginx -t && sudo systemctl reload nginx
sudo certbot --nginx -d govulnlab.example.com
```

配套要改的应用配置：

```
GVL_SECURITY_TRUST_PROXY=true    # 才会信任 X-Forwarded-For
GVL_SESSION_SECURE=true          # Cookie 只在 HTTPS 下发送
GVL_APP_ENV=prod
GVL_LOG_FORMAT=json
```

**`trust_proxy` 的安全含义**：打开后应用会信 `X-Forwarded-For`。如果应用还能被直连（绕过 Nginx），攻击者就能伪造该头绕过 IP 限流。因此打开它的前提是：**应用端口只监听 127.0.0.1，或者安全组只允许 Nginx 访问**。

---

## 4. 发布与回滚

### 4.1 发布流程（推荐）

```
构建 → 单元测试 → 冒烟（staging）→ 迁移（独立 job）→ 滚动替换二进制 → 观察 → 完成
```

1. `make test-race` + `bash scripts/smoke.sh` 必须全绿；
2. 迁移是**先于**新版本启动的独立步骤，避免“新代码访问旧表结构”；
3. 二进制替换用 `install` 到临时路径再 `mv`（原子替换），随后 `systemctl restart`；
4. 观察 `journalctl` 里的 5xx、`readyz` 状态、登录失败率。

### 4.2 回滚

| 变更类型 | 回滚方式 |
| --- | --- |
| 仅代码 | 换回上一个二进制 + `systemctl restart`（迁移脚本应保持向后兼容：只加列不删列） |
| 加了新表/新列 | 旧代码通常仍可运行；不要回滚迁移，等下一个版本再清理 |
| 必须回滚迁移 | 手写 `down` SQL 并先在备份库演练，生产禁止直接 `DROP` |

迁移脚本的硬规则：**新增字段给默认值、不重命名、不删列**。这样任意相邻两个版本都能共存，滚动发布期间不会炸。

### 4.3 备份

```bash
# 每天全量 + binlog 增量
mysqldump -uroot -p --single-transaction --routines --triggers govulnlab | gzip > govulnlab-$(date +%F).sql.gz
```

Redis 只存会话/计数/限流令牌，属于**可丢失数据**：丢了以后用户重新登录即可，不需要备份（这也是选用 Redis 存会话而非存业务数据的原因之一）。

---

## 5. 运维手册（常见故障）

| 现象 | 可能原因 | 处理 |
| --- | --- | --- |
| 启动报 `mysql: Access denied` | 口令没通过环境变量传入，或 user 不对 | `echo $GVL_DB_PASSWORD`；确认 `db.user` |
| 启动报 `执行迁移 0001_init.sql 失败: Error 1064` | 迁移脚本被手改坏，或数据库版本过低 | `mysql --version`；本地先 `-cmd=migrate` 复现 |
| `readyz` 返回 503 | MySQL 或 Redis 不通 | `docker compose ps` / `systemctl status mysql redis`；看日志里的 `mysql:` 或 `redis:` 前缀 |
| 登录突然全部 429 | 触发登录限流或账号锁定 | 看 `audit_log` 的 `login_failure`；等待锁定窗口或清 `gvl:login:fail:*` |
| 页面 500 且日志有 `template` 关键字 | 模板渲染字段缺失 | 本地跑 `go test ./internal/webui/` 快速定位 |
| 中文变问号 | 库/表/连接字符集不一致 | 确认建库 `utf8mb4`、DSN 含 `charset=utf8mb4` |
| 拿不到真实客户端 IP | 未开 `trust_proxy` 或 Nginx 没传头 | 二者必须同时满足 |

清空限流/锁定状态（演示环境可用，生产慎用）：

```bash
redis-cli --scan --pattern 'gvl:login:fail:*' | xargs -r redis-cli del
redis-cli --scan --pattern 'gvl:rl:*'        | xargs -r redis-cli del
```

---

## 6. 横向扩展

- **应用无状态**：会话、限流、计数都在 Redis，直接多副本 + 负载均衡即可；
- **Redis 成为共享依赖**：需要主从/哨兵，避免单点；
- **MySQL 读写分离**：当前所有查询走主库，读多时可以把列表/审计查询指向只读副本（代码里没有强制主库的读，接副本只需改 DSN 或引入两个 `*sql.DB`）；
- **审计表分区**：按 `created_at` 月分区，避免单表无限增长。

---

## 7. Windows 本机部署提示（开发环境）

Windows 没有 systemd，通常直接跑进程：

```powershell
# 起服务
$env:GVL_DB_PASSWORD = '***'
Start-Process -FilePath ".\bin\govulnlab.exe" -ArgumentList "-cmd=serve" -WindowStyle Hidden

# 查端口
Get-NetTCPConnection -State Listen -LocalPort 8080

# 停服务
Get-Process govulnlab | Stop-Process -Force
```

注意：`mysqld`/`redis-server` 如果装成了 Windows 服务，普通权限下 `Start-Service` 可能因权限不足失败；开发时可以直接 `Start-Process` 起进程，或用 Docker Desktop。
