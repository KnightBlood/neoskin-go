# NeoSkinGo

> 最后更新：2026-11-06

> 仓库：<https://github.com/KnightBlood/neoskin-go>

Go / Goravel 实现的 Minecraft 皮肤托管平台后端，迁移自 `blessing-skin-server-Laravel13`（6.0.11_patch）。

> **当前状态：迁移主体完成（2026-11-04 校准）。**
> Phase 0–6 完成；Phase 7 插件系统 80%（Go 原生 Hook/Filter/Event，PHP 插件代码不可执行）；
> Phase 8 以本地 `go build/vet/test` 与 CI workflow 为准。已实现认证/找回/邮箱验证、
> OAuth2、用户中心、皮肤库、管理端全部页面、安装向导、在线更新与四库矩阵。
> 具体百分比与剩余项以 [PROGRESS.md](./PROGRESS.md)、[ACCEPTANCE.md](./ACCEPTANCE.md) 为准。

## 技术栈

- **语言**：Go 1.26
- **框架**：Goravel v1.18（Laravel 风格）
- **ORM**：Goravel Orm (GORM)
- **数据库**：SQLite / MySQL / MariaDB / PostgreSQL / SQL Server（与 Laravel 一致；矩阵见 [DB-MATRIX.md](./DB-MATRIX.md)）
- **认证**：Session + Cookie / OAuth2（Passport 兼容）
- **模板**：Go `html/template`
- **前端**：webpack + yarn 1.x（复用现有）
- **图像**：`image` + `golang.org/x/image` + WebP 编码（默认 WebP，`?png` 回退 PNG）

## 项目结构

```
NeoSkinGo/
├── internal/db/             # 五方言连接 + 17 张表安装器 + 矩阵测试
├── app/console/commands/    # neoskin:install
├── app/facades/             # Goravel facade
├── config/                  # app/database/http/session...
├── routes/                  # web.go / grpc.go
├── resources/views/         # Go template（页面模板已迁移）
├── public/                  # 静态资源（app/ 由 PHP 源项目复制）
├── storage/                 # textures / uploads
├── tests/                   # feature 测试
└── go.mod
```

## 快速开始

```sh
# 1. 安装依赖
go mod tidy

# 2. 配置数据库（APP_KEY 必须是 32 个可打印字符；PHP 的 base64 key 需另行处理）
cp .env.example .env
# 编辑 .env 填写 DB_* / APP_KEY
# 若需与 PHP 站点互通签名 URL / laravel_token，再设置 NEOSKIN_APP_KEY=<PHP 的 APP_KEY，含 base64: 前缀>

# 3. 建表（幂等，17 张表）
go run . artisan neoskin:install

# 4. 启动服务
go run .
# 或
go build -o neoskin && ./neoskin
```

健康检查：`GET /__neoskin/health` → `ok`。

已接通的端点：

- `GET /` — 首页（HTML，含 `window.blessing` / CSRF meta / 导航与版权）
- `GET /lang/{locale}.js` — 前端语言包（`blessing.i18n`）
- `GET /{player}.json` / `GET /csl/{player}.json` — Minecraft 皮肤/披风 JSON
- `GET /textures/{hash}` / `GET /csl/textures/{hash}` — 原始纹理
- `GET /avatar/{tid}`、`/avatar/hash/{hash}`、`/avatar/player/{name}`、`/avatar/user/{uid}`
  — 头像（`?size=`、`?3d`；无纹理时返回默认头像）
