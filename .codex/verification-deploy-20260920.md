# 2026-09-20 生产部署验收

结论：部署成功。北京时间 15:13:43 切换，生产 https://ai.3efs.com/ 已运行本次修复版本。

## 发布身份与范围

- 权威仓库：`C:\_MY_WORK\X-DEEIX-custom`，`custom`。
- Commit：`076397e423afda9abbcc0ab0d54b64230ab45a59`，版本 `0.3.6`。
- 镜像：`deeix-chat:076397e423af`，linux/amd64。
- 生产 config digest：`sha256:5519c2f42bf89449b193a0b185bbc48549dc5ea47a6775a40a65f25766aac135`。
- 从精确提交的 git archive 构建；保留原有附件/缩略图未提交修改，不打包这些 WIP。未执行 Git push。
- 修复个人模型图片恢复、视频回查和连续 HTML 制品预览；功能验证详见 `verification-lhxcxyw-functional-20260920.md`。

## 先备份后迁移

- 在线预演备份：`/opt/backups/deeix-chat-076397e423af-preflight-20260920T070750Z`。
- 最终停机备份：`/opt/backups/deeix-chat-076397e423af-20260920T071301Z`。
- 旧应用停止后生成最终 PostgreSQL custom-format dump、存储卷、数据卷、配置和旧镜像备份；所有 SHA256 校验通过后才启动新应用并执行迁移。
- 生产 dump 已恢复到隔离临时数据库，用精确候选源码编译的 migration-only helper 完成真实 PostgreSQL 迁移演练。没有启动任务 worker 或访问真实模型。
- 使用 Go/GORM 启动迁移，未执行历史 MySQL 方言 SQL 文件。
- 演练前后及生产迁移后：用户 1、消息 21414、路由 16、文件 92，数量一致。
- 新表 `llm_user_models`、`project_workspaces`、`project_files`、`project_imports` 及个人模型唯一索引均已验证。

## 上线检查

- 发布脚本 `DEPLOYMENT_OK`，后置检查 `POSTCHECK_OK`。
- 内部和公网 version 精确匹配提交；实际镜像 digest 匹配；重启次数 0。
- `/readyz`：database 和 Redis 均 ok；healthz 正常。
- 切换后日志扫描未发现 error/fatal/panic 等异常。
- Sandbox 容器 ID、启动时间、重启计数、镜像及 HMAC 一致性检查通过，healthz 正常；未重启 Sandbox。
- 真实域名登录态浏览器检查：聊天首页、画布工具栏/空态、项目已有对话读取与输入框、已有文件列表成功渲染；浏览器 error 日志为空。
- 画布截图可见窄窗口顶部标题换行；未将此视为运行阻断，也未声称全部响应式视觉验收完成。
- 前置验证包括前端 68 项测试、类型检查、生产构建、相关 Go 测试、隔离运行时 6 项恢复/权限场景及 HTML Preview 实际渲染。
- 未在线发起付费模型生成，真实供应商生成质量及端到端响应不在本次上线冒烟结论内。

## 回滚

发布包与脚本：`/opt/deeix-chat/releases/076397e423af/`。

```sh
ssh cutclass-vps 'bash /opt/deeix-chat/releases/076397e423af/remote-deploy.sh rollback'
```

回滚会停止候选应用，从切换备份恢复数据库，保留失败数据库并恢复旧镜像 `deeix-chat:cedaf023cf6d`。这会回到备份时点，切换之后的新写入不会自动合并。脚本语法和完整数据库恢复/迁移演练已验证；本次发布未实际触发生产回滚。

## 临时资源

- 构建与 SSH/SCP 会话已结束；记录的 Windows 构建 PID 29528、27836、26672 均不存在。
- 本地 smoke 容器 `codex-deploy-smoke076` 已移除，18942 无监听。
- VPS 演练容器、临时数据库 `deeix_rehearsal_076397e423af` 和凭据 env 均已清理并独立复核。
- 本轮浏览器测试标签已关闭。保留生产应用、发布包及备份以供服务和恢复使用。
- 发布日志：`release/076397e423af/deployment.log`；演练日志：`release/076397e423af/rehearsal-result.log`。
