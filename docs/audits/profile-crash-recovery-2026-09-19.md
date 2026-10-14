# Profile 导入/删除崩溃恢复修复

本轮接续发布质量审计，修复此前确认但未处理的“目录已经变化、SQLite 尚未提交时进程退出”问题。工作区仍基于 release/v1.8.1-fork.7（7234bec）；没有升级版本号、打包、操作真实用户 Profile 或发布。

## 恢复协议

新增 SQLite migration 18：`profile_file_transactions`。记录只包含事务 ID、目录操作计划、是否提交；不保存脚本内容、LaunchCode 或凭据。没有新依赖或后台常驻程序。

顺序为：

1. 完成导入包校验、解压或删除前所有权检查。
2. 在 SQLite 持久化完整文件操作计划，确认成功后才开始改名。备份名由目标目录和事务 ID 确定，位于同一父目录。
3. 将旧目标改名保留，再把导入暂存目录放入目标；删除只暂存旧目标。
4. **同一个 SQLite 事务**修改业务表并设置 journal 的 committed 标记。删除时 Profile、插件绑定、插件配置、插件 runtime 和 LaunchCode 行一起提交/回滚。
5. 提交后更新内存；移除暂存备份，最后删除 journal。文件清理失败保留 journal，等待重启重试。

启动顺序现在是：打开数据库 → migration → 恢复 journal → 加载 Manager/Profile/LaunchCode → API 服务与自动回收站清理。migration 或恢复失败时停止启动，不加载一个文件状态尚未确定的实例列表。

恢复以数据库中的 committed 为准：

- **未提交**：恢复旧目录；删除此次新建但未提交的导入目录。SQLite 自动回滚被强杀时尚未提交的业务事务。
- **已提交**：保留新目录/已删除的数据库状态，只完成备份清理。
- **无法读取提交结果**：不猜测、不删除，保留 journal 和文件。Commit 返回值不明确时先读取同一事务的持久化标记；仍无法确认则要求重启恢复。
- **恢复再次中断**：已恢复的旧目标、已移除的备份均可再次处理。仅在全部文件处理成功后才移除记录。

同一时刻仅允许一个未完成的文件事务。未解决的记录会阻止新的导入/物理删除事务、浏览器启动及扩展安装/移除；已有浏览器的停止入口仍可使用。

## 修改位置

| 位置 | 行为 |
|---|---|
| `backend/internal/profiletxn/transaction.go` | 持久化 intent、目录操作、同事务提交标记、幂等恢复、根目录与 junction/symlink 边界检查 |
| `backend/internal/database/sqlite.go` | 追加 migration 18，不修改旧 migration |
| `backend/app_profile_package_api.go` | Profile 与携带的扩展目录使用同一 journal；SQLite 模式不再依靠仅存于内存的 swap 清单 |
| `backend/app_profile_package_database.go` | 新版关系快照与旧版 Profile 包均在业务提交中写入 marker |
| `backend/internal/browser/profile_delete.go` | SQLite 模式下关系删除与主记录原子提交；手动彻底删除与自动过期清理共用实现 |
| `backend/internal/launchcode/service.go` | 成功提交删除后清除缓存，不再另做一次数据库删除 |
| `backend/app_profile_transaction_recovery.go`、`app_startup.go` | 启动恢复及失败阻断 |
| `profile_transaction_guard.go`、启动与扩展入口 | 未完成恢复时阻止继续改写相关文件 |

## 验证

### 独立进程强杀：14 个场景

`TestRecoverAfterProcessKilled` 对 import/delete 分别覆盖：

1. intent 已保存，目录尚未改动。
2. 旧目标已改名到备份，后续文件操作尚未完成。
3. 所有文件操作完成，业务 SQL 尚未执行。
4. 业务 SQL 和 committed marker 已写入尚未提交的 SQLite 事务。
5. SQLite 已提交，备份尚未清理。
6. 已恢复一个旧目录，恢复过程再次退出。
7. 已清理一份已提交备份，清理过程再次退出。

父测试等待子进程到达指定持久化状态，调用 `Process.Kill` 并 Wait，然后重新打开真实 SQLite/WAL 并执行恢复。不是返回 error，不会执行子进程 defer。目录半操作及恢复半操作场景通过构造与实现相同的实际 rename/remove 边界来稳定命中，避免靠定时竞争碰巧杀中。

每个场景均核对 SQL 新/旧状态、既有目录、新建目录、扩展目录、无关目录和 journal，并再恢复一次确认幂等。

### Windows 文件锁及真实调用链

- `TestWindowsLockedFileRetainsJournalAndRetryRecovers`：使用 Windows 文件句柄禁止共享删除，分别阻塞未提交回滚和已提交清理；确认 journal/旧备份未丢失，关闭句柄后恢复成功。
- `TestDurableProfileDeletionKeepsRelationsAtomic`：真实 SQLite DAO + LaunchCodeService，验证成功删除及 SQL trigger 拒绝删除两种情况；检查五张业务表、目录、LaunchCode 内存缓存。
- `TestProfileImportJournalCommitFailureRollsBackFilesAndDatabase`：触发 committed 标记写入失败，确认业务行回滚、journal 清理；去掉故障后重试，核对真正导入的 Preferences 内容。
- `TestPendingProfileTransactionBlocksBrowserStartUntilStartupRecovery`：实际启动入口被 pending journal 阻断；启动恢复函数恢复目录后解除阻断。
- `TestJournalRejectsOverlappingOrOutsideDirectories`：拒绝根目录本身、越界和父子重复目标；另包含 symlink 父目录下尚不存在路径的检查，主机不允许创建 symlink 时仅该分支跳过，本轮汇总日志未单独证明该分支执行。
- `TestRecoveryPreservesFilesWhenJournalCannotBeRead`：数据库不可读时保留备份，重新打开后恢复。
- 整仓 `go test ./...`、`go vet ./...`、`git diff --check` 通过。本轮未改动前端，上一轮 TypeScript/Vite production build 通过证据仍有效。

## 兼容性及剩余边界

- 标准应用启动始终使用 SQLite，新旧格式的 Profile 包均覆盖。非 SQLite 的测试/旧配置 fallback 保留原有普通错误回滚，不能宣称该 fallback 具备本轮持久化崩溃恢复能力。
- 新版本只新增 journal 表，不搬迁既有 Profile，也不改写既有插件用户数据。成功恢复后 journal 清空。
- 旧版留下的 `.profile-package-backup-*` / `.delete-staging-*` **不自动猜测处理**，因为它们没有与提交结果关联的日志；仍需按具体数据库与目录证据恢复。
- 全局备份导入使用原有固定业务表清单，不会导入另一台机器的本地恢复 journal。全局备份逐文件合并本身不属于本轮协议，上一轮对 LevelDB 合并的疑点仍保留。
- 不应在 journal 尚未解决时退回不理解 migration 18 的旧程序。配置根路径改变、备份目录丢失、文件持续被外部进程锁定时，恢复可能暂停并保留文件，需要解决实际路径/锁定原因后重启。
- 这是进程崩溃/强杀恢复验证，不是断电、磁盘损坏或文件系统硬件故障认证。没有修改真实用户数据库，也没有运行安装版升级或 GUI 扩展样本验收。
- 上一轮 Talisman 提交钩子拦截仍未通过豁免或绕过处理；本轮代码保留在本地，不推送、不发布。
