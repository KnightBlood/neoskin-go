# NeoSkinGo — 进度快照

> 每次会话结束更新本文件，并在回复末尾给出「进度」一行。
> 最后更新：2026-11-06。
> 仓库：<https://github.com/KnightBlood/neoskin-go>

## 1. 总览

| 指标 | 数值 |
| --- | --- |
| 路由条目（PHP 实际） | **174** |
| 唯一路径 | **145** |
| 已接通的源路径 | **145/145 唯一路径**；方法级以 `ACCEPTANCE.md` 说明为准（仅 `QUERY` 非标准方法未注册） |
| 未接通唯一路径 | **0** |
| Go 代码规模 | `internal/` + `app/controllers` 约 50 个文件 |
| 测试 | `go build ./...` / `go vet ./...` / `go test ./...` 全绿 |
| 数据库矩阵 | SQLite + MySQL/MariaDB/PostgreSQL/WiltonDB；本机 2026-11-01 实测，CI `db-matrix` job 常备 |

> 说明：`/{player}.json`、`/csl/{player}.json` 由同一个 `/:player` handler 处理，
> 已按功能计入覆盖。

## 2. 阶段进度

| 阶段 | 进度 | 说明 |
| --- | --- | --- |
| Phase 0 骨架 | 100% | Goravel v1.18、config、routes、health |
| Phase 0.5 数据库矩阵 | 100% | 17 表安装器、四库矩阵（本地 + CI 常态） |
| Phase 1 数据层/模型 | 80% | 全部业务表 + OAuth + Option；更新包元数据走文件系统 |
| Phase 2 认证/授权 | 100% | 登录/注册/验证码/登出、OAuth2 全端点、Bearer + scope、找回/重置/验证/绑定、`NEOSKIN_APP_KEY` 签名 URL、locale 检测 |
| Phase 3 业务控制器 | 100% | Auth/User/Player/Closet/Texture/Skinlib/Report + avatar/preview 渲染 |
| Phase 4 管理端 | 90% | 全部页面与 API 已接通（Users/Players/Closet/Options/Customize/Score/Resource/i18n/Communications/Status/Chart/Plugins/Update）；剩余交互细节对齐 |
| Phase 5 安装向导 | 100% | `/setup` 欢迎页、数据库表单、信息表单、完成页（HTML）+ 建表/管理员/install.lock；全局 `RedirectToSetup` 已启用（未安装 302、请求期自动 `update`） |
| Phase 6 模板/页面 | 100% | 首页 + 认证页 + 用户中心 + 皮肤库 + 管理端全部页面（含通信可视化编辑器）+ i18n |
| Phase 7 插件系统 | 80% | 清单/启停/删除/上传/wget/市场 + Go 原生 Hook 7 方法/Filter/24 事件；插件市场缓存与 PHP 插件代码执行未支持 |
| Phase 8 测试/CI | 95% | 本地 `go build/vet/test ./...` 全绿（2026-11-04）；race/覆盖率/浏览器 E2E/四库矩阵 job 已配置；性能压测与 67 用例逐条对照未完成 |

## 3. 已完成的功能

- **基础**：Goravel 骨架、5 库连接（`DB_*`/`DATABASE_URL`）、健康检查、17 表安装器
  `artisan neoskin:install`。
- **PHP 兼容**：bcrypt `$2y$`、SHA-256、`Y-m-d H:i:s`（Asia/Shanghai）、Laravel
  session key、`NEOSKIN_APP_KEY`（原串 + base64 解码）、Laravel 签名 URL、HS256。
- **认证**：登录/登出/注册/验证码/CSRF、`/api/user`、找回密码（签名 URL + 邮件）、
  重置密码、邮箱验证、绑定邮箱、重发验证邮件。
- **OAuth2**：scopes、clients CRUD、authorize/approve/deny、authorization_code /
  refresh_token / personal_access、tokens 列表/撤销、`token/refresh` + `laravel_token`。
- **API 鉴权**：Bearer JWT + scope 中间件；session 视为 `*`；`/api/*` 无 CSRF。
- **用户中心**：资料、头像、积分信息、签到、深色模式、站内信。
- **玩家/衣柜/皮肤库/举报**：用户侧与管理侧主要 API，含纹理上传与积分联动。
- **管理端**：用户、玩家、衣柜、options/customize/score/resource、翻译、通信、
  通知、仪表盘、插件管理与市场、**在线更新**（ZIP 校验/暂存/备份/回滚/安装）。
- **安装向导**：数据库连接测试、写 `.env`、建表、生成 OAuth 密钥、创建超级管理员、
  写 `storage/install.lock`。
