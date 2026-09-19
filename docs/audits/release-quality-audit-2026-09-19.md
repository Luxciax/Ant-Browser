# Ant Browser 发布质量审计（2026-09-19）

后续状态：本文“跨文件/SQLite 操作缺少进程崩溃后的事务恢复”已继续修复，协议与强杀测试结果见 [Profile 崩溃恢复报告](profile-crash-recovery-2026-09-19.md)。以下正文保留首轮审计时的证据与判断。

## 基线、范围和边界

- 基线：`release/v1.8.1-fork.7` / `7234becd280b1bb656172860525815963fdeeb8b`，与远端 tag 相同。已 fetch 核对：远端 master 为 `ce3085e`，发布分支包含其 Xray/扩展数据修复，并额外包含 fork.7 权限修复。
- 实际修改位于现有 release worktree，而非旧的 `fix/stability-audit-20260916` 工作区。原有 go.mod、Wails runtime、打包目录未纳入本次修改或提交。
- 审计重点：Wails API → maintenance/profile locks → Manager/DAO → SQLite、Profile 文件 → Chromium/代理进程；同时检查了扩展配置 UI、Profile 包导入导出、全局备份、应用退出/接管、单实例锁、路径解析和发布安装脚本。
- 没有启动用户现有浏览器实例、迁移用户真实数据、执行安装/卸载、发布或推送。所有新增数据和故障注入均在测试临时目录。
- 这是代码链路审计和自动化验证，不等于全功能 GUI 验收、真实扩展兼容认证或长期稳定性证明；没有声称逐行证明全部代码无缺陷。

## 架构与数据流

1. 前端通过 Wails 调用 `backend/app_*`。多数持久化修改由 `maintenanceMu` 串行化，启动/停止另有 Profile 生命周期锁；进程监控通过代际检查避免旧进程退出事件覆盖新启动。
2. `browser.Manager` 保存内存 Profile，SQLite DAO 持久化配置；Chromium 的实际登录态、扩展数据和权限位于用户数据目录。应用重启后需要通过进程发现/CDP 恢复运行态，数据库记录并非进程存活的充分证据。
3. 扩展至少有三个身份/状态层：全局扩展目录和包、Profile 选用配置与 runtime 索引、Chromium 实际 runtime ID 与 Preferences/存储目录。启动准备负责安装和迁移，删除需要同时回滚目录、注册与 DAO 状态。
4. Xray 与 sing-box 桥接按节点 key 复用，以引用计数固定浏览器使用的桥接；意外退出需要保持原端口恢复。Mihomo 是独立连接栈，本次未改变选栈规则。
5. Profile 包采用暂存目录、目录交换、SQLite 事务、内存发布的顺序。全局备份是分模块合并并报告部分成功，不具备同等事务语义。

## 已确认并修复

### 1. P1：用户脚本权限迁移遗漏首次安装及 runtime 索引恢复路径

- **问题 → 触发**：旧 runtime ID 有 `user_scripts_enabled=true`；首次升级迁移后脚本权限丢失。若新代码目录已存在而 runtime 索引丢失，第一次启动只恢复索引，必须再启动一次才修复旧权限/数据。
- **根因**：fork.7 的权限迁移只在 `repairLegacyProfileExtensionStorage` 调用；实际安装循环迁移存储后直接删除旧注册。`recoverExistingPersistentExtensionRuntime` 成功又提前返回。
- **位置**：`backend/internal/browser/extension_persistence.go`，`ensurePersistentExtensionInstalledContext`。
- **修复**：清理旧 ID 前迁移权限；索引恢复后继续走已安装修复路径。沿用已有“新 ID 明确选择优先”的规则，不强制覆盖 false。
- **兼容/回滚**：无需数据库 schema 迁移；权限迁移失败恢复安装前快照并报告错误。已经在旧版本中丢失且没有原记录的选择无法凭空恢复。
- **验证**：`TestAuditInitialRuntimeMigrationPreservesUserScriptsPermission`、`TestAuditRecoveredRuntimeMigratesLegacyPermissionOnFirstLaunch` 均先在原代码上失败，修复后通过；既有权限选择保留、现有数据恢复测试通过。

