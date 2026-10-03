# Bang Downloader

Bang 是一个同时提供桌面界面和命令行的下载工具。Go 后端通过独立的 aria2 子进程处理 HTTP(S)、磁力链接和种子下载，桌面界面使用 Wails 2、Vue 3 和 TypeScript。

当前代码版本为 **2.0.1**，内嵌下载引擎版本为 **aria2 1.37.0**。

## 当前状态与下载

截至 2026-10-03，2.0.1 已通过[四平台原生构建与集成验收](https://github.com/cnuo774-hash/bang-downloader/actions/runs/37126435060)，以及[前后端测试和 Go 可达漏洞扫描](https://github.com/cnuo774-hash/bang-downloader/actions/runs/37126423994)。桌面窗口、文件选择器和视觉检查仍保留为人工验收项，详细记录见 [RELEASE.md](RELEASE.md)。

仓库目前尚未创建 GitHub Release。可登录 GitHub，在上述原生构建运行页面的 **Artifacts** 中下载对应产物，解开 Actions 下载的外层 ZIP，再解开其中的平台归档：

| 平台 | Actions 产物 | 平台归档中的程序 |
| --- | --- | --- |
| macOS Apple Silicon（arm64） | `bang-macos-arm64` | `Bang.app` |
| macOS Intel（amd64） | `bang-macos-amd64` | `Bang.app` |
| Windows x64（amd64） | `bang-windows-amd64` | `bang.exe` |
| Linux x64（amd64） | `bang-linux-amd64` | `bang` |

当前构建流程覆盖以上四个目标。发行归档已包含 aria2、许可说明及 aria2 对应源码，运行时无需单独安装 aria2。

桌面界面使用系统 WebView：Windows 需要 WebView2 Runtime，Linux 需要 GTK 3 和 WebKitGTK 4.1；Linux 引擎还使用 OpenSSL 与 zlib。开发所需的系统依赖参见 [Wails 官方安装说明](https://wails.io/docs/gettingstarted/installation/)。

## 桌面使用

打开 `Bang.app`、`bang.exe` 或 Linux 的 `bang` 即可进入桌面界面。程序不带参数时启动 UI，也可以显式使用 `--ui`。

1. 点击「新建任务」，粘贴 HTTP(S) 地址或磁力链接，也可以选择本地 `.torrent` 文件。
2. 选择保存目录并提交。首次运行的默认目录为当前用户的 `Downloads`。
3. 在任务列表查看进度、速度和错误信息，暂停或恢复未完成任务，移除任务记录。
4. 在「偏好设置」保存默认下载目录和全局下载限速。限速可填写 `512K`、`10M`、`1G`，留空或填 `0` 表示不限速。

移除任务会停止该任务并移除记录，已经下载的文件会保留。列表采用分页和虚拟滚动，支持「全部」「进行中」「已完成」筛选。

退出程序时会保存未完成会话并关闭下载引擎；再次启动会载入会话。磁力链接和远程种子产生的后续文件任务会继续被跟踪，BitTorrent 文件完成后停止做种。

## 命令行使用

命令行与桌面界面使用同一个程序、配置和会话。同一用户缓存目录只允许一个实例运行，使用 CLI 前需退出正在运行的 Bang 窗口。

以下示例假设程序已加入 `PATH`。macOS 也可以直接运行 `./Bang.app/Contents/MacOS/bang`，Linux 使用 `./bang`，Windows PowerShell 使用 `.\bang.exe`。

```sh
# 启动桌面界面
bang --ui

# HTTP(S) 文件，保存到指定目录
bang 'https://example.com/file.zip' -o ./downloads

# 本地种子文件
bang ./example.torrent --output ./downloads

# 远程种子；文件下载完成后才结束等待
bang 'https://example.com/example.torrent' -o ./downloads

# 磁力链接；替换为实际链接
bang 'magnet:?xt=urn:btih:<实际的信息哈希>' -o ./downloads

# 同一文件的 HTTP(S) 备用来源，可重复指定 --mirror
bang 'https://primary.example.com/file.zip' \
  --mirror 'https://mirror.example.com/file.zip' \
  -o ./downloads

bang --help
bang --version
```

| 参数 | 含义 |
| --- | --- |
| `--ui` | 启动桌面界面 |
| `-o`、`--output`、`--output=目录` | 本次任务的保存目录；省略时使用配置中的默认目录 |
| `--mirror`、`--mirror=地址` | HTTP(S) 备用源，可重复使用；主来源和备用源必须指向同一文件 |
| `-h`、`--help` | 显示用法 |
| `-v`、`--version` | 显示 Bang 与 aria2 版本 |

一次 CLI 调用接收一个主来源。`--ui` 单独使用；备用源仅适用于 HTTP(S) 主来源。保存目录支持绝对路径、相对路径和 `~`，不存在时会创建。`-o` 只影响当前任务，修改默认目录请使用桌面偏好设置。

CLI 会载入已有会话，并等待本次添加的任务及其种子后续任务完成。成功返回 `0`，下载失败或引擎异常返回 `1`，参数解析错误返回 `2`。运行期间可用 `Ctrl+C` 结束程序，未完成会话会保存。

## 配置与本地数据

| 数据 | macOS | Linux | Windows |
| --- | --- | --- | --- |
| 配置目录 | `~/Library/Application Support/Bang/` | `$XDG_CONFIG_HOME/Bang/`，未设置时为 `~/.config/Bang/` | `%AppData%\Bang\` |
| 缓存目录 | `~/Library/Caches/Bang/` | `$XDG_CACHE_HOME/Bang/`，未设置时为 `~/.cache/Bang/` | `%LocalAppData%\Bang\` |

配置目录中的 `config.json` 保存默认目录、全局限速、引擎版本和首次生成的随机 RPC 密钥。缓存目录包含释放后的引擎、`session.txt`、`history.json` 和会话锁；终态历史最多保留 200 条。桌面日志写入缓存目录的 `bang.log`，CLI 的诊断输出在终端中查看。

RPC 使用回环地址和随机端口，密钥通过临时配置传入 aria2，启动后移除该文件，不出现在子进程命令行或前端配置中。提交问题时请移除配置中的密钥及日志中的私人路径、下载地址。

## 从源码开发

仓库使用 Go 1.26.8、Node.js 22、pnpm 10 和 Wails 2.10.2。需要当前平台的 C/C++ 工具链及 Wails 系统依赖；macOS 需要 Xcode Command Line Tools。

```sh
git clone https://github.com/cnuo774-hash/bang-downloader.git
cd bang-downloader

cd frontend
pnpm install --frozen-lockfile
pnpm build
pnpm test
cd ..
```

Linux 的 Ubuntu 构建环境还需安装以下依赖，与原生构建工作流一致：

```sh
sudo apt-get update
sudo apt-get install -y build-essential pkg-config libgtk-3-dev libwebkit2gtk-4.1-dev libssl-dev zlib1g-dev
```

在目标平台准备内嵌引擎后启动开发模式：

```sh
# macOS
bash scripts/prepare-aria2.sh
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev

# Linux（WebKitGTK 4.1）
bash scripts/prepare-aria2.sh
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev -tags webkit2_41
```

Windows PowerShell：

```powershell
.\scripts\prepare-aria2.ps1
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev
```

准备脚本替换当前平台的引擎资源。macOS 和 Linux 从经过固定 SHA-256 校验的官方源码编译，Windows 使用经过校验的官方二进制归档。macOS 构建还检查引擎是否依赖 Homebrew 库。开发构建在缺少内嵌引擎时可以使用 `PATH` 中的 aria2；带 `release` 标签的发行构建要求实际内嵌资源。

## 构建与验证

在对应平台的本机执行构建。先完成前端安装和引擎准备，再运行：

```sh
# macOS / Windows
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build -clean -tags release

# Linux（WebKitGTK 4.1）
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build -clean -tags 'release webkit2_41'
```

应用输出位于 `build/bin/`。本机 Wails 构建生成程序；完整的许可与源码归档由 [native builds 工作流](.github/workflows/release.yml)组装。

macOS 的后端检查与真实内嵌引擎测试示例：

```sh
go test -race ./...
go vet -tags release ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.8.0 -tags release ./...
BANG_REQUIRE_EMBEDDED=1 go test -race -tags 'integration release' ./...
```

Linux 为以上 Go 命令加入 `webkit2_41` 标签。Windows 通过 `$env:BANG_REQUIRE_EMBEDDED = "1"` 设置集成测试要求。集成测试使用临时目录和本地 HTTP 服务，覆盖下载内容、备用源、暂停恢复、会话续传、种子后续任务、失败退出、历史移除和会话互斥。

普通 push 和 pull request 执行 [tests 工作流](.github/workflows/tests.yml)。手动运行 `native builds` 会在四个平台测试、构建并上传归档；推送 `v*` 标签且全部平台成功后，工作流才创建 [GitHub Release](https://github.com/cnuo774-hash/bang-downloader/releases)，附带发行文件、aria2 源码和 `SHA256SUMS`。完整验收标准见 [RELEASE.md](RELEASE.md)。

## 项目结构

```text
main.go                  CLI 入口与桌面生命周期
app.go                   桌面后端绑定、目录选择与偏好设置
internal/config/         配置加载、迁移与保存
internal/downloader/     来源和保存路径校验
internal/engine/         aria2 资源、RPC、任务、会话与子进程管理
frontend/                Vue 3 / TypeScript 桌面界面
scripts/                 各平台 aria2 准备脚本
.github/workflows/       自动化测试与原生构建
RELEASE.md               发行验收标准和验证记录
```

## 许可与反馈

Bang 使用 [MIT License](LICENSE)。内嵌的 aria2 使用 GPL-2.0-or-later，第三方许可说明见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。版本变更见 [CHANGELOG.md](CHANGELOG.md)，开发贡献见 [CONTRIBUTING.md](CONTRIBUTING.md)，安全问题报告方式见 [SECURITY.md](SECURITY.md)。

请仅下载有权访问和分享的内容。BitTorrent 下载会参与节点交换，其他节点可看到你的 IP 地址。
