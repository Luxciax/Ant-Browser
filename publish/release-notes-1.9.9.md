# Ant Browser 1.9.9

本版本是 1.9.8 之后的功能整合版本，重点完成环境模型、运行时证据、自动化工作流、控制面与发布工具链升级。

### 环境与运行一致性

- 统一 Persona / EnvironmentView v2 / LaunchPlan / Kernel Capability Manifest / NetworkPolicy。
- 环境模板增加目标 OS、设备类别、内核族、支持级别与 required capabilities。
- 模板兼容性区分 eligible 与 verified；缺少真实兼容内核时明确显示 requires-compatible-kernel。
- 生命周期明确区分普通重启、克隆新身份、保留身份迁移、仅迁移站点数据与显式刷新身份。

### Workflow / RPA

- 支持 action、set、if、loop、sleep。
- 支持变量插值、条件、最多 100 次循环、超时、最多 5 次重试与副作用重试保护。
- 支持 breakpoint、paused/interrupted/failed 恢复、运行历史与 everyMinutes / dailyTime 调度。
- 支持 iframe、open shadow root、文件上传、页面抽取与现有 PageSession 复用。

### 控制面与安全

- HTTP / MCP / Desktop 接入统一控制面。
- API Key 支持能力范围、轮换与撤销。
- 回收站支持恢复、永久删除和过期清理。
- Sync 明确为现有备份通道状态，不伪装成实时双向同步。
- Windows 可选 antbrowser:// 协议注册；默认不修改系统代理、TUN、防火墙或 DNS。
- OpenList / S3 凭据改用 SecretRef；backup.local.yaml 不再保存明文秘密值。

### 网络证据

- Host-source 网络观测支持 DNS、TCP、TLS、ALPN、HTTP protocol/status/timing。
- 私网、回环、link-local、multicast 与重定向绕过均受阻止。
- 未取得的 JA3、JA4、QUIC、HTTP/3、browser-route、server-seen 证据保持 not_observed。

### UI 与工具链

- Fish UI 新增环境契约、NetworkPolicy、Workflow/RPA、控制中心、网络证据和回收站入口。
- Wails 升级到 v2.17.1，Go 基线升级到 1.26。
- React Router DOM 升级到 7.18.6，Vite 升级到 8.3.8。
- 前端完整 npm audit 为 0；production build、Go 测试、go vet、全模块编译与完整 Windows Wails build 已通过。
- 新增手动 Quality Gate。

### 下载

- AntBrowser-Setup-1.9.9.exe：Windows x64 安装包。
- AntBrowser-1.9.9-windows-amd64-portable.zip：Windows x64 便携包。
- SHA256SUMS.txt：产物 SHA256 校验值。
