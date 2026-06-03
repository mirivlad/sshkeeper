#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

APP=sshkeeper
VERSION=$(git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=${VERSION}"

echo "==> Building ${APP} ${VERSION}..."
go build -ldflags "${LDFLAGS}" -o bin/${APP} .

echo "==> OK: bin/${APP}"
ls -lh bin/${APP}
