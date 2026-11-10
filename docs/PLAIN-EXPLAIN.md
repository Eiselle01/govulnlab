# 这个项目到底在做什么（大白话版）

> 这份文档不用任何专业术语开场。每一段都对应仓库里的真实代码，你可以边看边打开文件对照。
> 如果只想看结论，读第 1 节和第 7 节就够了。

---

## 1. 一句话：它同时解决两个问题

**问题一：写不出一个"能用"的网站。**
很多学生项目只有几个页面，没有登录态管理、没有事务、没有日志，程序跑起来但一被追问就崩。

**问题二：知道有漏洞，但不会修，也证明不了自己修了。**
看过 SQL 注入的文章，知道"要过滤引号"，但没见过它在真实业务代码里长什么样，更不知道修完之后怎么验证。

这个项目把两件事**放在同一个仓库里对照**：

| 你看到的页面 | 背后是哪份代码 | 作用 |
| --- | --- | --- |
| 登录、下单、改密、后台（正常网站） | `internal/service/`、`internal/store/` | 这是**安全版**：所有输入都被当成"数据"，不会变成"命令" |
| 地址栏加 `/lab` 打开的 5 个页面 | `internal/lab/`、`store/lab.go` | 这是**漏洞版**：故意写错，让你亲手把洞打出来 |

**为什么这样设计有价值**：光有漏洞页 = 玩具；光有业务页 = 看不出你懂安全。两者并排放，你才能说清"这里如果写错会怎样、我怎么改对的、改完怎么验证"。

---

## 2. 五个漏洞，五组代码对照

每组只有三段：**漏洞怎么写 → 安全怎么写 → 一句话差别 → 怎么自己验证**。

### 2.1 SQL 注入：把"数据"变成了"命令"

**漏洞代码**（`internal/lab/sqli.go`，第 34 行）：

```go
rawSQL := "SELECT id, username, role FROM lab_users WHERE username = '" +
    username + "' AND password = '" + password + "' LIMIT 1"
user, err := h.Store.LabQueryUserRaw(r.Context(), rawSQL)
```

用户输入直接**拼进** SQL 字符串。如果输入 `admin' OR '1'='1' -- `，拼出来的句子就变成：

```sql
SELECT ... WHERE username = 'admin' OR '1'='1' -- ' AND password = '' LIMIT 1
```

`OR '1'='1'` 让条件永远成立，`--` 把后面密码判断注释掉。**于是不用知道密码也能登录。**

**安全代码**（`internal/store/user.go`，第 35 行）：

```go
row := s.DB.QueryRowContext(ctx,
    `SELECT id, username, password_hash, role, status, created_at FROM users WHERE username = ?`, username)
```

注意区别：SQL 里写的是 `?`，用户输入是**单独一个参数**传进去的。
数据库驱动会把"句子结构"和"数据"分开发送，所以输入里带引号、带 `OR` 都只是普通文字，永远不会变成 SQL 语法。

**一句话差别**：不是"过滤了引号"，而是**数据和命令被彻底分开**。

**自己验证**：

```powershell
# 1) 靶场：同样的输入能登录进去
curl.exe -s -X POST http://127.0.0.1:8080/lab/sqli --data-urlencode "username=admin' OR '1'='1' -- " -d "password=x"

# 2) 业务登录：同样的输入只会得到"用户名或密码错误"
curl.exe -s -X POST http://127.0.0.1:8080/auth/login --data-urlencode "username=admin' OR '1'='1' -- " -d "password=x"
```

### 2.2 XSS：存进去没关系，**输出**时才出事

**漏洞代码**（`internal/lab/xss.go`，第 25 行）：

```go
unsafe = append(unsafe, htmltemplate.HTML(n.Content))
```

`html/template.HTML` 是一个"免检通行证"：告诉模板引擎"这串内容是可信的 HTML，别转义"。
于是用户提交的 `<script>alert(1)</script>` 会被浏览器**当成代码执行**。

