# Release Packaging

This document describes the manual release flow for sshkeeper.

## Create a Tag

Use a semantic version tag:

```bash
git status --short
git tag -a v0.2.0 -m "sshkeeper v0.2.0"
git push origin v0.2.0
```

The release script uses `git describe --tags --match 'v*' --always --dirty` by
default. The `--match 'v*'` filter matters: nightly builds move a `nightly` tag
across `main`, and without the filter `git describe` would pick that tag and
stamp binaries `nightly` instead of `v<last release>-<n>-g<sha>`. You can also
pass the version explicitly:

```bash
./release.sh v0.2.0
```

or:

```bash
VERSION=v0.2.0 ./release.sh
```

For reproducible archives, the script uses `SOURCE_DATE_EPOCH`. By default it
uses the timestamp of the latest git commit. To force a specific timestamp:

```bash
SOURCE_DATE_EPOCH=1760000000 ./release.sh v0.2.0
```

## Run Release Checks

Before packaging, run:

```bash
make release-check
```

This runs:

- `go test ./...`
- `go vet ./...`
- native `go build`
- `linux/amd64` cross-build with `CGO_ENABLED=0`
- `linux/arm64` cross-build with `CGO_ENABLED=0`
- `darwin/amd64` cross-build with `CGO_ENABLED=0`
- `darwin/arm64` cross-build with `CGO_ENABLED=0`
- `windows/amd64` cross-build with `CGO_ENABLED=0`

## Build Artifacts

Run:

```bash
./release.sh v0.2.0
```

Expected files in `dist/`:

```text
sshkeeper_v0.2.0_linux_amd64.tar.gz
sshkeeper_v0.2.0_linux_arm64.tar.gz
sshkeeper_v0.2.0_darwin_amd64.tar.gz
sshkeeper_v0.2.0_darwin_arm64.tar.gz
sshkeeper_v0.2.0_windows_amd64.zip
checksums.txt
```

Each archive contains:

- `sshkeeper` or `sshkeeper.exe`
- `README.md`
- `LICENSE`
- `docs/guide.md`

## Verify Checksums

From the `dist/` directory:

```bash
sha256sum -c checksums.txt
```

Expected result: every archive reports `OK`.

## Publish in GitHub Release

Upload these files to the release:

- all five platform archives
- `checksums.txt`

Release notes should mention platform status:

- Linux and macOS are primary release targets.
- macOS builds are available as tar.gz for amd64 and arm64 and require the
  system `ssh` client.
- Windows build is experimental and requires OpenSSH Client available as
  `ssh.exe` in `PATH`.

## Packaging TODO

Prepare these package channels after the first archive-based release:

- deb package
- Arch PKGBUILD / AUR
- rpm later
- Homebrew tap
- Scoop manifest
- Winget later
