这是 1.9.0 的针对性热修复版本，修复 ScriptCat / Tampermonkey `Allow User Scripts` 在 Ant Browser 每次启动后重新关闭的问题。

### 修复

- 修复启动准备阶段反复重写 Chromium `Secure Preferences`：当同一扩展的 runtime ID、版本和持久化路径均匹配时，Chromium 写回的 `location=8` command-line 注册现在视为稳定状态，不再被强制改回 `location=1`。
- 兼容 Chromium 148 实际 Profile 结构：`location=8` 条目即使不包含内嵌 manifest，只要磁盘扩展 artifact 的路径和版本匹配，也不会被误判为损坏。
- 修正 `Allow User Scripts` 迁移存储位置。`user_scripts_enabled` 属于受保护的 extension-specific preference；Windows 上迁移现在写入 `Default/Secure Preferences`，同时兼容 fork.7 / 1.9.0 曾写入普通 `Default/Preferences` 的旧格式。
- 保留 `--load-extension` 启动方式，避免为了修复权限持久化而导致扩展本身无法加载。
- 新增真实重启形状回归：`location=8`、`creation_flags=38`、无内嵌 manifest、已存在用户脚本开关与保护 MAC 的条目必须保持不变。

### 下载

- `AntBrowser-Setup-1.9.1.exe`：Windows x64 安装版。
- `AntBrowser-1.9.1-windows-amd64-portable.zip`：Windows x64 便携版。
- `SHA256SUMS.txt`：上述两个文件的 SHA256 校验值。

### 升级说明

- 可直接从 1.9.0 / fork.7 覆盖升级；安装器继续沿用已有安装目录兼容逻辑。
- 升级后，若当前 ScriptCat 的 `Allow User Scripts` 已经处于关闭状态，需要手动打开一次；之后正常退出并重新启动实例，该状态应保持，不需要每次重新勾选。
- 本 hotfix 不修改 ScriptCat 已安装脚本和扩展存储数据。
