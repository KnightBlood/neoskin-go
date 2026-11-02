# NeoSkinGo — PORT-NOTES

> 记录与 PHP 源/Java/.NET 参考实现的契约、决策与踩坑。新增映射关系追加到这里。

## 1. 当前状态

- 2026-11-01：Phase 0 / 0.5 完成，Phase 1 进行中。
  - Goravel v1.18 骨架、5 库配置、`internal/db` 原生 SQL 安装器。
  - `artisan neoskin:install` 创建 16 张业务表 + `migrations`（第 17 张）。
  - 矩阵测试（原始 SQL + 仓储层）在本机 MySQL/MariaDB/PostgreSQL/WiltonDB 全部通过。
  - PHP 兼容层：bcrypt `$2y$` 验证/生成、SHA-256、`Y-m-d H:i:s`（Asia/Shanghai）。
  - 仓储：User / Player / Texture。
  - 端点：`/{player}.json`、`/csl/{player}.json`、`/textures/{hash}`、
    `/csl/textures/{hash}`（已手工 HTTP 验证 200/404）。
  - 认证：Goravel session + CSRF middleware；`POST /auth/login`（邮箱或角色名）、
    `POST /auth/logout`、`GET /api/user`；会话键沿用 Laravel
    `login_web_59ba36addc2b2f9401580f014c7f58ea4e30989d`。
  - 注册：`GET /auth/register`、`POST /auth/register`、`GET /auth/captcha`
    （JPEG，session 存 phrase，提交时 `Pull` 一次性校验）；options 表驱动
    `register_with_player_name` / 长度 / `regs_per_ip` / `user_initial_score`。
  - 手工验证：注册 → 自动登录 → `/api/user` → `/NewPlayer.json` 均 200。
  - 用户中心：`GET/POST /user/profile`（昵称/密码/邮箱/删号）、
    `POST /user/profile/avatar`、`GET /user/score-info`、`POST /user/sign`、
    `PUT /user/dark-mode`；手工验证昵称/头像/签到（间隔限制）/深色模式均正常。
  - 玩家：`GET/POST /api/players`、`DELETE /api/players/{pid}`、
    `PUT /api/players/{pid}/name`、`PUT|DELETE /api/players/{pid}/textures`，
    以及 `/user/player` 同组路由；创建扣 `score_per_player`、删除按
    `return_score` 返还，改名后 Minecraft JSON 立即生效（手工验证通过）。
  - 衣柜：`GET/POST /api/closet`、`GET /user/closet/ids`、
    `PUT|DELETE /api/closet/{tid}` 及 `/user/closet` 同组路由；
    分页 JSON（current_page/data/links/pivot）与 PHP 对齐，收藏/取消收藏
    同步 likes 和 `score_award_per_like`（手工验证 add/list/rename/remove）。
  - 皮肤库：`GET /skinlib/list`（filter/keyword/sort/uploader/隐私过滤）、
    `GET /skinlib/info|show/{tid}`、`GET /texture/{tid}`、
    `PUT /texture/{tid}/name|privacy|type`、`DELETE /texture/{tid}`；
    删除时按 hash 引用计数删文件，隐私切换按积分公式扣/返
    （手工验证 list/info/rename/type/privacy/delete）。
  - 上传：`POST /texture`（multipart）——PNG 解码/重编码消毒、宽高 64×32 倍数、
    比例（steve/alex 2 或 1，alex 不能 2；cape 只能 2）、`max_upload_file_size`、
    SHA-256 去重（public 或本人已有 → code 2）、积分计算、写文件、自动加入衣柜；
    手工验证上传/重复检测通过。
  - 举报：`POST /skinlib/report`（重复检测、`reporter_score_modification`）、
    `GET /user/reports`、`GET /admin/reports[/list]`、`PUT /admin/reports/{id}`
    （reject/delete/ban，处理 `return_score`/奖励/封禁）；管理端校验 admin 权限。
    手工验证提交 → 我的举报 → 管理列表 → reject 全流程通过。
  - 管理端用户：`GET /admin/users/list`（搜索/分页）、
    `PUT /admin/users/{uid}/{email|nickname|password|permission|score|verification}`、
    `DELETE /admin/users/{uid}`，含目标权限校验（不能操作同级/更高权限，不能改自己
    permission）；manual 验证列表/昵称/积分/验证状态与 403 规则通过。
  - 管理端玩家：`GET /admin/players/list`、`PUT /admin/players/{pid}/{name|owner|textures}`、
    `DELETE /admin/players/{pid}` 及 `/api/admin/players/*`；含 owner 权限校验，
    手工验证列表/改名/改 owner/设置材质通过。
  - 管理端选项：`GET|POST /admin/options|customize|score|resource`，白名单字段保存、
    复选框缺省写 false、`site_url` 去尾斜杠/`index.php`、签到分数组合
    `sign_score_from,to`；手工验证读取/保存/回读通过。
  - 翻译管理：`GET /admin/i18n/list`、`POST /admin/i18n`（302 到页面）、
    `PUT|DELETE /admin/i18n/{id}`；`text` 列按 Spatie LanguageLine 语义返回
    locale→文本 map；手工验证 create/list/update/delete 通过。
  - 通知公告：`GET|POST /admin/communications`、`preview/import`、
    `PUT /admin/communications/{id}`、`POST /{id}/withdraw`；
    发布 message/both 时按 audience 写 `notifications`（type=SiteMessage，
    id 为 UUID v4，data 含 title/content/format/communication_id/delivery_token）。
  - 站内信：`GET /user/inbox`、`GET /user/inbox/{id}`（读取即标记已读）、
    `POST /user/notifications/{id}`、`GET /api/user/notifications`；
    手工验证发布 → inbox 出现 → 详情 → mark read 通过。
  - **已解决（2026-11-02）**：登录/注册即调用 `session.Regenerate(true)` 轮换
    session ID（session fixation 防护），随后再调一次
    `session.WriteCookie(ctx, sess)` 重写下发新 ID（gin 驱动 `removeSetCookie`
    同名覆盖，framework 注释亦明确要求轮换后重写）。- 路由契约：**174** 条（`codegraph/routes.json` 173 条漏记 `POST skinlib`）。
