# 前端重构计划（React 19.3 + Vite 8.3.3）

> 目标：在不改变后端 HTTP 契约（174 路由 / 17 表 / OAuth2 / Minecraft JSON）的前提下，
> 用 React 重写两套前端，替换现有 Twig/Go 模板页面。
>
> 状态：计划稿（未开工）。最后更新：2026-11-01。

## 0. 与既有约束的关系

`AGENTS.md` §2.6 要求「前端直接复用，不自己重写」，其目的是保护现有 Twig 契约与
`public/app` 预编译产物。本计划是**用户明确要求的重构**，因此按以下边界执行：

- `public/app`、`resources/assets` 与现有 Go 模板**全部保留**，继续作为
  `FRONTEND_MODE=legacy`（默认）的实现，随时可回滚。
- React 重写只通过新目录 `frontend/*` + `public/app-next/*` + 新增 SPA 壳路由落地，
  **不修改**既有模板的 DOM 契约（`window.blessing`、`#blessing-extra`、
  `#logout-button`、`data-mark`、`.locale`、`<main>`）。
- 后端 `174` 条路由的路径/方法/参数/JSON 结构保持不变；仅按 §3.2 新增页面级 JSON 接口。
- 待全部验收通过后，才考虑把默认值切到 `next`。

---

## 1. 技术选型与版本锁定

| 组件 | 锁定版本 | registry 最新 | 用途 |
| --- | --- | --- | --- |
| Node.js | **24.12.0 LTS** | — | 构建运行时（Vite 8 要求 `^20.19 \|\| >=22.12`） |
| pnpm | **12.16.1** | 12.16.1 | 包管理（两项目独立安装） |
| React / React DOM | **19.3.1** | 19.3.1 | UI 运行时（满足「19.3」要求；19.3.0 亦可用） |
| Vite | **8.3.3** | 8.3.5 | 构建与开发服务器（按需求锁 8.3.3） |
| `@vitejs/plugin-react` | **6.1.2** | 6.1.2 | React 插件（peer `vite@^8`） |
| TypeScript | **5.9.3** | 7.0.2 | 类型（暂不升 TS 7，见 1.1） |
| TDesign React | **1.18.6** | 1.18.6 | 站点端（公开站 + 用户中心）组件库 |
| Ant Design | **6.6.8** | 6.6.8 | 管理端组件库 |
| React Router | **8.5.1** | 8.5.1 | 路由（Data Router；peer React `>=19.2.7`） |
| TanStack Query | **5.105.1** | 5.105.1 | 服务端状态 / 缓存 / 重试 |
| Axios | **1.21.0** | 1.21.0 | HTTP（拦截器处理 CSRF / 401 / 419） |
| i18next / react-i18next | **26.6.0 / 17.1.10** | 同左 | 国际化（复用后端语言包） |
| dayjs | **1.11.23** | 1.11.23 | 时间（Asia/Shanghai） |
| Zod | 4.x（可选） | — | 表单/接口返回值校验 |
| Vitest / jsdom | **5.0.5 / 30.1.3** | 同左 | 单元/组件测试（peer `vite@^8`） |
| Testing Library | **16.3.3** | 16.3.3 | 组件测试 |
| Playwright | **1.64.1** | 1.64.1 | 浏览器 E2E |

> 版本以 `package.json` 精确锁定（不带 `^`），`pnpm-lock.yaml` 入库。
> 上表已在本机（Node 24.12.0）逐一核实 registry 可用性与 peer 依赖，核实日期 2026-11-01。

### 1.1 版本核实结论

- **需求指定的四个版本全部可用**：`react@19.3.0`、`vite@8.3.3`、
  `tdesign-react@1.18.6`、`antd@6.6.8` 均存在于 npm registry，可直接安装。
- **peer 兼容**：
  - `tdesign-react@1.18.6` → `react >= 16.13.1` ✅
  - `antd@6.6.8` → `react >= 18.0.0` ✅
  - `@vitejs/plugin-react@6.1.2` → `vite ^8.0.0` ✅
  - `vitest@5.0.5` → `vite ^8.0.0` ✅
  - `react-i18next@17.1.10` → `i18next >= 26.2.0` ✅
