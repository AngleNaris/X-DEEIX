# 群组文字重叠修复

工作区：`C:/_MY_WORK/X-DEEIX-custom`，custom 分支，基线 c202574f。当前主工作区 DEEIX-Chat 的 dev 分支没有群组模块；本次未修改 dev，也未部署或推送。

## 根因与修改

之前 7b3c8258 修复的是事件重放幂等性。本次通过真实 MessageAgentGroupTrace 组件和 45 段中英混排思考文本复现截图中的绘制重叠：运行中 AccordionItem 使用 trace-sweep，祖先 background-clip:text 与透明文字填充导致嵌套滚动区外的字形绘制到正文区域。DOM 高度本身正常，所以事件去重或只限制思考区高度不能解决。

将群组扫光移至成员标题 MarkerContent；工具调用行同样将扫光从整行移至工具标题。保留长思考滚动和展开/折叠行为。增加现有 Node 测试中的结构回归保护，禁止可包含滚动详情的 AccordionItem/li 使用文字遮罩。

## 验证

- 修改前真实浏览器复现同类覆盖：`output/group-layout-before.png`。
- 修改后同一组件与数据：`output/group-layout-after.png`；正确切换应用深色主题后的截图：`output/group-layout-dark-after.png`。人工检查正文无思考字形覆盖。
- 375×812、768×1024、1024×768、1440×1000、1920×1080 下正文起点位于思考滚动区之后，思考文字没有透明填充或扫光祖先，页面没有横向溢出。
- 验证追加 20 段内容、运行/完成切换、思考区收起/展开及 200% CSS zoom；没有调用模型或写入真实会话。
- 12 项 Node 回归测试通过（conversation-streaming-state、group-run-event-policy、group-run-recovery）；TypeScript 全量检查、修改文件 Biome lint、git diff --check 通过。
- 临时组件页面的全局 branding/version 请求遇到本地 8080 CORS 限制，使用默认品牌；不影响群组组件渲染。未验证 VPS 或完整线上会话。

## 清理

临时页面和浏览器检查脚本已移除。Next 根 PID 59464、子进程 14452/45500/40328，Playwright daemon 32828、Chrome 62928 及其记录的子进程已停止；3321 监听已释放。没有保留持续进程；原有工作区文件和服务不变。