- **图片格式**：头像/预览默认输出 WebP（`nativewebp`），带 `?png` 时输出 PNG，与 PHP GD 行为一致。
- **其它**：`GET /api`、`GET /raw/{tid}`、`.well-known/change-password`、
  `_ignition/*`（debug+local 门控，与 .NET 参考一致）。
- **渲染**：2D/3D 头像、皮肤正反面预览、披风预览（`internal/minecraft`；默认 WebP、`?png` 输出 PNG；
  与 PHP `Blessing\Renderer` 输出尺寸一致，平均通道误差约 1.2–1.6/255）。

## 4. 未接通的路由

无。145 条唯一路径全部注册并有实现（`GET /api` 支持任意方法、`/{player}.json` 与
`/csl/{player}.json` 共用一个 handler）。

## 5. 变更记录

### 2026-11-06（后端功能补齐：自动升级 + CLI + client_credentials）

- 新增全局中间件 `app/http/middleware/redirect_to_setup.go`：未安装时除 `setup*`
  外一律 302 `/setup`；已安装且 `options.version` 落后时请求期执行 `update`。
- 新增 `internal/update/upgrade.go`：`update` 的共享实现（5.0.0 `home_pic_url`
  从 `bg.jpg` 改为 `bg.webp`、版本号、幂等建表、清缓存、补 `install.lock`），
  用互斥锁串行化，失败可在下个请求重试。
- 新增 CLI：`update`、`bs:install {email} {password} {nickname}`、`options:cache`、
  `plugin:enable|disable {name}`、`salt:random [--show]`，通过 `artisan` 入口执行。
- `POST /oauth/token` 新增 `client_credentials` grant（Passport 默认启用）与
  HTTP Basic 客户端认证（authorization_code / client_credentials 均支持）。
- 测试新增 `internal/update/upgrade_test.go`、`redirect_to_setup_test.go`；
  `go build/vet/test ./...` 全绿。

### 2026-11-04（文档校准 + 方法级路由补齐）

- **路由方法级对齐**：`POST /skinlib` 补为 `POST /texture` 的同处理器别名；
  `/auth/captcha`、`/admin/customize|options|score|resource`、
  `/admin/plugins/config/:name`、`/.well-known/change-password` 改为
  `router.Any`（`POST` 仍由页面处理器内部转给保存逻辑，其余方法渲染页面）。
  重跑路由差异：228 个「方法×路径」仅剩 `QUERY`（非标准方法）与
  `/{player}.json`/`/csl/{player}.json` 的 catch-all 形状差异。
- **README/AGENTS 状态修正**：删除「Phase 0/0.5 完成、Phase 1/2 进行中」
  与「尚无 Go 代码」等过期描述，统一指向 `PROGRESS.md`/`ACCEPTANCE.md`。
- **MIGRATION 清单校准**：Phase 0.5 CI 改为已配置；Phase 2 去掉“进行中”；
  Session 与 Phase 5 的已知差异保留明确说明；Phase 8 的 67 用例说明改为
  “未逐条对照”。
- **ACCEPTANCE/DB-MATRIX**：补方法级路由说明；数据库矩阵区分「本机实测日期」
  与「CI db-matrix job」。
- **本地验证**：`go build ./...`、`go vet ./...`、`go test ./... -count=1`
  全部通过。

### 2026-11-02（上一轮）

- **文档校准**：
  - `ACCEPTANCE.md`：插件事件口径改为「24 个常量 + PHP 原生 5 个派发点已
    对齐，另派发业务事件供 Go 插件监听」；Hook 7 方法标注为已完成；新增
    race/覆盖率与安全用例行；浏览器 E2E 覆盖范围按实际用例校准。
  - `PROGRESS.md`：数据库矩阵 100%（CI 常态）、管理端 90%、插件系统 80%；
    下一步收敛为文档校准后的前端 P0。
  - `MIGRATION.md`：分阶段清单加「以 PROGRESS 为准」说明；Phase 3/4/6/7/8
    勾选与现状对齐（页面、WebP、插件 Hook/市场、E2E/安全/race）。
  - `PORT-NOTES.md`：补充后台页面守卫、浏览器 E2E 资源策略、CI 门禁。

