# Security Policy

## Supported versions

安全修复应用于最新发布版本和 `main` 分支。

## Reporting a vulnerability

请通过 GitHub 的 Private vulnerability reporting 功能报告安全问题，不要公开安全细节。报告中请包含影响范围、复现步骤和建议修复方式；请勿附带真实的私有种子或下载内容。

Bang 的 UI 设计为仅监听 `127.0.0.1`。将其修改为公开网络监听时，调用者需自行增加身份验证、TLS 和访问控制。
