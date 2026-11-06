# NeoSkinGo — Go / Goravel 迁移计划

> 最后更新：2026-11-06

> **目标**：把 `blessing-skin-server-Laravel13`（6.0.11_patch，Laravel 13.38 + PHP 8.5）完整迁移到 Go 1.26 + Goravel v1.18，保持 **HTTP 接口字节级兼容** 与 **数据库结构/数据完全兼容**。

---

## 1. 硬约束（不可破坏）

1. **HTTP 接口零变更**：所有 **174** 条路由的路径、方法、请求参数名、请求体字段名、响应 JSON 结构必须与源系统完全一致（`codegraph/routes.json` 的 173 条漏记 `POST skinlib`）。包括：
   - 静态资源路由 `textures/{hash}`、`avatar/{...}`、`preview/{...}`、`{player}.json`、`csl/{player}.json`、`raw/{tid}`
   - OAuth2 15 条（Passport 标准：`oauth/authorize`、`oauth/token`、`oauth/clients`、`oauth/scopes`、`oauth/tokens`、`oauth/personal-access-tokens`、`oauth/token/refresh`）
   - Minecraft 客户端契约（`{player}.json`、`csl/{player}.json` 返回的 JSON 字段名）
2. **数据兼容**：17 张表（`users`、`players`、`textures`、`options`、`user_closet`、`reports`、`notifications`、`jobs`、`language_lines`、`scopes`、`communications`、5 张 `oauth_*`）的表名、字段名、字段类型、主键（`uid`/`pid`/`tid`，非 `id`）与存量数据必须保持可读。密码哈希（bcrypt `$2y$`）必须继续可验证；纹理 `hash`（64 位 SHA-256）算法保持。
3. **插件 API 契约**：`Hook`（7 个静态方法）、`PluginManager`（13 个方法）、`Plugin`（assets/readme/manifest）、24 个 Event、`config/menu.php`、`config/plugins.php` 的对外语义保持。
4. **patch 定制功能**：6.0.3 ~ 6.0.11_patch 全部功能（仪表盘、ZIP 在线更新、通知/公告中心、可视化编辑器、分页、更新源切换、PHP 8.5 适配层）保留。

---

## 2. 技术栈与版本基线

| 组件 | 选型 | 版本 | 说明 |
| --- | --- | --- | --- |
| 语言 | Go | 1.26.x | Goravel v1.18 要求 |
| 框架 | Goravel | v1.18.0 | Laravel 风格，facades 1:1 映射 |
| ORM | Goravel Orm (GORM) | v1.18 | 支持 MySQL/MariaDB/SQLite/PostgreSQL |
| 数据库 | SQLite / MySQL / MariaDB / PostgreSQL / SQL Server | — | 与 Laravel config/database.php 一致（默认 mysql） |
| 认证 | Goravel Auth (JWT) + 自建 OAuth2 | — | Passport → 自建 OAuth2 server（或 oauth2 库） |
| 模板 | Go `html/template` | — | 替代 104 个 `.twig` 文件（86 个入口模板 + 片段） |
| 前端 | 复用现有 webpack + yarn 1.x 产物 | — | `resources/assets/src` 131 个源文件 |
| 图像 | `image` + `golang.org/x/image` | — | 替代 imagick/intervention |
| 缓存 | Redis / 内存 | — | Goravel Cache facade |
| 队列 | Goravel Queue | — | 替代 Laravel Queue |

---

## 3. 源系统盘点

| 维度 | 数量 | 备注 |
| --- | --- | --- |
| 数据表 | 17 | 主键 `uid`/`pid`/`tid` |
| 路由 | **174** | admin 53 / api 33 / user 26 / oauth 15 / auth 14 / skinlib 7 / texture 6 / avatar 4 / setup 4 / csl 2 / preview 2 / raw 1 / textures 1 / {player}.json 1 / .well-known 1 / _ignition 3；`codegraph` 173 条，源码另有 `POST skinlib` |
| 控制器 | 20 | +1 基类 `Controller`；见 §5.2 映射表 |
| 中间件 | 12 | Authenticate, CheckRole, RejectBannedUser, EnsureEmailFilled, ... |
| Event | 24 | Player*/Texture/User*/Plugin*/Rendering* |
| Listener | 9 | CleanUpCloset, ResetPlayers, SendEmailVerification, ... |
| Service | 11 | Hook, Plugin, PluginManager, Option, Communications, ... |
| Twig 模板 | 104 | `resources/views/`（86 个页面/片段入口；另有 5 个 Blade：master/mails） |
| JS/TS 源 | 131 | `resources/assets/src`（含 tests 共 192） |
| 语言包 | 14 | de_DE, el_GR, en, es_ES, fr_FR, it_IT, ja_JP, ko_KR, nl_NL, pt_PT, ru_RU, zh_CN, zh_TW, overrides |
| 测试 | 67 | `tests/` 中 67 个具体 TestCase 类 |

