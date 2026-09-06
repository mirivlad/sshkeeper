#!/bin/sh
set -u
umask 077

SYSTEM_BINARY=${SSHKEEPER_SYSTEM_BINARY:-/usr/bin/sshkeeper}
STATE_FILE=${SSHKEEPER_STATE_FILE:-/var/lib/sshkeeper/package-legacy-paths}
action=${1:-remove}

case "$action" in
  1|upgrade|failed-upgrade|abort-install|abort-upgrade|disappear)
    exit 0
    ;;
  0|remove|purge)
    ;;
  *)
    exit 0
    ;;
esac

[ -f "$STATE_FILE" ] || exit 0

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

tab=$(printf '\t')
tac "$STATE_FILE" 2>/dev/null | while IFS="$tab" read -r owner path backup; do
  [ -n "$owner" ] || continue
  [ -n "$path" ] || continue
  [ -n "$backup" ] || continue

  managed=false
  if [ -L "$path" ] && [ "$(readlink "$path" 2>/dev/null || true)" = "$SYSTEM_BINARY" ]; then
    managed=true
    run_as "$owner" rm -f -- "$path" 2>/dev/null || managed=false
  elif [ ! -e "$path" ] && [ ! -L "$path" ]; then
    managed=true
  fi

  if [ "$managed" = true ] && { [ -e "$backup" ] || [ -L "$backup" ]; }; then
    if run_as "$owner" mv -- "$backup" "$path" 2>/dev/null; then
      printf 'sshkeeper: restored legacy binary: %s\n' "$path"
    else
      printf 'sshkeeper: warning: could not restore %s from %s\n' "$path" "$backup" >&2
    fi
  elif [ -e "$backup" ] || [ -L "$backup" ]; then
    printf 'sshkeeper: warning: %s changed while package was installed; legacy backup kept at %s\n' "$path" "$backup" >&2
  fi
done

rm -f -- "$STATE_FILE" 2>/dev/null || true
rmdir -- "$(dirname "$STATE_FILE")" 2>/dev/null || true
exit 0
