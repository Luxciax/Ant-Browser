# 商业对标：登录态连续性

基线 f7fda3a；仅本地开发与隔离测试，不启用真实云上传、不操作真实账号、不推送发布。

## 官方依据与结论

| 产品 | 官方明确能力 | 本项目借鉴 |
| --- | --- | --- |
| [AdsPower](https://help.adspower.net/docs/bLpgjY01) | 正常关闭后按类别同步 Cookie、密码、书签、Local Storage/IndexedDB、扩展和历史 | 登录态不止 Cookie；完整目录保持 |
| [GoLogin](https://support.gologin.com/en/articles/14404855-profile-synchronization) | 指纹、Cookie、网站存储及扩展随环境同步，关闭后上传，等待 Ready；不建议多设备并发 | 同步完成与进程停止分开；保留原环境 |
| [Multilogin](https://multilogin.com/help/en_US/how-to-transfer-browser-profiles) | 云环境转移包含设置、Cookie、Local Storage、会话、扩展及上次标签页 | 环境数据与标签页一致恢复 |
| [Dolphin Anty](https://docs.dolphin-anty.com/en/working-with-profiles/profile-cloud-sync) | 正常关闭后上传，绿勾表示完成；第二台设备应等第一台同步成功 | 不把本地保存/上传请求当成远端完成 |
| [MoreLogin](https://support.morelogin.com/en/articles/10198603-setting-center) | 可配置端到端加密及验证频率，关闭后同步 Cookie、缓存数据、扩展；提供会话锁 | 加密配置可复用、自动备份不遗漏登录态 |
| [比特浏览器](https://doc.bitbrowser.net/api-docs/browser-profiles) | API 独立提供 syncCookies、syncTabs、syncIndexedDb、syncLocalStorage 等字段 | 显式区分数据覆盖范围 |
| [Octo Browser](https://docs.octobrowser.net/en/profiles/browser-profile-settings/) | Local Storage/IndexedDB 与 Service Workers 也可能参与登录；按关闭保存 | 保留全目录而非只导出 Cookie |

以上为公开功能，不证明厂商内部 App-Bound、服务端密钥管理或并发协议的具体实现。没有购买/登录商业软件，也不声称完成实机对比。不能根据营销 AES 描述推断其算法细节。

## 本轮设计与交付条件

1. **自动备份登录态保护**：复用现有 OpenList 定时任务；用户主动开启后，迁移密码仅以 Windows DPAPI 包装存于本机独立文件，不放入 YAML、不返回前端、不进入备份。恢复设备必须掌握原密码。默认不启用、不改变既有任务目的地/时间；无法解密时明确失败，不明文降级。
2. **完整全量包加密**：在现有 ZIP 外层使用有版本的分块 AES-256-GCM 容器及 Argon2id 密钥派生；分块序号、长度、终止记录与文件头参与认证，拒绝截断、重排、篡改。大文件只用固定大小缓冲，打包时直接流式加密，不在备份/同步目录留下明文中间 ZIP。保留 ZIP 扩展名、现有云渠道及历史入口；密码验证和完整性校验在写入目标前完成。旧普通全量包、v1/v2 登录态包仍可导入。
3. **标签页会话存储**：采集已打开 HTTP(S) 页面各自的 sessionStorage，保留同源多标签页独立值。只在待恢复标签页首次文档脚本前写入，再移除一次性恢复脚本；不在以后导航重复回滚。无法读取的页面/框架明确计数/提示，不当作全量成功。Local Storage、IndexedDB、Service Workers 和扩展继续随冷态完整目录保存。
4. **可诊断性**：快照时间、范围、Cookie/标签页/存储项数量与未覆盖项；过旧/部分快照有明确提示。日志无 Cookie/存储值、密码、密钥或网页地址。Cookie 回读仅证明写入，不声称网站认证必然有效。

不建立新的账号服务器、后台常驻程序、云基础设施或跨设备锁。提供客户端备份/恢复能力，不声称能实现服务器撤销会话、通行密钥、硬件绑定令牌迁移。实例导出包本轮仍使用登录态载荷加密；整包加密覆盖全量备份，界面明确区别。

## 验收

- 密码配置重启读取、禁用、错误/损坏 DPAPI 不降级；定时路径确实使用受保护选项，测试不访问实际云。
- 多分块往返、错误密码、末尾截断/重排/篡改拒绝；输出中无网站存储合成明文；旧包路由继续可用。
- 指定 151 内核、临时目录、仅回环测试站点：HTTP Cookie、同源双标签页 sessionStorage 在网页首个脚本中正确恢复，后续导航不重置；不读取真实账号。
- 前端类型检查与现有相关回归，界面沿用原有备份弹窗；不重复正式打包。

状态：上述四项本地实现完成。没有启用任何实际定时任务或云上传。

## 实际证据

- 指定 151 内核：同源双标签页独立会话存储、首个网页脚本可读取、后续刷新不回滚通过；Cookie、持久 Cookie、CHIPS 分区 Cookie 的原有隔离迁移回归通过。测试仅访问回环站点。
- 多分块 AES-GCM 往返通过；错误密码、分块修改、截断、重排和尾部追加全部被拒绝。密文容器内不含合成网站令牌明文。
- 直接流式导出的写入中临时文件确认为密文容器，无明文中间包；解密后文件内容与 manifest 校验通过。缺失必要源文件时原有备份保持不变，不遗留临时输出；普通 ZIP 与历史扫描回归通过。
- 密码本机加密、重启读取、禁用、显式修复与损坏时不降级通过。实际自动导出入口使用整包加密；测试未调用真实云上传。
- 加密全量包经 `BackupRestoreWithLoginPassword` 实际执行解密、manifest 验证、数据库/文件恢复、运行时重载，待恢复 Cookie 保留；丢失 JSON 旁路元数据可安全重建历史。
- 界面沿用原备份弹窗：1440/800 宽度密码校验、错误密码重试、定时保护配置保存、成功清空密码与无内容溢出通过；界面绑定为明确模拟。前端类型检查与机械界面检测通过。

未验证：另一台物理 Windows 主机或真正重装、商业软件账号实机对比、商业网站真实账号、实际 v20 数据库、真实 OpenList/S3 云上传、Linux/macOS 登录态迁移。不能声称与商业产品全部能力相同，跨设备并发写入仍需用户避免。
