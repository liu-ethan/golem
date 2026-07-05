#!/usr/bin/env bash
# build-release.sh 交叉编译多平台 golem 二进制并生成 SHA256SUMS。
set -euo pipefail

VERSION="${1:-}"
OUT_DIR="${2:-dist}"

if [[ -z "$VERSION" ]]; then
	echo "usage: $0 <version> [out-dir]" >&2
	echo "example: $0 v0.1.0 dist" >&2
	exit 1
fi

mkdir -p "$OUT_DIR"

LDFLAGS="-s -w -X main.version=${VERSION}"

build() {
	local goos=$1 goarch=$2 ext=$3
	local platform=$goos
	[[ "$goos" == darwin ]] && platform=macos
	local name="golem-${platform}-${goarch}"
	echo "building ${name}${ext}..."
	CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" \
		go build -ldflags "$LDFLAGS" -o "${OUT_DIR}/${name}${ext}" ./cmd/golem
}

build linux amd64 ""
build windows amd64 ".exe"
build darwin amd64 ""
build darwin arm64 ""

cd "$OUT_DIR"
rm -f SHA256SUMS
if command -v sha256sum >/dev/null 2>&1; then
	sha256sum golem-* > SHA256SUMS
elif command -v shasum >/dev/null 2>&1; then
	shasum -a 256 golem-* > SHA256SUMS
else
	echo "warning: no sha256sum or shasum found, skipping SHA256SUMS" >&2
fi

echo "release artifacts in ${OUT_DIR}/"