---

## 4. 分层架构设计

```
NeoSkinGo/
├── main.go                     # 入口
├── bootstrap/                  # Goravel bootstrap（providers 注册）
├── app/
│   ├── Http/
│   │   ├── Controllers/        # 20 个控制器（1:1 映射）
│   │   ├── Middleware/         # 12 个中间件
│   │   └── Kernel.go           # 中间件组
│   ├── Models/                 # 17 个 Eloquent→Goravel Orm 模型
│   ├── Events/                  # 24 个 Event
│   ├── Listeners/              # 9 个 Listener
│   ├── Services/               # 11 个 Service
│   ├── Plugins/                # 插件系统（Go plugin / Yaegi）
│   └── Console/                # Artisan 命令等价
├── config/                     # 配置文件（patch.php, menu.php, ...）
├── database/
│   ├── migrations/             # 原样移植（SQL 兼容）
│   └── seeders/
├── routes/                     # web.go, api.go, static.go（路径 1:1）
├── resources/
│   ├── views/                  # Go template（从 Twig 转换）
│   └── assets/                 # 复用现有 webpack 产物
├── storage/                    # textures, uploads
├── tests/
└── go.mod
```

---

## 5. 数据层迁移

### 5.1 表结构映射（保持不变）

| 表 | 主键 | 关键字段 | 兼容性要求 |
| --- | --- | --- | --- |
| `users` | `uid` (uint) | email, nickname, score, avatar, password, ip, permission, last_sign_at, register_at | 密码哈希 bcrypt `$2y$` 跨语言可验证 |
| `players` | `pid` (uint) | uid, name, tid_skin (default -1), tid_cape (default 0), last_modified | `tid_skin=-1` 表示无皮肤 |
| `textures` | `tid` (uint) | name, type, hash(64), size, uploader, public, upload_at | hash 算法 SHA-256 保持 |
| `options` | `id` | option_name, option_value (longtext) | K-V 配置存储 |
| `user_closet` | 复合 (user_uid, texture_tid) | item_name | 收藏关联 |
| `reports` | `id` | tid, uploader, reporter, reason, status, report_at | — |
| `notifications` | `id` (uuid) | type, notifiable_type, notifiable_id, data, read_at, timestamps | Laravel 通知表格式 |
| `language_lines` | `id` | group, key, text, timestamps | i18n DB 存储 |
| `scopes` | `id` | name (unique), description | OAuth scope 定义 |
| `communications` | `id` (bigint) | author_id, legacy_key, delivery_token, title, kind, status, locale, editor, source, blocks, audience, published_at | 公告/站内信 |
| `jobs` | — | Laravel 队列表 | 可重建 |
| `oauth_*` (5) | — | Passport 标准表 | OAuth2 server 兼容 |

### 5.2 密码哈希兼容

- Laravel 使用 bcrypt（`$2y$10$...`）或 argon2。
- Go 使用 `golang.org/x/crypto/bcrypt`，`bcrypt.CompareHashAndPassword` 支持 `$2y$` 前缀。
- **必须**在 `User` 模型的 `VerifyPassword` 方法中统一使用 bcrypt 比较，确保存量密码可登录。

### 5.3 纹理文件存储

- 源：`storage/app/textures/{hash}` 或类似路径（需确认实际存储路径）。
- Go：保持相同路径布局，迁移时文件直接复制，数据库记录不变。

---

## 6. 接口层迁移（HTTP 契约）

### 6.1 路由映射策略

所有路由按原样移植，Goravel 路由定义：

```go
// routes/web.go
facades.Route().Get("/", controllers.HomeController{}.Index)
facades.Route().Prefix("auth").Group(func(router contract.Route) {
    router.Get("login", controllers.AuthController{}.Login)
    // ... 所有 auth 路由
})
// ... 逐条移植
```

### 6.2 关键路由组

