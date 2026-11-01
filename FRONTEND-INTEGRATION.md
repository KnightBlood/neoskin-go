# 前端集成（Go / NeoSkinGo）

> 共享前端源码位于 **`../NeoSkinFrontend`**（React 19.3 + Vite 8.3.3，两个 app：
> TDesign 站点端 / antd 管理端），不再放在本仓库内。
>
> 本文只描述 **Go 端口需要做的事**；契约以
> `../NeoSkinFrontend/docs/BACKEND-CONTRACT.md` 为唯一权威。
>
> 状态：未开工（`FRONTEND_MODE=legacy` 默认）。最后更新：2026-11-01。

---

## 1. 为什么拆出去

同一个前端要服务四个端口（Go / .NET / Spring Boot / Rust）。放在任何一个端口仓库里
都会让另外三个端口无法复用，也会导致契约类型、CSRF、i18n 多处漂移。因此：

| 位置 | 内容 |
| --- | --- |
| `../NeoSkinFrontend` | 前端源码、契约、构建产物、发布脚本 |
| `NeoSkinGo/public/app-next` | 构建产物副本（不入库） |
| `NeoSkinGo/resources/templates` | `legacy` 服务端渲染（保留，默认） |

---

## 2. 与现有实现的关系

- `public/app`、`resources/assets`、`resources/pages`、`resources/views` 全部保留，
  `legacy` 路径行为不变。
- 现有 174 条路由的路径/方法/字段/JSON 结构不变。
- `/lang/{locale}.js`、`/api/user`、`/skinlib/list`、`/api/admin/*` 等继续作为 SPA 的数据源。
- 新增内容只有：壳路由、`/api/bootstrap`、少量查询端点、`FRONTEND_MODE` 配置。

---

## 3. Go 端任务清单

### 3.1 配置

- `config/app.go` 读取 `FRONTEND_MODE`（默认 `legacy`）、`FRONTEND_NEXT_ROUTES`。
- `.env.example` 增加两项说明。
- 启动时读取 `public/app-next/manifest.json`；缺失或 `contractVersion != 1`
  时强制 `legacy` 并输出 warning。

### 3.2 静态与壳

- `routes/web.go`：
  - `GET /app-next/site`、`/app-next/site/{path...}`
  - `GET /app-next/admin`、`/app-next/admin/{path...}`
- 静态资源走 `public/app-next/{site,admin}/assets`，带不可变缓存头；
  路径含扩展名但文件不存在 → 404，其余回退 `index.html`。
- 壳注入：
  - `<meta name="csrf-token">`
  - `<script id="neoskin-bootstrap" type="application/json">…</script>`
- 前端 `index.html` 预置 `__NEOSKIN_CSRF__` / `__NEOSKIN_BOOTSTRAP__` 占位符，
  Go 端做字符串替换（HTML 属性转义 + JSON 转义）。

### 3.3 bootstrap 数据

复用现有页面数据构造器（`buildBaseData` / `userPageShell` / options service /
plugin hook），映射为契约 JSON：

| 契约字段 | Go 现有来源 |
| --- | --- |
| `user` | `currentUser` + `toPlayerJSON` 同级字段 + `Permission` + `verified` |
| `options` | `service.NewOption(conn, dialect).All()` |
| `features` | 由 options 推导（`user_can_register`、`email_verification`、`oauth`…） |
| `plugins` | `plugin.DefaultHook` 菜单/徽章 + `Styles()`/`Scripts()` |
| `locale` | `localeMiddleware.LocaleOf(ctx)` |
| `basePath` | `/app-next/{mode}/` |
| `contractVersion` | 常量 `1` |

禁止输出：`APP_KEY`、`NEOSKIN_APP_KEY`、数据库凭据、reCAPTCHA secret。

### 3.4 新增端点

| 方法 | 路径 | Go 落点 | 现状 |
| --- | --- | --- | --- |
| GET | `/api/bootstrap` | 新增 `BootstrapController`（或 `ApiController`） | ❌ 待做 |
| GET | `/api/user/inbox` | `NotificationController` 扩展 | ❌ 待做 |
| GET | `/api/user/inbox/{id}` | `NotificationController` 扩展（自动已读） | ❌ 待做 |
| GET | `/api/user/reports` | `UserController`/`ReportController` 扩展 | ❌ 待做 |
| GET | `/api/admin/communications` | `CommunicationsController` 扩展 | ❌ 待做 |
| GET | `/api/admin/communications/{id}` | 同上 | ❌ 待做 |
| GET | `/api/admin/dashboard` | `AdminController` 扩展 | ❌ 待做 |
| GET | `/api/admin/options/{group}` | `AdminOptionsController` 扩展 | ❌ 待做 |
| GET | `/api/admin/status` | `AdminController.Status` 拆 JSON 分支 | ❌ 待做 |
| GET | `/api/admin/update/info` | `UpdateController` 扩展 | ❌ 待做 |

已存在、可直接复用：

- `/api/user`、`/api/players`、`/api/closet`、`/api/closet/ids`
- `/skinlib/list`、`/skinlib/info/{tid}`
- `/user/player/list`、`/user/closet/list`、`/user/closet/ids`、`/user/score-info`
- `/admin/reports/list`、`/api/admin/reports`
- `/api/admin/users`、`/api/admin/players`、`/api/admin/closet/{uid}`
- `/admin/i18n/list`、`/admin/plugins/data`、`/admin/plugins/market/list`、`/admin/chart`
- `/lang/{locale}.js`、`/auth/captcha`、OAuth 三组端点

### 3.5 页面切换

- 命中 `FRONTEND_MODE=next` 且页面键在白名单时返回壳；
- 否则走现有模板；
- 第一批白名单建议：`site:home`、`site:auth`、`admin:dashboard`；
- 全部页面验收后默认设为 `next`，保留 `legacy` 一个版本周期。

### 3.6 发布脚本

`scripts/copy-frontend.ps1` 保持现状（复制 `public/app`），新增：

```powershell
# 从共享前端构建产物复制到本仓库（不入库）
.\scripts\copy-frontend-next.ps1 -Source ..\NeoSkinFrontend -Mode all
```

或者直接调用共享仓库脚本：

```powershell
..\NeoSkinFrontend\scripts\publish.ps1 -Backend D:\WORKspace\NeoSkin\NeoSkinGo
```

### 3.7 测试

- `tests/e2e` 增加：壳可访问、bootstrap 可解析、`/api/bootstrap` 字段齐全；
- 新增端点表驱动测试（SQLite fixture）；
- 浏览器 E2E：登录 → 首页 → 皮肤库 → 用户中心 → 管理端仪表盘；
- 保留现有 174 路由回归。

---

## 4. 验收

- [ ] `FRONTEND_MODE=legacy` 时所有行为与当前完全一致
- [ ] `FRONTEND_MODE=next` 时 `/app-next/site/`、`/app-next/admin/` 可启动
- [ ] bootstrap 字段与契约 §3.3 一致（含匿名与管理员两种会话）
- [ ] 10 个新增端点返回契约结构
- [ ] CSRF 缺失返回 419/403；未登录 API 返回 401
- [ ] 前端产物缺失时自动回退 `legacy`，不白屏
- [ ] 语言切换（`zh_CN`/`en`）正确
- [ ] CI：Go 测试 + 前端契约检查

---

## 5. 回滚

```env
FRONTEND_MODE=legacy
```

`public/app-next/` 可保留，不影响旧路径。
