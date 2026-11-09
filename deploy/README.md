# deploy/ —— 部署配置

| 文件 | 用途 |
| --- | --- |
| `systemd/govulnlab.service` | 裸机/虚机部署的 systemd 单元，含最小权限加固 |
| `systemd/govulnlab.env` | 该单元的 `EnvironmentFile` 模板（放口令，权限 0640） |
| `nginx/govulnlab.conf` | TLS 终止 + 反向代理 + 边缘限流 |

完整步骤见 [`../docs/DEPLOY.md`](../docs/DEPLOY.md)。