| 前缀 | 数量 | 中间件 | 说明 |
| --- | --- | --- | --- |
| `/` | 1 | — | 首页 |
| `/auth` | 14 | guest/authorize | 登录/注册/找回/重置/验证/绑定 |
| `/user` | 26 | authorize | 用户中心/档案/玩家/衣柜/通知 |
| `/admin` | 53 | authorize, role:admin | 管理面板全功能 |
| `/texture` | 6 | authorize, verified | 皮肤库上传/管理 |
| `/skinlib` | 7 | —/authorize | 皮肤库浏览 + `POST skinlib` 上传 |
| `/setup` | 4 | setup | 安装向导 |
| `/api` | 33 | auth:oauth + scope | REST API（OAuth2 保护） |
| `/oauth` | 15 | — | OAuth2 server（Passport 兼容） |
| 静态 | 11 | — | textures/avatar/preview/raw/csl |
| `/` 其它 | 4 | — | `.well-known` 1 + `_ignition` 3 |

### 6.3 Minecraft 客户端契约

`GET /{player}.json` 和 `GET /csl/{player}.json` 必须返回与源完全一致的 JSON：

```json
{
  "SKIN": { "url": "..." },
  "CAPE": { "url": "..." }
}
```

字段名、URL 格式、HTTP 状态码（404 时的响应）必须保持。

### 6.4 列表查询参数

源使用 `blessing/filter` search-string，查询参数：
- `sort` — 排序字段
- `fields` — 选择字段
- `limit` — 每页数量
- `from` — 偏移量

Go 侧需实现等价的查询解析器，保持参数名与排序语义一致。

---

## 7. 认证与授权

### 7.1 Session 认证

- 源：Laravel Session + Cookie。
- Go：Goravel Session facade + Cookie，或 JWT。为保持兼容，优先用 Session + Cookie（Goravel 支持）。

### 7.2 OAuth2 Server

- 源：Laravel Passport（5 张 oauth 表 + 15 条路由）。
- Go：需自建 OAuth2 server 或使用 `golang.org/x/oauth2` + 自建授权端点。
- **关键**：`oauth/clients`、`oauth/authorize`、`oauth/token`、`oauth/scopes` 等路由的请求/响应格式必须与 Passport 完全一致，因为 Minecraft 启动器和第三方客户端依赖这些端点。
- Scope 机制：`User.Read`、`Player.Read/ReadWrite`、`Closet.Read/ReadWrite`、`UsersManagement.*`、`PlayersManagement.*`、`ClosetManagement.*`、`ReportsManagement.*`、`Notification.Read/ReadWrite`。

### 7.3 中间件映射

| Laravel 中间件 | Go 实现 |
| --- | --- |
| `auth` / `authorize` | Session 验证中间件 |
| `guest` | 未登录重定向 |
| `role:admin` / `role:super-admin` | CheckRole 中间件 |
| `verified` | CheckUserVerified |
| `setup` | RedirectToSetup |
| `throttle` | Goravel RateLimiter |
| `auth:oauth` | OAuth2 Bearer token 验证 |
| `scope:*` | OAuth2 scope 检查 |

---

## 8. 插件系统

### 8.1 挑战

PHP 插件是运行时动态加载的（`plugins/` 目录下每个插件一个文件夹，含 `package.json` manifest）。Go 不支持运行时加载任意 Go 代码。

### 8.2 方案选型

| 方案 | 优点 | 缺点 | 推荐 |
| --- | --- | --- | --- |
| Go `plugin` 包 (.so) | 原生 | 仅 Linux/macOS，Windows 不支持 | ❌ |
| Yaegi (Go 解释器) | 跨平台，运行时执行 Go 脚本 | 性能、API 限制 | ⚠️ 备选 |
| 独立进程 + gRPC | 完全隔离，语言无关 | 复杂度高，性能开销 | ✅ 推荐 |
| WASM (wazero) | 跨平台，沙箱 | 生态不成熟 | ⚠️ |

**推荐方案**：插件以独立可执行文件/进程运行，通过 gRPC 与主进程通信，Hook 事件通过消息总线分发。插件 manifest（`package.json`）格式保持不变。

### 8.3 Hook 映射

| Hook 方法 | Go 实现 |
| --- | --- |
| `addMenuItem` | 事件总线 + 菜单注册表 |
| `addRoute` | 路由注册回调（启动时收集） |
| `addStyleFileToPage` / `addScriptFileToPage` | 页面资源收集器 |
| `addUserBadge` | 徽章注册表 |
| `sendNotification` | 通知服务 |
| `pushMiddleware` | 中间件链注册 |

