# Bang Downloader

Bang 是一个轻量、本地运行的下载工具。它用同一个命令处理磁力链接、`.torrent` 文件和普通 HTTP 文件，并保留一个按需启动的网页界面。

> 请仅下载你有权获取和分享的内容。Bang 不提供资源搜索、索引或内容服务。

## 特性

- 一个命令完成来源识别、路径创建和下载
- 支持相对路径、绝对路径和 `~` 用户目录
- 磁力与种子任务支持 aria2 断点续传，下载完成后不做种
- HTTP 下载先写入 `.part` 文件，成功后再替换目标文件
- UI 仅监听 `127.0.0.1`，不会暴露到局域网
- Python 标准库实现，无运行时 Python 第三方依赖
- 支持 macOS、Linux 和 Windows，要求 Python 3.9+

## 安装

先安装 aria2，它负责磁力链接和种子下载：

```bash
# macOS
brew install aria2

# Ubuntu / Debian
sudo apt install aria2

# Windows
winget install aria2.aria2
```

再安装 Bang：

```bash
git clone https://github.com/cnuo774-hash/bang-downloader.git
cd bang-downloader
python3 -m pip install .
```

也可以不安装，在源码目录直接使用 `./bang`。

## 命令行

```bash
# 磁力链接，默认保存到 ~/Downloads
bang 'magnet:?xt=urn:btih:...'

# 本地种子，保存到相对路径
bang ./movie.torrent -o ./downloads

# 使用绝对保存路径
bang /path/to/movie.torrent -o /Volumes/Data/downloads

# 普通 HTTP 文件
bang 'https://example.com/file.zip' -o ./downloads
```

查看所有参数：

```bash
bang --help
```

## 图形界面

```bash
bang --ui
```

程序会打开 <http://127.0.0.1:8765>。如不希望自动打开浏览器，或需要更换端口：

```bash
bang --ui --no-browser --port 9000
```

macOS 源码用户也可以双击 `start.command`。

## 开发

```bash
python3 -m unittest discover -s tests -v
python3 -m bang_downloader.cli --version
```

项目结构：

```text
bang_downloader/core.py   下载、路径和 aria2 命令构造
bang_downloader/cli.py    命令行入口
bang_downloader/ui.py     本地网页服务
bang_downloader/web/      UI 资源
tests/                    无网络单元测试
```

## 安全与隐私

Bang 在本机工作，不会把链接、种子或下载记录发送给第三方服务。BitTorrent 协议本身会向网络中的其他节点公开你的 IP 地址。发现安全问题请参阅 [SECURITY.md](SECURITY.md)。

## 贡献与许可

欢迎提交 issue 和 pull request，细节见 [CONTRIBUTING.md](CONTRIBUTING.md)。项目使用 [MIT License](LICENSE)。