- **CI 安全/质量门禁 + 浏览器 E2E + 后台页面守卫**（第 6 项差距）：
  - 新增 `tests/support`：抽出 E2E 共用的「构建 → 建表 → 启动 → 健康检查 →
    清理」harness，`tests/e2e` 改为复用。
  - 新增 `tests/e2e/security_test.go`（直连 SQLite 建管理员/普通用户）：
    - 未登录访问 `/admin`、`/admin/users`、`/admin/update`、`/user`、`/user/inbox`
      均 302 → `/auth/login`；`/api/user` 保持 JSON 401；
    - 缺 CSRF token 的 POST → 419；带 token 的空登录 → 422；
    - 登录后 session ID 轮换、`/admin` 可访问；登出后 `/user` 302；
    - remember cookie：HttpOnly、无 session 时恢复登录、篡改 HMAC 段被拒；
    - 普通用户访问后台页面与 `/admin/users/list` → 403。
  - 修复后台页面守卫：新增 `requireAdminPage`（对应 PHP `authorize` +
    `can:admin`）——未登录浏览器请求 302 登录页而不是 JSON 401；替换
    `adminPageData` 与 4 个页面入口；顺带修复 `UsersPage` 忽略
    `requireAdmin` 响应导致游客访问 nil deref 的隐患。
  - 新增 `tests/browser`（Playwright `playwright-core` + 系统 Chrome，不下载
    浏览器）：Go 测试启动真实服务并播种管理员，`node flows.mjs` 执行
    首页/登录/皮肤库/上传页/用户中心/后台三页流程，断言 `window.blessing`、
    关键元素与**零 console error**。默认跳过，`NEOSKIN_BROWSER_E2E=1` 启用。
  - `public/app` 仍不入库：CI 从上游 6.0.10 发布包下载前端产物（SHA-256
    固定校验）后再跑浏览器 E2E；本地用 `scripts/copy-frontend.ps1`。
  - `.github/workflows/ci.yml` 新增：
    - `race-coverage`：`go test ./... -race -covermode=atomic
      -coverprofile=coverage.out` + 覆盖率摘要 + artifact；
    - `browser-e2e`：Node 22 + `npm ci --prefix tests/browser` + 下载前端
      产物 + 系统 Chrome 跑 `go test ./tests/browser`。
  - 本地验证：`go build/vet/test ./...` 全绿；浏览器 E2E 本机通过（6.0.10
    产物，登录/上传页/后台均无 console error）。

- **422 校验消息本地化 + Laravel ValidationException 语义**（第 5 项差距）：
  - 新增 `app/http/controllers/validation.go`：`validationFailed` 的 `message`
    改为 Laravel `summarize` 语义——首条字段消息，多错追加
    `(and N more error[s])`（PHP 语言包同样不给该文案翻译）；`errors` 保持
    字段 => 消息数组。字段顺序按名称排序保证确定性（PHP 为规则声明顺序，
    仅多字段同时失败时首条可能不同，已记 `PORT-NOTES.md`）。
  - 新增 `vMsg` + `vRequired/vEmail/vUnique/vIn/vExists/vInteger/vBoolean/
    vURL/vConfirmed/vMinString/vMaxString/vSizeString/vMimes/vCaptcha/
    vPasswordMessage` 等辅助：文案全部取自
    `resources/lang/<locale>/validation.yml`，`:attribute` 使用
    `validation.attributes` 友好名（未定义时保留原字段名），占位符
    `:min/:max/:size/:values` 按 Laravel `makeReplacements` 替换。
  - 20 个控制器中 60+ 处硬编码英文校验消息全部替换；同时按 PHP 规则拆分
    分支以匹配真实文案：`required|email`（空 → required，格式错 → email）、
    `required|min|max`（空/过短/过长分别取 required/min.string/max.string）、
    `confirmed`、`size:32`、`mimes:png`、`max:255` 等。
  - 角色名校验重写为 PHP `Rules\PlayerName` 四规则（official/cjk/utf8/custom，
    custom 支持 `/pattern/flags`）并按字符数（mb_strlen 语义）判断 min/max；
    错误键按端点使用 `name`/`player_name`，规则文案取
    `user.player.player-name-rule.<rule>`。
  - `SkinlibController` 未验证邮箱的 403 文案改取 `auth.check.verified`
    （对齐 PHP `CheckUserVerified`）；`validation.captcha`/`validation.recaptcha`
    按是否配置 reCAPTCHA secret 选择。
  - 单测 `validation_test.go`：11 条中英文消息（含 attributes 回退、嵌套规则、
    占位符）+ 角色名四规则匹配；E2E 新增登录页 CSRF + 空登录请求用例，
    断言 zh_CN/en 的 422 `message`（含 more-errors 后缀）与字段消息。
  - 验证：`go build/vet/test ./...` 全绿。