### 8.4 Event 系统

24 个 Event → Go 的事件分发器（channel + handler 注册表），Listener 通过 `Event().Listen()` 注册。

---

## 9. 前端与视图层

### 9.0 决策：前端直接复用 blessing 项目（不自己重写）

**前端（`resources/assets/` + `public/app/`）不重写、不换技术栈、不做 SPA 改造**，
直接复用 `blessing-skin-server-Laravel13` 的既有前端：

| 资产 | 来源 | 处理方式 |
| --- | --- | --- |
| 前端源码 | `resources/assets/`（React 17 + TypeScript + webpack 5，131 个 JS/TS 文件） | 原样复用；构建沿用源仓库的 `package.json` / `webpack.config.ts` / `yarn.lock` |
| 预编译产物 | `public/app/`（`app.<hash>.js`、`style.<hash>.css`、`spectre.*`、`home-css.*`、`home.*`、字体/图片/wasm） | 原样复用到 `public/assets/app/`，**不重新打包**（哈希值被 `resources/views/assets/*.twig` 写死引用） |
| 运行时语言包 | `/lang/<locale>.js`（由 `JavaScript` 服务生成） | 后端生成，内容与源一致 |

**关键约束 —— 服务端模板是「前端契约」，必须逐字保住 DOM 结构。**
预编译 bundle 靠选择器定位挂载点，改 HTML 结构等于改前端。以下都是硬契约：

- `window.blessing = {...}`（`shared.head`）与 `#blessing-extra` 的 JSON；
- `#logout-button`、`data-mark="nickname"`、`.locale`、`.main-button`、`.splash*`；
- auth 页的 `<main></main>`（React 挂载点）、`#blessing-extra` 的
  `{tooManyFails, recaptcha, invisible}`；
- `data-toggle="dropdown"` / `data-communication-*` / `data-system-update-form` 等
  行为钩子。

因此 **Twig 模板是移植的规格说明书**（spec），不是可以用自由风格重写的草稿。
服务端换模板引擎时，输出结构必须与 Twig 渲染结果一致。

---

### 9.1 Twig → Go Template

104 个 `.twig` 文件（86 个入口模板 + 片段）→ Go `html/template`。关键转换：
- `{{ variable }}` → `{{ .Variable }}`
- `{% if %}` / `{% for %}` → `{{ if }}` / `{{ range }}`
- `{% extends %}` / `{% block %}` → `{{ template }}` / `{{ define }}`
- Twig 过滤器 → Go template 函数

> 注意：转换的验收标准是 **9.0 的 DOM 契约**，不是「看起来一样」。

### 9.2 前端资源

**不重新构建前端**：`public/app/` 的预编译产物原样拷贝到 `public/assets/app/`，
Go 静态文件服务提供；`/lang/<locale>.js` 由后端 `JavaScript` 等价服务在运行时生成。
仅当确需修改前端逻辑时才启用源仓库的 webpack + yarn 1.x 构建链。

### 9.3 通知/公告/编辑器

patch 定制的可视化组件编辑器（提示框、折叠卡片、拖动排序、源码模式）基于前端 JS，后端只需提供 CRUD API + 存储，Go 侧实现 `CommunicationsController` 等价逻辑。

公告选择必须对齐源 `Communications::currentAnnouncement()`：先当前语言、再空语言
（`locale=''`）；取 `kind='announcement'` 或已发布 `kind='both'`；排序
`updated_at desc, kind, id desc`；只有 `status='published'` 才渲染到用户仪表盘。
管理端编辑器要能回退到 `locale=''` 的公告，否则“默认语言公告打不开”。

---

### 9.4 前后端分离：前端开发模式与运行时契约

前端可独立运行，复用它对后端提出了 5 条硬要求（缺一条线上白屏）：

1. **URL**：一律 `${blessing.base_url}${url}`，`base_url` 来自 `shared.head` 注入的
   `window.blessing`。
2. **凭证**：`credentials: 'same-origin'` → 会话 cookie 必须同源可带。
3. **请求头**：所有请求带 `Accept: application/json`；写请求带 `X-CSRF-TOKEN`，
   值取自 `meta[name="csrf-token"]`。⚠ 因此「校验失败」必须回 **422 JSON**
   （`{message, errors: {field: [..]}}`），**不能 302**。