**安全代码**（`web/templates/guestbook.html`，第 21 行）：

```html
<div class="note-body">{{.Content}}</div>
```

没有任何"免检通行证"。`html/template` 在输出时会自动把 `<` 变成 `&lt;`，浏览器只会显示文字，不会执行。

另外还有第二道保险（`internal/middleware/headers.go`）：每个响应带一个随机 `nonce` 的 CSP 头，就算某处漏了转义，注入的 `<script>` 因为没有 nonce 也会被浏览器拦下。

**一句话差别**：XSS 的根因在**输出时没转义**，不是"入库时没过滤"。
（很多团队的错误做法是入库时做 HTML 过滤，结果把正常内容也改坏——`lab/xss.go` 的注释专门写了这一点。）

**自己验证**：打开 `/lab/xss` 提交 `<script>alert(1)</script>` → 弹窗；再去 `/guestbook` 提交同样的内容 → 只显示一串文字。

### 2.3 CSRF：浏览器会自动帮攻击者"带上你的身份"

**漏洞代码**（`internal/lab/csrf.go`）：改昵称接口只看 Cookie 里是谁，**不检查请求是从哪来的**。

攻击者做一个页面（`/lab/csrf/attacker`），里面藏一个自动提交的表单，指向你的改昵称接口。
你只要在登录状态下打开这个页面，浏览器就会**自动带上你的 Cookie**，昵称就被改了——你什么都没点。

**安全代码**（`internal/middleware/csrf.go`，第 46 行）：

```go
if !tokensEqual(token, s.CSRFToken) {
    rejectCSRF(w, r, mode, "CSRF Token 校验失败，请刷新页面重试")
    return
}
```

服务端在会话里存一个随机 Token，页面表单里也放一份，提交时必须对得上。
攻击者的页面在**别的域名**下，读不到你这边的 Token（Cookie 是 HttpOnly，脚本也读不到），所以伪造不出来。

**一句话差别**：Cookie 是"你是谁"，Token 是"这次操作确实是你点的"。前者浏览器会自动送，后者攻击者拿不到。

**自己验证**：`scripts/smoke.ps1` 的第 3 段：不带 Token 的写请求返回 **403**。

### 2.4 越权：只查了"有没有这个订单"，没查"是不是你的"

**漏洞代码**（`internal/store/lab.go`，第 92 行）：

```go
err := s.DB.QueryRowContext(ctx,
    `SELECT id, user_id, order_no, goods_name, amount_cent, status, created_at
     FROM lab_orders WHERE id = ?`, id)
```

只要把 URL 里的 `?id=1` 改成 `?id=2`、`?id=3`，就能把别人的订单一条条翻出来。这正是原始项目里的真实缺陷。

**安全代码**（`internal/store/order.go`，第 62 行）：

```go
q := `SELECT ` + orderColumns + `, u.username
      FROM orders o JOIN users u ON o.user_id = u.id
      WHERE o.id = ? AND o.user_id = ?`
return scanOrder(s.DB.QueryRowContext(ctx, q, orderID, userID))
```

查询时就把"是不是你的"写进条件里。不是你的 → 查不到 → 服务层返回 **404**（不是 403，因为 403 等于告诉攻击者"这个订单存在，只是不属于你"）。

**一句话差别**：不要"先查出来再判断"，**让数据库直接只返回该给你的那条**。

**自己验证**：登录 `admin` 后访问自己订单列表之外的 id，返回 404。

### 2.5 暴力破解：让攻击者试错的成本变高

**漏洞代码**（`internal/lab/bruteforce.go`）：每次提交都老老实实比对账号口令，**不计数、不锁定、不延迟**。
攻击者拿一本字典，一秒钟试几十次，总有一次撞对。

**安全代码**（`internal/service/auth.go`，第 124–127 行）：

