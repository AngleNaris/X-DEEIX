# 2026-09-22 上下文压缩修复生产部署验收

结论：部署成功。生产 https://ai.3efs.com/ 已运行上下文压缩预触发修复版本。

## 发布身份与范围

- 权威仓库：`C:\_MY_WORK\X-DEEIX-custom`，`custom`。
- Commit：`03071e525cb6c3ec851183b3a26324f11b276ed0`，版本 `0.3.6`。
- 镜像：`deeix-chat:03071e525cb6`，linux/amd64，config digest `sha256:2c3381d55cb0f079c10b57503cc4009355c652dacbff2f007e2fe891da1e1785`。
- 从精确提交的 git archive 构建（`release/03071e525cb6/source`）；未提交的 `.codex`/`release` 记录文件不打包。未执行 Git push。
- 修复内容：发送前同步上下文压缩（阈值取全局触发值/`ContextMaxInputTokens`/路由模型能力预算的最小值）+ 全文附件聚合 Token 预算（超额可检索降级 RAG、不可检索跳过并记 trace）。详见 commit `03071e52`。

## 发布前基线

- 旧镜像：`deeix-chat:976a90f5ac58`（custom HEAD，无本次修复）。
- Sandbox：`deeix-sandbox-mcp:7c6c0838115f`，未变更。
- 磁盘可用 8.24GB（门槛 3GB）。

## 验证

- 本地冒烟（最小生产配置，sqlite/memory）：healthz OK，`/api/v1/version` 返回精确 commit 与 buildTime。
- 镜像 ID 校验曾失败一次：本地 containerd 存储 `.Id` 返回 manifest list digest（`f0a72002…`），远端 load 后为 amd64 config digest（`2c3381d5…`）。已按远端实际值修正 manifest/脚本后重跑；tar、manifest、脚本 SHA256SUMS 全部通过。
- 远端脚本 `DEPLOYMENT_OK`：备份 `BACKUP_DIR=/opt/backups/deeix-chat-03071e525cb6-20260922T130728Z`（含 PostgreSQL custom-format dump、存储/数据卷、compose 渲染、旧镜像 gzip），备份 SHA256 全部校验通过后切换。
- 无数据库 schema 变更；启动迁移为既有 GORM AutoMigrate。
- 切换后：容器 `deeix-chat:03071e525cb6` running、restarts=0；`/readyz` db/redis ok；公网 `/api/v1/version` commit 精确匹配 `03071e525cb6…`；切换后日志 error/panic 扫描计数 0；sandbox 容器 ID/启动时间/重启计数不变。
- 生产设置核实：`context_compact_enabled=true`、`context_compact_trigger_tokens=65536`、`context_max_input_tokens=32000`、`context_token_budget_enabled=true`、`context_compact_preserve_recent_turns=8`——压缩已由管理员开启，预压缩修复部署即生效。

## 回滚

```sh
ssh cutclass-vps 'bash /opt/deeix-chat/releases/03071e525cb6/remote-deploy.sh rollback'
```

回滚恢复备份点数据库并回到镜像 `deeix-chat:976a90f5ac58`；切换后的新写入不会自动合并。

## 备注

- 未在线复放原 `f61a0b17…` 会话的真实 3.97MB 请求（不在本次冒烟范围）；被压缩的存量会话在下一轮发送时由预压缩路径自动恢复。
- 发布日志：`release/03071e525cb6/deployment.log`（VPS 端）。