### 2. P1：安装回滚删除未纳入快照的新 ID 用户数据

- **问题 → 触发**：部分迁移/导入后，新旧 ID 都存在存储，runtime 索引缺失；重新安装的最后一次 SQLite upsert 失败，新 ID 脚本数据被回滚删除。
- **根因**：快照仅包含检测到的旧 ID，恢复却会清理旧 ID 和目标 ID。
- **位置**：同文件 `ensurePersistentExtensionInstalledContext` 的快照准备。
- **修复**：安装前从包计算目标 runtime ID，一并纳入快照及恢复集合。
- **兼容/回滚**：不合并两份 LevelDB；两边原数据均保留，失败重试仍可进行，无 schema 变更。
- **验证**：`TestAuditInstallRollbackPreservesUnindexedTargetStorage` 注入 DAO 写入失败；修复前新数据确实被删除，修复后两侧内容不变且再次安装成功。

### 3. P1：sing-box pinned 桥接意外退出时跳过恢复且不通知

- **问题 → 触发**：浏览器仍持有桥接引用，sing-box 进程异常退出；实例代理永久失效，旧桥接保留为 restarting。
- **根因**：`watchBridge` 设置 `Restarting=true`，`restartBridgeOnSamePort` 又把该标志作为“无需恢复”条件。
- **位置**：`backend/internal/proxy/singbox_bridge_recovery.go`。
- **修复**：接受监控发起的恢复状态；复用按 key 的启动锁；提交新进程时重新检查当前对象、停止标志及实时引用数，不恢复已释放的旧引用。
- **兼容/回滚**：不改节点配置、选栈或端口协议；恢复失败继续原有失败通知，过期候选进程被终止。
- **验证**：`TestSingBoxWatcherDoesNotSuppressRecoveryOrFailureNotification` 修复前失败、修复后通过。`TestSingBoxRealProcessRecoversPinnedBridge` 使用仓库现有 Windows sing-box，在临时目录启动直连 SOCKS 桥接、强制结束测试进程，确认新 PID 在同一端口恢复、SOCKS 握手成功、释放后引用归零。没有连接外部代理节点。

### 4. P1：拒绝删除运行中插件后，仍回滚并重写活跃 Profile

- **问题 → 触发**：插件仍在运行中的实例里，用户点击删除，接口返回拒绝，但实际仍重写其文件和 runtime 记录。
- **根因**：内层删除在校验阶段拒绝，外层却无条件重放整份删除前快照。内层已有自己的失败回滚。
- **位置**：`backend/app_extension_api.go`，`BrowserExtensionDelete`。
- **修复**：内层失败不再执行外层重复回滚；仅恢复全局启用状态。后续目录/catalog 删除失败时仍保留原回滚流程，并在 Profile 回滚失败时保留快照。
- **兼容/回滚**：不改变删除成功语义；不会为拒绝操作再次触碰 Chromium 的活跃文件。
- **验证**：`TestRejectedExtensionDeletionDoesNotRewriteRunningProfile` 用 SQLite trigger 统计 runtime 写入，修复前为 1，修复后为 0；现有 catalog 删除失败完整恢复测试通过。

### 5. P1：共享/嵌套 Profile 目录被另一个实例的回收站清理删除

- **问题 → 触发**：通过编辑页的用户数据目录字段创建相同/父子目录的实例；将其中一个移入回收站后彻底删除或等到 72 小时自动清理，另一个实例的登录态/扩展数据也被删除。
- **根因**：原代码只检查目录位于管理根内，没有检查是否被其他实例持有。
- **位置**：新增 `backend/internal/browser/profile_directory_ownership.go`；接入 `profile_create.go`、`profile_update.go`、`profile_delete.go`。
- **修复**：持有 Manager 锁时检查活跃实例和回收站目录的相等及父子关系。创建/编辑在写配置前拒绝冲突；物理删除和自动清理在暂存目录前拒绝冲突。
- **兼容/回滚**：不移动或自动合并旧目录；旧冲突记录保留并返回可识别错误，需要明确归属后手动修正。没有新增数据库约束，也不在失败时改变原配置。
- **验证**：`TestProfileCreationRejectsOverlappingDirectories`、`TestProfileDeletionPreservesOtherProfilesSharedData` 原代码可复现数据被删；修复后通过。覆盖同目录、父子目录、自动过期、回收站仍持有目录；`TestProfileUpdateRejectsDirectoryOverlapWithoutChangingState` 验证编辑失败不改变内存状态和合法自编辑仍可用。

