#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/../.."
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT

system="$tmp/usr/bin/sshkeeper"
legacy_user="$tmp/home/test/.local/bin/sshkeeper"
legacy_local="$tmp/usr/local/bin/sshkeeper"
state="$tmp/var/lib/sshkeeper/package-legacy-paths"
mkdir -p "$(dirname "$system")" "$(dirname "$legacy_user")" "$(dirname "$legacy_local")"
printf 'packaged\n' > "$system"
printf 'old-user\n' > "$legacy_user"
printf 'old-local\n' > "$legacy_local"
chmod +x "$system" "$legacy_user" "$legacy_local"

# Discover ~/.local/bin/sshkeeper through passwd exactly as a real package install does.
passwd_file="$tmp/passwd"
printf 'test:x:1000:1000:test:%s:/bin/bash\n' "$tmp/home/test" > "$passwd_file"
env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" SSHKEEPER_PASSWD_FILE="$passwd_file" \
  packaging/scripts/postinstall.sh configure
test -L "$legacy_user"
test "$(readlink "$legacy_user")" = "$system"
test -f "${legacy_user}.legacy-backup"
env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" \
  packaging/scripts/postremove.sh remove
test "$(cat "$legacy_user")" = old-user

paths=$(printf '%s\n%s' "$legacy_user" "$legacy_local")
env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" SSHKEEPER_LEGACY_PATHS="$paths" \
  packaging/scripts/postinstall.sh configure

for path in "$legacy_user" "$legacy_local"; do
  test -L "$path"
  test "$(readlink "$path")" = "$system"
  test -f "${path}.legacy-backup"
done
test "$(wc -l < "$state")" -eq 2
test "$(stat -c %a "$state")" = 600

# Re-running postinstall on upgrade is idempotent.
env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" SSHKEEPER_LEGACY_PATHS="$paths" \
  packaging/scripts/postinstall.sh configure 0.4.0
test "$(wc -l < "$state")" -eq 2

env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" \
  packaging/scripts/postremove.sh upgrade
for path in "$legacy_user" "$legacy_local"; do test -L "$path"; done

env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" \
  packaging/scripts/postremove.sh remove

test ! -e "$state"
test ! -L "$legacy_user"
test ! -L "$legacy_local"
test "$(cat "$legacy_user")" = old-user
test "$(cat "$legacy_local")" = old-local

# If a user replaces the package-managed redirect, uninstall must not overwrite it.
printf 'old-again\n' > "$legacy_user"
env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" SSHKEEPER_LEGACY_PATHS="$legacy_user" \
  packaging/scripts/postinstall.sh configure
rm -f "$legacy_user"
printf 'user-replacement\n' > "$legacy_user"
env SSHKEEPER_MIGRATION_RUN_AS_CURRENT=1 SSHKEEPER_SYSTEM_BINARY="$system" SSHKEEPER_STATE_FILE="$state" \
  packaging/scripts/postremove.sh remove
test "$(cat "$legacy_user")" = user-replacement
test "$(cat "${legacy_user}.legacy-backup")" = old-again

echo "legacy migration tests: OK"