- 业务控制器/模板/OAuth2 尚未开始。

## 2. 已确认的跨语言契约

| 主题 | 结论 |
| --- | --- |
| 路由 | 174 条；`POST skinlib` 是上传入口，`GET skinlib/upload` 只是页面 |
| 请求体 | 前端 `blessing.fetch` 的 POST/PUT/DELETE 默认发 JSON；后端需合并 query+form+JSON |
| CSRF | `X-CSRF-TOKEN` 头优先，表单 `_token` 兜底；校验失败回 422 JSON |
| 图片 | 头像/预览默认 WebP（quality 100），`?png` 回退 PNG；带 ETag/304/Last-Modified |
| 缓存 | 头像/预览进程内缓存；`enable_avatar_cache` / `enable_preview_cache` 控制 TTL |
| 公告 | `currentAnnouncement()`：当前语言 → 空语言；`announcement` 或已发布 `both`；仅 published 渲染 |
| 模板 | 104 个 `.twig`（86 个入口模板/片段），复用 Java/.NET 已验证的 DOM 契约 |

## 3. 技术决策

- **ORM/Schema**：Goravel v1.18 + GORM 作为框架，但建表不走 `migrate:fresh`：
  Goravel 的 SQL Server grammar 依赖 `sys.allocation_units` / `sp_msforeachtable`，
  Babelfish（WiltonDB）不支持。统一用 `internal/db` 的显式 SQL 安装器，
  类型/默认值/索引直接对齐 PHP migrations；`artisan neoskin:install` 调用它。
- **驱动**：Goravel 官方 mysql / postgres / sqlite / sqlserver 驱动；
  `internal/db` 直接用 `database/sql` + go-sql-driver/mysql、pgx、go-mssqldb、
  ncruces/go-sqlite3。
- **时区**：`app.timezone = Asia/Shanghai`，与 PHP 一致。
- **APP_KEY**：Goravel 要求 32 个可打印字符；PHP 的 `base64:` 是二进制密钥，
  不能直接通过 Goravel 的 env 校验。`internal/laravel` 读取 `NEOSKIN_APP_KEY`：
  `Key()` 返回 base64 解码后的字节（用于 `laravel_token` HS256），
  `RawKey()` 返回原始字符串（用于 Laravel 签名 URL 的 `hash_hmac`）；
  Goravel 的 `APP_KEY` 仅框架内部用。
- **API 鉴权**：`/api/*` 接受 `Authorization: Bearer <RS256 JWT>`，按
  `scope:` 中间件校验；session 登录等价于 Passport TransientToken（`*`）。
  `/api/*` 不启用 CSRF（与 Laravel api 中间件组一致），见 `routes/web.go`。
- **laravel_token**：`POST /oauth/token/refresh` 下发 HS256 JWT
  （claims `sub/csrf/exp`），密钥为解码后的 APP_KEY，已与 PHP
  `hash_hmac` 输出对照验证。
- **签名 URL**：`internal/laravel.SignedPath` 生成相对签名链接，
  `ValidSignature` 按 Laravel 的「去掉 signature 后的原始查询串」重算，
  index 顺序与 PHP 无关（以实际收到的查询串为准）；重置/验证邮件使用它。
