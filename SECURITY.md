# Security Policy

## Supported versions

安全修复应用于最新发布版本和 `main` 分支。

## Reporting a vulnerability

请使用 GitHub 的 Private vulnerability reporting 功能报告安全问题，不要在公开 issue 中披露细节。报告中请包含影响范围、复现步骤和建议修复方式；请勿附带真实的私有种子、RPC 密钥或下载内容。

## Security model

- aria2 RPC 只监听 `127.0.0.1`，并要求随机生成的 RPC secret。
- RPC secret 保存在用户配置目录，文件权限在支持的平台上限制为当前用户可读写。
- 保存路径经过清理并转换为绝对路径，拒绝 NUL 字符和指向普通文件的目录路径。
- 内嵌 aria2c 只会释放到用户缓存目录；缓存不可用时才回退到系统临时目录。
- 关闭 Wails 窗口会关闭 aria2；正常 RPC 关闭超时后，Unix 终止整个进程组，Windows 关闭带有 `KILL_ON_JOB_CLOSE` 的 Job Object。

用户主动选择的绝对路径以及包含 `..` 的相对目录会被解析为本地绝对路径，这是命令行工具的预期能力。Bang 不以提升权限运行，也不绕过操作系统文件权限。
