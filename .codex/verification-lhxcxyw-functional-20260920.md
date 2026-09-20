# custom 合并后实际功能测试（2026-09-20）

## 修复复验：三处已通过（2026-09-20）

用户要求修正后，本轮修复了个人渠道图片重试、视频重查和 HTML 续写预览。下方原始核验保留为问题发现历史；其失败结论已由本节复验更新。此前未覆盖的真实供应商、生产数据库等边界不变。

### 修复

- 媒体恢复统一经 `buildMediaRecoveryRoute`，严格解析 run 中 `user-model-N`；个人路由按原用户及模型读取、核验原上游 ID，使用原协议/模型名和当前有效密钥。复用现有 header 合并逻辑，平台路由仍走原分支。错误、越权或被移动的路由不会回退到平台渠道，无 schema 变更。
- 合并 HTML 制品保留稳定 ID，并记录各段来源 ID；Preview 按来源映射完整制品。续写后的独立新文档/代码块保持各自索引，避免选到无关制品。
- 上轮非法协议 500→400 修复继续保留。

### 本轮验证

- Go 渠道、会话、HTTP 渠道三个包全部通过，修复后二进制构建成功。新增用例覆盖个人/平台分流、非法绑定、账户边界、上游变化/禁用、header 保留。
- 前端 68/68 单测、typecheck、生产构建通过。新增三项制品用例覆盖多段续写、续写后新文档、不同消息相同代码的身份隔离。
- 修复后的真实应用 HTTP 复验 6/6：图片 503 下载失败后恢复、视频轮询中断后恢复，均成功下载并保存原始产物字节；其他账号请求被拒。图片再次重试返回 expired、视频再次恢复返回 400；每个场景只有一次上游生成 POST。卡片、制品分享撤销、群组配置和流式聊天复验通过。
- 浏览器分别点击原文与续写段 Preview，均打开完整制品，iframe 显示 MERGE HTML 与 Continuation verified；截图已检查。
- `git diff --check` 通过。没有重新跑上一轮已经通过的全量 Go / canvas / API 契约检查；本轮未修改 API DTO 或画布。

证据位于 `output/merge-functional-20260920/`：`repair-api-results.json`、`repair-backend.log`、`repair-frontend-test.log`、`repair-typecheck.log`、`repair-build.log`、`repair-html-snapshot.txt`、`repair-html-second.txt`、`repair-html-preview.png`。

### 清理与交付边界

- 测试容器 `codex-merge-app-repair`（Linux PID 775945、18940）已停止并自动移除；修复构建容器已退出移除。
- Python fixture PID 26452（18941）与 Playwright mergerepair PID 29892 已关闭；关闭前记录完整进程树，关闭后所有已记录 PID 均不存在。
- `repair-cleanup.json` 确认剩余进程和监听端口均为空；docker codex-merge 容器列表为空。无保留服务。
- 源码未提交/推送/部署；保留用户原有 frontend 修改。生产供应商未调用，恢复验证仍使用本地模拟上游配合真实应用与存储。

---

## 首轮核验结论（修复前）

目标工作区 `C:\_MY_WORK\X-DEEIX-custom`，分支 `custom`，HEAD `d4ac49fe`。作者功能以选择性移植方式落地，非来源 main 全量合并；来源及取舍见 `verification-lhxcxyw-integration-20260919.md`。

本轮完成隔离服务、真实 HTTP、SQLite/文件存储与浏览器交互验证。**尚不能判定完整验收通过**：29 个 HTTP 场景中 27 个通过、2 个媒体恢复场景失败；另有 HTML 续写的浏览器预览失败。修复了个人模型非法协议返回 500 的错误映射，现返回 400。其余缺陷保留复现证据，未扩大本轮变更到路由恢复或制品状态管理。

## 环境与边界

- 当前工作树（含用户原有未提交 frontend 修改）构建，而非纯 HEAD 发布制品。
- 当前编译的 Go 程序与 Next 静态产物挂载至独立容器，`127.0.0.1:18940`；健康检查成功，版本标记为 dev。
- 本地 OpenAI 兼容 fixture：`127.0.0.1:18941`。应用认证、鉴权、数据库、文件存储、流处理、下载与浏览器均为真实链路；模型回答、图片/视频生成结果由 fixture 提供，不代表真实供应商可用性。
- 独立测试账号、SQLite 与 storage；未改生产配置、账号、额度或数据。未 push、未部署。原始 `C:\_MY_WORK\DEEIX-Chat` dev 工作区未修改。
- 证据目录：`output/merge-functional-20260920/`。`api-results.json` 23/23；`additional-results.json` 4/6。

## 实测矩阵

