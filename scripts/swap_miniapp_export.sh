#!/usr/bin/env bash
set -euo pipefail

TARGET_DIR="${1:-}"
INCOMING_DIR="${2:-}"
BACKUP_DIR="${3:-}"

if [ -z "$TARGET_DIR" ] || [ -z "$INCOMING_DIR" ]; then
  echo "Usage: swap_miniapp_export.sh <target-dir> <incoming-dir> [backup-dir]" >&2
  exit 2
fi
case "$TARGET_DIR" in
  /|. ) echo "ERROR: refusing unsafe miniapp target: $TARGET_DIR" >&2; exit 1 ;;
esac
if [ ! -d "$INCOMING_DIR" ]; then
  echo "ERROR: incoming miniapp artifact does not exist: $INCOMING_DIR" >&2
  exit 1
fi
if [ -e "$TARGET_DIR" ]; then
  if [ ! -d "$TARGET_DIR" ] || [ -L "$TARGET_DIR" ]; then
    echo "ERROR: fixed miniapp target must be a real directory: $TARGET_DIR" >&2
    exit 1
  fi
  if [ -z "$BACKUP_DIR" ]; then
    echo "ERROR: backup directory is required when the fixed target exists" >&2
    exit 1
  fi
  if [ -e "$BACKUP_DIR" ]; then
    echo "ERROR: miniapp backup destination already exists: $BACKUP_DIR" >&2
    exit 1
  fi
fi

old_target_moved=0
restore_on_failure() {
  local status=$?
  trap - EXIT INT TERM HUP
  if [ "$status" -ne 0 ] && [ "$old_target_moved" -eq 1 ] && [ ! -e "$TARGET_DIR" ] && [ -e "$BACKUP_DIR" ]; then
    if mv "$BACKUP_DIR" "$TARGET_DIR"; then
      echo "Restored the previous fixed miniapp artifact after a failed swap." >&2
    else
      echo "ERROR: failed to restore previous fixed miniapp artifact: $BACKUP_DIR" >&2
    fi
  fi
  exit "$status"
}
trap restore_on_failure EXIT
trap 'exit 130' INT
trap 'exit 143' TERM
trap 'exit 129' HUP

if [ -e "$TARGET_DIR" ]; then
  old_target_moved=1
  mv "$TARGET_DIR" "$BACKUP_DIR"
fi
mv "$INCOMING_DIR" "$TARGET_DIR"
old_target_moved=0

trap - EXIT INT TERM HUP
