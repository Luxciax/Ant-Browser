这是针对 ScriptCat / Tampermonkey `Allow User Scripts` 重启后再次关闭问题的根因修复版本。

### 根因修复

- 停止在正常启动、升级和 runtime 恢复过程中手工维护 Chromium 自己的受保护扩展注册 (`Default/Secure Preferences`)。
- 1.9.1 会把 Chromium 合法写回的扩展状态误判为不匹配，并重写 `location` / `creation_flags` / manifest / protection MAC。真实 Chromium 148 A/B 已复现：这种改写会在 Chromium 正常退出后导致 `user_scripts_enabled` 被删除，下一次启动时 `Allow User Scripts` 重新关闭。
- 1.9.2 改为：Ant Browser 只维护扩展 artifact，继续通过 `--load-extension` 让 Chromium 自己维护 location、creation_flags、manifest、MAC、时间戳和用户脚本授权。
- 删除基于 Chromium 内部 location/manifest 判断“注册是否正常”的稳态 matcher，避免未来 Chromium 改变合法注册形状时再次破坏用户权限。

### 验证

- `go test ./...` 通过。
- `go vet ./...` 通过。
- 真实 Chromium 148 A/B：control 正常退出后保留 `user_scripts_enabled=true`，第二次启动 `chrome.userScripts.getScripts()` 正常；模拟 1.9.1 改写的 Profile 则会丢失授权。
- 全新空 Profile smoke：完全不预写 `Secure Preferences`，ScriptCat service worker 仍可正常加载，Chromium 自行创建扩展注册。
- 安装器继续优先复用 canonical / fork.7 历史 `InstallLocation`，避免覆盖升级时跑到新的默认目录。

### 下载

- `AntBrowser-Setup-1.9.2.exe`：Windows x64 安装版。
- `AntBrowser-1.9.2-windows-amd64-portable.zip`：Windows x64 便携版。
- `SHA256SUMS.txt`：两份产物的 SHA256 校验值。

### 升级说明

从 1.9.1 / fork.7 可直接覆盖安装。若当前 ScriptCat 的 `Allow User Scripts` 已处于关闭状态，升级后手动打开一次，然后正常关闭并重新启动浏览器实例；之后状态应由 Chromium 自己持久保存。