- `GET /preview/{tid}`、`/preview/hash/{hash}` — 皮肤正反面 / 披风预览（`?height=`）
- `GET /auth/login` — 登录页（HTML，前端 JS 渲染表单）
- `POST /auth/login` — 登录（JSON/form）
- `POST /auth/logout` — 登出
- `GET /auth/register` — 注册页（HTML）
- `POST /auth/register` — 注册（邮箱/密码/角色名/验证码，自动登录）
- `GET /auth/captcha` — JPEG 验证码
- `GET /api/user` — 当前登录用户
- `GET /user` — 用户仪表盘（HTML：侧边栏/公告/积分说明）
- `GET /user/profile` / `POST /user/profile` — 资料（昵称/密码/邮箱/删号）
- `POST /user/profile/avatar` — 设置头像
- `GET /user/score-info` — 积分/使用量
- `POST /user/sign` — 签到
- `PUT /user/dark-mode` — 切换深色模式
- `GET /api/players` / `POST /api/players`
- `DELETE /api/players/{pid}` / `PUT /api/players/{pid}/name`
- `PUT|DELETE /api/players/{pid}/textures`
- `/user/player` 下的同组路由
- `GET /api/closet` / `POST /api/closet`
- `PUT|DELETE /api/closet/{tid}`、`/user/closet/{tid}`
- `GET /user/closet/ids` — 衣柜纹理 ID 列表
- `GET /skinlib` — 皮肤库浏览页（HTML 壳，前端调用 `/skinlib/list`）
- `GET /skinlib/list` — 皮肤库分页（含隐私过滤）
- `GET /skinlib/info/{tid}` / `GET /texture/{tid}`
- `GET /skinlib/show/{tid}` — 皮肤库详情页（预览器 + 侧栏 + extra）
- `PUT|DELETE /texture/{tid}[/name|/privacy|/type]` — 改名/隐私/类型/删除
- `POST /texture` — 上传（multipart：`file` / `name` / `type` / `public`）
- `POST /skinlib/report` — 举报纹理
- `GET /user/reports` — 我的举报页（HTML 表格，分页）
- `GET /admin/reports[/list]` / `GET /api/admin/reports` — 管理端举报列表
- `PUT /admin/reports/{id}` / `PUT /api/admin/reports/{id}` — 审核（delete/ban/reject）
- `GET /admin/users/list` / `GET /api/admin/users` — 用户列表
- `PUT /admin/users/{uid}/{email|nickname|password|permission|score|verification}`
- `DELETE /admin/users/{uid}`（及 `/api/admin/*` 同组）
- `GET /admin/players/list` / `GET /api/admin/players`
- `PUT /admin/players/{pid}/{name|owner|textures}`、`DELETE /admin/players/{pid}`
- `/api/admin/players/*` 同组
- `GET|POST /admin/options` / `customize` / `score` / `resource` — 站点配置读取/保存
- `GET /admin/i18n/list`、`POST /admin/i18n`、`PUT|DELETE /admin/i18n/{id}` — 翻译管理
- `GET|POST /admin/communications`、`preview`、`import`、`PUT /{id}`、`POST /{id}/withdraw`
- `GET /user/inbox`、`GET /user/inbox/{id}` — 站内信列表/详情页（HTML，自动已读）
- `POST /user/notifications/{id}` — 标记已读（JSON）
- `GET /api/user/notifications` / `POST /api/user/notifications/{id}`
- `POST /admin/notifications/send` / `POST /api/admin/notifications` — 发送站内信
- `GET /admin` — 仪表盘统计（users/players/textures/storage）
- `GET /admin/chart` — 最近 31 天注册/上传曲线
- `GET /admin/status` — 运行状态（版本/环境/数据库）
- `GET /admin/plugins/data` / `manage` — 插件清单/启用/禁用/删除
- `GET /admin/plugins/readme/{name}`、`config/{name}`、`upload`、`wget`
- `GET /admin/plugins/market/list`、`POST /admin/plugins/market/download`
- `GET /oauth/scopes`、`GET|POST /oauth/clients`、`PUT|DELETE /oauth/clients/{id}`
- `GET|POST|DELETE /oauth/authorize`、`POST /oauth/token`
- `GET /oauth/tokens`、`DELETE /oauth/tokens/{id}`、`POST /oauth/token/refresh`
- `GET|POST /oauth/personal-access-tokens`、`DELETE /oauth/personal-access-tokens/{id}`
- `GET|POST|DELETE /admin/closet/{user}`、`/api/admin/closet/{user}`
- `GET /auth/forgot` / `POST /auth/forgot` — 找回密码（签名 URL 邮件）
- `GET|POST /auth/reset/{uid}` — 重置密码（Laravel 签名校验）
- `GET|POST /auth/verify/{user}` — 邮箱验证
- `GET|POST /auth/bind` — 绑定邮箱
- `POST /user/email-verification` — 重发验证邮件
- `GET|POST /setup`、`ANY /setup/database`、`GET /setup/info`、`POST /setup/finish`
- `GET /admin/update`、`POST /admin/update/{download|upload|install}`
- `GET /api` — 站点信息（blessing_skin/spec/copyright/site_name）
- `GET /raw/{tid}` — 原始纹理下载（受 `allow_downloading_texture` 控制）
- `GET /.well-known/change-password` — 重定向到 `/user/profile`
- `GET /_ignition/health-check`、`POST /_ignition/{execute-solution|update-config}`

## 数据库矩阵

本机用 `..\tools\scripts\db.cmd start all` 启动 MySQL 3307 / MariaDB 3308 /
PostgreSQL 5433 / WiltonDB 1433，设置对应 `DB_*` 后运行矩阵测试。
命令与跨库坑位见 [DB-MATRIX.md](./DB-MATRIX.md)；跨语言版见工作区根目录
`../DATABASE-MATRIX-PLAYBOOK.md`（不随仓库提交）。

## 文档索引

| 文档 | 内容 |
| --- | --- |
| [README.md](README.md) | 项目入口、快速开始 |
| [AGENTS.md](AGENTS.md) | agent 行为约束 |
| [PROGRESS.md](PROGRESS.md) | 进度快照与变更记录 |
| [ACCEPTANCE.md](ACCEPTANCE.md) | 验收清单 |
| [MIGRATION.md](MIGRATION.md) | 迁移计划与阶段 |
| [PORT-NOTES.md](PORT-NOTES.md) | 偏差、决策与踩坑 |
| [DEPLOYMENT.md](DEPLOYMENT.md) | 构建、配置、部署与升级 |
| [DB-MATRIX.md](DB-MATRIX.md) | 数据库矩阵 |

## 硬约束

1. HTTP 接口零变更（路径/参数/响应结构）
2. 数据完全兼容（17 张表 + 存量数据 + bcrypt 密码）
3. 插件 API 契约保持
4. patch 定制功能保留

## 许可证

与源项目一致。