- **插件 Hook 3 方法补齐 + `ConfigureRoutes` 派发**（第 4 项差距）：
  - `internal/plugin/hooks.go`：`Hook` 补 `AddRoute`（存回调）、`ApplyRoutes`
    （路由注册完成后统一执行回调）、`PushMiddleware`/`PushedMiddlewares`
    （收集全局中间件，由 `routes/web.go` 挂入 web 组链尾）、
    `SendNotification(uids, title, content, conn, dialect)`（直接写 notifications
    表，`data` 仅 `{title, content}`，等价 PHP `Hook::sendNotification` 的
    SiteMessage 无 communication 关联场景；各通知读取路径对缺失 `format` 键安全）。
  - 事件派发口径复核：PHP 全源 `event(new` 命中仅 5 处（ConfigureRoutes +
    PluginManager 4 个插件生命周期事件）；`UserTryToLogin` 与抽象基类 `Event`
    在 PHP 同样从不派发——Go 保持不派发即等价，无需接线。
  - `routes/web.go`：web 组中间件链改为「5 个既有中间件 + Hook pushed 链尾」；
    核心路由注册完成后 `plugin.DefaultHook.ApplyRoutes(facades.Route())` +
    `Dispatch(EventConfigureRoutes)`（payload = goravel route.Router，
    时序与 PHP `RouteServiceProvider:36` 一致）。
  - 单测 `internal/plugin/hooks_test.go` 新增三个：用 framework 自带
    `mocks/route` 验证 AddRoute/ApplyRoutes 回调收到路由器、nil 跳过、
    重复 ApplyRoutes 可重入；PushMiddleware 顺序与返回拷贝语义；
    SendNotification 用 sqlite 建库验证落库行数、`type`、`data` 键集合与
    `read_at` 为 NULL。
  - `go.mod`：引入 framework 的 route mocks（间接依赖 `stretchr/objx`）。
  - 验证：`go build/vet/test ./...` 全绿；e2e 启动真实服务覆盖新派发路径。
  - 说明：in-tree 尚无 DefaultHook 消费方（插件为外部扩展点），故以单测
    与 e2e 覆盖；不做浏览器验证。

- **限流对齐 PHP 路由 `throttle:N,1`**（第 3 项差距）：
  - 新增 `app/http/middleware/throttle.go`：复刻 Laravel `ThrottleRequests` +
    `Illuminate\Cache\RateLimiter` —— 签名 `sha1(method|path|ip)`、Goravel Cache
    固定窗口（`:timer` 存过期时间戳 + 计数 `*int64`，因 memory 驱动 Increment
    只支持指针类型）、`tooManyAttempts/hit/availableIn` 语义与 PHP 一致；
    超限返回 429 + `Retry-After` + `X-RateLimit-{Limit,Remaining,Reset}`、
    响应体 `Too Many Attempts.`，成功响应同样带 `X-RateLimit-*` 头。
  - 路由接入：`POST /admin/communications/preview`（90/min）、
    `/admin/communications/import`（30/min）、`/admin/update/upload`（6/min）、
    `/admin/update/install`（3/min）——与 `routes/web.php` 第 129/130/209/210 行一致。
  - 另补 PHP RouteServiceProvider 的 api 组全局 `throttle:60,1`：
    `APIThrottle(60)` 挂在 web 组链上（仅 `/api` 前缀生效，`APP_ENV=testing` 跳过）。
  - 单测 `throttle_test.go`（固定窗口计数、窗口过期重置、签名独立、指针计数回填）；
    手工 HTTP：install 第 4 次 429（Rem 2→1→0，RA=60）、import 30/min 计数正常、
    api 60/min 连续 63 次 → 恰好 3 个 429、非 api 页面不受影响。

- **remember me「记住我」对齐 PHP `Auth::login($user, $keep)`**：
  - `auth.Login(ctx, users, uid, remember)`：登录即 `session.Regenerate(true)` 轮换
    session ID（消除 session fixation TODO），再调 `session.WriteCookie` 重写 cookie
    （gin 驱动 `removeSetCookie` 同名覆盖）；`keep` 为真时走 Laravel
    `ensureRememberTokenIsSet`（token 空则生成 60 位随机值并落库）+ 下发
    `remember_web_59ba36…` 长效 cookie（5 年、HttpOnly、SameSite=Lax、Path=/）。
  - `CurrentUser`：session 未命中时回退 remember cookie（等价 `userFromRecaller`：
    uid + remember_token + 密码 HMAC 三段匹配，改密后旧 cookie 自动失效），
    命中后重建 session 登录态。
  - `auth.Logout(ctx, users)`：等价 Laravel `logout()` —— 记住 cookie 存在则下发
    过期 cookie，remember_token 非空则轮换（`cycleRememberToken`）使所有已记住设备失效。
  - 登录态读 session 还写入 Laravel 同名键 `password_hash_login_web_59ba36…`。
  - 偏差（已记录 `PORT-NOTES.md`）：PHP recaller cookie 值经 `EncryptCookies`
    整体加密；Go 版以 `uid|token|hmac(sha256, password_hash, NEOSKIN_APP_KEY)`
    替代（不加密但不可伪造），第三段语义与 PHP `hash_equals` 校验一致。
