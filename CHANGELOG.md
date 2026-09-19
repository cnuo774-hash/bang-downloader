# Changelog

## 2.0.0 - 2026-09-18

- 使用 Go 重写后端，并将 CLI 与桌面应用整合为同一个入口。
- 使用 Wails、Vue 3 和 TypeScript 重写界面。
- 正式构建内嵌固定版本 aria2 1.37.0，运行时安全释放到缓存目录。
- 增加 Unix 进程组和 Windows Job Object 生命周期管理。
- 增加随机 RPC 密钥、配置迁移、会话、历史、下载目录和限速持久化。
- 增加并发 RPC 串行化、分页任务 API、虚拟列表和增量事件。
- 增加 macOS、Windows 和 Linux 原生 Release 构建工作流。

## 0.3.0 - 2026-09-15

- 提供 Python 版 `bang` 命令和本地网页界面。
- 支持磁力链接、本地或远程种子以及 HTTP 文件。
- 支持相对、绝对和用户目录路径。
