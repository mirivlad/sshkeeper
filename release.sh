#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

APP=sshkeeper
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=${VERSION}"
DIST_DIR="dist"

echo "==> Building release ${APP} ${VERSION}..."

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

# Linux amd64
echo "==> linux/amd64..."
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o "${DIST_DIR}/${APP}" .
tar -czf "${DIST_DIR}/${APP}_${VERSION}_linux_amd64.tar.gz" -C "${DIST_DIR}" "${APP}"
rm -f "${DIST_DIR}/${APP}"

# Linux arm64
echo "==> linux/arm64..."
GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -ldflags "${LDFLAGS}" -o "${DIST_DIR}/${APP}" .
tar -czf "${DIST_DIR}/${APP}_${VERSION}_linux_arm64.tar.gz" -C "${DIST_DIR}" "${APP}"
rm -f "${DIST_DIR}/${APP}"

echo "==> Done."
ls -lh "${DIST_DIR}/"*.tar.gz
