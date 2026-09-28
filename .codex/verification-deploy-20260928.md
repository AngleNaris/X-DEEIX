# 2026-09-28 custom 分支 VPS 部署验收

## 结论

部署成功。生产 `https://ai.3efs.com/` 已运行 custom 候选 `6a49c38cc0ae6f8d2c484d60a0cd7e966918c59d`，版本 `0.4.4-beta.1`。

## 发布身份

- 仓库：`C:\_MY_WORK\X-DEEIX-custom`
- 分支：`custom`
- Commit：`6a49c38cc0ae6f8d2c484d60a0cd7e966918c59d`
- 镜像：`deeix-chat:6a49c38cc0ae`
- 镜像 Config ID：`sha256:0524e6599b3483079ed2020593a3549156650dffe820e9294eea72b23eb0c56b`
- 平台：`linux/amd64`
- 构建时间：`20260928T094209Z`
- Sandbox MCP：复用 `deeix-sandbox-mcp:7c6c0838115f` 与 `deeix-sandbox-base:7c6c0838115f`，未重启

## 制品与传输

- 发布目录：`/opt/deeix-chat/releases/6a49c38cc0ae/`
- `deeix-chat-6a49c38cc0ae-linux-amd64.tar`：VPS `sha256sum -c` 通过
- `manifest.env`：VPS `sha256sum -c` 通过
- `remote-deploy.sh`：VPS `sha256sum -c` 通过
- 本地冒烟：healthz、readyz、`/api/v1/version` 均通过；版本 API 精确返回完整 SHA

## 备份与切换

- 切换时间：`2026-09-28T11:03:51Z`
- 备份目录：`/opt/backups/deeix-chat-6a49c38cc0ae-20260928T110254Z`
- 旧应用镜像：`deeix-chat:ff04da5a03c8`
- 数据库：PostgreSQL custom-format dump、restore list、数据库预检和备份 SHA256 均通过
- 文件卷：脚本记录 `deeix-chat-app-storage` 卷元数据；数据库 dump 不包含应用文件卷
- 回滚入口：

```bash
ssh cutclass-vps 'bash /opt/deeix-chat/releases/6a49c38cc0ae/remote-deploy.sh rollback'
```

## 部署后门禁

- 应用容器：`running`，`RestartCount=0`
- 应用本地 `healthz`：通过
- 应用本地 `readyz`：DB/Redis 均 `ok`
- 公网 `/api/v1/version`：精确返回 SHA 与版本
- Sandbox `/healthz`：通过，容器启动时间和 `RestartCount=0` 未变化
- Compose 生效镜像：`deeix-chat:6a49c38cc0ae`
- 应用切换后 fatal/panic/segmentation/unhandled/error 扫描：0 命中
- Sandbox 切换后同类日志扫描：0 命中
- VPS `/opt` 可用空间：约 `6.3G`

## 范围与遗留

- 支持边界仍为单应用实例、多用户；未扩大到多实例协调。
- Qwen、Gemini、豆包、OCR 等真实 Provider 未在本次部署中启用或执行 canary。
- 登录后真实域名业务旅程仍需按计划由产品侧验证：登录、普通聊天、自定义 CRUD、Agent Group、制品分享与刷新持久化。
