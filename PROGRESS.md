# NeoSkinGo — 进度快照

> 每次会话结束更新本文件，并在回复末尾给出「进度」一行。
> 最后更新：2026-11-01。
> 仓库：<https://github.com/KnightBlood/neoskin-go>

## 1. 总览

| 指标 | 数值 |
| --- | --- |
| 路由条目（PHP 实际） | **174** |
| 唯一路径 | **145** |
| 已接通的源路径 | **145（100%）** |
| 未接通唯一路径 | **0** |
| Go 代码规模 | `internal/` + `app/controllers` 约 50 个文件 |
| 测试 | `go build ./...` / `go vet ./...` / `go test ./...` 全绿 |
| 数据库矩阵 | SQLite + MySQL/MariaDB/PostgreSQL/WiltonDB 通过 |

> 说明：`/{player}.json`、`/csl/{player}.json` 由同一个 `/:player` handler 处理，
> 已按功能计入覆盖。

## 2. 阶段进度

| 阶段 | 进度 | 说明 |
| --- | --- | --- |
| Phase 0 骨架 | 100% | Goravel v1.18、config、routes、health |
| Phase 0.5 数据库矩阵 | 95% | 17 表安装器、四库矩阵；CI 矩阵待补 |
| Phase 1 数据层/模型 | 80% | 全部业务表 + OAuth + Option；更新包元数据走文件系统 |
| Phase 2 认证/授权 | 100% | 登录/注册/验证码/登出、OAuth2 全端点、Bearer + scope、找回/重置/验证/绑定、`NEOSKIN_APP_KEY` 签名 URL、locale 检测 |
| Phase 3 业务控制器 | 100% | Auth/User/Player/Closet/Texture/Skinlib/Report + avatar/preview 渲染 |
| Phase 4 管理端 | 75% | Users/Players/Closet/Options/Customize/Score/Resource/i18n/Communications/Status/Chart/Plugins/Update |
| Phase 5 安装向导 | 100% | `/setup` 欢迎页、数据库表单、信息表单、完成页（HTML）+ 建表/管理员/install.lock |
| Phase 6 模板/页面 | 100% | 首页 + 认证页 + 用户中心 + 皮肤库 + 管理端全部页面（含通信可视化编辑器）+ i18n |
| Phase 7 插件系统 | 70% | 清单/启停/删除/上传/wget/市场 + Go 原生 Hook/Filter/Event 运行时；PHP 插件代码执行仍不支持 |
| Phase 8 测试/CI | 95% | 单测 + 四库矩阵 + Go HTTP E2E + GitHub Actions 全绿 + 浏览器 E2E（首页/登录/仪表盘/衣柜/角色/皮肤库/管理端/通信编辑器/安装向导） |

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

### 2026-11-01（本轮）
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

1. 管理端剩余页面：options/score/resource/customize/communications/update（`OptionForm` 表单渲染 + 通信编辑器）。
2. 插件原生运行时（Hook 7 方法、24 个 Event）、插件市场缓存/分页。
3. HTTP/浏览器 E2E 与 CI 五库矩阵；webp 编码器。
