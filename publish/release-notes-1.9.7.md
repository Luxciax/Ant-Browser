# Ant Browser 1.9.7

本版本将 Clearcote 内核接入现有指纹浏览器配置链路，并保留旧版 fingerprint-chromium 行为。

### Clearcote 内核适配

- 浏览器 Core 现在记录独立的指纹后端契约，可在 generic-chromium、fingerprint-chromium 和 Clearcote 之间选择。
- Clearcote 支持稳定 Persona 自动模式、手动硬件/GPU 覆盖和受管设备画像导入。
- 自动 Persona 模式不会把手动硬件、GPU 和屏幕覆盖带入启动参数；切换后仍可恢复之前保存的配置。
- 受管画像在启动时通过临时压缩载荷传入，不把画像明文写入持久化配置。

### 内核识别与检测

- 自动识别官方 Clearcote 与 fingerprint-chromium 内核来源，并用隔离的能力探针确认真实接口。
- 指纹检测页按后端显示对应的 CPU、内存、屏幕、DPR、触摸、GPU 和时区结果。
- 浏览器启动、重启、并发环境和跨后端切换均保持独立用户数据与后端设置。

### 兼容性与验证

- 未选择指纹后端的旧 Core 继续使用 generic-chromium；现有 fingerprint-chromium 序列化格式保持兼容。
- 已通过 Go 后端测试、前端构建、适配器回归、Go build 和 Windows 原生 Clearcote/fingerprint-chromium 内核集成验证。
- 正式 Windows 与 Linux 产物由 GitHub Actions 以本版本提交和标签构建。

### 下载

- Windows x64 安装包：`AntBrowser-Setup-1.9.7.exe`
- Windows x64 便携包：`AntBrowser-1.9.7-windows-amd64-portable.zip`
- Linux amd64/arm64 deb 与 tar.gz 包由 Linux 发布工作流上传。
- `SHA256SUMS.txt`：Windows 产物 SHA256 校验值。