- 登出/改密/改邮箱/删号四处调用点同步新签名；`UserRepository` 新增
  `UpdateRememberToken`，`userColumns`/scan 补 `remember_token` 列，`model.User`
  补 `RememberToken` 字段。
- 验证：`go build/vet/test ./...` 全绿；手工 HTTP：登录（keep=true）响应同一请求
  内 session ID 轮换 + remember cookie 下发；仅凭 remember cookie（无 session id）
  访问 `/user/profile` 恢复登录态；篡改 HMAC 段被拒；logout 后 DB token 轮换 +
  旧 cookie 失效；不带 `keep` 的登录不下发 remember cookie。
- 登录安全对齐 PHP `AuthController`：新增 `internal/captcha` 失败计数器：
  - 按进程内缓存实现按 IP 的登录失败计数（失败 +1 且刷新 1h TTL，成功登录清零），超过 3 次要求验证码；
  - 登录页 `#blessing-extra` 按 IP 计数输出 `tooManyFails`；
  - 未注册账号返回 `code:2 + auth.validation.user`（用户不存在），且不计入失败次数（与 PHP 一致）。
- 新增 reCAPTCHA v2 线上校验：`verifyCaptcha(ctx, options, input)` 实现 PHP `Rules\Captcha` 语义——配置 `recaptcha_secretkey` 时走 `recaptcha.net/api/siteverify`，否则消费本地 session 验证码；接入登录（失败 >3 次）、注册、忘记密码三条链路。
- 单测：`internal/captcha`（siteverify 成功/失败/无可达网络、失败计数增/查/清零/过期）；HTTP 手工验证：失败计数 1..4 → 第 5 次触发 422 captcha、登录页 `tooManyFails:true`、重启后归零、未注册账号 code 2。

### 2026-11-01（本轮）
- API 错误/成功消息接入语言包：新增 `t(ctx,key,...)` 请求级翻译器（locale + `language_lines` 覆盖），覆盖认证/找回/资料/玩家/衣柜/皮肤库/举报/管理端/插件/市场/更新等消息；实测 `?lang=zh_CN` 与 `?lang=en` 返回对应文案。
- 新增 `ACCEPTANCE.md`（HTTP/数据/前端契约/patch 功能/插件/CI 验收清单与已知限制）、`DEPLOYMENT.md`（构建/env/建表/前端资源/反向代理/迁移/更新/回滚）、`scripts/verify.ps1`。
- 浏览器 E2E 修复：JSON `Content-Type` 去 charset（前端 fetch 严格匹配）、登录响应 `data.redirectTo`、admin `blessing-extra` 空对象、玩家/衣柜页补充 `#previewer` 与 rule/length/unverified、安装向导使用带 hash 的 CSS；新增 `scripts/copy-frontend.ps1` 复制预编译前端资源。
- 新增 `.github/workflows/ci.yml`：build/vet/test + MySQL/MariaDB/PostgreSQL/SQL Server 四库矩阵；新增 `tests/e2e`（构建二进制 → 建表 → 启动 → 探测首页/认证/安装/语言包/API/头像/权限跳转）。
- `BaseData` 增加 `ExtraHead` / `ExtraFoot`，`buildBaseData` 派发 `RenderingHeader` / `RenderingFooter`，页面 head/foot 输出插件追加内容。
- 新增安装向导页面：`setup_welcome` / `setup_database` / `setup_info` / `setup_finish`（Spectre 表单 + CSRF + 当前值回填）。
- 插件接入：用户/管理端侧边栏追加 `DefaultHook` 菜单项，用户面板渲染插件徽章，`layout` 输出插件样式/脚本；已派发全部核心事件：`UserRegistered` / `UserLoggedIn` / `UserAuthenticated` / `UserProfileUpdated` / `PlayerWillBeAdded` / `PlayerWasAdded` / `PlayerWillBeDeleted` / `PlayerWasDeleted` / `PlayerProfileUpdated` / `PlayerRetrieved` / `TextureDeleting` / `RenderingBadges` / `RenderingHeader` / `RenderingFooter` / `ConfigureUserMenu` / `ConfigureAdminMenu` / `ConfigureExploreMenu` / `PluginWasEnabled` / `PluginWasDisabled` / `PluginWasDeleted` / `PluginBootFailed`。
- 新增 `internal/plugin` 原生扩展 API：`Filter`（add/apply/remove/priority）、`Hook`（菜单/样式/脚本/徽章）、24 个事件常量与同步 `Dispatcher`；默认单例 `DefaultHook/DefaultFilter/DefaultDispatcher`，页面 head/foot 已输出插件样式与脚本。
- 新增 `GET /` 首页：真实 HTML（`window.blessing`、csrf meta、导航/语言菜单/用户菜单、
  介绍区、版权、`#blessing-extra`），静态资源名自动从 `public/app` 发现。
