# Custom 交互与数据边界修正计划

## 状态

- 任务：`local:01M1730YP754VE2BDRQ1J59H1E`
- 阶段：实施与验证
- 目标环境：单应用实例 + 多用户
- 工作树：保留现有脏改动、候选制品和验证证据，不回退、不清理

## 问题审查结论

- `F-1 / repository_fact / premise`：群组标题、群组自动标题和召回证据问题均已由当前代码确认。标题已有项目/群组字段却只渲染群组名；群组消息路径提前进入独立编排器；工具定义被写成 `sourceType=tool` 后被前端当作召回证据。
- `F-2 / repository_fact / technical`：记忆的 `Scope` 目前实际承担类别含义，所有记忆均为用户级全局数据。污染根因不是缺少 project/conversation scope，而是任务事实可被写入长期记忆、非偏好类别也会自动召回；会话已有历史与摘要，项目已有文件/卡片，不应再用长期记忆复制这些上下文。用户偏好继续作为明确的全局常驻记忆无条件注入。
- `F-3 / repository_fact / risk`：制品顶层执行、公开文件分享和系统 Vision 都跨越信任边界。不能用同源 Blob 直接执行不可信代码，不能公开存储路径，也不能把“容器/Provider 存在”当成 Vision 已可用。
- `F-4 / repository_fact / technical`：知识库内容不是独立文章，而是 `KnowledgeBase -> FileObject -> FileChunk`。AI 交互工具应复用个人知识库、文本文件写入和现有提取/RAG 索引链，不能建立第二套内容模型。

## 关键方案取舍

### 记忆

1. project/conversation scope：会重复会话上下文，并把跨项目污染降级为同项目跨任务污染，不采用。
2. **采用：用户级全局记忆 + category 路由**。类别为 identity/activity/context/preference/capability/experience；preference 无条件注入，capability/experience 走 RAG 自动召回，其他类别只提供按需读取提示。
3. 照搬 LobeHub 完整 topic memory/layer/metadata：覆盖过大，且项目不是可靠的任务边界；只借鉴分类和白盒管理，不复制其存储拓扑。

### 知识库 AI 工具

1. 新建独立知识库内容表：会与现有 FileObject/FileChunk、上传、提取和索引形成双事实源，不采用。
2. **采用：平台工具包装现有个人知识库文件链**。创建文本文件并关联、修改可写文本并重建索引、删除默认只解除关联。
3. 工具只操作当前用户的个人知识库；内置知识库继续由管理员维护，AI 只能读取。

### 未读状态

1. 仅在前端维护 Set：实现快，但刷新、重启和多标签页会丢失，不满足“未查看”的稳定语义。
2. **推荐：服务端 read marker**。按用户自有会话持久化读取时间/终态，前端只负责进入会话和活动回复完成时确认已读。

### 制品新标签页

1. 同源 Blob/`document.write`：代码少，但不可信脚本可能读取同源浏览器数据，不采用。
2. **推荐：一次性渲染令牌 + 顶层 HTTP 响应**。由服务端返回带 CSP sandbox 的纯制品文档，新标签页无宿主 UI、无 iframe。
3. 独立 renderer 子域：隔离最强，但需要额外 DNS、证书和部署拓扑，本轮不引入；若制品成为外部发布平台再升级。

## 分阶段实施

### P0 契约与回归夹具冻结

- 为 9 项行为分别建立失败用例，先证明当前缺陷可稳定复现。
- 固定记忆 category/召回策略、知识库内容工具、文件分享状态、渲染令牌状态、Vision 错误码、会话 running/unread 状态机。
- 更新 Go DTO/Swagger/生成 TypeScript 契约的预期变更清单。
- 记录当前普通聊天、群组、卡片、制品缩略图和文件 API 基线。

完成条件：所有新用例在未修复代码上按预期失败，枚举与权限边界无歧义。

### P1 无迁移的共享根因修复

#### 1. 群组标题组合

