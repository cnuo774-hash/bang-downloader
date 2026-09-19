# Contributing

感谢你改进 Bang Downloader。

1. Fork 仓库并从 `main` 创建分支。
2. 保持改动聚焦，并为行为变化添加测试。
3. 在 `frontend` 目录执行 `pnpm install --frozen-lockfile && pnpm build`。
4. 在项目根目录执行 `go test ./...` 和 `go vet ./...`。
5. 涉及桌面界面时，在当前目标平台运行 Wails 开发模式或正式构建进行验证。
6. 提交 pull request，说明问题、实现方式和验证结果。

Wails 使用平台原生 WebView 和 CGO。Windows、macOS、Linux 的正式构建应在对应操作系统上完成，不接受从 Linux 交叉编译 Windows/macOS 发布包。

请不要提交真实私有磁力链接、种子内容、RPC 密钥、认证信息、下载历史、日志或本地隐私路径。测试应使用虚构数据，并且不得依赖真实下载任务。

正式发布使用 aria2 1.37.0。升级 aria2 时必须同时更新版本常量、准备脚本、测试、第三方说明，并验证三个平台的进程退出行为。
