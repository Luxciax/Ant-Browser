# Ant Browser

> 面向本地多环境隔离、代理绑定、浏览器内核管理和自动化的 Wails 桌面应用。

[![Release](https://img.shields.io/github/v/release/Luxciax/Ant-Browser?sort=semver)](https://github.com/Luxciax/Ant-Browser/releases)
[![Platform](https://img.shields.io/badge/Platform-Windows%20%7C%20Linux%20%7C%20macOS-blue)](https://github.com/Luxciax/Ant-Browser)
[![Issues](https://img.shields.io/github/issues/Luxciax/Ant-Browser)](https://github.com/Luxciax/Ant-Browser/issues)

当前源码版本：`1.9.9`

完整版本历史见 [CHANGELOG.md](CHANGELOG.md)。

## 当前能力

### 环境与生命周期

- 创建、编辑、启动、停止、重启、复制、迁移和回收浏览器环境。
- Persona / EnvironmentView / LaunchPlan / Kernel Capability Manifest 统一描述配置、实际应用状态和运行证据。
- 环境身份 seed 与数据库 Profile ID 分离；迁移和恢复保持显式身份语义。
- 兼容模板区分配置资格与实际验证状态；缺少兼容内核时不会标记为 verified。

### 代理与网络

- Xray + sing-box 组合栈和独立 Mihomo 栈。
- 支持代理导入、分组、测速、健康检测、预热、链式代理及运行时恢复。
- NetworkPolicy 管理 DNS、WebRTC、代理失败策略和网络关联行为。
- Network Observation 可记录主机侧 DNS/TCP/TLS/ALPN/HTTP 观测，并阻止私网/回环等 SSRF 目标。
- 未实际观测到的 JA3/JA4、QUIC/HTTP3、browser-route、server-seen 等证据保持 `not_observed`。

### 内核、扩展与数据

- 多浏览器内核管理和运行前能力校验。
- 扩展安装、启停、默认策略、单环境配置及数据迁移。
- 本地、OpenList、S3 备份与恢复；实例包支持冲突预览和迁移语义。
- OpenList/S3 凭据通过 SecretRef 保存；`backup.local.yaml` 不保存秘密明文。

### 自动化与控制面

- 自动化脚本库、运行记录和外部调用。
- Workflow/RPA 支持变量、条件、循环、断点、重试、副作用保护、调度和中断恢复。
- LaunchServer 提供本机 HTTP、MCP、统一 CDP 控制面和 capability scopes。
- 支持批量任务、回收站、备份通道同步状态、诊断信息和可选 Windows `antbrowser://` 协议。

当前 HTTP 路由以 `backend/internal/launchcode/server_http.go` 为准；API 权限以 `backend/internal/launchcode/auth.go` 为准。

## 浏览器内核

应用不会把“配置了某个字段”等同于“内核已验证支持该能力”。

推荐内核来源：

- [adryfish/fingerprint-chromium](https://github.com/adryfish/fingerprint-chromium)

也可以在应用的内核管理中配置其他兼容内核。跨 OS/移动模板需要对应真实兼容运行时才能进入 verified 状态。

## 快速开始

### Windows

从 Releases 下载安装包或便携包。便携包解压后运行 `ant-chrome.exe`。

源码开发：

```powershell
bat\dev.bat
bat\dev.bat live
```

Windows 发布：

```powershell
bat\publish.bat both
```

产物写入 `publish\output\`。

### Linux

```bash
./scripts/dev.sh
bash publish/linux/publish-linux.sh --arch amd64
bash publish/linux/publish-linux.sh --arch arm64
```

### macOS

macOS 当前为 unsigned 构建，必须在对应架构的原生 macOS runner/主机上打包：

```bash
bash publish/mac/publish-mac.sh --arch amd64
bash publish/mac/publish-mac.sh --arch arm64
```

## 开发基线

当前工具链版本由源码配置决定：

- Go：`go.mod`
- Wails：`go.mod` 中的 `github.com/wailsapp/wails/v2`
- 前端依赖：`frontend/package.json` / `frontend/package-lock.json`
- 产品版本：`wails.json`，并由一致性检查要求与前端版本同步

先运行仓库一致性检查：

```bash
node tools/check-repo-consistency.mjs
```

核心质量门禁：

```bash
go test -p=1 ./backend/... ./internal/... ./tools/...
go vet ./backend/... ./internal/... ./tools/...
go test -run '^$' -p=1 ./...
npm --prefix frontend ci
npm --prefix frontend audit --audit-level=high
npm --prefix frontend run build
```

手动 GitHub Actions 质量门禁见 `.github/workflows/quality.yml`。

## 数据与敏感信息

不要提交：

- `data/app.db`
- 用户 Profile / Cookie / 浏览器存储
- `backup.local.yaml`、`secrets.v1.json`
- API Key、代理凭据
- 本机构建缓存和 `publish/output`

发布 staging 会执行敏感配置检查。

## 目录

- `backend/`：应用服务、LaunchServer、备份、环境、代理和自动化逻辑
- `backend/internal/`：数据库、浏览器、代理、MCP、自动化等内部模块
- `frontend/src/ui-v2/fish/`：当前用户界面
- `frontend/src/modules/`：前端领域 API、类型、hooks、helpers、复用组件
- `publish/`：跨平台发布脚本与运行时清单
- `tools/`：诊断、运行时同步、公开发布与仓库检查工具
- `skills/`：Ant Browser / OpenClaw 集成 Skill
- `docs/audits/`：历史审计和验证材料

## 文档

- 当前架构：[docs/architecture.md](docs/architecture.md)
- 安全与运行证据：[docs/security-and-evidence.md](docs/security-and-evidence.md)
- 证据来源规范：[docs/evidence-provenance-v1.md](docs/evidence-provenance-v1.md)
- Windows 开发/发布：[bat/README.md](bat/README.md)
- Linux 发布：[publish/linux/README.md](publish/linux/README.md)
- macOS 发布：[publish/mac/README.md](publish/mac/README.md)
- 发布说明：[publish/release-notes-1.9.9.md](publish/release-notes-1.9.9.md)
- 版本历史：[CHANGELOG.md](CHANGELOG.md)

## License

当前仓库未包含独立 `LICENSE` 文件。
