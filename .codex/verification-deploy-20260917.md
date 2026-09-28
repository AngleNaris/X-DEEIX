# VPS 部署验证：2026-09-17

- 范围：Agent 知识库/对话分页读取、工具调用与结果持久化回放、稳定工具顺序、DeepSeek 专用缓存用量字段隔离。实现验证见 `verification-agent-context-20260917.md`；该文件的“未提交/部署”描述是开发验收时点，本记录为后续发布结果。
- 仓库：`C:/_MY_WORK/X-DEEIX-custom`，分支 custom，提交 `cedaf023cf6d583b9bf1ae5e779ef1dbef324790`。仅提交本任务 23 个文件，原有前端未提交改动保留并排除。未 Git push。
- 镜像：`deeix-chat:cedaf023cf6d`，linux/amd64，版本 0.3.6；从该提交的 git archive 构建。
- 镜像配置摘要：`sha256:343cf175ad8b68f624bf49a4f462dd4654a5e59b53c308006f35cdf8e4de373a`，由 Docker tar manifest 核对，线上实际镜像 ID 一致。
- 发布包 SHA256：`54a61c3fa9076a1bae2f1d6e4198464254fe10c712cdb22e91d3245ffa6feb59`。发布包、manifest、部署脚本在远端校验通过，脚本 Bash 语法检查通过。

## 发布与回滚

- VPS：`cutclass-vps`；公网 `https://ai.3efs.com/`。
- 切换时间：2026-09-17 06:43:42 UTC（北京时间 14:43:42）。仅重建 app。
- 旧镜像：`deeix-chat:193640a93d8c`。
- 备份：`/opt/backups/deeix-chat-cedaf023cf6d-20260917T064243Z`，242 MiB，包含数据库 dump、可读取的 restore list、数据/存储卷、配置、旧镜像及全部校验文件；校验通过。
- 发布目录：`/opt/deeix-chat/releases/cedaf023cf6d`。
- 部署前与切换前均检查无 pending/running 活动任务；原有 paused_retryable 群组保留。
- 无新增依赖、环境变量或数据库 schema。仅替换原本只含 app image 的 Compose override。
- 复用既有部署脚本，修正日志失败触发 ERR 回滚、回滚失败立即退出，并增加切换前活动任务检查。健康/版本失败会回滚。本轮部署成功，未实际执行回滚。

应用回滚命令（不覆盖数据库和文件）：

```sh
ssh cutclass-vps 'bash /opt/deeix-chat/releases/cedaf023cf6d/remote-deploy.sh rollback'
```

## 验证结果与边界

- 发布前受影响四个 Go 包测试和 `go build ./...` 已通过；本轮干净归档生产镜像构建成功。
- 实际镜像本地 SQLite/memory 冒烟：readyz、精确提交版本、前端 HTML 200、无 fatal/error 启动日志。
- 线上 DB/Redis readyz 正常，公网 healthz 正常，内外版本匹配完整 SHA；实际镜像摘要一致，app running、重启 0。
- PostgreSQL vector 0.8.1 与三个 embedding vector 列预检通过；HMAC 一致性和 Sandbox shared/imports 目录权限通过。
- Sandbox 容器 ID `4751817b7fb79773d20643a43451a24c2ddaee3e42556389229d002c55323ecd`、启动时间 `2026-08-31T03:20:01.603139121Z`、重启 0 均保持不变，健康接口正常。
- 真实浏览器沿用已有登录态，聊天主页、知识库及六份文件列表、已有 DeepSeek 对话正文/分支/用量展示均成功加载；知识库截图布局正常。
- 最终 06:45:38 UTC 复核健康、精确版本、重启 0、部署后 app/Sandbox 错误日志门禁通过。浏览器控制台未另行采集。
- 未发送付费模型请求或修改账号/提供商配置，因此不宣称线上缓存命中率已提高。新工具历史快照从升级后的调用开始积累，旧数据无法补造；首次请求可能重建缓存。
- 未在生产重跑审批、凭据 CRUD、跨用户隔离等无关写入流程；相关本地权限/回放/用量回归证据见实现验证记录。

## 临时资源

- 前台 Buildx：Windows pwsh 9324、docker 42892、docker-buildx 23984，2026-09-17 14:37:42 起，无监听端口；正常完成退出，最终按 PID 复查不存在。异常退出方式为终止该任务进程树。
- 本地冒烟容器 `codex-deeix-deploy-smoke-20260917`，ID `103f5e931dd8`、Linux PID 2361561，06:41:27 UTC 启动，127.0.0.1:3322；try/finally 中 `docker rm -f -v`，复查容器不存在、3322 无监听。
- 远端发布 Bash PID 517948、dump 包装 PID 518854、pg_dump PID 518873 均自然退出，最终按 PID 复查不存在；均为前台任务，无新增服务端口。
- SSH/SCP 前台命令已完成，本次浏览器标签页已关闭。未启动额外 MCP 服务。生产 app 为用户授权持续服务；镜像、源码归档、发布包与备份保留用于追溯/回滚。
