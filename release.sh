#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

APP=sshkeeper
# --match 'v*' ignores the rolling `nightly` tag; see build.sh for the details.
VERSION=${VERSION:-${1:-$(git describe --tags --match 'v*' --always --dirty 2>/dev/null || echo "dev")}}
LDFLAGS="-s -w -X main.version=${VERSION}"
DIST_DIR="dist"
SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH:-$(git log -1 --format=%ct 2>/dev/null || date +%s)}

echo "==> Building release ${APP} ${VERSION}..."
echo "==> SOURCE_DATE_EPOCH=${SOURCE_DATE_EPOCH}"

rm -rf "${DIST_DIR}"
mkdir -p "${DIST_DIR}"

package_docs() {
	local package_dir="$1"

	cp README.md LICENSE "${package_dir}/"
	if [[ -f docs/guide.md ]]; then
		mkdir -p "${package_dir}/docs"
		cp docs/guide.md "${package_dir}/docs/"
	fi
}

normalize_package() {
	local package_dir="$1"
	local binary="$2"

	# Permissions must not depend on the builder's umask. Without this, a host
	# with umask 002 packages 664/775 while one with umask 022 packages
	# 644/755, and the archives differ even though every file inside is
	# byte-identical.
	find "${package_dir}" -type d -exec chmod 755 {} +
	find "${package_dir}" -type f -exec chmod 644 {} +
	chmod 755 "${package_dir}/${binary}"

	find "${package_dir}" -exec touch -h -d "@${SOURCE_DATE_EPOCH}" {} +
}

build_tarball() {
	local goos="$1"
	local goarch="$2"
	local package_dir="${DIST_DIR}/${APP}_${VERSION}_${goos}_${goarch}"
	local archive="${DIST_DIR}/${APP}_${VERSION}_${goos}_${goarch}.tar.gz"

	echo "==> ${goos}/${goarch}..."
	rm -rf "${package_dir}"
	mkdir -p "${package_dir}"

	GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" -o "${package_dir}/${APP}" .
	package_docs "${package_dir}"
	normalize_package "${package_dir}" "${APP}"
	tar --sort=name --owner=0 --group=0 --numeric-owner --mtime="@${SOURCE_DATE_EPOCH}" -cf - -C "${DIST_DIR}" "$(basename "${package_dir}")" | gzip -n > "${archive}"
	rm -rf "${package_dir}"
}

build_zip() {
	local goos="$1"
	local goarch="$2"
	local package_dir="${DIST_DIR}/${APP}_${VERSION}_${goos}_${goarch}"
	local archive="${DIST_DIR}/${APP}_${VERSION}_${goos}_${goarch}.zip"

	echo "==> ${goos}/${goarch}..."
	rm -rf "${package_dir}"
	mkdir -p "${package_dir}"

	GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags "${LDFLAGS}" -o "${package_dir}/${APP}.exe" .
	package_docs "${package_dir}"
	normalize_package "${package_dir}" "${APP}.exe"
	(cd "${DIST_DIR}" && find "$(basename "${package_dir}")" -print | sort | zip -X -q "$(basename "${archive}")" -@)
	rm -rf "${package_dir}"
}

build_tarball linux amd64
build_tarball linux arm64
build_tarball darwin amd64
build_tarball darwin arm64
build_zip windows amd64

(cd "${DIST_DIR}" && sha256sum *.tar.gz *.zip > checksums.txt)

echo "==> Done."
ls -lh "${DIST_DIR}/"*.tar.gz "${DIST_DIR}/"*.zip "${DIST_DIR}/checksums.txt"