- `ChatArea` 使用已有 `agentGroup.contextLabel`，显示 `项目名 + 群组名`；空态使用同一个纯函数/格式规则。
- 保留项目-only、角色-only、无上下文行为；非法旧数据缺项目名时仅显示群组名作为降级。
- 使用同级字重和字号，整体截断并提供完整 tooltip/accessible label。

#### 2. 召回证据

- 后端工具 guidance block 保留工具数量，不再生成 definition `SourceRefs`。
- 前端排除旧 trace 中仅代表 definition 的 `sourceType=tool`；实际 `tool_call/tool_result/native_tool_result` 只有在存在调用/制品引用时才显示。
- 不改变发送给模型的 tools 数组，不改变工具执行、审计和历史 ContextArtifact。

#### 3. 群组自动标题

- 群组成功终态计算现有 `MetadataRefreshHint`，并在最终 assistant 消息持久化成功后调度 metadata 生成。
- 调度从群组成功路径直接进入现有生成器，不依赖顶层 `Billable=true`。
- 沿用“只替换默认标题/只在首次有效内容时运行”的幂等保护；失败、暂停、取消不调度。

完成条件：AC-1、AC-2、AC-11 通过，普通消息 metadata 和工具执行契约无变化。

### P2 全局长期记忆与分层召回

#### 数据模型

- 长期记忆仍只按 `user_id` 全局归属，不增加 project/conversation scope 或绑定列。
- `category` 使用 identity/activity/context/preference/capability/experience；召回方式由类别固定派生，不额外持久化 recall mode。
- 复用现有 `Scope` 存储列承载 category，应用/API 改用 category 语义并兼容旧字段，保留 `(user_id, memory_key)` 唯一键。

#### 写入

- HTTP/UI/API Contract 与 `save_memory` 支持 category，不接受 project/conversation 绑定。
- 只允许保存描述用户且预计长期有效的事实；项目、歌曲、制品、任务状态和会话临时决策必须留在会话、项目文件或卡片中。
- 保留平台写工具 auto/ask、审计、配额与脱敏；AI 默认写入 on-demand 类别，不得默认生成自动召回记忆。

#### 召回

- preference 保持现有无条件注入，但继续受固定 token budget 限制；capability/experience 先按类别过滤，再复用现有向量检索、相似度阈值、topK、token budget 和关键词回退。
- identity/activity/context 的正文不进入普通 prompt，只注入包含类别和数量的短提示，告知 AI 可调用 `list_memories` 主动读取。
- 扩展 `list_memories` 的可选 category/query/limit；有 query 时复用同一 RAG 选择器，返回内容受总字符/token 上限约束并明确标记为不可信用户数据。
- Prompt fingerprint、ContextArtifact 和召回证据只记录实际自动召回或工具读取的记忆；可用提示本身不算召回证据。

#### 迁移

- `profile -> identity`、`preference -> preference`、`custom/global/未知旧值 -> context`。
- 旧 custom/context 仅按需读取，不自动召回；设置页标记为待复核并允许用户重新分类。
- 不新增 scope 列或唯一索引迁移；SQLite/PostgreSQL 重复启动保持幂等，旧版本仍可读取原存储列。

完成条件：AC-3、AC-4 通过，200 条用户配额、向量检索和关键词回退性能门槛保持。

### P3 运行中与未读状态分离

- 保留现有 `streamingPublicIDs` 作为 running 状态源；去掉 running 小圆点。
- 会话标题/整行增加从左到右扫光，复用 `trace-sweep` 动画参数；`prefers-reduced-motion` 下改为静态强调。
- 增加持久化 read marker API/字段。列表 DTO 返回 `hasUnread`，依据最新成功 assistant 终态与 read marker 计算。
- 当前活动会话完成回复时标记已读；后台会话完成时保留未读；进入会话立即标记已读并乐观更新。
- 小圆点只绑定 `hasUnread`，与 running 可同时存在但互不覆盖。

完成条件：AC-5 通过；切换会话、后台完成、刷新、重启、同用户多标签页和 A/B 用户隔离均有证据。