4. **错误分支**：422 → 取 `errors[field][0]`；419 → CSRF 提示；403/400 → 弹
   `message`；其它非 2xx → 抛异常，若 body 含 `exception` + `trace` 会**渲染堆栈**。
5. **成功体**：`Content-Type: application/json` 才按 JSON 解析；业务层再看
   `{code, message, data}` 的 `code !== 0`。
6. **请求体**：`blessing.fetch` 的 POST/PUT/DELETE 默认发 **JSON**
   （`Content-Type: application/json`），只有文件上传用 `FormData`。后端读取参数
   必须合并 query + form + JSON（**体覆盖 query**），不能只读 form，否则创建 OAuth
   应用、登录等写接口会 422。CSRF 用 `X-CSRF-TOKEN` 头，表单兜底读 `_token`。

**开发模式**：`yarn dev`（webpack-dev-server）默认 `0.0.0.0:8080`、`publicPath=/app/`、
`Access-Control-Allow-Origin: *`、HMR。dev 下产物名**无哈希**（`app.js`），
`resources/views/assets/*.twig` 会由 `HtmlWebpackPlugin` 重写成
`//localhost:8080/app/`；prod 才写 `{{ cdn_base }}/app/<hash>`。
⚠ `assets/*.twig` 是**构建产物**（`assets/.gitignore` = `*` + `!.gitignore`），
所以后端模板不能把哈希路径写死，否则 dev 模式不可用 —— 需要 debug 开关切换资源根。

**前端测试**：46 个 jest 用例（ts-jest + jsdom），`tests/setup.ts` 注入
`window.blessing` 与 `i18n`（读 `resources/lang/en/front-end.yml`），各用例自行
mock `fetch`。这些 mock 断言了 URL/方法/body/响应形状，等价于一份可执行的接口契约。

**路由表生成**：`yarn build:urls` 执行
`php artisan route:list --json --columns=uri,name` 生成
`resources/assets/src/scripts/urls.ts`（已提交，复用前端时无需重生成）。若需重生成，
后端须提供等价的 `route:list --json --columns=uri,name` 输出。

## 10. 图像处理（纹理渲染）

源使用 imagick + intervention/image：
- 皮肤/披风预览渲染
- 头像生成（avatar by player/user/hash/texture）
- 纹理缩略图

Go 替代：`image` 标准库 + `golang.org/x/image`（draw, bmp, png）+
WebP 编码器（纯 Go 或 cgo 方案，需在 `go.mod` 记录选型）。需实现：
- PNG 读写；**WebP 编码（quality 100）**
- 图像缩放/裁剪
- 2D 头像、**3D 等距头像（`?3d`）**、皮肤正反预览、披风预览

**输出语义（与 PHP/Java/.NET 对齐）**：

- 默认 **WebP**（`image/webp`）；带 `?png` 才输出 PNG；WebP 编码不可用时回退 PNG。
- 头像：渲染后按 `size` 缩放；回退头像 `avatar2d.png` / `avatar3d.png` 也要走同一格式。
- 预览缓存键：`preview-t<tid>-<format>`（**不含 height**，与源一致）；
  头像键：`avatar-<mode>-t<tid>-s<size>-<format>`。
- `enable_avatar_cache` / `enable_preview_cache` 为真时 TTL 1 年，否则 1 分钟
  （进程内缓存即可，对应 `Cache::remember`）。
- 响应带 `Cache-Control: public, max-age=<cache_expire_time>`、`ETag`（内容摘要）、
  `Last-Modified`，并处理 `If-None-Match` / `If-Modified-Since` → 304。

**注意**：源的 `skinview3d` 是前端 3D 渲染，但后端也提供 `?3d` 头像；
原始纹理端点返回 PNG，Content-Type 必须正确。

---

## 11. 国际化

- 14 种语言包（`resources/lang/`）。
- Go：使用 `golang.org/x/text` 或自建 i18n，读取 JSON/YAML 翻译文件。
- `language_lines` 表保持（DB 驱动的翻译覆盖）。

---

## 12. 分阶段实施计划

> 说明：以下是迁移初期的计划清单，保留作路径记录；实际完成情况与百分比以
> `PROGRESS.md` 为准（部分条目已超出最初范围，例如浏览器 E2E、race/覆盖率）。

