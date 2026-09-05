#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

APP=sshkeeper
VERSION=${1:-${VERSION:-}}
NFPM_BIN=${NFPM_BIN:-nfpm}
NFPM_RELEASE=${NFPM_RELEASE:-1}

if [[ -z "$VERSION" ]]; then
  echo "usage: $0 <version>" >&2
  exit 2
fi
if ! command -v "$NFPM_BIN" >/dev/null 2>&1; then
  echo "nfpm is required to build .deb/.rpm packages" >&2
  exit 1
fi

PKG_VERSION=${VERSION#v}
if [[ -z "${SOURCE_DATE_EPOCH:-}" ]]; then
  if git rev-parse --verify -q "${VERSION}^{commit}" >/dev/null; then
    SOURCE_DATE_EPOCH=$(git log -1 --format=%ct "$VERSION")
  else
    SOURCE_DATE_EPOCH=$(git log -1 --format=%ct 2>/dev/null || date +%s)
  fi
fi
export SOURCE_DATE_EPOCH
TMP_DIR=$(mktemp -d)
trap 'rm -rf "$TMP_DIR"' EXIT

build_one() {
  local goarch="$1"
  local rpmarch
  local tarball="dist/${APP}_${VERSION}_linux_${goarch}.tar.gz"
  local package_root="${TMP_DIR}/${APP}_${VERSION}_linux_${goarch}"
  local extracted="${package_root}/${APP}"

  case "$goarch" in
    amd64) rpmarch=x86_64 ;;
    arm64) rpmarch=aarch64 ;;
    *) echo "unsupported package arch: $goarch" >&2; return 1 ;;
  esac
  if [[ ! -f "$tarball" ]]; then
    echo "missing Linux release archive: $tarball" >&2
    return 1
  fi

  tar -xzf "$tarball" -C "$TMP_DIR"
  if [[ ! -x "$extracted" ]]; then
    echo "missing binary in $tarball" >&2
    return 1
  fi

  export NFPM_ARCH="$goarch"
  export NFPM_VERSION="$PKG_VERSION"
  export NFPM_RELEASE
  export NFPM_BINARY="$extracted"
  export NFPM_README="${package_root}/README.md"
  export NFPM_LICENSE="${package_root}/LICENSE"
  export NFPM_GUIDE="${package_root}/docs/guide.md"

  "$NFPM_BIN" package --config packaging/nfpm.yaml --packager deb \
    --target "dist/${APP}_${PKG_VERSION}-${NFPM_RELEASE}_${goarch}.deb"
  "$NFPM_BIN" package --config packaging/nfpm.yaml --packager rpm \
    --target "dist/${APP}-${PKG_VERSION}-${NFPM_RELEASE}.${rpmarch}.rpm"
}

build_one amd64
build_one arm64

echo "==> Linux packages:"
ls -lh dist/*.deb dist/*.rpm
