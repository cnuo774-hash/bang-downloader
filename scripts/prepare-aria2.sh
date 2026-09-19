#!/usr/bin/env bash
set -euo pipefail

version="1.37.0"
root="$(cd "$(dirname "$0")/.." && pwd)"
asset_dir="$root/internal/engine/binaries"
temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/bang-aria2.XXXXXX")"
trap 'rm -rf "$temp_dir"' EXIT

case "$(uname -s):$(uname -m)" in
  Linux:x86_64)
    archive="aria2-x86_64-linux-musl_static.zip"
    url="https://github.com/abcfy2/aria2-static-build/releases/download/${version}/${archive}"
    curl -fL --retry 3 "$url" -o "$temp_dir/aria2.zip"
    unzip -q "$temp_dir/aria2.zip" -d "$temp_dir/unpacked"
    binary="$(find "$temp_dir/unpacked" -type f -name aria2c -print -quit)"
    target="$asset_dir/linux-amd64-aria2c"
    ;;
  Darwin:arm64|Darwin:x86_64)
    archive="aria2-${version}.tar.xz"
    url="https://github.com/aria2/aria2/releases/download/release-${version}/${archive}"
    curl -fL --retry 3 "$url" -o "$temp_dir/aria2.tar.xz"
    echo "60a420ad7085eb616cb6e2bdf0a7206d68ff3d37fb5a956dc44242eb2f79b66b  $temp_dir/aria2.tar.xz" | shasum -a 256 -c -
    tar -xJf "$temp_dir/aria2.tar.xz" -C "$temp_dir"
    source_dir="$temp_dir/aria2-${version}"
    (cd "$source_dir" && ./configure \
      --with-appletls \
      --without-gnutls \
      --without-openssl \
      --without-libnettle \
      --without-libgcrypt \
      --without-libcares \
      --without-libxml2 \
      --without-libssh2 \
      --without-sqlite3 && make -j"$(sysctl -n hw.ncpu)")
    binary="$source_dir/src/aria2c"
    if [[ "$(uname -m)" == "arm64" ]]; then target="$asset_dir/darwin-arm64-aria2c"; else target="$asset_dir/darwin-amd64-aria2c"; fi
    ;;
  *)
    echo "Unsupported Unix build host: $(uname -s) $(uname -m)" >&2
    exit 1
    ;;
esac

test -x "$binary"
install -m 0700 "$binary" "$target"
"$target" --version | head -n 1 | grep -F "aria2 version ${version}"
if [[ "$(uname -s)" == "Darwin" ]] && otool -L "$target" | grep -Eq '/opt/homebrew|/usr/local'; then
  echo "aria2c unexpectedly depends on package-manager libraries" >&2
  exit 1
fi
echo "Embedded $target"
