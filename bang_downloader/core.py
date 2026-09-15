"""Shared download and path handling primitives."""

from __future__ import annotations

import os
import shutil
from pathlib import Path
from typing import Callable, Optional, Union
from urllib.parse import unquote, urlparse
from urllib.request import Request, urlopen

from . import __version__

CHUNK_SIZE = 256 * 1024


PathValue = Union[str, os.PathLike]


def find_aria2c() -> Optional[str]:
    return shutil.which("aria2c")


def resolve_output(value: Optional[PathValue]) -> Path:
    path = Path(value or "~/Downloads").expanduser()
    if not path.is_absolute():
        path = Path.cwd() / path
    return path.resolve()


def resolve_source(value: PathValue) -> str:
    source = str(value).strip()
    if source.lower().startswith(("magnet:", "http://", "https://")):
        return source
    path = Path(source).expanduser()
    if not path.is_absolute():
        path = Path.cwd() / path
    return str(path.resolve())


def source_kind(source: str) -> str:
    lower = source.lower()
    if lower.startswith("magnet:"):
        return "magnet"
    if lower.startswith(("http://", "https://")):
        return "torrent" if urlparse(lower).path.endswith(".torrent") else "url"
    path = Path(source)
    return "torrent" if path.is_file() and path.suffix.lower() == ".torrent" else "invalid"


def aria2_command(source: str, target: Path, executable: Optional[str] = None) -> list[str]:
    engine = executable or find_aria2c()
    if not engine:
        raise FileNotFoundError("aria2c was not found")
    return [
        engine,
        "--dir", str(target),
        "--continue=true",
        "--seed-time=0",
        "--summary-interval=1",
        "--enable-color=false",
        source,
    ]


def safe_url_filename(source: str) -> str:
    name = Path(unquote(urlparse(source).path)).name
    return name if name not in ("", ".", "..") else "download"


def format_bytes(value: int | float) -> str:
    size = float(value)
    for unit in ("B", "KB", "MB", "GB", "TB"):
        if size < 1024 or unit == "TB":
            return f"{size:.1f} {unit}"
        size /= 1024
    return f"{size:.1f} TB"


def download_http(source: str, destination: Path, progress: Optional[Callable[[int, int], None]] = None) -> Path:
    """Download an HTTP resource and atomically rename its .part file."""
    destination.mkdir(parents=True, exist_ok=True)
    output = destination / safe_url_filename(source)
    partial = output.with_name(output.name + ".part")
    request = Request(source, headers={"User-Agent": f"BangDownloader/{__version__}"})
    try:
        with urlopen(request, timeout=30) as response, partial.open("wb") as stream:
            total = int(response.headers.get("Content-Length") or 0)
            downloaded = 0
            while True:
                chunk = response.read(CHUNK_SIZE)
                if not chunk:
                    break
                stream.write(chunk)
                downloaded += len(chunk)
                if progress:
                    progress(downloaded, total)
        partial.replace(output)
    except Exception:
        # Keep a partial file so a failed transfer never replaces a good file.
        raise
    return output