### P4 制品正文、纯页面与下载

#### 内容归属

- 在 assistant 输出最终化时识别 thinking 块中的受支持制品 fenced block，将制品 block 移入 visible body；非制品代码与真实 reasoning 保留在 trace。
- 若正文已有相同制品则去重；消息持久化、分享、导出和刷新后均以修正后的 body 为事实源。
- 前端继续只从 assistant body 提取制品，避免建立第二套 trace 制品状态。

#### 操作

- 在现有制品工具栏加入 ExternalLink 与 Download 图标；下载继续复用 `downloadBlob` 和类型化文件名。
- 增加短时一次性 render token：用户可从未保存或已保存制品创建令牌；令牌绑定用户、kind、内容摘要、过期时间和使用状态。
- 新标签页直接请求顶层 render endpoint；响应为纯制品文档，无 DEEIX header/frame，设置 CSP sandbox、no-store、nosniff、no-referrer、frame-ancestors none。
- 公开分享页增加下载，并可生成受限公开 render token；撤销分享后不能再生成或使用新 token。
- 保持制品列表 DTO 不含 code、列表零 iframe、静态缩略图懒加载。

完成条件：AC-6、AC-7、AC-8 通过；令牌并发单次消费、过期、跨用户和撤销竞态有自动化测试。

### P5 独立文件分享

- 新增 FileShare 记录：share token、user/file、title/fileName/MIME/size 快照、active/revoked、nullable expiresAt、时间戳。
- 认证 API：创建/刷新分享、查看状态、撤销；公开 API：读取元数据、受控预览内容、下载。
- 每次公开访问验证 active、未过期、文件仍属于分享者且对象存在；不返回 StoragePath。
- 公开页复用制品分享的页面壳与文件预览映射：图片、PDF、音频、视频、纯文本可预览；未知/可执行 MIME 只提供下载。
- 内容响应复用 `filecontent.Write` 的 disposition、nosniff、CSP、CORP 和短缓存；下载使用 attachment。
- 现有“会话分享中的附件”路径保持兼容，不迁移成独立 FileShare。

完成条件：AC-9 通过；owner/other-user/missing、撤销、过期、对象删除、MIME 欺骗和缓存头全覆盖。

### P6 系统 Vision 图片提取

- 在现有 OCR engine 枚举增加 `system_vision`，不新增第二套图片处理队列。
- 通过小型共享端口复用 `system_multimodal_analyze` 的图片模型选择、route resolver、capability JSON、协议 adapter、附件授权和审计；禁止复制 Provider 客户端。
- 提取结果写入现有 processing/extract 文本、engine/source、错误码和完成时间；供非 Vision 文本模型和 RAG 复用。
- 设置页仅在系统多模态已启用且配置 image model 时允许选择；默认关闭。旧 `rapidocr/llm/...` 行为不变。
- 失败不删除源图片、不写伪造文本、不自动启用其他 Provider；用户可切换引擎后重新处理。

完成条件：AC-10 通过；本地用 fake route/model 验证，生产只在受控非敏感图片 canary 通过后启用。

### P7 知识库 AI 系统工具

- 新增 `list_knowledge_bases` 与 `list_knowledge_base_contents` 只读工具，返回当前用户可见的 opaque ID、名称、文件名、类型、更新时间和索引状态，不暴露 owner/storage path。
- 新增 `save_knowledge_base_content` 写工具：无 file ID 时创建用户文本文件并关联个人知识库；有 file ID 时只允许修改当前用户可写文本文件，并复用现有 overwrite/reindex 链。
- 新增 `delete_knowledge_base_content` 写工具：默认只将文件移出指定个人知识库；如需删除源文件，继续显式调用现有 `delete_file`，由引用检查和配额回收处理。
- 所有写工具复用现有 platform tool auto/ask 审批、限流、审计和当前鉴权 user ID；跨用户或内置知识库写入统一返回 not found/不可写。
- 创建的对象存储写入、FileObject 持久化和知识库关联必须具备失败补偿；修改触发去抖重建索引，移除后该知识库检索立即不可见。

