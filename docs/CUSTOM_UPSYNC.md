# Custom 上游同步维护手册

- 适用分支：`custom`（自有 fork `origin` = AngleNaris/X-DEEIX，官方上游 `upstream` = DEEIX-AI/DEEIX-Chat）。
- 起点快照：`custom` 已含 `upstream/dev@c4e3514f`（`merge-base custom upstream/dev = c4e3514f`，`custom...upstream/dev = 193 ahead / 0 behind`）。
- 本轮整理：`f9bb7d1b` 之后 8 个提交（`4a5a4303` ~ `ceea77f6`），工作区干净。

## 1. 三分支关系（一句话）

`upstream/dev`（官方）→ `dev`（本地镜像）→ `custom`（定制分支）；同步永远从上游拉、经 `dev` 中转、再并入 `custom`，不跳级。

## 2. 标准同步流程

```bash
git fetch upstream
git checkout dev && git merge upstream/dev        # 先在 dev 消化上游
git checkout custom && git merge dev              # 再把 dev 并入 custom
git merge-base custom upstream/dev                # 记录新基线
git rev-list --left-right --count custom...upstream/dev
```

- 解决冲突时只允许“复用上游、改自己”：
  - 契约以 `backend/internal/ports/*` 为准，`backend/internal/infra/*` 只做别名/委托实现。
  - 新增能力先抬进 `ports`（见 `ports/llm/adapter.go`、`ports/llm/types.go`、`ports/mcp/contract.go`）。
  - 组合根（`backend/internal/app/app.go`）显式注入：`objectstore.New`、extraction 引擎工厂。
- 禁止：扩大 `layering_test.go` 白名单、删除测试、弱化契约、整目录覆盖。

## 3. 层级闸门（必跑）

- 分层：`backend/internal/transport/http/layering_test.go:94-102`（`allowlist` 必须保持为空）。
- 后端全量（Linux CGO，`libsqlite3-dev gcc webp`，`CGO_ENABLED=1 GOTOOLCHAIN=auto`）：
  `go test -count=1 ./...`，目标 0 FAIL（本轮 `output-backend-round168.log`：110 ok）。
- 前端：`apps/web` 下 `tsc --noEmit`（0）、`pnpm test`（本轮 64 tests / 63 pass / 1 skipped）、`pnpm build`（本轮 34 页）。

## 4. 提交分组（本轮 8 批，可复用）

| 顺序 | 主题 | 内容 |
|---|---|---|
| 0 | chore | `.gitignore` 操作产物忽略、`theme.test.mjs` 挂入测试 |
| 1 | release | 版本 `0.4.4-beta.1`、锁文件、swagger/api-contract 生成物 |
| 2 | refactor | ports 契约上移 + infra 别名/委托 + `app.go` 组合根 |
| 3 | refactor | application 走 ports（channel/settings/skill/rag/upload/extraction/conversation 网关） |
| 4 | feat | application 定制保留（群组/制品/平台工具/trace/知识库/技能/硬删除） |
| 5 | feat | transport/handler + persistence + llm 传输层 |
| 6 | feat | apps/web 定制 + `verification.md` + `.codex` 验证归档 |
| 7 | docs | `.codex` 过程笔记归档 |

生成物只能用工具重建：swag（`backend/docs`）、`pnpm api:generate`（`packages/api-contract`）、`scripts/sync-*.mjs`（`apps/web/shared/generated`、`public/vendor`、`public/pwa/generated`、`app/fonts`）。

## 5. 不进版本库的东西

- `/output/`、`.codex-runtime-*/`、`.codex-candidate-config-*.yaml`、`/.playwright-cli/`、`/.agents/`、`/.mancode/`、`*.tar`、`/AGENTS.md`（mancode bootstrap）、`/backend/conversation_build_errors.txt`。
- `apps/web/public/vendor/`、`apps/web/public/pwa/generated/` 由 `postinstall/prebuild` 重建。
- `verification.md` 与 `.codex/verification-*` 是发布基线，要提交；过程性 `.json/.patch/plan` 归档从严。

## 6. 下次同步检查单

- [ ] `git fetch upstream` 后记录 `upstream/dev` HEAD、merge-base、ahead/behind。
- [ ] 生成物差异：swagger paths、api-contract、web generated。
- [ ] `layering_test` 白名单仍空；失败按“端口替换→组合根注入→测试适配”顺序修。
- [ ] 后端全量 + 前端三件套通过后，按第 4 节分组提交。
