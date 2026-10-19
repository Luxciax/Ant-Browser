# Clearcote 适配与验收

基于 `Luxciax/Ant-Browser` 的 `c3383863de7cc850015eda679d51aca91820919d`，实施日期：2026-10-19。

## 使用

1. 在内核管理中添加 Clearcote 的目录或 `chrome.exe`，指纹后端可以自动检测，也可明确选择 Clearcote。
2. 下载内核时，可选择 fingerprint-chromium Release 或 Clearcote Release；下载完成后记录官方来源和指纹后端。
3. 在实例编辑中选择该内核。两套界面均按所选内核显示配置。
4. Clearcote 默认自动 Persona。Seed 留空时，由后端根据实例 ID 生成稳定种子。高级手动模式才输出 CPU、RAM、屏幕、可用屏幕、色深、DPR、Touch 和 GPU 覆盖值。
5. 导入真实设备 Profile 时，选择 Clearcote 官方采集工具导出的 JSON。AntBrowser 将它复制到 `data/fingerprint-profiles`，配置只保存文件引用。启动时按官方 SDK 格式进行 gzip（level 9）和 base64 编码。
6. 高级手动模式也可选用 Profile；显式原生覆盖参数优先于导入 Profile，缺少的项目由 Profile 或 Persona 提供。
7. 切回 fingerprint-chromium 只改变内核及指纹参数，保留 UserDataDir、Cookie、存储、插件、代理和 Clearcote 专属设置。

## 实现范围

- 内核模型、JSON/YAML、SQLite、实例包和备份合并保存 `fingerprintBackend`。SQLite 追加迁移 19，旧实例无需迁移；已有无元数据的内核维持旧指纹契约，新导入的未知内核回退普通 Chromium。
- 前端独立的 fingerprint-chromium / Clearcote / generic Adapter；原 `fingerprintSerializer.ts` 保留兼容入口。
- Go 启动矩阵根据实际解析的 CoreId 选择 Adapter，沿用原有进程、CDP、代理桥、插件和窗口启动框架。
- 自动 Persona / Profile / 高级手动模式、原生字段映射及跨内核配置保留。
- Profile JSON 验证、受管副本、启动时编码；诊断启动参数省略 Profile payload。JSON 限制为 5 MiB，编码 payload 限制为 24,000 字符，以预留 Windows 命令行空间。
- 三层识别：官方 Release URL、`ant-core.json`、隔离的 CDP native override probe。探测结果只标记 `clearcote-compatible`，不声称官方来源。用户明确选择优先于自动识别。
- 检测页和两套自测报告从当前 Adapter 获取 RAM、色深、Touch、屏幕、可用屏幕、DPR、GPU 等期望值。自动 Persona 的派生值未被伪造成显式期望。
- 新增 GitHub Actions Windows 检查流程，保留现有发布流程。

应用专属配置以 `--ant-clearcote-config=<JSON>` 元数据记录保存在原有 `fingerprintArgs` 中，用于保留模式、手动值和 Profile 文件引用。该记录不含编码后的 Profile payload，也不会传给浏览器。Clearcote 启动 Adapter 会恢复其他内核编辑器无法表达的字段。

## 官方依据及实测修正