- **在线更新**：`internal/update` 复刻 `App\Services\UpdatePackage`，
  暂存目录 `storage/app/system-updates/{id}`，锁文件用 `O_CREATE|O_EXCL`
  （Windows 无 flock）；session 里存 `Info` 的 JSON 字符串。
- **Setup**：`/setup/*` 直接读取/写入 `.env` 并用新参数**另开**数据库连接，
  避免 Goravel 启动时缓存的连接；数据库变更需重启进程后由框架重新加载。
- **HTTP 超时**：`http.request_timeout` 从 3s 调整为 30s，否则更新源探测
  超过 3s 会被 Goravel 直接返回 408。
- **邮件**：走 Goravel Mail；`MAIL_HOST` 为空时直接返回错误（源为异常 → code 2）。
  邮件正文暂用 zh_CN 文案硬编码，翻译加载器未移植。
- **Ignition**：仅 `APP_DEBUG=true` 且 `APP_ENV=local|development` 启用；
  `execute-solution` 无解决方案注册表，恒 404；`update-config` 写
  `~/.ignition.json`（与 .NET 参考一致）。
- **Minecraft 渲染**：`internal/minecraft` 逐段移植 .NET 版（其本身是 PHP
  `Blessing\Renderer` 的 bug-for-bug 移植：体积点键截断、四肢守卫检查 8*hd 写入
  4*hd、腿部 z 偏移条件）。与 PHP 输出对比：尺寸完全一致，平均通道误差
  约 1.2–1.6/255（GD 扫描线与 Go 浮点取整差异）。
- **webp**：目前只输出 PNG（`image/png`），不区分源 `?png`；Go 侧未引入 webp 编码器。
- **头像回退**：默认头像/默认皮肤通过 `go:embed` 内嵌在 `internal/resources`。
- **模板**：Go `html/template`，验收以 Java/.NET 的 DOM 契约为准，不追求 Twig 语法。
- **remember me（2026-11-02）**：PHP `Auth::login($user, $keep)` 的 recaller
  cookie 值会被 Laravel 的 `EncryptCookies` 中间件整体加密（APP_KEY）；Go 版
  不实现该对称加密层，cookie 值改为
  `uid|remember_token|hmac(sha256, password_hash, APP_KEY)`：
  cookie 名仍为 `remember_web_59ba36addc2b2f9401580f014c7f58ea4e30989d`，
  HttpOnly + SameSite=Lax + Path=/，有效期 5 年（`auth.reminders.expire`
  默认 2628000 分钟）。校验语义与 PHP 等价：token 对应
  `retrieveByToken(id, token)`；第三段密码 HMAC 对应 PHP
  `hashPasswordForCookie` 的 `hash_equals`（改密后旧 cookie 立即失效）；
  `logout()` 轮换 remember_token（`cycleRememberToken`）并下发过期 cookie。
- **限流（2026-11-02）**：Laravel 的 `throttle:N,1` 走 `Illuminate\Cache\RateLimiter`
  于 Cache 的固定窗口；Go 在 `app/http/middleware/throttle.go` 自实现同语义
  （签名 sha1(method|path|ip)；Goravel memory cache 的 Increment 只认 `*int64`
  指针，计数基值/回填均存指针）。api 组生产环境全局 `throttle:60,1`
  （`APIThrottle`，testing 跳过）由 RouteServiceProvider 确认移植。
- **插件 Hook 7 方法（2026-11-02）**：`addRoute` 以回调形式保存，核心路由注册
  完成后由 `Hook.ApplyRoutes` 统一执行，再派发 `ConfigureRoutes`（payload 为
  goravel `route.Router`；PHP 为 `$event->router`）。`pushMiddleware` 收集后由
  `routes/web.go` 挂到 web 组中间件链尾——Go 的中间件链在 boot 期一次性建立，
  因此要求 push 在路由注册前调用；PHP 运行期 push 同样只对后续请求生效，
  语义等价。`sendNotification` 直接写 notifications 表（type
  `App\Notifications\SiteMessage`、notifiable_type `App\Models\User`，data 仅
  `{title, content}`），等价 SiteMessage 无 communication 关联的场景；Go 各
  通知读取路径用 `data["format"]` 取值，缺失为 nil，安全。
- **事件派发口径（2026-11-02 复核）**：PHP 全源 `event(new` / `dispatch(new`
  仅命中 5 处：`RouteServiceProvider:36`（ConfigureRoutes）与
  `PluginManager` 的 PluginBootFailed/WasEnabled/WasDisabled/WasDeleted；
  `Event::dispatch` 无命中。因此 `UserTryToLogin` 与抽象基类 `Event` 在 PHP
  从不派发，Go 保持不派发即等价，不需要额外接线。
