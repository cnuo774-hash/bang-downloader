"""Backward-compatible source checkout entry point."""

from bang_downloader.cli import main


if __name__ == "__main__":
    raise SystemExit(main())