完成条件：AC-15 通过；创建、修改、移除、审批、跨用户、内置库只读、索引失败与重复调用均有测试。

### P8 全量验证与发布门禁

- T0/T1：Biome、TypeScript、Node tests、Go unit/contract、API 生成一致性。
- T2：SQLite/PostgreSQL 向前迁移、旧数据、唯一约束、事务、用户隔离和同进程并发。
- T4：1440x900、768x1024、390x844 浏览器旅程；每条关键写链执行“写入 -> 刷新 -> 服务端回显”。
- T5：系统 Vision 仅受控 canary；未验证 Provider 保持关闭。
- T6：单实例双用户 100 并发/10,000 请求基线，记录 QPS/P95/P99、内存、RestartCount 和资源泄漏。
- 更新 `CUSTOM_TEST_PLAN.md`、`verification.md` 与部署/回滚说明；只有 exact SHA 通过全部启用功能门禁后才能形成新候选。

完成条件：AC-12、AC-13、AC-14 通过；结果按 Passed/Failed/Blocked/Not Run 分列，无未解释跳过。

## 重点测试矩阵

| 领域 | 主流程 | 边界/失败 | 回归 |
| --- | --- | --- | --- |
| 标题 | 项目 + 群组；新/已有会话 | 长标题、缺项目降级、移动端 | 项目、角色、普通空态 |
| 证据 | 实际卡片/记忆/工具结果 | 仅定义不显示、旧 trace | Prompt tools 仍完整发送 |
| 记忆 | preference 常驻；capability/experience 自动 RAG；其他类型按需读取 | 同项目新任务、任务事实拒绝、旧 custom、提示注入 | 配额、审批、向量/关键词回退、跨用户 |
| 会话状态 | running sweep -> unread dot -> read | 后台完成、刷新、重启、多标签 | 收藏/项目/角色/最近列表 |
| 制品 | 正文展示、纯页、下载 | thinking 混合、重复、令牌过期/越权 | 静态缩略图、编辑、分享撤销 |
| 文件分享 | 创建、预览、下载、撤销 | 过期、MIME 欺骗、删除、跨用户 | 会话分享附件、私有文件 API |
| Vision | system_vision 成功落库 | 关闭、缺模型、capability/adapter/Provider 失败 | RapidOCR/LLM OCR、文本模型降级 |
| 群组标题 | 首次成功自动命名 | 失败/暂停/人工标题/重复回调 | 普通会话标题与标签 |
| 知识库工具 | 创建文本、修改、移出、索引更新 | 内置库写入、跨用户、二进制、关联/索引失败 | 文件、知识库 UI、RAG、审批审计 |

## 发布与回滚

- Schema 仅允许向前兼容的新增列/表/索引；发布前备份 PostgreSQL 并验证 `pg_restore --list`。
- 先发布代码且保持 system Vision 关闭；完成 UI/API/迁移验收后再做单独 Provider canary。
- 紧急回滚：关闭 Vision、新分享入口和知识库写工具；旧代码继续读取原记忆存储列并忽略 nullable 字段/新表；一次性 render token 为短时内存状态，无需迁移回滚。
- 不从脏工作树构建，不复用当前 `4bd131737ca3` 作为修改后的候选；实施完成后必须生成新的 exact SHA 与不可变镜像。

## 计划自检

- 已覆盖全部 9 项及普通功能回归，没有把多实例纳入本轮。
- 没有新增前端依赖；标题、扫光、下载、分享页、工具/多模态路由均复用现有能力。
- 必要新增数据事实源只限于会话 read marker 和独立文件分享；记忆复用原表，知识库工具复用 FileObject/FileChunk，制品 render token 使用单实例短时状态。
- 安全边界未被“最小改动”简化掉：可执行制品、公开文件、外部 Vision 和知识库 AI 写入均有明确隔离、权限、审批、失败和回滚门禁。