- 新增用户中心仪表盘 `/user`：`user.base` 布局、header/sidebar/user-panel/side-menu 片段、公告卡片、积分说明与 `usage-box`/`email-verification` 钩子。
- 新增用户中心页面：`/user/profile`（头像/昵称/密码/邮箱/删号表单）、`/user/player`、`/user/closet`、`/user/oauth/manage`。
- 新增 `/user/reports`（服务端举报表格 + 分页）、`/user/inbox`（站内信列表 + 分页 + 已读状态）、`/user/inbox/{id}`（详情，自动标记已读）。
- 新增皮肤库页面：`/skinlib`（浏览壳）、`/skinlib/show/{tid}`（预览器 + 侧栏 + `download/currentUid/admin` extra）、`/skinlib/upload`（上传壳 + 名称规则/积分/内容政策 extra，需登录且邮箱已验证）。
- 新增管理端页面：`admin-open/close` 布局与侧边栏、`/admin` 仪表盘（统计卡 + 图表容器）、`/admin/users`（含 `currentUser` extra）、`/admin/players`、`/admin/reports`、`/admin/plugins/manage`、`/admin/plugins/market`。
- 新增 `/admin/i18n` 页面（翻译表格容器 + 新增表单 + CSRF + 成功提示）。
- 新增通信可视化编辑器：`?new=1` / `?announcement=1` / `?edit=id` 打开，组件画布/属性面板/源码导入/实时预览/发布或草稿；接入 `communications.js`，更新走 `POST /admin/communications/update/{id}`。
- 新增 `/admin/communications` 历史列表页：消息表格（标题/位置/状态/已读-投递/更新时间）、编辑与撤回入口、分页、一次性通知。
- 新增 `/admin/options` 页面：常规选项（站点/注册/上传/角色名/材质/内容策略）、SEO meta、reCAPTCHA、更新源与插件仓库四个表单；本地化字段写入 `<key>_<locale>`。
- 新增 `/admin/score` 页面：积分换算、举报奖励、签到配置、分享奖励四个表单；`sign_score_from/to` 提交时合并保存为 `sign_score`。
- 新增 `/admin/resource` 页面：强制 SSL、资源地址、缓存失效时间、CDN 地址、头像/预览缓存开关与清除缓存入口。
- 新增 `/admin/customize` 页面：导航栏/侧边栏配色选择、首页配置（图片/图标/透明导航/隐藏介绍/固定背景/版权偏好/版权文字）、自定义 CSS/JS 表单。
- 新增 `/admin/update` 页面：版本信息、ZIP 上传/校验、运行包详情与确认安装表单、注意事项、上传进度脚本；`Show` 的 JSON 逻辑保留为内部实现。
- 新增 auth 页面：`auth-login/register/forgot/reset/bind/verify`（`<main></main>` + `#blessing-extra` 前端契约）。
- 新增 `internal/i18n`：读取 `resources/lang/<locale>/*.yml`（已同步全部 14 个语言目录），
  支持 `language_lines` 数据库覆盖、`:placeholder`、别名与 en 回退。
- 新增 `GET /lang/{locale}.js`：生成 `blessing.i18n = {...}`（源为静态文件，Go 用路由等价实现）。
- 新增 `DetectLanguage` 中间件：`lang` 参数 → `locale` cookie → `Accept-Language` → 默认，
  并回写 cookie（对应 `DetectLanguagePrefer`）。
- 新增 `resources/pages/*.tmpl`（Go `html/template`）：head/foot/copyright/语言/用户菜单/首页；
  放在 `resources/views` 之外，避免被 Goravel 自带 view 引擎在启动时解析。
- 新增 `internal/update`：ZIP 路径/文件数/大小/重复/符号链接校验、composer 识别、
  版本/必需文件/编译资源检查、payload 解压、备份与回滚安装；含单测。
