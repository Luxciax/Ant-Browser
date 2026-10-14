# Ant Browser 1.9.4

本版本包含代理节点导入扩展和扩展菜单显示修复。

### 代理节点导入

- 支持直接粘贴单条或多条节点链接，不再要求必须使用订阅 URL。
- 支持明文/Base64 订阅、Clash YAML/JSON、单节点/节点数组和 sing-box JSON outbounds。
- 支持 HTTP(S)、SOCKS5、VMess、VLESS、Trojan、SS、Hysteria/HY2、TUIC、AnyTLS。
- URL 拉取、手动粘贴和自动刷新统一使用同一套解析逻辑；不能保留的配置会明确报错，不会静默导入残缺节点。
- sing-box 的 direct、selector、urltest 等路由出站不会被误当作代理节点导入。

### 界面修复

- 修复扩展管理页面三点菜单靠近右侧时内容被窗口裁切。
- 菜单会根据上下空间自动翻转、限高滚动，并在滚动、缩放和主题切换时保持正确位置。
- 支持键盘导航、Esc 返回焦点和点击外部关闭。

### 兼容性修复

- 修复 HTTPS 节点导入后被降级为 HTTP。
- 修复 IPv6 节点地址拼接。
- 修复 TUIC 的 UDP、中继、0-RTT 和 heartbeat 参数转换。

### 验证说明

- 已通过相关 Go 导入/连接栈测试和 TypeScript 类型检查。
- GitHub Actions 将在 Windows 发布分支上执行完整测试、vet、安装包/便携包构建和产物校验。
- 本版本不更换用户配置的指纹浏览器内核，不代表已完成专用 Chromium 151 的完整指纹认证。
