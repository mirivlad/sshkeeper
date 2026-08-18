# Release Packaging

Releases are published by GitHub Actions. Pushing a `v*` tag is the whole
release procedure; the rest of this document describes what that automation
runs, and how to reproduce it by hand when needed.

## Workflows

| Workflow | Trigger | Result |
|----------|---------|--------|
| `ci.yml` | push to `main`, every pull request | `gofmt`, `go vet`, `go test` on Linux and macOS, plus a cross-build of all five release targets |
| `release.yml` | push of a `v*` tag | runs `make release-check`, then `release.sh`, then publishes the GitHub release |
| `nightly.yml` | push to `main` | rebuilds the tip of `main` and replaces the `nightly` prerelease |

`release.yml` builds through `release.sh` rather than reimplementing packaging,
so a local run produces byte-identical archives.

### Release notes

`release.yml` looks for `docs/releases/<tag>.md`. If that file exists it becomes
the release body; otherwise GitHub generates notes from commit history. Write
the file before pushing the tag when a release deserves a real description.

### The nightly prerelease

`nightly.yml` force-moves a rolling `nightly` tag to the tip of `main` and
republishes a prerelease from it. It is marked prerelease deliberately, so
GitHub's `Latest` badge stays on the newest `v*` release.

Because that tag moves, version discovery in `build.sh` and `release.sh` is
pinned with `--match 'v*'`. Without the filter `git describe` would select
`nightly` and stamp binaries with it instead of `v<last release>-<n>-g<sha>`.
Keep the filter if you touch those scripts.

## Create a Tag

Use a semantic version tag. Pushing it is what triggers `release.yml`:

```bash
git status --short
git tag -a v0.2.0 -m "sshkeeper v0.2.0"
git push origin v0.2.0
```

The remaining sections describe the manual equivalent, which is still the way
to test packaging locally or to recover if Actions is unavailable.

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

`release.yml` does this automatically on tag push. To publish by hand, upload:

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