- 新增 `/admin/update`（show/download/upload/install），完整运行包走
  SHA-256 校验 + 暂存 + 会话确认 + 安装；旧版完整包直接解压。
- `config/app.go` 增加 `version`、`update_source`；`config/http.go` 请求超时默认 30s
  （原 3s 会让更新源探测返回 408）。
- 新增 `auth/forgot`、`auth/reset`、`auth/verify`、`auth/bind`、
  `user/email-verification`；`internal/laravel` 增加签名 URL 生成（与 PHP 校验互通）。
- 新增 `GET /api`、`GET /raw/{tid}`（含 Last-Modified）、
  `.well-known/change-password` 重定向、`_ignition/*` 三个端点。
- 新增 `/setup` 安装向导四端点（连接测试、写 `.env`、建表、超级管理员、install.lock）。
- 新增 `internal/minecraft`：Canvas（GD 风格扫描线多边形/双线性缩放）、Point3、
  Polygon、SkinRenderer（逐段对齐 .NET/PHP 的 bug-for-bug 移植）。
- 新增 `/avatar/{tid}`、`/avatar/hash/{hash}`、`/avatar/player/{name}`、
  `/avatar/user/{uid}`（`?size=`、`?3d`、缺省头像回退）与
  `/preview/{tid}`、`/preview/hash/{hash}`（皮肤正反面 / 披风）。
- 内嵌默认头像与默认皮肤（`internal/resources`）；渲染器单测覆盖尺寸/空图/披风。
- 管理端衣柜、个人令牌、Bearer/scope 等上一轮内容保持通过。
- 文档：`README.md`、`MIGRATION.md`、`PORT-NOTES.md`、`PROGRESS.md`。

### 2026-11-01（上一轮）
- OAuth2 授权码/个人令牌/refresh、Bearer + scope、Laravel 签名基础、管理端衣柜。

## 6. 下一步

1. 文档校准（PROGRESS 阶段百分比、ACCEPTANCE Hook 7 方法表述等）。
2. 前端 P0：`NeoSkinFrontend` workspace + `apps/site`(TDesign) / `apps/admin`(antd)
   / `packages/core` / `packages/contract` 骨架（写前端代码前不动后端文档）。

---

## 8. 共享前端 P1（2026-11-06）

- 新增 `GET /api/bootstrap`（契约 v1：`contractVersion/apiBase/locale/siteName/version/basePath/mode/csrfToken/locales/user/options/features/plugins`）。
- 新增 SPA 壳 `GET /app-next/{site,admin}`、`/app-next/{site,admin}/*`：
  - 静态资源直出并带 `Cache-Control: public, max-age=31536000, immutable`；
  - 非静态路径回退 `index.html`，注入 `<meta name="csrf-token">`、`<meta name="neoskin-mode">` 与 `#neoskin-bootstrap`；
  - 带扩展名的缺失文件返回 404，不回退 HTML。
- 构建产物已发布到 `public/app-next/{site,admin}`（12 个文件 + `manifest.json`）。
- `FRONTEND_MODE` 默认 `legacy`，不改变现有 PHP 兼容页面。
- 实现：`app/http/controllers/spa_controller.go`、`routes/web.go`。
- 联调验证：`/app-next/site/auth/login` 登录 → `/user` → `/app-next/admin/` 仪表盘，0 console error。

---

## 9. 共享前端 P3（2026-11-07）

- 新增用户中心契约端点（不改变既有 174 条 PHP 路由）：
  - `GET /api/user/inbox?page=&per_page=`：分页 + `unread`，条目含 `preview`/`format`/`communication_id`；
  - `GET /api/user/inbox/{id}`：`{code,data}`，返回渲染后 `content` 并自动写 `read_at`；
  - `GET /api/user/reports?page=&per_page=`：当前用户举报分页，含 `texture_name`。
- 实现：`app/http/controllers/notification_controller.go`（`Inbox`/`Message`/预览清洗）、
  `app/http/controllers/report_controller.go`（`Track`）、`internal/repo/notification.go`
  （`CountUnreadByUser`）、`routes/web.go`。
- 验证：`go build` / `go test ./...` 全绿；本机 SQLite 浏览器联调站内信列表/详情（已读）与举报分页，
  前端 `contract-check.mjs` 匿名 401 断言通过。

### 2026-11-07（P4 增量）
- 新增 `GET /api/admin/dashboard`：用户/角色/材质数量与存储空间（KB + 人类可读）。
- 实现：`app/http/controllers/admin_controller.go`（`Dashboard`）、`routes/web.go`。
- 验证：`go build` / `go test ./...`；管理端仪表盘浏览器联调通过。

