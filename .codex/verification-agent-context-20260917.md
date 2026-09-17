# Agent 工具读取与模型上下文优化验证

目标仓库：`C:/_MY_WORK/X-DEEIX-custom`，基线 `103e40d9`。本轮仅修改后端，保留原有前端工作；未提交、推送或部署。

## 行为与边界

- `read_conversation` 默认读取最新 20 条，单页最多 50 条；`before_id` 读取更早消息，`message_id + offset` 续读长正文。返回父消息 ID 以识别分支。正文不包含附件二进制或工具轨迹。
- 新增 `read_knowledge_base_content`，使用已有知识库可见性及文件成员关系校验，支持内置库与个人库。与 `read_file` 共用 UTF-8 安全的字节游标，默认 8 KiB、最大 32 KiB。知识库更新 schema 明确 title/content 二选一。
- 分页工具输出超出模型预算时，若剩余预算足够容纳重试 JSON，返回原始游标与较小的读取参数；不把未读内容错误标成已读。预算连重试说明也容纳不下时仍受现有硬预算约束。
- 平台工具定义按名称排序，消除 Go map 随机顺序导致的请求前缀变化。
- 普通对话将当前轮模型可见的工具调用、结果及阶段信息保存在现有 `chat_run_events` 的 `tool_history` scope。保存前保留现有脱敏与工具输出预算规则，读取不受 UI trace 的 1 MiB 截断限制。回放绑定用户、当前会话、所选分支消息及对应 run，避免混入旧重试或引用会话的工具序列。
- 回放适用于所有模型路由，并保持 call ID、参数、结果及已有 ThoughtSignature 字段。历史推理文本遵循现有 passback 设置。超过有效模型输入预算时按完整旧用户轮次移除，不拆开工具调用/结果配对。Responses 的下一轮状态指纹包含工具序列。
- DeepSeek 专用 `prompt_cache_hit_tokens` / `prompt_cache_miss_tokens` 仅在请求上游模型名为 `deepseek` 或 `deepseek-*`（含命名空间）且使用 OpenAI/OpenRouter 协议时解释。不会根据响应自报模型名或网关域名改变其他模型用量。miss 属于普通输入，不是缓存写入。
- 修复所有输入均命中缓存时，普通输入用量错误回退到 prompt 估算值的问题；普通对话与 AgentTurn 使用同一判断。

## 验证

在已有 Linux 镜像 `codex-deeix-go-test:20260830` 中运行，Go 1.26.5、CGO 可用；不连接生产数据库或付费模型 API。

```sh
go test ./internal/application/conversation ./internal/application/knowledgebase ./internal/infra/llm ./internal/infra/persistence/postgres/conversation -count=1 -timeout=180s
go build ./...
```

受影响四个包完整测试通过。新回归覆盖：

- 65 条消息翻页无重复遗漏，长中文/emoji 正文逐字节续读还原，错误 UTF-8 游标与越权拒绝。
- 内置知识库通过可见文件入口读取，权限错误原样拒绝；原知识库服务测试继续覆盖文件成员关系与真实所有者身份。
- 多次组装工具定义保持一致；输出压缩保留可重试的原始游标。
- DeepSeek 流式事件、最终流式结果及非流式用量一致；GPT/Claude/Qwen/Gemini 请求即使响应自报 DeepSeek 也不解释专用字段；缓存 0%/100%、无效计数及重复归一化边界。
- 工具序列保存与回放、签名与推理策略、Responses 前缀一致性、完整轮次裁剪、孤儿结果拒绝、允许后续轮复用已完成调用 ID、凭据脱敏。
- 实际 prompt 组装路径覆盖 DeepSeek、OpenAI Responses、Anthropic、Gemini，保留工具参数/结果，排除其他会话的工具序列。
- 仓储超过 1 MiB 载荷完整读取、覆盖更新、用户隔离与旧 run 隔离。

初次检查修复了新文件导入别名错误；裁剪测试修正为真正超预算的输入。最终真实发送路径复核补齐了普通对话的 UserID/ConversationID 传递。独立只读审查与主代理源码复核均已执行。

Windows 原生应用包测试受 sqlite-vec CGO 构建条件限制，已使用 Linux 容器完成验证。

## 尚未证明的事项

- 未调用真实 DeepSeek 账户，未测量上线后的缓存命中率或费用降幅。专用字段解析修正内部统计；稳定前缀和工具回放才直接减少可避免的前缀变化。时间/脚本模板、模型切换、上下文压缩和供应商缓存淘汰仍会影响命中率。
- 旧对话未保存完整模型可见工具序列，无法补造原样回放；新快照从更新后的工具调用开始积累。原有审计结果仍保留。
- Agent Group 的不同成员/attempt 仍使用既有独立上下文及摘要/ledger，不把多个成员的工具序列伪装成同一个 assistant 的历史。
- 当前通用 Message 类型之外的提供方加密 reasoning item 不在此次新增存储字段范围内；已有工具 ThoughtSignature 会保留。

## 发布与回滚

不增加依赖、环境变量或数据库 schema。后续发布需构建并部署后端。回滚代码后旧版本会忽略独立 `tool_history` scope；无需删除快照数据。旧请求前缀与新工具 schema/排序不同，更新后的首次请求可能需要重新建立缓存。

## 临时资源

仅启动本任务专用的前台测试/构建容器，均使用 `--rm`，无发布端口；退出方式为测试超时/命令完成，异常时只停止对应容器。

- `codex-context-tests-20260917`：容器 `0dc8d10c96ca`，Linux 根 PID `2295262`。
- `codex-context-tests-20260917b`：完成后已自动移除，检查时已无运行对象。
- `codex-context-full-20260917`：容器 `eab14ec72b71`，Linux 根 PID `2303587`。
- `codex-context-build-20260917`：容器 `0cbf7d801af7`，Linux 根 PID `2306929`。
- `codex-context-verify-20260917`：容器 `7d8647431cfe`，Linux 根 PID `2321352`。

完成记录：最终四包全量测试及 `go build ./...` 顺序执行退出码 0，后端 `git diff --check` 通过。按本轮专用容器名称复查 `docker ps -a` 无匹配对象；所有测试/构建容器及其进程树均已退出，无发布端口，无有意保留的持续进程。
