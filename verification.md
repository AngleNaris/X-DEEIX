# Custom 分支发布候选验证

## 结论

custom 已形成可进入 VPS 发布流程的本地不可变候选：

- 候选提交：4bd131737ca3a2a920a5fae74cba9ba65f96389a
- 支持边界：单应用实例 + 多用户
- 结果：当前启用的自定义功能、原功能回归、单实例并发、安全隔离和 Sandbox 生命周期门禁通过
- 状态：Local Release Candidate Passed
- 尚未执行：push、VPS 上传、生产切换、生产域名登录后浏览器验收

本结论不是“已经部署成功”，也不覆盖多实例协调、未启用的真实 Provider/OCR 或最新 upstream/dev 兼容性。

## 基线

- 日期：2026-08-29
- 工作树：C:\_MY_WORK\X-DEEIX-custom
- 分支：custom
- HEAD：4bd131737ca3a2a920a5fae74cba9ba65f96389a
- origin/custom：43c32e4e71848c0f4d697d276190c3a8bb30f1ad
- upstream/dev：aca10647a838c86ceac3ec0a93bdcd744d960d42
- Merge Base：b435a2fd84424ac1a0bb56f4e20a4fea8f391a49
- 分叉：上游独有 89，Custom 独有 130
- 本地相对 origin/custom ahead 2，未 push

工作树中的 frontend/next-env.d.ts 修改和未跟踪治理/测试证据文件未纳入候选提交，也未被回退。

## 不可变制品

目录：output/release/4bd131737ca3

| 制品 | SHA256 |
| --- | --- |
| deeix-chat-4bd131737ca3-linux-amd64.tar | 5479c6cdd6eb7c278ad89bf8c6780025c682b7e9e62aa170cebb6895688ff189 |
| deeix-sandbox-4bd131737ca3-linux-amd64.tar | 98a8ddad2bfa7f53327e09cdf778e693fff16e5147670956aa29cae85411ec75 |

SHA256SUMS 已实际校验通过。Sandbox tar 同时承载 exact-SHA Sandbox MCP/Base 发布镜像。

| 镜像 | Image ID | 平台 |
| --- | --- | --- |
| deeix-chat:4bd131737ca3 | sha256:8e78ee66931192787f093576adebceb870817433100617ab6822edc1de74ae3a | linux/amd64 |
| deeix-sandbox-mcp:4bd131737ca3 | sha256:d3353216a98ba41c26efb661ba9ffa7b65235b17850d2101a10b12028a218a3f | linux/amd64 |
| deeix-sandbox-base:4bd131737ca3 | sha256:65c6b578a2c45cf358e00f2d6a8d26db0427d6d8b0c90c80221f6c0b0be02407 | linux/amd64 |

manifest.env 的 commit、三个 image ID、版本 0.3.6 和平台均与实际制品一致。

## 发布门禁

| 验证项 | 结果 | 证据 |
| --- | --- | --- |
| Frontend lint/typecheck/tests/build | Passed | lint 662 files；Node tests 36/36；Next.js 40 routes |
| Agent Group 跨端契约 | Passed | Frontend 6/6；Backend 两个目标包 |
| API Contract | Passed | Swagger/TypeScript 重生成无漂移，类型检查通过 |
| Backend | Passed | go test ./...、go vet ./...、go build ./... |
| Sandbox/MM | Passed | race、vet、定向生命周期测试 |
| CI/Compose | Passed | actionlint 1.7.7；根目录三份 Compose 与 Sandbox deploy Compose 当前 HEAD 均可渲染 |
| PostgreSQL 单实例集成 | Passed | schema/root-vector、Agent Group、Conversation、Billing 五组通过 |
| exact 运行身份 | Passed | /healthz、/readyz 正常；/api/v1/version 精确返回当前完整 SHA |
| Custom API | Passed | 80/80 |
| 原功能矩阵 | Passed | 45/45 |
| Platform Tools 开关/审批 | Passed | 39/39 |
| 审批终态/重启 | Passed | 66/66 |
| Sandbox 真 Docker | Passed | exact MCP/Base 镜像 94/94 |
| T6 单实例双用户负载 | Passed | 100 并发、10,000/10,000、0 error、QPS 1778.96、P95 187.99ms、P99 275.84ms |
| 运行资源/日志 | Passed | RestartCount 0；约 50.24 MiB；fatal/error 扫描 0 命中 |
| exact 匿名浏览器 | Passed | /chat 正确跳转登录页；DOM 与视觉渲染正常；console 0 warning/0 error |

完整源码门禁与 PostgreSQL 集成运行在 684c55d0 应用源码上；当前第二个提交仅修改发布文档和 Sandbox Compose/.env 发布约束。当前 HEAD 已重新执行 exact 镜像构建、四组黑盒、Sandbox 94 项、T6、Compose、版本 API 和浏览器渲染。

## 功能覆盖

自定义功能已覆盖：

- Agent Group 配置、成员/主管、持久化、隔离、关闭态与会话数据
- Platform Tools 管理开关、用户模式、审批阻断、批准/拒绝、并发单赢家、重启恢复
- 多用户 memory 隔离、200 条配额边界和并发准入
- credentials 创建、轮换、删除、跨用户隔离及响应脱敏
- Sandbox scope 隔离、shared/imports 权限、replay 防护、任务取消、symlink 拒绝、reset、shutdown drain
- 卡片、制品源码编辑、公开分享与撤销

原功能已覆盖：

- 角色、项目、卡片、文件、普通会话、NDJSON 流与恢复
- 私有资源跨用户不可读/不可写
- Agent Group 原有 CRUD

## 异常记录

Sandbox 首轮 exact 测试在第 10 项失败，原因是手工启动测试环境时未预置 imports 文件和恶意 symlink 夹具；认证、MCP 初始化和工具注册均正常。补齐测试夹具后同一 exact 镜像通过 94/94，未修改产品代码。

## 未覆盖与发布约束

- Qwen、Gemini、豆包、OCR、多模态和其他真实 Provider 未调用，必须在生产配置中保持关闭，直到各自完成受控 canary。
- 当前分支落后刷新后的 upstream/dev 89 个提交；本候选只证明固定 0.3.6 基线，不声明兼容最新 upstream。
- exact 候选没有执行登录后的生产域名浏览器旅程。VPS 切换后必须验证登录、普通聊天、至少一个自定义 CRUD、版本 API 和反向代理路由。
- 数据库备份恢复演练、VPS checksum 复核、原子切换和回滚演练属于目标环境发布门禁。
- 多实例、跨实例审批恢复、分布式锁和多 Worker 协调不在本轮范围。

## 资源回收

- 本轮内置浏览器测试页已关闭，Browser tab 列表为空。
- 候选、历史候选、PostgreSQL 门禁、Sandbox 门禁容器均已删除。
- 专用网络、候选/PG/构建缓存/Sandbox 会话卷均已删除。
- Mock Provider PID 19828 及子进程已退出。
- 端口 18080、18081、18771 均已释放。
- Docker/WSL/Windows 临时目录和临时构建脚本、旧源码 tar 已删除。
- 有意保留：三个 4bd131737ca3 exact 镜像、release tar、SHA256SUMS、manifest.env 和可复现测试证据。

## VPS 发布判定

当前候选可以进入 docs/CUSTOM_DEPLOYMENT.md 定义的 VPS 发布流程。发布时必须使用 exact-SHA 镜像、在 VPS 重新校验 SHA256、准备备份/回滚材料、原子切换，并在真实域名完成健康、版本、路由、日志和登录后浏览器门禁。