| 功能 | 结果与证据 |
| --- | --- |
| 个人渠道开关、审批保护 | 禁用返回 403；pending_approval 无法自行变为 active；跨账号读取 404；响应不暴露密钥 |
| 个人模型与连通性 | 模型创建、列表、上游发现 4 个模型、连通测试通过；浏览器点击测试显示正常 |
| 非法协议 | 复现原 500，修复后 HTTP 400；新增 handler 回归覆盖非法协议、缺协议及真实内部错误 |
| 项目文件 | 写入、读取、覆盖、ZIP 下载、ZIP 导入、删除通过；路径穿越/ZIP 穿越及跨账号拒绝；浏览器 Project files 打开 hello.txt，编辑器显示 overwritten |
| 会话系统提示词 | 修改、清空、重新读取及跨账号拒绝通过；项目提示词和会话提示词实际进入上游请求 |
| 普通聊天与消息删除 | 个人模型调用、回复持久化、跨账号删除拒绝、本账号删除成功；另测 NDJSON 流式聊天 |
| 图像生成/编辑 | 正常生成和 multipart 编辑通过，流结束、消息 success、附件保存和文件字节读取通过 |
| 视频生成 | 提交、轮询、下载、保存与读取通过；浏览器播放到 1 秒结尾，readyState=4，error=null |
| 图片下载重试 | **失败**：注入 503 下载失败产生 pending artifact，原账号重试返回 llm.model_route_not_configured；其他账号请求被拒绝 |
| 视频任务重查 | **失败**：任务提交成功后注入轮询 400，原账号重查返回 llm.model_route_not_configured；其他账号请求被拒绝 |
| 签名媒体/缩略图 | 无签名 401；有效签名返回真实图片、no-store；变更 variant 签名失效；跨账号文件读取 404 |
| 文件公开分享 | 创建、下载、撤销后 404，通过 |
| 创作画布 | 浏览器创建提示词与生成节点、拖拽连线、选个人模型、点击生成，输出图片可见；刷新后 3 节点/连线/图片保持；图片 naturalWidth=640；canvas.png 已视觉检查 |
| 卡片 | 创建、更新、列表读回、跨账号拒绝、删除，通过 |
| 制品管理 | 创建、更新、读取、跨账号拒绝、公开分享、撤销、删除，通过 |
| 角色与 Agent Group | 隔离环境开启默认关闭的功能后，角色/群组创建、成员 reasoningEffort 修改并读回、跨账号拒绝、删除，通过；未验证多 Agent 模型编排 |
| HTML 续写 | 两条流式回复持久化通过；**预览失败**：1280 与 1600 宽度点击两段 Preview 均未出现预览面板/iframe；html-wide-preview.txt 与快照保留 |

## 首轮缺陷定位（现已修复）

### 1. 个人渠道媒体恢复未重建正确路由

`backend/internal/application/conversation/service_media_artifact_retry.go:88` 和 `service_media_requery.go:95` 都调用 `BuildRouteForUpstream`。`backend/internal/application/channel/service_routing.go:750` 使用平台 `GetUpstreamByID`；个人渠道生成路由则由 `service_routing_user.go:30` 构造。恢复没有区分个人渠道与平台渠道。必须保持原用户/原上游边界修复，不能仅按数字 ID 随意回退至其他渠道。

图片首次注入损坏图片得到 expired 是正常行为：解码失败不属于可重试下载。修正 fixture 为 HTTP 503 后才复现上述路由缺陷。图片重试、视频重查均未到达恢复成功，因此不声称已验证成功恢复后的幂等性或不重复计费。

### 2. HTML 续写的 Preview 选择 ID 与合并制品 ID 不匹配

`frontend/features/chat/model/chat-artifacts.ts:507` 的 appendBlock 为合并结果追加 `:merged` ID；`frontend/features/chat/hooks/use-chat-artifacts.ts` 的 openArtifact 却通过单条消息 extractArtifactsFromContent 得到 ID，再选择全会话 artifacts。复现中两个按钮都没有打开预览。需将按钮选择映射到实际合并制品，并验证原始段/续写段均能打开完整内容。

## 本轮代码修复与自动检查

- `backend/internal/transport/http/channel/handler_user_model.go`：ErrInvalidAdapter / ErrProtocolRequired 映射至 400。
- `backend/internal/transport/http/channel/handler_user_model_test.go`：验证直接/包装协议错误为 400，真实数据库错误仍为 500。
- 前端单测 65/65、画布逻辑测试 41/41、typecheck、生产构建通过。
- Go 1.26.5 Linux/CGO/SQLite/cwebp 环境全量 `go test ./... -count=1 -timeout=180s` 通过；错误映射修改后 targeted HTTP channel 测试通过，重新编译程序并实测 400。
- `pnpm api:check` 通过，Swagger/TypeScript 契约无漂移。
- `git diff --check` 通过。没有提交或推送本轮修复。用户原有 frontend WIP 保留。

初期缺 cwebp/SQLite 链接依赖、错误 fixture 字段/响应断言及前端 API 指向 :8080 已修正，不作为产品缺陷。测试用 Next 构建指定 NEXT_PUBLIC_API_BASE_URL=http://127.0.0.1:18940。

## 未覆盖范围

真实供应商鉴权、限流/计费与产物差异；生产 PostgreSQL 迁移；真实外部模型与多 Agent 编排；XAI 视频延长；项目工具由模型发起的调用；渠道预设完整表单/管理员审批批准与拒绝；运行中断线恢复与取消；完整响应式/可访问性回归。上述不标记通过。知识库、Skills 等保留入口不等于重新完成业务验收。

## 资源回收

- 停止并自动移除 `codex-merge-app-20260920`（Linux PID 748965，18940）。构建/测试容器均已退出；最终 docker ps -a --filter name=codex-merge 无结果。
- fixture Python PID 20456、21588、22388 已退出；Playwright mergeui 根 PID 3484、22188 已退出，CLI close 成功。
- 最终检查上述 PID 及直接子进程无结果，18940/18941 无监听。没有有意保留的测试服务。
- 临时 test-auth.json 已删除并验证不存在；隔离数据库、storage、脚本、日志与截图保留作复现证据，不用于生产。