- **偏差与取舍**（记录于此，P0 落地时同步 `PORT-NOTES.md`）：
  1. React 取 **19.3.1**（19.3 线最新补丁），不取 19.3.0。
  2. Vite 按需求锁 **8.3.3**，不跟 8.3.5。
  3. TypeScript 取 **5.9.3**：虽然 registry 最新为 7.0.2，但 TS 7（原生重写）
     对 `antd`/`tdesign` 类型定义与构建插件链仍属新变量，P0 不冒险；
     TS 7 升级单独立项。
  4. React Router 取 **8.5.1**（peer 要求 React `>=19.2.7`，与本项目匹配）；
     若 P1 发现 Data Router API 不兼容，回退 `7.18.5`（`version-7` tag，仍在维护）。
  5. pnpm 取 **12.16.1**（计划初稿写的 10.x 已过期）。
- **可选 peer 无需安装**：`@vitejs/plugin-react` 的
  `oxc-transform-react` / `@rolldown/plugin-babel` / `babel-plugin-react-compiler`
  均为 optional，默认不装。

### 1.2 两个独立项目

```text
frontend/
  site/     # TDesign：首页、登录/注册/找回/验证、皮肤库、用户中心
  admin/    # Ant Design：管理端全部页面
```

- 两个项目**独立** `package.json`、`vite.config.ts`、构建产物与发布目录。
- 暂不引入共享包；公共逻辑（csrf、请求封装、i18n 加载、类型）各自维护，
  后续如重复度上升，再抽 `frontend/shared`（不改变“两项目”部署边界）。
- 构建产物：
  - `public/app-next/site/`（`base: /app-next/site/`）
  - `public/app-next/admin/`（`base: /app-next/admin/`）

---

## 2. 项目划分

### 2.1 `frontend/site`（TDesign）

| 模块 | 页面（对应后端路径） |
| --- | --- |
| 首页 | `/` |
| 认证 | `/auth/login`、`/auth/register`、`/auth/forgot`、`/auth/reset/:uid`、`/auth/verify/:user`、`/auth/bind` |
| 皮肤库 | `/skinlib`、`/skinlib/show/:tid`、`/skinlib/upload` |
| 用户中心 | `/user`、`/user/profile`、`/user/player`、`/user/closet`、`/user/reports`、`/user/inbox`、`/user/inbox/:id`、`/user/oauth/manage` |
| 其它 | 错误页（403/404/419/500）、语言切换、深色模式 |

### 2.2 `frontend/admin`（antd）

| 模块 | 页面 |
| --- | --- |
| 仪表盘 | `/admin`（统计卡、注册/上传曲线） |
| 用户/玩家 | `/admin/users`、`/admin/players` |
| 举报 | `/admin/reports` |
| 站点配置 | `/admin/options`、`/admin/customize`、`/admin/score`、`/admin/resource` |
| 内容 | `/admin/communications`（列表 + 组件编辑器 + 预览 + 撤回）、`/admin/i18n` |
| 插件 | `/admin/plugins/manage`、`/admin/plugins/market` |
| 系统 | `/admin/status`、`/admin/update` |

管理端复用站点端的认证状态（同一会话），但使用独立的 `ConfigProvider` / 路由 / 菜单。

---

## 3. 与后端的契约策略

### 3.1 页面接入方式

后端当前为每个页面路由渲染 HTML。重构采用**渐进切换**：

1. 新增静态目录 `/app-next/site|admin`（Go `Static` 提供，带长期缓存）。
2. 新增 SPA 壳路由（如 `/app-next/*`），用于联调。
3. 通过环境变量 `FRONTEND_MODE=legacy|next`（默认 `legacy`）与
   `FRONTEND_NEXT_ROUTES`（逗号分隔）逐页切换：命中时返回 SPA 壳（注入 CSRF /
   `window.blessing`），否则走现有模板。
