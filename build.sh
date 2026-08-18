#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

APP=sshkeeper
# --match 'v*' keeps the rolling `nightly` tag from hijacking the version: a
# plain `git describe --tags` picks whichever tag is nearest, so a nightly build
# would otherwise stamp binaries "nightly" instead of v<last release>-N-g<sha>.
VERSION=$(git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo "dev")
LDFLAGS="-s -w -X main.version=${VERSION}"

echo "==> Building ${APP} ${VERSION}..."
go build -ldflags "${LDFLAGS}" -o bin/${APP} .

echo "==> OK: bin/${APP}"
ls -lh bin/${APP}
