import io
import tempfile
import unittest
from contextlib import redirect_stderr, redirect_stdout
from pathlib import Path
from unittest.mock import patch

from bang_downloader.cli import build_parser, main, run_download


class CliTests(unittest.TestCase):
    def test_parser_accepts_download_and_ui_modes(self):
        download = build_parser().parse_args(["demo.torrent", "-o", "files"])
        self.assertEqual(download.source, "demo.torrent")
        self.assertEqual(download.output, "files")
        ui = build_parser().parse_args(["--ui", "--port", "9000", "--no-browser"])
        self.assertTrue(ui.ui)
        self.assertEqual(ui.port, 9000)
        self.assertTrue(ui.no_browser)

    def test_missing_source_prints_help(self):
        output = io.StringIO()
        with redirect_stdout(output):
            code = main([])
        self.assertEqual(code, 2)
        self.assertIn("usage: bang", output.getvalue())

    def test_missing_torrent_is_rejected_without_creating_output(self):
        with tempfile.TemporaryDirectory() as directory:
            output = Path(directory, "output")
            errors = io.StringIO()
            with redirect_stderr(errors):
                code = run_download("missing.torrent", str(output))
            self.assertEqual(code, 2)
            self.assertFalse(output.exists())

    def test_torrent_reports_missing_aria2(self):
        with tempfile.TemporaryDirectory() as directory:
            torrent = Path(directory, "sample.torrent")
            torrent.write_bytes(b"torrent")
            with patch("bang_downloader.cli.find_aria2c", return_value=None), redirect_stdout(io.StringIO()), redirect_stderr(io.StringIO()):
                code = run_download(str(torrent), str(Path(directory, "output")))
            self.assertEqual(code, 127)


if __name__ == "__main__":
    unittest.main()
