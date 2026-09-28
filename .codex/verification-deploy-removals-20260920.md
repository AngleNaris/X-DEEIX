# 功能移除版本部署验收

- 2026-09-20 16:26:29（北京时间）部署到 https://ai.3efs.com/。
- 本地提交：d43ee0f9d7257f310ccf5ae3d5788914235eaa66，custom，版本 0.3.6。未执行 Git push。
- 从精确提交 git archive 构建 Linux amd64 镜像 deeix-chat:d43ee0f9d725；运行镜像 digest：sha256:217f8a0f2d4c44a22433b112f56227bc78bf43cd220a71e509e252d223e0cb17。没有使用此前 localhost API 预览产物。
- 原有 9 个已跟踪 WIP 文件及 3 个缩略图文件保持未提交，12 个文件 SHA256 与提交前一致。

## 验证

- 精确提交前端构建成功、64/64 测试通过。首次测试因归档没有 react 依赖失败，临时连接已安装依赖后通过，连接已删除。
- Linux Go 1.26.5：channel/conversation/settings 应用包及 channel/conversation HTTP 包测试通过，go build ./... 通过。首次 sh -lc 重置 PATH 导致找不到 Go，改用 sh -c 后通过。
- 精确镜像隔离运行：readyz/version 通过，9 项实际 API 检查通过，含移除接口 404 和原有模型、技能、提示词、卡片、知识库接口 200。
- 发布包远端 SHA256 校验通过。无数据库 schema 变更；仍在停止旧应用后备份数据库，再启动候选版本。
- PostgreSQL custom-format dump 非空、pg_restore --list 通过；数据库、存储卷、数据卷、配置和旧镜像备份 SHA256 全部校验通过。
- 公网与内部版本均精确匹配提交，运行镜像 digest 匹配，应用 running、RestartCount=0，数据库和 Redis ready。
- Sandbox MCP 容器 ID、启动时间、重启次数和镜像保持不变，健康检查及应用/Sandbox HMAC 一致性通过。
- 用户 1、消息 21414、平台路由 16、文件 92，与此前生产记录一致。切换后日志扫描未发现 error/fatal/panic。
- 公网旧用户渠道、用户模型和用户渠道预设接口 404。登录态浏览器实际检查聊天页无重复控件、模型分组菜单正常、设置导航无用户渠道；原有资源导航保留，浏览器 error 日志为空。
- 未调用真实付费模型生成，未测试供应商生成质量。

## 备份与回滚

备份目录：/opt/backups/deeix-chat-d43ee0f9d725-20260920T082556Z。

发布包：/opt/deeix-chat/releases/d43ee0f9d725/。

回滚命令：

```sh
ssh cutclass-vps 'bash /opt/deeix-chat/releases/d43ee0f9d725/remote-deploy.sh rollback'
```

该保守回滚脚本会停止应用、恢复备份数据库并保留失败数据库，恢复上一镜像 deeix-chat:076397e423af。会回到备份时点，后续写入不自动合并。本次未触发回滚；脚本语法检查通过，数据库恢复流程沿用此前已演练版本。

## 临时资源

- Go 检查容器 codex-removal-release-tests（Linux PID 895326）结束并自动移除；镜像冒烟容器 codex-removal-release-smoke（Linux PID 908591）已删除，任务容器查询为空，18942 无监听。
- 构建会话 91169、Go 检查 86440、测试及导出 95714、上传 93229、部署 65907 均结束；浏览器验收页已关闭。
- 保留生产应用、不可变发布包及备份，无保留的临时测试服务。
- 日志与包位于 release/d43ee0f9d725/。
