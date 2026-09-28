# 2026-09-24 图片编辑协议门禁误杀修复生产部署验收

结论：部署成功。生产 https://ai.3efs.com/ 已运行包含 `IsImageEditAdapter` 修复的版本，
`e04d0cfb…` 会话上报的 `media.route_protocol_mismatch`（图片编辑任务 503）根因已消除。

## 发布身份与范围

- 权威仓库：`C:\_MY_WORK\X-DEEIX-custom`，`custom`（本次由修复分支
  `custom-fix-image-edit-adapter-gate` fast-forward 合入）。
- Commit：`69b3360ea1a15cc592a4292019ad4a4817450ab4`，版本 `0.3.6`。
- 镜像：`deeix-chat:69b3360ea1a1`，linux/amd64，config digest
  `sha256:edaa2cce3e1081de2846edaf0c475eafc094e66880e28588db3ecde3c6c24945`。
- 从精确提交的 git archive 构建（`release/69b3360ea1a1/source`）；未执行 Git push。
- 修复内容：`IsImageEditAdapter` 补上 `openai_image_generations`（路由层与
  generations 适配器端点切换早已支持，门禁漏加导致编辑任务必报 503）。
  另在 `TestImageAdapterCapabilities` 加回归断言。详见 commit `69b3360e`。

## 发布前基线

- 旧镜像：`deeix-chat:03071e525cb6`（部署前线上版本）。
- Sandbox：`deeix-sandbox-mcp:7c6c0838115f`，未变更。
- 磁盘可用 7.0GB（门槛 3GB）。

## 构建与传输备注（与上次不同）

- 本机直连 Docker Hub 已不通（`auth.docker.io` 超时）；已在本机 Docker daemon.json
  加 `registry-mirrors: ["https://docker.m.daocloud.io"]` 并重启 Docker 后构建成功。
- `SHA256SUMS` 必须 LF 换行：首版用 CRLF 写出导致远端 `sha256sum -c` 读不到文件，
  已修正（`manifest.env` 本就是 LF，无需改）。
- 本地冒烟（sqlite/memory 最小配置）：healthz OK，`/api/v1/version` commit 精确匹配
  `69b3360e…`。冒烟所需 env：`DATABASE_DRIVER=sqlite`、`SQLITE_PATH`、`CACHE_DRIVER=memory`、
  `JWT_SECRET`、`DATA_ENCRYPTION_KEY`、`PUBLIC_API_BASE_URL/PUBLIC_WEB_BASE_URL`（https）。

## 验证

- 远端脚本 `DEPLOYMENT_OK`：备份 `BACKUP_DIR=/opt/backups/deeix-chat-69b3360ea1a1-20260924T085500Z`
  （含 PostgreSQL custom-format dump、旧镜像 gzip、compose 渲染），备份 SHA256 全部校验通过后切换。
- 无数据库 schema 变更；启动迁移为既有 GORM AutoMigrate。
- 切换后：容器 `deeix-chat:69b3360ea1a1` running、restarts=0；公网 `/api/v1/version`
  commit 精确匹配 `69b3360e…`；切换后日志 error/panic 扫描计数 0（脚本内已执行）；
  sandbox 容器镜像/启动时间/重启计数不变。
- 部署后磁盘剩余 6.5GB。

## 回滚

```sh
ssh cutclass-vps 'bash /opt/deeix-chat/releases/69b3360ea1a1/remote-deploy.sh rollback'
```

回滚恢复备份点数据库并回到镜像 `deeix-chat:03071e525cb6`；切换后的新写入不会自动合并。

## 待用户侧确认

- 在原报错会话（`?conversation_id=e04d0cfbc8ce478486c3a7f6fcb8b32d`）中重试一次图片编辑，
  确认不再出现“模型路由协议与图片任务不匹配”。
