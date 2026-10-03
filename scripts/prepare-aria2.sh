#!/usr/bin/env bash
set -euo pipefail

version="1.37.0"
root="$(cd "$(dirname "$0")/.." && pwd)"
asset_dir="$root/internal/engine/binaries"
temp_dir="$(mktemp -d "${TMPDIR:-/tmp}/bang-aria2.XXXXXX")"
trap 'rm -rf "$temp_dir"' EXIT

download_with_fallback() {
  local output="$1"
  shift
  local url
  for url in "$@"; do
    echo "Downloading aria2 from $url"
    rm -f "$output"
    if curl --http1.1 \
      --fail --location --show-error \
      --connect-timeout 15 --max-time 180 \
      --retry 2 --retry-delay 2 --retry-all-errors \
      "$url" -o "$output"; then
      return 0
    fi
    echo "Download source failed or timed out; trying the next source." >&2
  done
  echo "All aria2 download sources failed." >&2
  return 1
}

verify_sha256() {
  local expected="$1"
  local file="$2"
  local actual
  actual="$(shasum -a 256 "$file" | awk '{print $1}')"
  if [[ "$actual" != "$expected" ]]; then
    echo "Checksum mismatch for $file" >&2
    echo "Expected: $expected" >&2
    echo "Actual:   $actual" >&2
    return 1
  fi
}

case "$(uname -s):$(uname -m)" in
  Linux:x86_64)
    target="$asset_dir/linux-amd64-aria2c"
    tls_options=(--without-gnutls --with-openssl)
    jobs="$(getconf _NPROCESSORS_ONLN)"
    ;;
  Darwin:arm64|Darwin:x86_64)
    if [[ "$(uname -m)" == "arm64" ]]; then target="$asset_dir/darwin-arm64-aria2c"; else target="$asset_dir/darwin-amd64-aria2c"; fi
    tls_options=(--with-appletls --without-gnutls --without-openssl)
    jobs="$(sysctl -n hw.ncpu)"
    ;;
  *)
    echo "Unsupported Unix build host: $(uname -s) $(uname -m)" >&2
    exit 1
    ;;
esac

archive="aria2-${version}.tar.xz"
source_archive="$temp_dir/aria2.tar.xz"
download_with_fallback "$source_archive" \
  "https://github.com/aria2/aria2/releases/download/release-${version}/${archive}" \
  "https://sourceforge.net/projects/aria2/files/stable/aria2-${version}/${archive}/download"
verify_sha256 "60a420ad7085eb616cb6e2bdf0a7206d68ff3d37fb5a956dc44242eb2f79b66b" "$source_archive"
tar -xJf "$source_archive" -C "$temp_dir"
source_dir="$temp_dir/aria2-${version}"
(cd "$source_dir" && ./configure \
  "${tls_options[@]}" \
  --without-libnettle --without-libgcrypt --without-libcares \
  --without-libxml2 --without-libssh2 --without-sqlite3 && make -j"$jobs")
binary="$source_dir/src/aria2c"
mkdir -p "$root/build/third-party"
cp "$source_archive" "$root/build/third-party/aria2-${version}-source.tar.xz"
cp "$source_dir/COPYING" "$root/build/third-party/ARIA2-COPYING"

test -x "$binary"
install -m 0700 "$binary" "$target"
"$target" --version | head -n 1 | grep -F "aria2 version ${version}"
if [[ "$(uname -s)" == "Darwin" ]] && otool -L "$target" | grep -Eq '/opt/homebrew|/usr/local'; then
  echo "aria2c unexpectedly depends on package-manager libraries" >&2
  exit 1
fi
echo "Embedded $target"
