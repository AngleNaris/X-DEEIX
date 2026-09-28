# VPS 部署验证：2026-09-10

- 范围：群组/工具轨迹文字重叠修复；上传和生成图片可选 WebP 存储；存储额度超限错误分类与文案。
- 来源：custom 提交 `0ddfeeaa4516310da68741085a8dcd6f21393e0e`，包含 `b27ffc9a400ebb4c46d290749ae5bd727e41de61`。从 git archive 构建，无 Git push。两个原工作区的其他改动保留。
- 镜像：`deeix-chat:0ddfeeaa4516`，linux/amd64。VPS 配置摘要 `sha256:ad6422bb059a1b7bbd37922c1dc4bf499ce48899a4a4c675495b44cd10acb535`。
- 发布包 SHA256：`b483d356d681739970e4ba399ed3ea986c5337e5ffc24195ec46650e60b2da16`；远端全部 checksum 通过。
- 本地 Docker 使用镜像索引 ID，VPS 使用配置 ID；首次校验在切换前安全退出。核对 tar manifest.json 后修正预期配置摘要，重新验证及部署成功。

## 发布与回滚

切换时间：2026-09-10 03:37:18 UTC（北京时间 11:37:18）。仅重建 app 服务。

备份：`/opt/backups/deeix-chat-0ddfeeaa4516-20260910T033658Z`，含 PostgreSQL dump、restore list、存储和数据卷、配置、旧镜像及校验文件，校验全部通过。首次切换前退出的备份 `...-20260910T033524Z` 同样保留。

回滚应用命令（恢复旧镜像及 override，不自动覆盖数据库或文件）：

```sh
ssh cutclass-vps 'bash /opt/deeix-chat/releases/0ddfeeaa4516/remote-deploy.sh rollback'
```

本次无需新增环境变量或变更启动命令；镜像内新增 cwebp 依赖。图片模式默认 original，线上 UI 确认保留原图；存储配额仍为 100 MB。不会转换历史图片。

## 验证

- 本地：61 项前端测试、TypeScript、相关 lint、8 个 Go 包测试通过；生产镜像构建成功，实际 cwebp 转换测试通过。
- 群组组件：已在真实浏览器复现原问题并验证修复，详见 verification-group-rendering-20260910.md。
- 镜像冒烟：SQLite/memory readyz 正常，版本精确匹配 SHA，cwebp 1.2.4 可执行。
- VPS：readyz 的 DB/Redis 均 ok，公网 healthz 正常，内外版本均精确匹配最终 SHA；app running，重启次数 0。
- Sandbox 容器 ID、启动时间和重启数不变，健康接口及 HMAC 一致性检查通过。
- 部署后错误/fatal/panic 日志扫描无匹配。容器未配置 Docker HEALTHCHECK，健康状态采用实际 HTTP readiness 验证。
- 已登录浏览器：实际聊天内容及后台正常加载；文件与检索的图片存储下拉包含保留原图/WebP 无损/WebP 有损，截图检查布局正常，控制台无 error。未修改线上设置，未发起付费生成或上传真实文件；未重跑线上群组任务。

## 临时资源

- 本轮本地测试容器 deeix-release-tests-20260910、镜像提取容器 deeix-release-inspect-20260910 已移除。
- 冒烟容器 deeix-release-smoke-20260910（Linux PID 645421，127.0.0.1:3322）已停止并连同匿名卷移除；容器列表为空，3322 监听已释放。
- 本轮 CUA 验证标签页已关闭，SSH/SCP 和构建前台命令已退出。生产 app 为用户授权持续服务；保留发布镜像与归档用于回滚。