```go
key := s.failKey(ip, username)
if n, err := s.failCount(ctx, key); err == nil && n >= int64(s.sec.LoginMaxFail) {
    return nil, apperr.TooManyRequests(fmt.Sprintf("登录失败次数过多，请 %d 分钟后再试", s.sec.LoginBlockMinutes))
}
```

每失败一次就在 Redis 里 +1，超过 5 次锁定 15 分钟。另外还有两个细节：

- 用户不存在时也故意做一次同成本的密码比较（`dummyHash`），让"账号不存在"和"密码错误"**耗时一样**，攻击者没法靠响应快慢判断账号是否存在；
- 提示语统一说"用户名或密码错误"，不告诉对方到底错在哪。

**一句话差别**：不是把密码藏起来，而是**让每次猜都变贵、并且让猜错的人看不出线索**。

**自己验证**：连错 6 次，第 6 次返回 **429**（这条已实测记录在 `docs/SECURITY.md`）。

---

## 3. 一个正常请求，是怎么从浏览器走到数据库的

拿"下单"举例（`POST /orders`）：

```
浏览器表单
  ↓ 1. 中间件（cmd/govulnlab/main.go 里的中间件链）
      先查会话（Redis）→ 再查 CSRF Token → 通过才放行
  ↓ 2. handler（internal/handler/orders.go）
      只负责：读表单、调 service、把结果渲染成页面
  ↓ 3. service（internal/service/order.go）
      业务规则：商品名非空、长度 ≤128、"99.99" 转成 9999 分
  ↓ 4. store（internal/store/order.go）
      一个事务里做两件事：插入订单 + 插入审计日志，任何一步失败全部回滚
  ↓ 5. MySQL
```

两个值得记住的设计：

1. **金额用"分"存整数**（`service/order.go` 里 `ParseAmount`）：
   `0.1 + 0.2 = 0.30000000000000004` 这种浮点误差，在钱上是事故。所以 "99.99" 先转成 `9999` 再存，显示时再除回去。
2. **审计和订单必须一起成功**（`store/order.go` 的 `CreateOrder`）：
   如果订单写成功、审计写失败，事后就查不到"谁在什么时候下了什么单"。安全审计的价值就在于**不能有洞**。

---

## 4. 除了修漏洞，它还解决了哪些"工程问题"

这些不是安全专属，是"能不能部署"的分界线：

| 问题 | 解决方式 | 代码位置 |
| --- | --- | --- |
| 登录状态存哪 | 存 Redis，Cookie 里只放随机 ID（不存用户信息，也不用 JWT，因为要能"改密即踢下线"） | `internal/session/session.go` |
| 请求太多把服务打满 | Redis + Lua 脚本做原子令牌桶限流 | `internal/middleware/ratelimit.go` |
| 程序被攻击者打崩 | panic 恢复中间件 + 请求体大小限制 + 服务器超时 | `middleware/recover.go`、`main.go` |
| 关服务时请求被截断 | 收到退出信号后，等正在处理的请求做完再退 | `cmd/govulnlab/main.go` |
| 上线时表结构怎么建 | SQL 写文件、编译进二进制，启动时自动迁移，已执行过的跳过 | `migrations/`、`internal/store/migrate.go` |
| 数据错乱怎么查 | 每个请求一个 `request_id`，日志和审计表都能对上 | `internal/middleware/accesslog.go`、`audit_log` 表 |

---

## 5. CI/CD 是什么，为什么你现在"不用操作"

### 它是什么（一句人话）

> **CI**：自动批改作业。你交代码，机器自动检查有没有写坏。
> **CD**：批改通过后，自动打包/自动部署。

### 你项目里对应的东西

文件：`.github/workflows/ci.yml`。它规定了一条流水线，每次你 `git push`，GitHub 的机器就自动按顺序做 5 件事：

1. 检查代码格式（`gofmt`）
2. 静态检查（`go vet`）
3. 跑单元测试 + 竞态检测（`go test -race`）
4. 编译（`go build`）
5. 打包 Docker 镜像（只在推 `main` 分支时做）