### 6. P1：Preferences 替换在崩溃窗口内暂时不存在

- **问题 → 触发**：写配置先把原 Preferences 改名为备份，再把临时文件改名到原路径；两步之间进程被结束，下一次 Chromium 启动会看到配置缺失。
- **根因**：两个 rename 并非一次原子替换；普通错误回滚无法应对进程已退出。
- **位置**：`backend/internal/browser/extension_persistence.go`，`writeProfileJSON`。
- **修复**：复用已有 `fsutil.AtomicWriteFile`，同卷临时文件写入并 Sync 后直接替换；Windows 使用 `MoveFileEx(REPLACE_EXISTING | WRITE_THROUGH)`。
- **兼容/回滚**：JSON 内容与 UTF-8 编码保持兼容，未增加新恢复格式；该修复不宣称磁盘硬件故障/所有断电情形可恢复，也不自动处理以前残留的 `.backup-*` 文件。
- **验证**：`TestProfileJSONWriteFailureKeepsDestination`、`TestProfileJSONAtomicReplacementPreservesUnrelatedFields` 及已有 fsutil 原子替换失败测试通过。未实际在 Windows 断电。

### 7. P1：插件配置弹窗旧请求覆盖新目标，加载失败仍允许保存

- **问题 → 触发**：打开 A 后关闭并迅速打开 B，A 的异步读取晚到；或 B 加载失败后保存，A 的旧选择会被用于 B。
- **根因**：effect 没有失效清理，错误路径仅结束 loading，保留了旧数据；保存不检查当前目标是否成功加载。
- **位置**：`frontend/src/modules/browser/components/ProfileExtensionModal.tsx`、`pages/ExtensionManagementModals.tsx`、`frontend/src/ui-v2/fish/FishExtensionModals.tsx`。
- **修复**：effect 取消标记、清空旧选择、加载成功的目标 ID、按钮与保存入口双重校验；同时覆盖经典 UI 和 Fish UI 的批量实例限制弹窗。
- **兼容/回滚**：未改 API、数据库或已有选择；失效读取和加载失败不会发出保存请求。批量保存仍是原有多个 API 请求，不新增事务语义。
- **验证**：调用顺序可直接证明原问题；TypeScript 与 production build 通过。本仓库没有为这三个组件配置现成的 UI 测试运行器，本次未新增测试依赖，也未声称完成延迟请求的 GUI 实测。

## 已确认但暂未修复

### P1：跨文件/SQLite 操作缺少进程崩溃后的事务恢复

- `app_profile_package_api.go` 的 `replaceProfileUserDataDirWithBackup` 先交换目录，后续 `restoreProfilePackageDatabase` 才提交数据库；回滚清单仅存在于内存，defer 仅处理返回错误。被强制结束时，新目录、旧数据库与 `.profile-package-backup-*` 可以并存。
- `profile_delete.go` 的 `.delete-staging-*` 也有“目录已改名，关系/主记录尚未提交”的窗口。代码检索未发现这些标记的启动恢复流程。
- 原文件一般仍在暂存/备份目录，并非必然物理丢失，但正常 UI 不会自动识别并恢复；当前普通失败回滚测试不能代表进程崩溃恢复通过。
- 暂不盲目“启动时把备份改名回去”：缺少持久化事务阶段时，无法区分数据库已提交后的正常清理残留和真正应回滚的事务。正确修复需要持久化事务日志、幂等恢复顺序和 kill-point 故障矩阵，涉及独立的恢复协议设计。本次没有用猜测性目录处理引入二次数据覆盖。

## 可疑但证据不足

