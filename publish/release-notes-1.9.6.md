# Ant Browser 1.9.6

本版本重点完善跨设备登录态迁移、整包备份保护和恢复失败诊断。

### 登录态与备份

- 全量备份支持流式整包加密，备份目录不生成明文中间 ZIP。
- 定时备份可使用 Windows 本机加密保存迁移密码，密码无法解密时不会降级为明文备份。
- 恢复已打开页面的 sessionStorage，并保持同源不同标签页的独立状态。
- 快照增加页面存储覆盖统计、过旧提示和未覆盖警告。
- 登录态页面恢复未完成时不发布浏览器“已启动”状态，保留重试标记，避免旧会话覆盖新会话。

### 验证

- 使用项目配置的 Chromium 151 指纹浏览器核心完成 Cookie、sessionStorage、整包加密和恢复失败路径测试。
- 错误密码、篡改、截断、重排和追加数据均会被拒绝。

### 下载

- AntBrowser-Setup-1.9.6.exe：Windows x64 安装包。
- AntBrowser-1.9.6-windows-amd64-portable.zip：Windows x64 便携包。
- SHA256SUMS.txt：产物 SHA256 校验值。
