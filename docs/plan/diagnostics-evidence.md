# 故障证据与日志记录

目标：复现 2026-10-14 的启动器退出/调试失联路径，避免把窗口仍存活误报为实例停止；每次故障给出事实、证据和未取得证据的原因。

实现范围：

1. 实际浏览器 PID/创建时间/进程句柄跟踪；调试失联时区分 alive、exited、unknown。记录 launcher exit、handoff、stop intent、退出码。
2. 原始 CDP 事件来源、可空崩溃代码、目标生命周期、网络阶段和取消事件、采集失败原因。
3. 全局 session/event sequence、操作关联、关键事件同步落盘、Flush 队列屏障和日志健康计数。
4. 按实例/运行保留上下文，重要故障自动保存有界脱敏 JSON；异步采集资源、Windows WER、现有转储元数据。默认不导出二进制转储。
5. 浏览器内核错误日志和代理运行/预检输出分离，恢复前保留错误尾部；桥接与故障运行关联。
6. 日志页实例/运行/故障筛选、事实时间线和选定故障导出。

边界：复用当前连接栈和 CDP；采集不导航用户页面，不新增外部 watcher，不改 Windows 全局转储策略。日志只保留域名/阶段/错误码，不记录 Cookie、页面内容、代理 Secret。权限不足、字段缺失和截断显式报告，未知原因不推断为 OOM/GPU。

验收：

- 真实进程存活但 CDP 关闭不会标记 stopped；未知进程状态不标记 exited；PID 创建时间防复用。
- 崩溃无 errorCode 时保持 null，记录来源；网络取消与连接失败分级。
- 日志队列拥塞不丢 ERROR/关键事件；屏障完成后可从磁盘读出；磁盘失败保留内存和健康错误。
- 多实例日志洪峰后可独立读取故障和该运行上下文；迟到事件不能污染新运行。
- 导出脱敏，缺失 OS 证据明确描述；故障与资源采集不阻塞浏览器锁。
- 代理重启之前的 stderr 不被新运行覆盖。

当前状态：实现与验收完成，正式发布和 GitHub 推送未执行。沿用现有日志页样式（Impeccable harden），没有新增常驻程序、全局转储策略或大依赖（lean-mode）。

实现细节：实际进程与调试连接分别判断，托管和脱离启动器两种路径均采样；Windows 保留进程句柄/创建时间并记录退出码、工作集、私有内存、句柄数、可用提交内存和诊断卷空间。CDP 记录 GPU/子进程身份、页面/Worker 目标、网络阶段和采集失败；子进程列表缺项不推断为退出。代理退出/恢复证据在恢复前关联到实例，预检和运行输出独立轮换。故障包只轮换带本程序 producer 标记且文件名与 ID 一致的文件；历史采集中断明确标记 interrupted。每运行环形最多 256 条/512KB，加 32 条重要事件；故障包最多 20 个，时间线 <= 1MB、事实 <= 512KB。

验收证据（2026-10-14，本机）：

- `go test ./backend/... -count=1 -timeout=60s` 全部通过（native opt-in 单独执行）。
- `go vet ./backend ./backend/internal/logger ./backend/internal/diagnostics ./backend/internal/proxy` 通过；前端 `tsc --noEmit` 通过。
- 真实 Windows 子进程：端口关闭不误报停止；实际退出后保留同一创建时间并读到退出码 23；发现权限不足保留 unknown。
- 指定 151 内核 `TestNativeKernelDiagnosticEvidence` 通过：运行版本 Chrome/151.0.7922.137，HTTP403、ERR_CONNECTION_REFUSED、renderer.crashed、GPU/子进程信息和取消观察均验证。只使用临时 headless profile 和本地测试站点。
- 项目 bin/Xray、sing-box 真实预检（成功和错误配置）以及 sing-box 同端口恢复通过；旧输出不覆盖、退出/恢复观察顺序通过。
- 队列满时关键 ERROR 同步落盘、Flush 屏障、磁盘失败/恢复、跨实例洪峰、缺失崩溃代码、网络取消分级、故障 JSON 重启恢复和导出脱敏通过。
- 日志页独立开发 fixture 验证环境/运行/故障选择、选定故障导出调用、窄窗口无横向溢出。UI 未安装到主人正在运行的 Ant 中；安装包必须由 GitHub Actions 生成。

明确未解决项：

- 已存在的 `TestNativeBrowserDiagnosticsAndWorker` 在指定 151 内核临时 headless 环境仍失败：America/New_York 时区和 CPU=8 不匹配，navigator.webdriver=true。原断言保留，没有伪装成通过；这不等于证明日常有头指纹配置失败，须另行按实际指纹配置核验。
- 没有证明 17:40 故障根因；日志系统记录症状/证据缺口，不将 1006、ERR_ABORTED 或空崩溃码推断为 OOM/GPU。
- 脱离启动器后的 stderr 继承无法保证；Windows 无匹配 WER/已有 dump 时显式列出缺口，不生成或上传二进制转储。
- 本机没有可用 GCC，未运行 Go race 检测；已有并发/实际进程路径测试保留。