- **校验消息（2026-11-02）**：422 字段文案来自
  `resources/lang/<locale>/validation.yml`，`:attribute` 用
  `validation.attributes` 映射（Laravel `getDisplayableAttribute` 语义）。
  `message` 按 Laravel `ValidationException::summarize` 取首条字段消息，
  多错追加 `(and N more error[s])`。偏差：PHP 首条消息按规则声明顺序，Go
  调用方用 map 收集错误，按字段名排序取首条——只在多字段同时失败时可能
  选中不同的一条（每条消息内容一致）。`errors` 的 JSON 键顺序在 Go 中由
  `encoding/json` 排序，PHP 为插入顺序（JSON 对象顺序无语义影响）。
- **后台页面守卫（2026-11-02）**：PHP 后台页面走 `authorize` + `can:admin`
  中间件；Go 新增 `requireAdminPage`（未登录 302 → `/auth/login`，非管理员
  403），API/JSON 端点继续用 `requireAdmin`（401 JSON）。修复 `UsersPage`
  曾忽略守卫响应、游客访问时 nil deref 的问题。
- **浏览器 E2E 资源（2026-11-02）**：`public/app` 属构建产物不入库。CI 的
  `browser-e2e` job 下载上游 `v6.0.10_patch` 发布包（SHA-256 固定
  `bb94f837…`）并只取 `public/app`，本地用 `scripts/copy-frontend.ps1`。
  浏览器测试用 Playwright（`playwright-core` + 系统 Chrome，不下载
  Chromium），由 `tests/browser` 的 Go 测试启动服务、播种管理员后执行
  `flows.mjs`；缺资源/Node 时自动跳过。
- **CI 质量门禁（2026-11-02）**：`race-coverage` job 用
  `go test ./... -race -covermode=atomic -coverprofile` 并上传覆盖率
  artifact；`browser-e2e` job 与 `build-test` 并行依赖。安全 E2E 覆盖
  CSRF 419、session 轮换、登出失效、remember cookie 篡改、越权 403。

## 4. 数据库矩阵（已跑通）

本机数据库：`..\tools\scripts\db.cmd`（MySQL 3307 / MariaDB 3308 / PostgreSQL 5433 /
WiltonDB 1433）。命令与坑位见 `DB-MATRIX.md`。

```powershell
go test ./internal/db -run TestMatrix -v
```

已确认的坑：

- SQLite 会掩盖重复打开连接、布尔/时间戳隐式转换等问题。
- `boolean` 与 `tinyInteger` 不是一回事：`users.verified` 是 bool，
  `textures.public` 是 smallint/TINYINT，写 `0/1`。
- PostgreSQL：字符串不能直接写 timestamp；用 `$N::timestamp` 或 `time.Time`。
- SQL Server：`public` 是保留字；参数占位符是 `@pN`（不是 `?`）；
  `SCOPE_IDENTITY()` 必须与 INSERT 同批次。
- 整型：INT/SERIAL/IDENTITY 可能 int32，COUNT 是 int64；扫描到 int64。

## 5. 待办

1. Phase 1/2：用户/玩家/纹理模型与仓储、bcrypt `$2y$`、Session/Cookie（PHP encrypter）。
2. Phase 3：Auth/User/Player/Closet/Texture/Skinlib/Report 控制器。
3. Phase 4：管理端（公告/通知/选项/翻译/插件/更新）。
4. Phase 6：86 个入口模板 + DOM 契约。
5. CI：5 库矩阵 + AOT/静态构建冒烟。

## 6. 页面与 i18n

- **模板目录**：Go 页面模板放在 `resources/pages/*.tmpl`，不放进 Goravel 的
  `resources/views` —— 后者会在启动时被框架的 view 服务提供者全量解析，遇到
  `t` 等自定义函数会直接启动失败。
- **翻译**：`internal/i18n` 读取 `resources/lang/<locale>/*.yml`（与 PHP 同步），
  并用 `language_lines` 表覆盖；缺 key 时回退 en，再回退原始 key。
- **locale 检测**：`DetectLanguage` 中间件复刻 `DetectLanguagePrefer` 的顺序
  （`lang` 参数 → `locale` cookie → `Accept-Language` → 默认），并回写 locale cookie。
- **语言包**：PHP 是写 `public/lang/<locale>.js` 静态文件；Go 用 `GET /lang/{locale}.js`
  路由即时生成同样的 `blessing.i18n = {...}`，避免依赖部署目录可写。
- **webp 头像**：目前只有 PNG 编码器，`<picture>` 的 source 也改为 `image/png`，
  避免浏览器按 webp 类型取到 PNG 后解码失败。

## 7. Go 版本- `go.mod` 要求 **Go 1.26**：依赖链中的 `golang.org/x/crypto v0.58.0` 声明 `go 1.26.0`，因此 CI 的 `setup-go` 固定 `1.26`。本地开发使用 1.27 亦可向下兼容编译。

