# Bang Downloader

Bang 是一个本地优先的下载工具，同时提供命令行和 Wails 桌面界面。它支持磁力链接、`.torrent` 文件和 HTTP(S) 地址，正式发布包会内嵌固定版本的 aria2c，用户无需另行安装 aria2。

> 请只下载你有权获取和分享的内容。Bang 不提供资源搜索、索引或内容服务；BitTorrent 协议会向网络中的其他节点公开你的 IP 地址。

## 功能

- Go 后端和 Wails + Vue 3 桌面界面
- 支持磁力链接、本地 `.torrent` 和 HTTP(S) 下载
- 保存目录支持相对路径、绝对路径和 `~`
- 正式构建内嵌 aria2 1.37.0，运行时释放到用户缓存目录
- 窗口关闭时先请求 aria2 正常退出，超时后终止整个进程组或 Windows Job Object
- aria2 RPC 只监听 `127.0.0.1`，并使用首次启动时生成的随机密钥
- 下载目录、全局限速、会话和历史记录持久化
- 分页 API、虚拟列表和增量任务事件，适合较大的任务列表
- macOS、Windows 和 Linux 分别在原生 GitHub Actions runner 上构建

## 使用

从 [GitHub Releases](https://github.com/cnuo774-hash/bang-downloader/releases) 下载对应平台的发布包。正式发布版本已经包含 aria2c。

```bash
# 启动桌面界面；不带参数时同样会启动 UI
bang --ui
bang

# 磁力链接，使用设置中的默认目录
bang 'magnet:?xt=urn:btih:...'

# 本地种子和相对保存目录
bang ./movie.torrent -o ./downloads

# 绝对路径也可以写在来源之前
bang --output /Volumes/Data/downloads /path/to/movie.torrent

# HTTP(S) 文件
bang 'https://example.com/file.zip' -o ~/Downloads

# HTTP(S) 超时或失败时切换备用源（可重复指定 --mirror）
bang 'https://primary.example.com/file.zip' \\
  --mirror 'https://mirror.example.com/file.zip' -o ~/Downloads

bang --help
bang --version
```

macOS 发布包中的命令行程序位于 `Bang.app/Contents/MacOS/bang`。可以直接从终端运行该文件，或将它链接到 `PATH` 中的目录。

## 数据位置

Bang 使用操作系统提供的标准用户目录，不会把 aria2c 释放到当前工作目录。

| 数据 | macOS | Linux | Windows |
| --- | --- | --- | --- |
| 配置 | `~/Library/Application Support/Bang/config.json` | `$XDG_CONFIG_HOME/Bang/config.json` | `%AppData%\Bang\config.json` |
| 缓存、日志、会话 | `~/Library/Caches/Bang/` | `$XDG_CACHE_HOME/Bang/` | `%LocalAppData%\Bang\` |

缓存目录包含释放后的 aria2c、`bang.log`、aria2 会话及最多 200 条历史任务。RPC 密钥保存在权限受限的配置文件中，不会显示在界面里。

## 开发

需要 Go 1.23+、Node.js 22、pnpm 10，以及当前平台的 Wails 系统依赖。发布包会内嵌固定版本的 aria2c，并在运行时释放到用户缓存目录。开发模式若缺少对应资源会回退到 `PATH` 中的 aria2c。HTTP 下载可通过重复 `--mirror` 提供备用源；aria2 会在连接超时、失败重试时切换 URI。

```bash
cd frontend
pnpm install --frozen-lockfile
pnpm build
cd ..

go test ./...
go vet ./...
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 dev
```

构建正式包：

```bash
# macOS / Linux，在目标平台本机执行
bash scripts/prepare-aria2.sh
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build -clean

# Windows PowerShell
./scripts/prepare-aria2.ps1
go run github.com/wailsapp/wails/v2/cmd/wails@v2.10.2 build -clean
```

Wails 依赖原生 WebView 和 CGO。不要在 Linux 上交叉编译 Windows 或 macOS 版本；仓库的 Release 工作流为每个平台使用各自的 runner。

macOS 的准备脚本优先从 GitHub Releases 下载 aria2 官方源码；连接超时或下载失败时会自动切换到 SourceForge 官方镜像。无论使用哪个来源，都必须通过固定的 SHA-256 校验才会参与构建。

## 项目结构

```text
main.go                     CLI 与 Wails 生命周期
app.go                      类型安全的前后端绑定
internal/config/            配置和 RPC 密钥持久化
internal/downloader/        来源和保存路径校验
internal/engine/            aria2 嵌入、RPC 与进程管理
frontend/                   Vue 3 / TypeScript 界面
scripts/                    固定版本 aria2 的构建准备脚本
.github/workflows/          测试和各平台原生发布构建
```

## 安全与许可

发现安全问题请参阅 [SECURITY.md](SECURITY.md)。Bang 自身使用 [MIT License](LICENSE)；发布包内嵌的 aria2 使用 GPL-2.0-or-later，详情见 [THIRD_PARTY_NOTICES.md](THIRD_PARTY_NOTICES.md)。
