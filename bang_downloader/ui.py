"""Local web interface for Bang Downloader."""

from __future__ import annotations

import base64
import binascii
import json
import mimetypes
import os
import re
import subprocess
import tempfile
import threading
import time
import uuid
import webbrowser
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path
from typing import Dict, Optional

from .core import aria2_command, download_http, find_aria2c, resolve_output, source_kind

WEB = Path(__file__).resolve().parent / "web"
MAX_REQUEST_SIZE = 16 * 1024 * 1024
TASKS: Dict[str, dict] = {}
TASKS_LOCK = threading.Lock()


def _now() -> str:
    return time.strftime("%H:%M:%S")


def _update(task_id: str, **values) -> None:
    with TASKS_LOCK:
        TASKS[task_id].update(values, updated_at=_now())


def _http_task(task_id: str, source: str, target: str) -> None:
    try:
        def progress(downloaded: int, total: int) -> None:
            percent = round(downloaded / total * 100, 1) if total else None
            _update(task_id, progress=percent, downloaded=downloaded, total=total)

        output = download_http(source, resolve_output(target), progress)
        _update(task_id, status="completed", progress=100, message=f"已保存到 {output}")
    except Exception as exc:
        _update(task_id, status="error", message=str(exc))


def _torrent_task(task_id: str, source: str, target: str, temporary_file: Optional[str] = None) -> None:
    engine = find_aria2c()
    if not engine:
        _update(task_id, status="waiting", message="需要安装 aria2c 才能下载磁力链接或种子")
        if temporary_file:
            Path(temporary_file).unlink(missing_ok=True)
        return
    try:
        destination = resolve_output(target)
        destination.mkdir(parents=True, exist_ok=True)
        command = aria2_command(temporary_file or source, destination, engine)
        _update(task_id, status="downloading", message="下载引擎已启动")
        process = subprocess.Popen(command, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, text=True)
        for line in process.stdout or []:
            match = re.search(r"(\d+(?:\.\d+)?)%", line)
            if match:
                _update(task_id, progress=float(match.group(1)), message=line.strip())
        code = process.wait()
        _update(task_id, status="completed" if code == 0 else "error",
                progress=100 if code == 0 else TASKS[task_id].get("progress"),
                message="下载完成" if code == 0 else f"aria2c 退出码 {code}")
    except Exception as exc:
        _update(task_id, status="error", message=str(exc))
    finally:
        if temporary_file:
            Path(temporary_file).unlink(missing_ok=True)


class Handler(BaseHTTPRequestHandler):
    server_version = "BangDownloader"

    def log_message(self, *_args) -> None:
        return

    def _json(self, data, status: int = 200) -> None:
        body = json.dumps(data, ensure_ascii=False).encode()
        self.send_response(status)
        self.send_header("Content-Type", "application/json; charset=utf-8")
        self.send_header("Content-Length", str(len(body)))
        self.send_header("X-Content-Type-Options", "nosniff")
        self.end_headers()
        self.wfile.write(body)

    def do_GET(self) -> None:
        if self.path == "/api/status":
            with TASKS_LOCK:
                items = list(TASKS.values())
            self._json({"engine": bool(find_aria2c()), "tasks": items})
            return
        relative = "index.html" if self.path in ("/", "") else self.path.lstrip("/")
        file_path = (WEB / relative).resolve()
        if file_path.is_file() and WEB.resolve() in file_path.parents:
            content = file_path.read_bytes()
            self.send_response(200)
            self.send_header("Content-Type", mimetypes.guess_type(str(file_path))[0] or "application/octet-stream")
            self.send_header("Content-Length", str(len(content)))
            self.send_header("X-Content-Type-Options", "nosniff")
            self.end_headers()
            self.wfile.write(content)
            return
        self.send_error(404)

    def do_POST(self) -> None:
        if self.path != "/api/add":
            self.send_error(404)
            return
        try:
            length = int(self.headers.get("Content-Length", 0))
            if length <= 0 or length > MAX_REQUEST_SIZE:
                self._json({"error": "请求为空或种子文件超过 16 MB"}, 413)
                return
            payload = json.loads(self.rfile.read(length))
            source = str(payload.get("source", "")).strip()
            target = str(resolve_output(str(payload.get("target", "")).strip()))
            file_data = payload.get("file_data")
            file_name = Path(str(payload.get("file_name", ""))).name
            if not source and not file_data:
                self._json({"error": "请提供磁力链接、HTTP 地址或种子文件"}, 400)
                return
            kind = "torrent" if file_data else source_kind(source)
            if kind == "invalid":
                self._json({"error": "无法识别下载来源"}, 400)
                return
            if file_data and not file_name.lower().endswith(".torrent"):
                self._json({"error": "文件必须使用 .torrent 扩展名"}, 400)
                return
            raw = base64.b64decode(file_data, validate=True) if file_data else None
            if file_data and not raw:
                self._json({"error": "种子文件为空"}, 400)
                return
            task_id = uuid.uuid4().hex[:10]
            task = {"id": task_id, "name": file_name or source[:70], "kind": kind, "target": target,
                    "status": "queued", "progress": 0, "message": "已加入队列", "created_at": _now()}
            with TASKS_LOCK:
                TASKS[task_id] = task
            if file_data:
                descriptor, temporary_file = tempfile.mkstemp(prefix="bang-", suffix=".torrent")
                with os.fdopen(descriptor, "wb") as stream:
                    stream.write(raw)
                worker = threading.Thread(target=_torrent_task, args=(task_id, source, target, temporary_file), daemon=True)
            elif kind in ("magnet", "torrent"):
                worker = threading.Thread(target=_torrent_task, args=(task_id, source, target), daemon=True)
            else:
                worker = threading.Thread(target=_http_task, args=(task_id, source, target), daemon=True)
            worker.start()
            self._json(task, 201)
        except (ValueError, TypeError, json.JSONDecodeError, binascii.Error) as exc:
            self._json({"error": str(exc)}, 400)
        except Exception as exc:
            self._json({"error": str(exc)}, 500)


def serve_ui(port: int = 8765, open_browser: bool = True) -> int:
    host = "127.0.0.1"
    url = f"http://{host}:{port}"
    server = ThreadingHTTPServer((host, port), Handler)
    print(f"Bang UI：{url}\n按 Ctrl+C 退出")
    if open_browser:
        threading.Timer(0.4, lambda: webbrowser.open(url)).start()
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        print("\nBang UI 已关闭")
    finally:
        server.server_close()
    return 0
