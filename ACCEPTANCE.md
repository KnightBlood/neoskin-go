# 验收清单（NeoSkinGo）

> 对照 `AGENTS.md` / `MIGRATION.md` 的硬约束。最后更新：2026-11-04。

## 1. HTTP 契约

| 项目 | 目标 | 状态 |
| --- | --- | --- |
| PHP 实际路由 | 174 条 | ✅ 唯一路径 145/145；179 条 Go 注册（含辅助路由，方法级说明见下） |
| 唯一路径覆盖 | 145/145 | ✅ 100% |
| 路由分组 | admin / api / user / oauth / auth / skinlib / texture / avatar / preview / raw / setup / csl / `.well-known` / `_ignition` | ✅ 全部注册 |
| 响应 JSON 结构 | 与 PHP `json()` 一致（`code/message/data`） | ✅ 关键路径 |
| JSON Content-Type | `application/json`（前端严格匹配） | ✅ 中间件统一 |
| 分页结构 | `current_page/data/links/...` | ✅ |
| OAuth2 | authorize / token / refresh / personal tokens / clients / scopes / tokens | ✅ Passport 兼容（RS256 JWT、bcrypt secret） |
| Bearer 鉴权 | `/api/*` scope 校验 | ✅ session 视为 `*` |
| 签名 URL / laravel_token | 与 PHP `hash_hmac` 兼容 | ✅（需 `NEOSKIN_APP_KEY`） |

> 方法级说明：`GET /{player}.json` 与 `GET /csl/{player}.json` 共用 catch-all；
> `POST /skinlib` 已作为 `POST /texture` 的同处理器别名注册；PHP `Route::any`
> 路径（`/api`、`/setup/database`、`/auth/captcha`、`/admin/customize|options|
> score|resource`、`/admin/plugins/config/{name}`、`/.well-known/change-password`）
> 已改为 Goravel `Any`。唯一例外是 `QUERY` 方法：Goravel 未注册该非标准 HTTP
> 方法，其余 GET/POST/PUT/PATCH/DELETE/OPTIONS 均覆盖。

## 2. 数据兼容

| 项目 | 状态 |
| --- | --- |
| 16 张业务表 + `migrations`（共 17） | ✅ `internal/db` 安装器 |
| 表名/字段/主键（`uid`/`pid`/`tid`） | ✅ 对齐 PHP migrations |
| bcrypt `$2y$` 密码 | ✅ 可验证 |
| 纹理 SHA-256 hash | ✅ |
| 四库矩阵（MySQL/MariaDB/PostgreSQL/SQL Server） | ✅ 本机 2026-11-01：MySQL/MariaDB/PostgreSQL + WiltonDB（SQL Server 协议）；CI `db-matrix` 用真 SQL Server |
| 存量数据读取 | ✅ 只增不改，不做自动改写 |

## 3. 前端契约

| 钩子 | 状态 |
| --- | --- |
| `window.blessing`（version/locale/base_url/site_name/route） | ✅ |
| `#blessing-extra`（页面 JSON） | ✅（管理端兜底 `{}`） |
| `<main></main>`（认证页 JS 渲染点） | ✅ |
| `#logout-button` / `data-mark="nickname"` / `.locale` | ✅ |
| 用户面板、侧边栏、面包屑、页脚版权 | ✅ |
| `public/app` 静态资源 | ✅ 部署时复制（`scripts/copy-frontend.ps1`） |
| WebP / `?png` | ✅ 默认 WebP，PNG 回退 |
| 浏览器 E2E（首页/登录/用户中心/皮肤库/上传页/管理端） | ✅ CI（Playwright + 系统 Chrome，零 console error） |

## 4. patch 功能

| 功能 | 状态 |
| --- | --- |
| 管理员仪表盘入口 | ✅ |
| ZIP 运行包在线更新（SHA-256/暂存/备份/回滚/安装） | ✅ |
| 通知与公告（公告/站内信、草稿、发布、编辑、撤回、已读） | ✅ |
| 组件式编辑器（文本/标题/提示/折叠/分隔线、源码导入、实时预览） | ✅ |
| 统一分页样式 | ✅ |
| 更新源 / 插件仓库配置 | ✅ |
| 上传上限 / OpenResty / 1Panel 提示 | ✅ 文本提示（无 PHP ini 限制） |

## 5. 插件系统

| 项目 | 状态 |
| --- | --- |
| 清单扫描 / 启停 / 删除 / 上传 / wget / 市场 | ✅ |
| Hook 7 方法语义（菜单/路由/样式/脚本/徽章/通知/中间件） | ✅ Go 原生 API |
| Filter（add/apply/remove/priority） | ✅ |
| 24 个事件 + Dispatcher | ✅ 24 个常量；PHP 原生 5 个派发点（ConfigureRoutes + 4 个插件生命周期）已对齐，另派发业务事件供 Go 插件监听 |
| PHP 插件代码执行 | ❌ 不支持（Go 无法执行 PHP；与 .NET AOT 策略一致） |

## 6. 测试与 CI

| 项目 | 状态 |
| --- | --- |
| `go build ./...` | ✅ |
| `go vet ./...` | ✅ |
| `go test ./... -count=1` | ✅（含 `tests/e2e`） |
| `go test ./... -race` + 覆盖率 | ✅ CI `race-coverage`（coverage artifact） |
| 安全用例（CSRF/session 轮换/remember 篡改/越权） | ✅ `tests/e2e/security_test.go` |
| GitHub Actions `build-test` | ✅ |
| GitHub Actions 四库矩阵 | ✅ `db-matrix`（真 MySQL/MariaDB/PostgreSQL/SQL Server） |
| 浏览器 E2E | ✅ CI（Playwright + 系统 Chrome；前端产物按 SHA-256 校验下载） |

## 7. 已知限制

1. **PHP 插件不可执行**：仅提供 Go 原生 Hook/Filter/Event 扩展点；PHP 插件需改写为 Go 扩展。
2. **PHP session cookie 加密未实现**：使用 Goravel session；签名 URL 与 `laravel_token` 已兼容。
3. **翻译覆盖**：页面/语言包与业务 API 消息走 YAML + `language_lines` 覆盖；422 校验消息同样取自 `resources/lang/<locale>/validation.yml`（含 `:attribute` 友好名与 `message` 首条错误语义）。
4. **安装向导**：写入 `.env` 后需重启进程（Goravel 启动时加载配置）。
5. **`_ignition`**：仅 debug + local/development 启用，`execute-solution` 恒 404（无解决方案注册表）。
6. **浏览器 E2E 依赖前端产物**：`public/app` 不入库；CI 从上游 6.0.10 发布包下载并按 SHA-256 校验，本地用 `scripts/copy-frontend.ps1`。

## 8. 复验命令

```powershell
go build ./...
go vet ./...
go test ./... -count=1
powershell -ExecutionPolicy Bypass -File scripts\copy-frontend.ps1
npm install --prefix tests\browser
$env:NEOSKIN_BROWSER_E2E='1'; go test ./tests\browser -v -count=1
go test ./tests/e2e -v -count=1
```
