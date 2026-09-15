"""Command-line interface for Bang Downloader."""

from __future__ import annotations

import argparse
import os
import subprocess
import sys
from typing import List, Optional

from . import __version__
from .core import aria2_command, download_http, find_aria2c, format_bytes, resolve_output, resolve_source, source_kind


def build_parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(
        prog="bang",
        description="下载磁力链接、种子文件或 HTTP 文件到指定目录。",
        epilog="示例：bang example.torrent -o ./downloads    bang --ui",
    )
    parser.add_argument("source", nargs="?", help="magnet 地址、.torrent 文件路径或 HTTP 地址")
    parser.add_argument("-o", "--output", default="~/Downloads", help="保存目录，支持相对路径和绝对路径")
    parser.add_argument("--ui", action="store_true", help="启动本地图形界面")
    parser.add_argument("--port", type=int, default=int(os.environ.get("BANG_PORT", "8765")), help="UI 端口，默认 8765")
    parser.add_argument("--no-browser", action="store_true", help="启动 UI 时不自动打开浏览器")
    parser.add_argument("--version", action="version", version=f"Bang {__version__}")
    return parser


def run_download(source_value: str, output_value: str) -> int:
    source = resolve_source(source_value)
    kind = source_kind(source)
    destination = resolve_output(output_value)
    if destination.exists() and not destination.is_dir():
        print(f"错误：保存路径不是目录：{destination}", file=sys.stderr)
        return 2
    if kind == "invalid":
        print(f"错误：种子文件不存在或输入格式无法识别：{source_value}", file=sys.stderr)
        return 2
    destination.mkdir(parents=True, exist_ok=True)
    print(f"来源：{source}\n保存：{destination}")

    if kind == "url":
        def show_progress(downloaded: int, total: int) -> None:
            if total:
                text = f"下载中 {downloaded / total * 100:5.1f}%  {format_bytes(downloaded)} / {format_bytes(total)}"
            else:
                text = f"下载中 {format_bytes(downloaded)}"
            print(f"\r{text}", end="", flush=True)

        try:
            output = download_http(source, destination, show_progress)
            print(f"\n完成：{output}")
            return 0
        except (OSError, ValueError) as exc:
            print(f"\n下载失败：{exc}", file=sys.stderr)
            return 1

    engine = find_aria2c()
    if not engine:
        print("错误：磁力链接和种子文件需要 aria2c。macOS 可运行：brew install aria2", file=sys.stderr)
        return 127
    try:
        return subprocess.run(aria2_command(source, destination, engine), check=False).returncode
    except KeyboardInterrupt:
        print("\n下载已停止，可使用相同命令继续下载。")
        return 130


def main(argv: Optional[List[str]] = None) -> int:
    parser = build_parser()
    args = parser.parse_args(argv)
    if args.ui:
        from .ui import serve_ui
        return serve_ui(args.port, not args.no_browser)
    if not args.source:
        parser.print_help()
        return 2
    return run_download(args.source, args.output)


if __name__ == "__main__":
    raise SystemExit(main())
