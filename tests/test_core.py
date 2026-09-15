import io
import os
import tempfile
import unittest
from pathlib import Path
from unittest.mock import patch

from bang_downloader.core import (
    aria2_command,
    download_http,
    format_bytes,
    resolve_output,
    resolve_source,
    safe_url_filename,
    source_kind,
)


class FakeResponse(io.BytesIO):
    headers = {"Content-Length": "5"}

    def __enter__(self):
        return self

    def __exit__(self, *_args):
        self.close()


class CoreTests(unittest.TestCase):
    def test_relative_and_absolute_output_paths(self):
        with tempfile.TemporaryDirectory() as directory:
            previous = Path.cwd()
            try:
                os.chdir(directory)
                self.assertEqual(resolve_output("files"), Path(directory, "files").resolve())
                self.assertEqual(resolve_output("/tmp/bang"), Path("/tmp/bang").resolve())
            finally:
                os.chdir(previous)

    def test_source_detection(self):
        with tempfile.TemporaryDirectory() as directory:
            torrent = Path(directory, "sample.torrent")
            torrent.write_bytes(b"torrent")
            regular = Path(directory, "sample.txt")
            regular.write_text("text")
            self.assertEqual(source_kind("magnet:?xt=urn:btih:test"), "magnet")
            self.assertEqual(source_kind("https://example.com/file.torrent"), "torrent")
            self.assertEqual(source_kind("https://example.com/file.zip"), "url")
            self.assertEqual(source_kind(str(torrent)), "torrent")
            self.assertEqual(source_kind(str(regular)), "invalid")

    def test_source_path_is_resolved_from_working_directory(self):
        with tempfile.TemporaryDirectory() as directory:
            previous = Path.cwd()
            try:
                os.chdir(directory)
                self.assertEqual(resolve_source("a.torrent"), str(Path(directory, "a.torrent").resolve()))
            finally:
                os.chdir(previous)

    def test_aria2_command_has_resume_and_no_seeding(self):
        command = aria2_command("demo.torrent", Path("/tmp/downloads"), "/usr/bin/aria2c")
        self.assertEqual(command[0], "/usr/bin/aria2c")
        self.assertIn("--continue=true", command)
        self.assertIn("--seed-time=0", command)
        self.assertEqual(command[-1], "demo.torrent")

    def test_http_download_uses_partial_then_final_file(self):
        progress = []
        with tempfile.TemporaryDirectory() as directory, patch(
            "bang_downloader.core.urlopen", return_value=FakeResponse(b"hello")
        ):
            output = download_http(
                "https://example.com/hello%20world.txt",
                Path(directory),
                lambda done, total: progress.append((done, total)),
            )
            self.assertEqual(output.name, "hello world.txt")
            self.assertEqual(output.read_bytes(), b"hello")
            self.assertFalse(Path(str(output) + ".part").exists())
            self.assertEqual(progress, [(5, 5)])

    def test_helpers(self):
        self.assertEqual(safe_url_filename("https://example.com/"), "download")
        self.assertEqual(format_bytes(1536), "1.5 KB")


if __name__ == "__main__":
    unittest.main()