### Phase 0：项目骨架（1-2 天）
- [x] 初始化 Go module + Goravel 项目
- [x] 配置数据库连接（SQLite / MySQL / MariaDB / PostgreSQL / SQL Server）
- [x] 移植全部 17 张表的 migration（`internal/db` 原生 SQL；不要用 AutoMigrate 改结构）
- [x] 验证 `go build` + 数据库连接

### Phase 0.5：数据库矩阵（1-2 天）
- [x] 五方言的建表/分页/自增/布尔/时间戳实现
- [x] 环境变量门控的 `DatabaseMatrixTest`（建表 + 用户/纹理 CRUD）
- [x] 本机 `tools\scripts\db.cmd` 四种真实库全部通过
- [x] CI 五库矩阵（GitHub Actions 四库 + SQLite；见 `.github/workflows/ci.yml`）
- [x] 记录踩坑到 `DB-MATRIX.md` / `PORT-NOTES.md`

### Phase 1：数据层 + 模型（3-5 天）
- [ ] 17 个 Model 定义（当前已落 User/Player/Texture 三个）
- [x] 密码哈希兼容验证（bcrypt `$2y$`）
- [x] 纹理 hash 算法确认与实现
- [x] 单元测试：CRUD + 关联查询（SQLite + 四库矩阵）

### Phase 2：认证与授权（3-5 天）
- [x] Session 认证 + Cookie（Goravel session/CSRF；PHP session cookie 加密未做，签名 URL/laravel_token 已兼容）
- [x] OAuth2 Server（Passport 兼容：authorize/token/clients/scopes/tokens/personal token/refresh + Bearer/scope）
- [x] 中间件链（session + CSRF + OAuth scope + 限流；auth/role 在控制器内校验）
- [x] Scope 机制

### Phase 3：核心业务控制器（7-10 天）
- [x] AuthController（登录/注册/验证码/找回/重置/验证/绑定）
- [x] UserController（档案/头像/积分/签到/深色模式/重发验证邮件）
- [x] PlayerController（CRUD/改名/纹理设置 + 页面）
- [x] ClosetController（分页/IDs/add/rename/remove + 页面）
- [x] TextureController（上传/类型/隐私/删除/raw/avatar/preview；WebP 默认输出）
- [x] SkinlibController（浏览/详情/改名/隐私/类型/删除/上传/举报）
- [x] ReportController（提交/我的举报/管理列表/review + 页面）
- [x] ClosetManagementController / PlayersManagementController / UsersManagementController

### Phase 4：管理面板（5-7 天）
- [x] AdminController（仪表盘/图表/状态）
- [x] CommunicationsController（CRUD/预览/导入/撤回/投递 + 编辑器页面）
- [x] NotificationsController（站内信列表/详情/已读/发送）
- [x] OptionsController（选项保存/读取 + 表单页面）
- [x] TranslationsController（列表/新增/修改/删除 + 页面）
- [x] PluginController（清单/启停/删除/readme/upload/wget + 页面）
- [x] UpdateController（ZIP 校验/暂存/备份/回滚/安装）
- [x] MarketController（列表/安装 + 市场页面）

### Phase 5：安装向导 + 设置（2-3 天）
- [x] SetupController（数据库/信息/完成）
- [~] CheckInstallation 中间件（setup 路由内联检查 install.lock；全局 RedirectToSetup 未启用，已登记为已知差异）

### Phase 6：前端视图（5-7 天）
- [x] Twig → Go template 转换（首页/认证/用户中心/皮肤库/管理端全部页面 + 布局/共享片段）
- [x] 静态资源服务（`/app/*` 扫描 `public/app` 哈希文件名）
- [x] 前端产物接入（构建沿用源仓库，产物不入库；`scripts/copy-frontend.ps1`）

### Phase 7：插件系统（5-7 天）
- [x] 插件 manifest 解析
- [x] Hook 事件总线（7 方法：菜单/路由/样式/脚本/徽章/通知/中间件）
- [x] 插件管理（启用/禁用/删除/上传）
- [x] 插件市场下载
- [ ] 插件市场缓存/分页（待补）
- [ ] PHP 插件代码执行（设计上不支持，需改写为 Go 扩展）

### Phase 8：测试与验证（3-5 天）
- [x] HTTP 接口契约测试（路径/参数/响应）
- [x] 数据兼容性测试（存量数据读取）
- [x] 安全用例（CSRF/session 轮换/remember 篡改/越权）
- [x] 浏览器 E2E（Playwright + 系统 Chrome，CI）
- [x] race detector + 覆盖率 artifact（CI）
- [ ] 性能压测
- [ ] 67 个测试用例逐一移植（当前为单测 + E2E 覆盖，未逐条对照）