4. 全部页面验收后设为 `next`；保留 `legacy` 回滚开关一个版本周期。

SPA 壳模板只保留：

```html
<meta name="csrf-token" content="...">
<script>window.blessing = { version, locale, base_url, site_name, route }</script>
<script>window.blessing.i18n = ...</script>
<div id="root"></div>
```

> 认证页 `/oauth/authorize` 保留服务端渲染（第三方重定向场景，避免额外 JS 依赖）。

### 3.2 需要的 JSON 接口补齐

现有 `/api/*`、`/skinlib/list`、`/user/score-info` 等可直接使用；以下需新增或调整：

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/bootstrap` | 站点配置、当前用户、权限、CSRF、语言、上传上限 |
| GET | `/api/user/inbox` | 站内信分页（`page`），返回 `data/links/total` |
| GET | `/api/user/inbox/:id` | 站内信详情（自动已读） |
| GET | `/api/user/reports` | 我的举报分页 |
| GET | `/api/admin/communications` | 公告/站内信历史分页（含已读/投递数） |
| GET | `/api/admin/communications/:id` | 编辑器初始数据（blocks/source/audience） |
| GET | `/api/admin/status` | 运行状态（版本/环境/DB/插件） |
| GET | `/api/admin/update/info` | 更新信息（latest/current/can_update/package） |
| GET | `/api/i18n/:locale` | （可选）语言包 JSON；否则复用 `/lang/:locale.js` |

已有接口保持路径/字段不变；新增接口统一 `{code,message,data}` 或分页结构。

### 3.3 认证 / CSRF

- 会话 Cookie 由 Go 下发；SPA 所有请求 `credentials: 'same-origin'`。
- 启动时读取 `<meta name="csrf-token">`，写请求统一带 `X-CSRF-TOKEN`。
- 401 → 跳转 `/auth/login?redirect=...`；419 → 提示刷新；403 → 403 页。
- Bearer 模式（第三方）由后端 `/api/*` 支持，前端不依赖。

### 3.4 国际化

- 启动时加载 `/lang/<locale>.js`（`blessing.i18n = {...}`），
  或调用 `/api/i18n/:locale`，注入 i18next 资源。
- 语言键与 PHP 版一致（`auth.*`、`user.*`、`skinlib.*`、`admin.*`、`general.*`）。
- 切换语言：写入 `locale` cookie（后端已支持）+ i18next changeLanguage。
- 日期/数字统一 dayjs + `Asia/Shanghai`；不依赖浏览器本地时区。

### 3.5 组件对齐原则

- 业务字段名、提示文案、校验规则、分页参数（`page`）、上传字段（`file`）与 PHP 版一致。
- 交互位置对齐 Twig：导航栏、侧边栏、用户面板、面包屑、页脚版权、`#blessing-extra` 数据。
- 不复制 Twig 的 HTML 结构，但保留可测试的语义（`data-mark="nickname"`、`#logout-button`、
  `.locale`、`<main>`），供现有插件/脚本兼容。

---

## 4. 组件映射（Twig/AdminLTE → TDesign / antd）

### 4.1 站点端（TDesign）

| PHP 组件 | React 方案 |
| --- | --- |
| `layout-top-nav` 导航栏 | `Layout.Header` + `Menu` + `Dropdown` |
| `sidebar` / `side-menu` | `Layout.Sider` + `Menu`（子菜单） |
| `user-panel` 头像/昵称/徽章 | `Avatar` + `Tag` + `Space` |
| `small-box` 统计卡 | `Card` + `Statistic` |
| `shared.grid` 栅格 | `Row` / `Col` |
| 材质列表 / 分页 | `List` + `Pagination` + `Image` |
| 表单（登录/注册/资料） | `Form` + `Input` + `Checkbox` + `Select` + `Button` |
| 验证码 | `Image`（`/auth/captcha`）+ `Input` |
| 弹窗/删号确认 | `Dialog` + `Form` |
| 通知/错误提示 | `MessagePlugin` / `NotificationPlugin` |
| 上传（皮肤库） | `Upload`（手动上传，字段 `file`） |
| 标签页（皮肤/披风） | `Tabs` |
| 深色模式 | `ConfigProvider` + CSS 变量（对齐 `dark-mode`） |

### 4.2 管理端（antd）

| PHP 组件 | React 方案 |
| --- | --- |
| 管理端布局 | `Layout` + `Menu` + `Breadcrumb` |
| 统计卡 / 图表 | `Statistic` + `Card` + 图表库（ECharts，按需） |
| 用户/玩家/举报表格 | `Table` + `Pagination` + `Space` |
| 筛选/搜索 | `Form` + `Input.Search` + `Select` |
| 编辑弹窗 | `Modal` + `Form` |
| 配置表单 | `Form` + `InputNumber` + `Switch` + `Select` + `Upload` |
| 翻译表格 | `Table` + 行内编辑 `Form` |
| 通信列表 | `Table` + `Tag` + `Popconfirm`（撤回） |
| 通信编辑器 | 自定义 BlockEditor（antd `Button`/`Input`/`Collapse`）+ 右侧预览 |
| 插件管理/市场 | `Table`/`List` + `Card` + `Upload` |
| 更新页 | `Card` + `Descriptions` + `Upload` + `Progress` |
| 全局提示 | `App.useApp()` → `message`/`modal`/`notification` |

---

## 5. 目录结构

```text
frontend/site/
  package.json
  vite.config.ts
  index.html
  src/
    main.tsx
    app/            # Router、Providers、布局
    pages/          # home/auth/skinlib/user
    components/     # Navbar、Sidebar、TextureCard、Previewer
    api/            # client.ts（csrf/401）、auth.ts、skinlib.ts、user.ts
    i18n/           # 资源加载、useT
    stores/         # 仅客户端状态（主题、语言）
    types/          # 与后端 JSON 对齐的类型
    styles/         # 主题变量、覆盖
  tests/            # Vitest + Testing Library
  e2e/              # Playwright

frontend/admin/
  （同上，pages/admin/*，antd ConfigProvider）
```

---

## 6. 阶段计划（建议 7 周）

| 阶段 | 周期 | 交付物 | 退出标准 |
| --- | --- | --- | --- |
| P0 基座 | 1 周 | 两个 Vite 项目、TS 配置、路由、请求层、i18n、主题、CI（lint/test/build） | `pnpm build` 通过；空壳页面可访问 |
| P1 认证 + 首页 | 1 周 | 首页、登录/注册/找回/重置/验证/绑定、SPA 壳与 CSRF | 与现有后端联调通过；Playwright 登录用例 |
| P2 皮肤库 | 1 周 | 浏览/详情/上传/举报、预览器（2D/3D/披风） | 上传→列表→详情闭环；头像/预览 URL 正确 |
| P3 用户中心 | 1 周 | 仪表盘、资料、角色、衣柜、举报、站内信、OAuth 应用 | 全部页面无 console error；分页/已读正确 |
| P4 管理端 | 1.5 周 | 仪表盘、用户、玩家、举报、配置、翻译、插件、市场、更新 | 表格/表单 CRUD 与现有 JSON 接口对齐 |
| P5 通信编辑器 | 0.5 周 | 组件画布、属性面板、源码导入、实时预览、撤回 | 与 `communications` 契约一致；预览 HTML 正确 |
| P6 切换与验收 | 1 周 | `FRONTEND_MODE=next`、性能/可访问性、E2E、文档 | 174 路由回归通过；可一键回滚 legacy |

---

## 7. 测试与质量

- **单元/组件**：Vitest + Testing Library；表单校验、权限分支、空状态。
- **契约**：TS 类型 + Zod 校验关键响应；分页/错误码快照。
- **E2E**：Playwright 驱动真实 Go 后端（SQLite fixture），覆盖：
  - 注册/登录/登出、CSRF 失效
  - 皮肤上传/收藏/删除、头像/预览格式
  - 管理端用户/玩家/举报/配置/通信发布
  - 语言切换（zh_CN/en）、深色模式
- **可访问性**：axe（关键页面 0 critical）。
- **性能**：Lighthouse（首页/皮肤库 LCP < 2.5s，CLS < 0.1）；路由级代码分割。
- **视觉回归**（可选）：Playwright screenshot 对比关键页面。

---

## 8. CI 与部署

```yaml
frontend:
  setup:
    - actions/setup-node@v4 with node-version: 24.12.0
    - corepack enable && corepack prepare pnpm@12.16.1 --activate
  site:
    - pnpm --dir frontend/site install --frozen-lockfile
    - pnpm --dir frontend/site lint
    - pnpm --dir frontend/site test
    - pnpm --dir frontend/site build
  admin:
    - pnpm --dir frontend/admin install --frozen-lockfile
    - pnpm --dir frontend/admin lint
    - pnpm --dir frontend/admin test
    - pnpm --dir frontend/admin build
  e2e:
    - pnpm --dir frontend/site exec playwright test
    - pnpm --dir frontend/admin exec playwright test
```

> 两个项目**不使用 pnpm workspace**（独立 `lockfile`、独立安装），
> 因此 CI 用 `--dir` 而不是 `--filter`。

- 构建产物提交策略：**不入库**（与 `public/app` 一致），发布时由 CI 产出并复制到
  `public/app-next/{site,admin}`。
- 静态缓存：`/app-next/*` 带 hash 文件 `Cache-Control: immutable`；`index.html` 不缓存。
- 回滚：`FRONTEND_MODE=legacy` + 保留 `public/app` 旧资源。

---

## 9. 风险与对策

| 风险 | 影响 | 对策 |
| --- | --- | --- |
| 版本可用性 | — | **已排除**：四个指定版本均已核实可用（见 1.1） |
| TypeScript 7 / 生态升级 | 构建链回归 | P0 锁 TS 5.9.3；升级单独排期 |
| React Router 8 API 变化 | 路由改造返工 | P1 先跑通最小 Data Router；不兼容则回退 7.18.5 |
| CSRF/session 与 SPA 不匹配 | 登录/写操作 419 | 统一 `X-CSRF-TOKEN`，后端壳注入；E2E 覆盖 |
| `Content-Type` 严格匹配 | 接口被当文本 | 后端已统一 `application/json` |
| 上传/大文件限制 | 皮肤库不可用 | 与后端 `max_upload_file_size`、Nginx `client_max_body_size` 对齐 |
| 通信编辑器复杂度 | 富文本/预览不一致 | 先复刻既有 blocks 协议（`type/title/content/tone/expanded`），预览走 `/admin/communications/preview` |
| 管理端表格字段差异 | 数据错位 | 以现有 JSON 字段为唯一契约，写类型 + 契约测试 |
| 深色模式/主题不统一 | 视觉割裂 | CSS 变量 + 两库主题算法，颜色表复用 PHP `navbar_color/sidebar_color` |
| 插件/第三方脚本依赖 DOM | 功能失效 | 保留 `data-mark`、`#logout-button`、`.locale` 等钩子；提供 `window.blessing` |

---

## 10. 验收清单

- [ ] 两个项目独立安装/构建/发布，版本与本文一致
- [ ] site/admin 全部页面路由与 PHP 版一一对应
- [ ] 登录态、CSRF、401/419、语言、深色模式正确
- [ ] 皮肤库上传/详情/预览/举报闭环
- [ ] 用户中心（资料/角色/衣柜/举报/站内信/OAuth）闭环
- [ ] 管理端 CRUD、通信发布/撤回、插件、更新页闭环
- [ ] 组件与字段命名/提示文案对齐 PHP 版（语言包键一致）
- [ ] Playwright E2E 全绿；Lighthouse 达标
- [ ] `FRONTEND_MODE` 可一键回滚 legacy
- [ ] 文档：README、部署文档、迁移说明同步
