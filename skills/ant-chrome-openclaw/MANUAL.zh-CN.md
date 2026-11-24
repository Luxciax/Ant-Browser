# ant-chrome-openclaw 使用说明

## 安装

把 `skills/ant-chrome-openclaw` 复制到 OpenClaw 的 `skills` 目录，或使用仓库自带安装脚本。命令从仓库根目录执行。

Windows:
```powershell
pwsh -File skills/ant-chrome-openclaw/scripts/install_ant_chrome_openclaw.ps1 -SetDefaultProfile
```

Linux:
```bash
bash skills/ant-chrome-openclaw/scripts/install_ant_chrome_openclaw.sh --set-default-profile
```

自动探测失败时使用脚本的 `TargetSkillsDir/ConfigFile`（Windows）或 `--target-skills-dir/--config-file`（Linux）参数。API Key 只通过安装参数或宿主 secret/config 传递，不写进提示词或仓库。

## 使用

调用 Skill 时直接描述目标、实例标识和允许的副作用，不需要把内部 API 检查步骤复制进提示词。优先使用精确 `launchCode` 或 profile ID。

示例：
```text
使用 ant-chrome-openclaw skill。
启动 BUYER_001，等浏览器可接管后打开 https://example.com。
```

```text
使用 ant-chrome-openclaw skill。
执行已保存脚本 news-query-txt，沿用脚本默认目标和参数。
```

```text
使用 ant-chrome-openclaw skill。
检查当前 active 实例和 BUYER_001 状态；如果不是同一个实例，不要自动切换。
```

## 边界

- Ant Browser 的 `runtime/stop` 才是停止实例；外部浏览器客户端自行 detach 不等于停止 Ant Browser。
- 多个实例匹配时不要静默挑选。
- 只有实际返回 ready/可用 CDP target 后才接管。
- 具体 HTTP 路由以 `backend/internal/launchcode/server_http.go` 为准。
