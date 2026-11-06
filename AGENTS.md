# AGENTS.md — NeoSkinGo (Go / Goravel) 迁移约束

> 最后更新：2026-11-06

本文件约束所有 agent 在 `NeoSkinGo/` 目录内的行为。

---

## 1. 目标与基线

- **目标**：将 `blessing-skin-server-Laravel13`（6.0.11_patch）完整迁移到 Go 1.26 + Goravel v1.18。
- **基线**：`D:\WORKspace\NeoSkin\blessing-skin-server-Laravel13`（只读参考，禁止修改）。
- **当前版本**：迁移主体已完成；仓库已包含完整 Go 代码、测试与 CI。实际进度、剩余项与已知限制以 `PROGRESS.md` / `ACCEPTANCE.md` 为准。

## 2. 硬约束（不可破坏）

1. **HTTP 接口零变更**：所有 174 条路由的路径、方法、请求参数名、请求体字段名、响应 JSON 结构必须与源完全一致。详见 `MIGRATION.md` §6。
2. **数据完全兼容**：17 张表的表名/字段名/类型/主键（`uid`/`pid`/`tid`）与存量数据必须可读。密码哈希 bcrypt `$2y$` 必须可验证。纹理 `hash` 算法 SHA-256 保持。
3. **插件 API 契约**：Hook（7 方法）、PluginManager（13 方法）、Plugin、24 个 Event 的对外语义保持；
   PHP 插件代码不可执行，原生扩展按 D-001 重写（见根目录 `DECISIONS.md`）。
4. **patch 功能保留**：6.0.3 ~ 6.0.11_patch 全部功能。
5. **禁止修改源目录**：`blessing-skin-server-Laravel13`、`blessing-skin-server`、`blessing-skin-server-patch` 为只读参考。
6. **前端直接复用，不自己重写**：`resources/assets/`（前端源码）与
   `public/app/`（预编译产物）原样复用源项目，**不重新打包、不换技术栈、
   不做 SPA 改造**。静态产物拷贝到 `public/assets/app/`，`/lang/<locale>.js`
   由后端生成。服务端模板（Twig）是**前端契约**：`window.blessing`、
   `#blessing-extra`、`#logout-button`、`data-mark="nickname"`、`.locale`、
   `<main></main>`、`data-*` 钩子必须逐字保住。详见 `MIGRATION.md §9.0`。

## 3. 技术栈锁定

| 组件 | 版本 |
| --- | --- |
| Go | 1.26.x |
| Goravel | v1.18.0 |
| ORM | Goravel Orm (GORM) |
| 数据库 | SQLite / MySQL / MariaDB / PostgreSQL / SQL Server |
| 模板 | Go `html/template` |
| 前端 | webpack + yarn 1.x（复用现有） |

## 4. 禁止事项

1. 禁止改变数据库表结构语义（新增迁移必须增量，不改既有迁移）。
2. 禁止破坏插件 API。
3. 禁止改动 `.env` 运行时行为。
4. 禁止引入未记录的 Go 内部 API。
5. 禁止提交 `vendor/`、`node_modules/`、`.env`。

## 5. 代码风格

- 遵循 Go 标准（gofmt + golangci-lint）。
- 注释与文档使用中文，代码标识符使用英文。
- 前端资源与翻译文件不得丢失。

## 6. 验证清单

```sh
go build ./...
go vet ./...
go test ./...
```

- [ ] `go build` 无错误
- [ ] 所有 174 条路由注册成功（PHP 源码实际；`codegraph/routes.json` 的 173 条漏记 `POST skinlib`）
- [ ] 数据库 migration 在 SQLite / MySQL / MariaDB / PostgreSQL / SQL Server 五种库执行无错误
- [ ] 数据库矩阵测试通过（见 `DB-MATRIX.md`；本地 `tools\scripts\db.cmd` + CI 的 sqlsrv job）
- [ ] 存量数据可正常读取
- [ ] bcrypt 密码验证通过
- [ ] OAuth2 端点与 Passport 响应一致
- [ ] Minecraft 客户端可正常获取皮肤/披风

数据库矩阵的 Go 端步骤见 `DB-MATRIX.md`；跨语言的坑位/症状速查见工作区根目录
`../DATABASE-MATRIX-PLAYBOOK.md`（不随仓库提交）。

## 7. 进度报告格式

agent 完成一个阶段后必须报告：
1. 已完成的 MIGRATION.md 阶段编号。
2. 修改的文件清单。
3. 失败/阻塞项与原因。
4. 下一阶段待办。

每次会话结束必须更新 `PROGRESS.md`（总览/阶段进度/变更记录/下一步），
并在回复末尾给出一行「进度」摘要。

---

## 8. 源代码图谱（CodeGraph）

源项目 `D:\WORKspace\NeoSkin\blessing-skin-server-Laravel13`（只读）已生成机读代码图谱，移植时**优先读取它**，不要靠反复 grep 源码推断契约。

| 文件 | 内容 | 用途 |
| --- | --- | --- |
| `CODEGRAPH.md` | 架构总览、17 张表、路由分组、插件 API | 先读这一份建立全局认知 |
| `codegraph/routes.json` | 173 条路由（method / uri / name / action / middleware） | 1:1 生成路由注册代码；源码另有 `POST skinlib`，实际 **174** |
| `codegraph/controllers.json` | 20 个控制器类名 | 对照拆分 Controller |
| `codegraph/models.json` | 6 个 app 层 Eloquent 模型 | 对照生成 Entity/Model |
| `codegraph/events.json` | 24 个 Event + 9 个 Listener | 事件总线移植 |
| `codegraph/services.json` | 11 个 Service | 业务层移植 |
| `codegraph/middleware.json` | 12 个中间件 | 过滤器/中间件链移植 |

使用纪律：

1. 路由以 `routes.json` 为唯一权威，**不得增删改**路径、方法或参数名。
2. 表结构以 `database/migrations/` 为准（`codegraph` 不含字段级 DDL），字段名/类型/默认值必须逐字对齐。
3. 若发现 `codegraph` 与源码不一致，以源码为准，并在本文件记录偏差。
4. 迁移过程中新增的任何映射关系，追加到本目录 `PORT-NOTES.md`，不回写源项目。