1. **Service Worker / IndexedDB 的 runtime ID 迁移完整性**：当前通过目录/文件名包含 ID 来选择迁移对象；Chromium 的共享 LevelDB 可能把 origin/ID 编码在键或元数据内。不能据此认定跨 ID 的完整恢复，需要真实 ScriptCat/Tampermonkey Profile 和实际内核版本验证。
2. **用体积认定 bootstrap 存储**：`shouldReplaceBootstrapExtensionStorage` 依据新目录不超过 4 KiB、旧目录相对更大判断覆盖。小目录未必没有有效的新用户选择；当前会保留安装前备份，但需真实扩展数据库样本确认误判边界，不能无条件改为保留导致旧 ScriptCat 恢复再次失效。
3. **全局备份逐文件合并有状态目录**：`backupSyncDir` 保留冲突文件并复制缺失文件，可能混入两个时间点的 LevelDB/MANIFEST/WAL。需要用真实备份前后快照证明影响，再决定整 Profile 替换、跳过冲突还是独立导入；不应直接把“冲突不覆盖”当成数据库可安全合并的证据。
4. **应用退出保留浏览器后的桥接接管**：现有浏览器、外部遗留 Xray/sing-box、内存引用表之间的归属需要真实“仅退出应用 → 重新打开 → 停止/重启实例”验证。原子文件和单元测试不覆盖残留进程、旧 PID 及端口复用的全部组合。

## 验证记录

- 修改前 `go test ./...` 通过，说明新增失败路径没有被现有测试覆盖。
- 新增 11 个 Go 测试函数（含一个需显式启用的实进程测试及多个子用例）。权限两条路径、回滚数据丢失、错误删除回滚、目录所有权、sing-box 恢复阻断均有修复前失败证据。
- 修改后 `go test ./...`：通过。
- 修改后 `go vet ./...`：通过。
- `npm --prefix frontend run build`：通过，脚本实际执行 TypeScript 编译与 Vite production build。
- 实进程命令：先把 `ANT_BROWSER_TEST_SINGBOX` 设为该工作区 `bin/sing-box.exe` 的绝对路径，再执行 `go test ./backend/internal/proxy -run '^TestSingBoxRealProcessRecoversPinnedBridge$' -count=1 -v`。通过，旧/新进程使用同一端口，进程仅属于测试。
- `git diff --check`：通过。
- 本地 commit 未生成：Talisman pre-commit 钩子标记报告中的完整 commit hash、测试固定字节串以及桥接 key/压缩 TSX 代码为疑似编码文本或 secret。未关闭钩子或添加豁免；仅本次修复与报告已暂存，原有用户改动保持未暂存。未进行任何 push。
- Race detector 未运行：现场 `CGO_ENABLED=0`，未发现 gcc；没有为本次审计安装编译器或更改主机工具链。
- 未执行真实安装版覆盖升级、系统权限变化、用户 Profile 的 GUI 回归或长期 soak test；未构建/发布新版本号或安装包。

## 总体判断与下一轮实机优先级

最脆弱的是扩展身份/权限/存储的三层同步、跨文件与 SQLite 的事务边界、应用退出后外部进程与桥接引用的重新归属。本次修复改善了明确路径，但剩余崩溃事务问题意味着不能宣称发布质量已完全闭环。

1. 使用隔离的真实 ScriptCat/Tampermonkey 样本：首次迁移、丢失 runtime 索引、连续两次冷启动、禁用/启用、升级后检查脚本、用户脚本开关、可选 host 权限、IndexedDB 与 worker 注册。
2. 安装版从 fork.5/6 升级 fork.7 后的修复版；便携版迁移路径；中文/长路径、只读或被杀软锁定文件。逐步观察真实 Preferences 和数据内容，而非只看 UI “已安装”。
3. Xray 和 sing-box 分别执行端口占用、网络不可达、桥接进程被结束、两个实例共用桥接、恢复过程中停止最后一个实例、仅退出应用后重开；检查无残留进程和错误引用。
4. 在 Profile 导入目录交换、SQLite commit、删除暂存的每个边界强制结束测试程序，重开核对目录/记录关系；持久化恢复协议应以这一矩阵作为验收。
