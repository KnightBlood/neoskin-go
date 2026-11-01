# 部署与升级（NeoSkinGo）

## 1. 环境要求

- Go **1.26+**（构建用；运行只需二进制）
- 数据库：SQLite / MySQL / MariaDB / PostgreSQL / SQL Server
- 可选：memcached/redis（Goravel 缓存）、SMTP

## 2. 构建

```sh
git clone https://github.com/KnightBlood/neoskin-go
cd neoskin-go
go build -trimpath -ldflags "-s -w" -o neoskin .
```

Windows：`go build -o neoskin.exe .`

## 3. 配置

复制 `.env.example` 为 `.env`：

```ini
APP_NAME=NeoSkin
APP_ENV=production
APP_KEY=<32 个可打印字符>       # Goravel 内部使用
APP_URL=https://skin.example.com
APP_HOST=127.0.0.1
APP_PORT=3000
HTTP_REQUEST_TIMEOUT=30

DB_CONNECTION=mysql            # sqlite/mysql/mariadb/pgsql/sqlsrv
DB_HOST=127.0.0.1
DB_PORT=3306
DB_DATABASE=blessing_skin
DB_USERNAME=...
DB_PASSWORD=...
DB_PREFIX=

NEOSKIN_STORAGE_ROOT=storage
# PHP 站点的 APP_KEY（含 base64: 前缀）；用于签名 URL 与 laravel_token
NEOSKIN_APP_KEY=

MAIL_HOST=
MAIL_PORT=587
MAIL_USERNAME=
MAIL_PASSWORD=
MAIL_FROM_ADDRESS=hello@example.com
MAIL_FROM_NAME=NeoSkin
```

> `DATABASE_URL` 优先级高于 `DB_*`。

## 4. 建表

```sh
./neoskin artisan neoskin:install
```

- 幂等，创建 16 张业务表 + `migrations`（共 17 张）。
- 可指向已有 PHP 数据库：表结构兼容，存量用户/玩家/纹理可直接读取。
- SQL Server/Babelfish 请使用该命令，不要使用 Goravel `migrate:fresh`。

## 5. 前端资源

```powershell
powershell -ExecutionPolicy Bypass -File scripts\copy-frontend.ps1 `
  -Source D:\path\to\blessing-skin-server-Laravel13
```

脚本把 `public/app`（预编译 JS/CSS）和 `favicon.ico` 复制到 `public/`。
`/lang/<locale>.js` 由后端即时生成，无需复制。

## 6. 启动

```sh
./neoskin
```

- 健康检查：`GET /__neoskin/health` → `ok`
- 首次部署也可访问 `/setup` 使用安装向导；写 `.env` 后需**重启进程**。

### systemd 示例

```ini
[Unit]
Description=NeoSkinGo
After=network.target mysql.service

[Service]
WorkingDirectory=/opt/neoskin
ExecStart=/opt/neoskin/neoskin
Restart=always
EnvironmentFile=/opt/neoskin/.env

[Install]
WantedBy=multi-user.target
```

## 7. 反向代理（Nginx / 1Panel）

```nginx
server {
    listen 443 ssl http2;
    server_name skin.example.com;

    client_max_body_size 50m;

    location / {
        proxy_pass http://127.0.0.1:3000;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
        proxy_set_header X-Forwarded-For $proxy_add_x_forwarded_for;
        proxy_set_header X-Forwarded-Proto $scheme;
    }
}
```

注意：

- 上传大小由 Nginx/1Panel 与本应用共同决定；应用上限见管理端「站点配置」。
- 静态资源由 Go 进程直接提供（`/app`、`/public`）；不要求额外 alias。
- HTTPS 下请设置 `APP_URL=https://...`。

## 8. OAuth2

- RSA 密钥自动生成于 `storage/oauth-private.key` / `oauth-public.key`（首次使用 OAuth 时）。
- 客户端 secret 以 bcrypt 存储；access token 为 RS256 JWT。
- 个人令牌：`/oauth/personal-access-tokens`；刷新：`POST /oauth/token/refresh`（需 `NEOSKIN_APP_KEY`）。

## 9. 在线更新

管理端 `/admin/update`：

1. 上传 ZIP 运行包（或从更新源下载完整包）。
2. 服务端校验路径/文件数/大小/SHA-256/版本/必需文件，暂存到 `storage/app/system-updates/<id>`。
3. 确认安装：备份原文件 → 替换 → 失败自动回滚。
4. 安装完成后建议重启进程。

安装包只允许覆盖 `app/ config/ database/ resources/ routes/ vendor/ public/(非上传目录) bootstrap/(非 cache)` 等白名单路径。

## 10. 备份与回滚

- 备份数据库 + `storage/`（纹理、OAuth 密钥、install.lock、更新暂存）。
- 回滚：恢复数据库与 `storage/`，换回上一版二进制即可。
- 更新包的备份文件保留在 `storage/app/system-updates/<id>/backup/`。

## 11. 从 PHP 版迁移

1. 停 PHP 站点写入，备份数据库与 `storage/`。
2. 将 Go 部署指向同一数据库，执行 `neoskin artisan neoskin:install`（不会破坏数据）。
3. 复制纹理目录到 `NEOSKIN_STORAGE_ROOT/textures/`。
4. 配置 `NEOSKIN_APP_KEY` 以兼容签名 URL / `laravel_token`。
5. 切换反向代理到 Go 进程，验证登录、皮肤库、Minecraft JSON。

## 12. 已知限制

- PHP 插件代码无法执行（使用 Go 原生 Hook/Event 扩展）。
- PHP session cookie 加密未实现（使用 Goravel session；签名 URL 已兼容）。
- 安装向导写 `.env` 后需重启。
- `_ignition` 仅 debug + local 可用。