- `POST /admin/i18n`：当 `Accept: application/json` 时返回 `{code:0,message}`（共享前端直接调用），否则保持原 302 跳转。

- 新增 `GET|PUT /api/admin/options/{group}`（`options|customize|score|resource`）：返回/保存站点配置、配色、积分、资源选项，供管理端 SPA 使用；布尔项统一 `"true"/"false"`。

- 新增 `GET /api/admin/communications`、`GET /api/admin/communications/{id}`（列表 / 编辑器数据）；`POST|PUT /admin/communications*` 与撤回在 `Accept: application/json` 时返回 JSON（否则保持 302）。

- 管理端 SPA 通过请求头 `X-Neoskin-Spa: 1` 标识；`POST|PUT /admin/communications*` 与撤回据此返回 JSON，其他请求保持 302（不改变 legacy 行为）。

- 新增 `GET /api/admin/update/info`：返回 `latest/current/error/can_update/package/upload_limits`，管理端更新页直接展示最新版本与可更新状态。

### 2026-11-08（bootstrap 开关 + 邮箱验证门禁）

- `app/http/controllers/spa_controller.go` 的 bootstrap options 增加
  `allow_downloading_texture`、`require_verification`、`player_name_length_min/max`、
  `user_initial_score`、`return_score`、`sign_after_zero`、`sign_gap_time`、
  `reporter_score_modification`、`reporter_reward_score`，供站点端按真实配置渲染。
- 新增 `app/http/controllers/verification.go`，并在角色、材质、上传、举报处理器接入：
  未验证用户访问 `/user/player*`、`/texture*`、`POST /skinlib`、`POST /skinlib/report`
  返回 403；`/api/*` 保持不门禁（与 PHP `api` 中间件组一致）。
- 验证：浏览器联调未验证用户 4 个写端点均 403、`/api/players` 200；
  恢复配置后页面正常；`go test ./...` 全绿。
- 共享前端产物已同步到 `public/app-next`（site `index-pnUYroLe.js` / admin `index-8f1tgAaz.js`）。

### 2026-11-08（P6 切换验收，Go 本机）

- `FRONTEND_MODE=next`：`GET /` → 302 `/app-next/site/`，`GET /admin` → 302 `/app-next/admin/`，
  两个 SPA 壳与 `/api/bootstrap` 均 200。
- 174 路由回归冒烟：按 `conformance/data/routes.json` 请求 78 条 GET 路由，0 个 500
  （其余 200/302/401/403/404 均为未登录、示例参数或权限语义）。
- `FRONTEND_MODE=legacy` 回滚：`GET /` 返回 legacy 页面（含 `#blessing-extra`），
  `/app-next/site/` 仍可访问，确认可一键回滚。
- 浏览器逐页检查站点/管理端主要页面，0 console error；axe WCAG A/AA 关键页 0 violation。
- Playwright 浏览器 E2E（`NEOSKIN_BROWSER_E2E=1`）通过：home/login/skinlib/upload/user/admin。

### 2026-11-08（NeoSkin 品牌与发布）

- 默认站点名/描述/公告、安装向导与更新包提示改为 NeoSkin；版权选项与页脚署名保留
  Blessing Skin Server 上游归属（见 `PORT-NOTES.md`）。
- 新增 `.github/workflows/release.yml`：推送 `v*` tag 后在 GitHub runner 构建并发布
  `neoskin-go_<version>_<platform>.zip`，包含 二进制、`public/app`、`public/app-next`、`resources` 与部署文档。
- 共享前端产物更新为 site `index-pnUYroLe.js` / admin `index-8f1tgAaz.js`。
- 验证：`go build ./...` / `go test ./...` 全绿；本机 next 模式 setup 页已显示 NeoSkin。
- 管理端写操作回归（Go 本机）：用户昵称/邮箱/积分/权限/验证状态/密码/删除，
  角色改名/转移/材质/删除，举报审核，选项保存，通信草稿/发布/撤回，
  翻译新增/更新/删除全部返回 `code:0`；修改自己的权限按 PHP 语义返回 403。
- 插件列表为空、市场列表与更新检查正常；插件启停/更新下载安装因本地无插件/非落后版本未执行。
- GitHub Release `v6.0.11` 已发布：Linux/Windows/macOS × x64/arm64 共 6 个 `neoskin-go_6.0.11_*.zip`。
- 专用环境验证插件启停与更新下载/安装；修复 SPA 更新安装：缺省 id 时使用会话待安装包，
  `Accept: application/json` / `X-Neoskin-Spa: 1` 返回 JSON 而非 302。
