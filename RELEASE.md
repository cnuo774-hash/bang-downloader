# Bang 2.0.1 发行验收

依据 README 的功能承诺，代码与构建产物必须满足以下标准。

## 自动化门禁

- CLI、前端 package.json 和桌面产品版本一致。
- Go 单元测试、竞态检查、静态检查和 Go 可达漏洞扫描通过。
- 前端 TypeScript 检查、生产构建和分页事件回归测试通过。
- macOS arm64、macOS amd64、Windows amd64、Linux amd64 在各自原生 runner 上构建成功。
- 每个目标平台使用实际内嵌 aria2 1.37.0 通过集成测试：HTTP 内容一致性、备用源切换、暂停、恢复、重启会话、种子元数据后续任务、失败退出、历史删除、保留文件和会话互斥。
- 引擎退出使 CLI 返回非零状态，正常退出释放进程及系统锁；终态历史最多保留 200 条，正在等待的任务不因历史清理丢失。
- RPC 只监听回环地址并使用随机密钥，密钥不出现在进程参数中，临时配置在启动后移除；发行构建使用 `release` 标签，缺少内嵌资源时禁止回退到 PATH。
- 下载的 aria2 归档通过固定 SHA-256 校验。发行包附带 Bang 许可、第三方说明、aria2 完整许可和固定版本源码；Windows 同时保留官方包的 OpenSSL 许可与构建说明。

## 复现

使用 Go 1.26.8、Node.js 22 和 pnpm 10。Linux 需安装 README 与工作流所列的原生依赖。

```sh
cd frontend
pnpm install --frozen-lockfile
pnpm test
pnpm build
cd ..
go test -race ./...
go vet ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 ./...
bash scripts/prepare-aria2.sh
BANG_REQUIRE_EMBEDDED=1 go test -race -tags 'integration release' ./...
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build -clean -tags release
```

Linux 将 `webkit2_41` 加入测试、扫描和 Wails 构建标签。Windows 使用 PowerShell 准备脚本以及 `$env:BANG_REQUIRE_EMBEDDED = "1"`。所有下载测试使用临时目录和本地 HTTP 服务，不依赖真实下载资源。

`native builds` 的 `workflow_dispatch` 用于完整发行候选验收，成功后可从 Actions 下载四个平台的归档。只有推送 `v*` 标签才会触发 GitHub Release，且所有平台构建必须成功。源码和发行文件的 SHA-256 清单一同发布。

## 验证记录（2026-10-03）

- 本地 macOS arm64：Go 竞态及集成测试、静态检查、前端生产构建、前端事件回归、零可达漏洞扫描与原生桌面打包通过。
- 代码验收提交：`adf67f3321366695784541768357af064b94d34f`。
- [通用测试与漏洞扫描](https://github.com/cnuo774-hash/bang-downloader/actions/runs/37126423994)通过：Go 竞态测试、静态检查、前端检查与生产构建、Go 可达漏洞扫描。
- 本地最终打包应用：CLI 版本正确，HTTP 下载内容逐字节一致，启动后临时 RPC 配置清理通过。归档包含程序、许可和 aria2 对应源码，校验清单位于 `build/releases/SHA256SUMS`。
- [四平台原生验收](https://github.com/cnuo774-hash/bang-downloader/actions/runs/37126435060)全部通过：macOS arm64、macOS amd64、Windows amd64、Linux amd64 的实际内嵌引擎竞态集成测试、静态检查、前端验证、桌面构建和许可归档成功。四个平台的发行归档可从该运行的 Artifacts 下载。
- 按用户确认，人工桌面检查保留为待验收项，本次完成自动化验证和仓库推送。
- 原生窗口点击与视觉验收未自动执行：当前系统未授予 Computer Use 权限，Browser 无法核实管理策略而拒绝本地页面访问。自动化测试覆盖前端状态逻辑和真实后端下载流程，不能代替窗口、文件选择器和视觉的人工检查。

## 人工桌面检查

发布前在目标操作系统打开程序，检查空列表、新建 HTTP 与种子任务、保存目录选择、暂停恢复、已完成任务移除、偏好保存、窗口关闭及重新打开。测试下载应使用本地或有权访问的资源。确认任务名称、路径与错误文本完整可读，键盘可访问按钮和弹窗，窗口关闭后没有残留 aria2 子进程。

## 波奇酱主题验证（2026-10-10）

本次前端主题修改保留下载后端和分页事件协议，采用本地打包的后藤一里角色素材。来源及版权说明见 `frontend/src/assets/bocchi/SOURCES.md`。

- 前端 TypeScript 检查、Vite 生产构建及 5 项任务状态回归测试通过。
- macOS arm64 Wails 发行构建通过，并更新到本机 `/Applications/Bang.app`。
- 实际桌面窗口检查了下载页、空列表、偏好设置、新建弹窗，以及下载中、暂停和完成状态的任务行。
- 本地 HTTP 任务经界面添加、暂停和恢复后完成，下载内容逐字节一致；测试文件位于临时目录。
- 弹窗自动聚焦来源输入，Tab/Shift+Tab 在可操作控件间循环，Escape 关闭后返回打开弹窗的按钮。

以上记录对应 macOS 本地主题验收；其他平台的窗口交互仍按人工检查清单验收。上方四平台工作流链接保留其原始验收提交的记录。