- [Clearcote SDK 参数与编码](https://github.com/clearcotelabs/clearcote-browser/blob/main/sdk/node/src/fingerprint.ts)
- [Clearcote 原生 switch 定义](https://github.com/clearcotelabs/clearcote-browser/blob/main/patches/000-fingerprint-switches.patch)
- [设备 Profile 采集与字段格式](https://github.com/clearcotelabs/clearcote-browser/tree/main/tools/fingerprint-collect)

真实 Profile 的设备字段使用官方采集格式，例如顶层 `hardware_concurrency`、`device_memory` 和 `screen.width`。它与 AntBrowser UI 的 camelCase 配置字段是不同格式；AntBrowser 编码原始采集 JSON，不重命名 Profile 字段。

文档建议的 `about:blank` probe 在本次实测中返回 `SecureContext:false, DeviceMemory:0, MaxTouchPoints:7`。RAM 接口属于安全上下文，不能据此断定内核不支持。实现改用临时本地 HTTP 页面并等待安全上下文，此后探测通过。

## 验证

| 项目 | 结果 |
| --- | --- |
| 前端旧 serializer 输出、顺序、反序列化回环 | 通过 |
| Clearcote 参数映射、自动/手动隔离、零 Touch、配置保留 | 通过 |
| Clearcote 官方 Release 推荐选择 | 通过 |
| 内核 SQLite 持久化、旧数据默认、显式覆盖、来源识别、probe 失败清理 | 通过 |
| Profile JSON → gzip → base64 回读及受管副本 | 通过 |
| `go test ./backend/...` | 通过 |
| `npm run build`（TypeScript + Vite） | 通过 |
| `go build ./backend` | 通过 |
| 两套真实编辑组件的浏览器交互 | 通过；使用隔离组件验收页和本地状态 |
| Windows 双内核原生验收 | 通过；见下表 |

真实内核版本与 SHA-256：

| 内核 | 版本 | 已核对的官方 SHA-256 |
| --- | --- | --- |
| Clearcote | 150.0.7871.114 / v0.1.0-pre.23 | `93fc03c45b931d8d82f714814318892929f44dd671b0993788332071d53f3135` |
| fingerprint-chromium | 150.0.7871.186 | `4d549c326e51ebbabf562fd365eb5380d9d4a81200da2c60f075c688d9a77e03` |

原生测试通过真实 Chromium 进程、既有启动参数构建器和既有 CDP 工具验证：

- Clearcote capability probe 成功，临时目录回收。
- CPU=8、RAM=4、屏幕=1366×768、可用屏幕=1366×728、DPR=1.25、Touch=7、Asia/Tokyo、GPU Vendor/Renderer 覆盖均返回预期值。
- 本地 HTTP 代理请求及未打包扩展的内容脚本实际生效。
- Clearcote 与旧内核同时运行，两个用户目录的 localStorage 隔离。
- 同一目录重启以及 Clearcote → fingerprint-chromium → Clearcote 后，Cookie/存储保留，稳定种子未改变。
- 真实 Profile 使 CPU=2、屏幕宽度=1920 生效；显式 CPU=8 参数覆盖 Profile。

复现命令（仓库根目录运行 Go；前端命令在 `frontend` 中运行）：

```powershell
go test ./backend/...
go build ./backend
node scripts/test-fingerprint-adapters.mjs
npm run build

$env:ANT_TEST_CLEARCOTE = '<Clearcote chrome.exe 绝对路径>'
$env:ANT_TEST_FINGERPRINT_CHROMIUM = '<fingerprint-chromium chrome.exe 绝对路径>'
go test ./backend -run '^TestNativeFingerprintBackendAcceptance$' -count=1 -v
```

本地详细日志位于 `.tmp/runtime-cache/backend-tests.log`、`targeted-tests.log`、`native-acceptance.log`，前端编译日志位于 `frontend/phase-build.log`。浏览器验收截图位于 `.tmp/runtime-cache/clearcote-ui-acceptance.png`。这些本地产物和测试内核均不提交。

## 尚未验证或发布

- 原生验收运行于 Windows headless Chromium；未构建或点击完整 Wails 桌面安装包及原生文件选择器。完整 App 直接在普通网页运行时，会因既有 Wails `Environment` 依赖出现空白；本次界面验收使用实际编辑组件的隔离页面，未扩修此既有问题。
- 未在 Linux/macOS 执行原生内核测试。
- 使用本地 HTTP 代理验证共用 Chromium 启动路径；未新增 Xray/Mihomo 配置，也未连接外部认证代理服务。
- 本地构建仅用于本次必要编译及系统集成验收。尚未触发 GitHub Actions、发布安装包、创建 PR 或 push；正式分发产物应由 CI 生成。