### 为什么现在不用操作

三个原因，都很实在：

1. **CI 跑在 GitHub 的服务器上，不在你电脑上**。你电脑不需要装任何东西，也不需要一直开机。
2. **本地已经有等价命令**：`make test`、`make vet`、`go build`——你已经在跑，效果一样，只是没截图。
3. **代码还没推上去**（本地比远程多 2 个提交），所以这套流水线**一次都没跑过**。

### 怎么操作（真要用时，3 步）

```powershell
cd C:\Users\Eiselle\Desktop\go-security-lab
git push            # 第一次会弹浏览器，登录你的 GitHub 账号授权
```

然后：

1. 打开 GitHub 仓库页面 → 顶部 **Actions** 标签；
2. 看到一条运行记录：绿色 ✅ = 全过；红色 ❌ = 某一步失败；
3. 点进去，左侧列出 5 个步骤，哪个红就点哪个看日志。

**你只需要会看颜色，不需要会读 YAML。**

> 补充：自动部署（CD 的后半段）我故意没做。原因有两个——靶场绝不能放到公网；自动部署需要服务器地址和密码，不该写进公开仓库。

---

## 6. Docker 是什么，为什么你现在"不用操作"

### 它是什么（一句人话）

> 把"程序 + 数据库 + 配置"装进一个盒子。别人拿到盒子，一条命令就能跑起来，不用再折腾环境。

### 为什么现在不用操作

你这台机器上，MySQL 装在 `F:\MySQL`，Redis 装在 `F:\Redis`，程序用 `go run` / `bin\govulnlab.exe` 直接跑——**已经能跑，26 项测试全过**。
Docker 只是把同样的东西**换一种装法**，不装不影响任何功能，也不影响简历（简历写的是"有容器化部署方案"，文件在仓库里，看得见）。

装它的真实代价：要先装 WSL2 → 需要管理员权限 → **重启电脑** → 之后常驻内存。

### 怎么操作（等你需要时）

装好 Docker Desktop 之后，只有三条命令：

```powershell
cd C:\Users\Eiselle\Desktop\go-security-lab
copy .env.example .env      # 把里面的口令改成你自己的
docker compose up -d --build    # 起 MySQL + Redis + 应用
docker compose logs -f app      # 看日志
```

然后浏览器还是打开 `http://127.0.0.1:8080`，用法完全一样。

**什么时候值得装**：① 要录"换台电脑也能一键跑"的演示；② 面试要讲容器化细节（Dockerfile 分层、非 root 运行、healthcheck 我都写了）；③ 以后要放到云服务器。

---

## 7. 一页速查：我现在该干什么

| 我想…… | 怎么做 | 结果在哪看 |
| --- | --- | --- |
| 看看项目长什么样 | 浏览器打开 `http://127.0.0.1:8080`，登录 `admin / Admin@123456` | 页面 |
| 亲手打一次 SQL 注入 | 打开 `http://127.0.0.1:8080/lab` | 页面回显"注入成功" |
| 确认所有功能没坏 | `pwsh -File scripts/smoke.ps1` | 末尾"26 通过 / 0 失败" |
| 把项目放到 GitHub | `git push`（会弹登录） | 仓库 Actions 页的绿勾 |
| 让别人也能跑 | 装 Docker → `docker compose up -d --build` | 同上，端口一样 |
| 准备面试怎么讲 | 看 `docs/RESUME.md` | 有逐条话术 |

---

## 8. 如果只记三句话

1. **这个项目 = 一个能用的网站 + 一个故意写坏的对照版**，同一套业务，两份代码，能指到行。
2. **漏洞的本质是"数据和命令没分开"**：SQL 注入是输入变成了 SQL，XSS 是输入变成了 HTML，越权是身份没参与查询条件。
3. **CI 是自动批改作业，Docker 是打包盒**——两个都是"以后再说也不影响现在"的工具，现在这套已经能跑、能演示、能讲。