---

## 13. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| OAuth2 Passport 兼容 | 高 | 使用 `golang.org/x/oauth2` + 自建端点，严格按 Passport 请求/响应格式 |
| 插件动态加载 | 高 | 采用进程隔离 + gRPC 方案 |
| Twig 模板转换 | 中 | 优先转换高频页面，低频页面可暂用 Go template 简化版 |
| 图像处理差异 | 中 | 头像/预览默认 WebP（`?png` 回退 PNG），渲染算法与 GD/Imagick 不逐字节一致；优先保证尺寸/结构 |
| 密码哈希兼容 | 高 | bcrypt `$2y$` 前缀 Go 原生支持，需验证 |
| Minecraft 客户端兼容 | 高 | `{player}.json` / `csl/{player}.json` 响应格式严格对照 |

---

## 14. 验证清单

```sh
go build ./...
go test ./...
# HTTP 契约测试
go test ./tests/contract/...
# 数据兼容测试
go test ./tests/datacompat/...
```

- [ ] `go build` 无错误
- [ ] 所有 **174** 条路由注册成功
- [ ] 数据库 migration 在 SQLite / MySQL / MariaDB / PostgreSQL / SQL Server 五种库执行无错误
- [ ] 数据库矩阵测试通过（本地 + CI；见 `DB-MATRIX.md`）
- [ ] 存量数据可正常读取（users/players/textures/options）
- [ ] bcrypt 密码验证通过
- [ ] OAuth2 端点与 Passport 响应一致
- [ ] Minecraft 客户端可正常获取皮肤/披风
- [ ] 前端页面正常渲染
- [ ] 67 个测试用例全部通过

---

## 15. 数据库矩阵（本地 + CI）

本机免安装数据库在 `..\tools`：

```powershell
..\tools\scripts\db.cmd start all
..\tools\scripts\db.cmd status
```

| 驱动 | 端口 | 账号 | 库 |
| --- | --- | --- | --- |
| MySQL 26.10 | 3307 | blessing / 123456 | `blessing_skin` |
| MariaDB 13.0.2 | 3308 | blessing / 123456 | `blessing_skin` |
| PostgreSQL 18.6 | 5433 | postgres / 123456 | `blessing_skin` |
| WiltonDB（TDS） | 1433 | wilton / 123456 | `blessing_skin` |
| SQLite | — | — | 文件 / `:memory:` |

矩阵测试（`DB_CONNECTION=mysql|mariadb|pgsql|sqlsrv` 时执行，否则 Skip）：
建 17 张表 → 用户 CRUD → 纹理 CRUD（`public` 小整数 + `upload_at` 时间戳 +
自增主键）→ 校验 `register_at` 为 `Y-m-d H:i:s`。步骤与命令见 `DB-MATRIX.md`。

必须提前知道的跨库差异：

- **SQLite 会掩盖问题**：重复打开连接、布尔写成 1、字符串写时间戳都能过。
- **`boolean` ≠ `tinyInteger`**：`users.verified` 是 bool；`textures.public` 是
  PG `smallint` / SQL Server `TINYINT`，写 `0/1`。
- **PostgreSQL 类型严格**：string 不能直接写 timestamp；用 `time.Time` 或
  `@p::timestamp`。读取 timestamp 后统一格式化。
- **SQL Server 保留字**：`public` 必须引用（`[public]`）。
- **SQL Server 自增**：`SCOPE_IDENTITY()` 必须与 `INSERT` 同一批次，或
  `OUTPUT INSERTED.[pk]`；单独命令返回 NULL（WiltonDB 可能掩盖）。
- **整型宽度**：`INT`/`SERIAL`/`IDENTITY` 可能 int32，`COUNT(*)` 是 int64；
  扫描到 `int64`。
- **MySQL DSN**：`charset=utf8mb4&parseTime=true`；MySQL 8 的
  `caching_sha2_password` 在非 TLS 下需允许公钥/原生密码。

真 SQL Server 必须由 CI 的 `sqlsrv` job 覆盖；WiltonDB 只作本机替身。
跨语言坑位速查见工作区根目录 `../DATABASE-MATRIX-PLAYBOOK.md`。
