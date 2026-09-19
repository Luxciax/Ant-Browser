本版本集中修复插件状态、代理恢复和 Profile 数据一致性问题。

### 修复

- 补齐插件首次迁移和 runtime 索引恢复时的用户脚本权限迁移，保护安装失败回滚时新旧 ID 的已有数据。
- 修复 sing-box 异常退出后无法自动恢复，保留原 SOCKS 端口并正确处理引用释放。
- 修复删除运行中插件被拒绝后仍重写 Profile；Preferences 使用原子替换。
- 防止共享或父子 Profile 目录被其他实例的回收站清理误删。
- 修复经典/Fish UI 插件配置弹窗的过期请求覆盖和加载失败后误保存。
- 为 Profile 导入和物理删除增加 SQLite 持久化事务日志，重启时恢复未提交操作或完成已提交操作的清理。

### 下载

- `AntBrowser-Setup-1.9.0.exe`：Windows x64 安装版。
- `AntBrowser-1.9.0-windows-amd64-portable.zip`：Windows x64 便携版。
- `SHA256SUMS.txt`：上述两个文件的 SHA256 校验值。

产物由 GitHub Actions 从本次发布提交构建，包含 Xray 和 sing-box；不附带 Chromium 内核，首次使用可在应用中下载。

### 升级与验证说明

- 升级前请备份重要 Profile。首次启动会追加数据库 migration 18；恢复日志未处理完成时请勿降级到旧版本。
- 自动化验收包含 Go 测试、go vet、TypeScript/production build、14 个进程强杀场景和 Windows 文件锁恢复测试。
- 真实 ScriptCat/Tampermonkey 全链路兼容、安装覆盖升级 GUI 验收及长期运行测试尚未完成。
- 旧版本留下但没有事务日志的暂存目录不会自动猜测恢复；全局备份逐文件合并不属于本次崩溃恢复协议。
