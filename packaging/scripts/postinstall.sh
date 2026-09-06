#!/bin/sh
set -u
umask 077

SYSTEM_BINARY=${SSHKEEPER_SYSTEM_BINARY:-/usr/bin/sshkeeper}
STATE_FILE=${SSHKEEPER_STATE_FILE:-/var/lib/sshkeeper/package-legacy-paths}
PASSWD_FILE=${SSHKEEPER_PASSWD_FILE:-/etc/passwd}

candidate_paths() {
  if [ -n "${SSHKEEPER_LEGACY_PATHS:-}" ]; then
    printf '%s\n' "$SSHKEEPER_LEGACY_PATHS" | while IFS= read -r path; do
      printf 'test\t%s\n' "$path"
    done
    return
  fi

  printf 'root\t%s\n' /usr/local/bin/sshkeeper
  if [ -r "$PASSWD_FILE" ]; then
    awk -F: '$3 == 0 || $3 >= 1000 { if ($6 != "" && $6 != "/") printf "%s\t%s/.local/bin/sshkeeper\n", $1, $6 }' "$PASSWD_FILE"
  fi
}

run_as() {
  owner=$1
  shift
  if [ "${SSHKEEPER_MIGRATION_RUN_AS_CURRENT:-}" = 1 ]; then
    "$@"
  elif [ "$owner" = root ]; then
    "$@"
  elif command -v runuser >/dev/null 2>&1; then
    runuser -u "$owner" -- "$@"
  else
    return 127
  fi
}

next_backup() {
  path=$1
  base="${path}.legacy-backup"
  if [ ! -e "$base" ] && [ ! -L "$base" ]; then
    printf '%s\n' "$base"
    return
  fi

  n=1
  while [ -e "${base}.${n}" ] || [ -L "${base}.${n}" ]; do
    n=$((n + 1))
  done
  printf '%s\n' "${base}.${n}"
}

record_migration() {
  owner=$1
  path=$2
  backup=$3
  state_dir=$(dirname "$STATE_FILE")
  if mkdir -p "$state_dir" 2>/dev/null; then
    chmod 700 "$state_dir" 2>/dev/null || true
    printf '%s\t%s\t%s\n' "$owner" "$path" "$backup" >> "$STATE_FILE"
    chmod 600 "$STATE_FILE" 2>/dev/null || true
  else
    printf 'sshkeeper: warning: cannot create migration state directory %s\n' "$state_dir" >&2
  fi
}

migrate_one() {
  owner=$1
  path=$2
  [ "$path" = "$SYSTEM_BINARY" ] && return 0
  [ -e "$path" ] || [ -L "$path" ] || return 0

  if [ -L "$path" ] && [ "$(readlink "$path" 2>/dev/null || true)" = "$SYSTEM_BINARY" ]; then
    return 0
  fi

  backup=$(next_backup "$path")
  if ! run_as "$owner" mv -- "$path" "$backup" 2>/dev/null; then
    printf 'sshkeeper: warning: cannot disable legacy binary %s as user %s\n' "$path" "$owner" >&2
    return 0
  fi

  if ! run_as "$owner" ln -s "$SYSTEM_BINARY" "$path" 2>/dev/null; then
    run_as "$owner" mv -- "$backup" "$path" 2>/dev/null || true
    printf 'sshkeeper: warning: cannot redirect legacy path %s to %s\n' "$path" "$SYSTEM_BINARY" >&2
    return 0
  fi

  record_migration "$owner" "$path" "$backup"
  printf 'sshkeeper: migrated legacy binary: %s -> %s (backup: %s)\n' "$path" "$SYSTEM_BINARY" "$backup"
}

tab=$(printf '\t')
candidate_paths | while IFS="$tab" read -r owner path; do
  [ -n "$owner" ] && [ -n "$path" ] && migrate_one "$owner" "$path"
done

exit 0
