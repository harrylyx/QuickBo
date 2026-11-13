#!/usr/bin/env bash
# Put libmpv-2.dll into mygo's Windows resource directories. mygo copies
# resources/windows-<arch>/ next to the executable, where
# internal/player/mpvlib/paths.go already looks for it, so no code change
# is needed to make the Windows installers play inside the app window.
#
# The mpv-dev builds and their SHA-256 sums are pinned so releases are
# reproducible. To upgrade, change TAG, the asset names and the sums.
set -euo pipefail

TAG=20261111
BASE="https://github.com/shinchiro/mpv-winbuild-cmake/releases/download/${TAG}"
X64_ASSET="mpv-dev-x86_64-20261111-git-bcde6b93cc.7z"
X64_SHA256="273c079b11ea5575822f972756f4c9a71149a5c10130353abeb8586738938452"
ARM_ASSET="mpv-dev-aarch64-20261111-git-bcde6b93cc.7z"
ARM_SHA256="a42c1cebf7366a957c7c321a20df414b80d4a55bbc65ded032d78b749c8d6445"

root=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

sum256() {
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum "$1" | cut -d' ' -f1
  elif command -v shasum >/dev/null 2>&1; then
    shasum -a 256 "$1" | cut -d' ' -f1
  else
    echo "需要 sha256sum 或 shasum" >&2
    exit 1
  fi
}

unpack() { # archive, destination
  local archive=$1 dir=$2
  if command -v 7z >/dev/null 2>&1; then
    7z x -y -o"$dir" "$archive" >/dev/null
  elif command -v 7zz >/dev/null 2>&1; then
    7zz x -y -o"$dir" "$archive" >/dev/null
  elif tar -tf "$archive" >/dev/null 2>&1; then # macOS bsdtar reads 7z
    tar -xf "$archive" -C "$dir"
  else
    echo "解包需要 7z / 7zz（Linux: apt install p7zip-full）或 bsdtar" >&2
    exit 1
  fi
}

bundle() { # asset, sha256, arch
  local asset=$1 sum=$2 arch=$3
  local archive="$work/$asset" dir="$work/$arch"
  echo "下载 $asset"
  curl -fsSL --retry 3 -o "$archive" "$BASE/$asset"
  local got
  got=$(sum256 "$archive")
  if [ "$got" != "$sum" ]; then
    echo "SHA-256 不符：$asset 期望 $sum，实际 $got" >&2
    exit 1
  fi
  mkdir -p "$dir"
  unpack "$archive" "$dir"
  local dll
  dll=$(find "$dir" -type f -name 'libmpv-2.dll' | head -n 1 || true)
  if [ -z "$dll" ] || [ "$(head -c 2 "$dll")" != "MZ" ]; then
    echo "$asset 里没有有效的 libmpv-2.dll" >&2
    exit 1
  fi
  local out="$root/resources/windows-$arch"
  mkdir -p "$out"
  cp "$dll" "$out/libmpv-2.dll"
  cp "$root/scripts/libmpv-NOTICE.txt" "$out/libmpv-NOTICE.txt"
  echo "已写入 $out/libmpv-2.dll ($(wc -c < "$out/libmpv-2.dll" | tr -d ' ') 字节)"
}

bundle "$X64_ASSET" "$X64_SHA256" amd64
bundle "$ARM_ASSET" "$ARM_SHA256" arm64
