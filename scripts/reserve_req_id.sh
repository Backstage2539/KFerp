#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
ACTIVE_FILE="$ROOT/ACTIVE_REQUIREMENTS.md"

usage() {
  cat <<'USAGE'
Usage:
  scripts/reserve_req_id.sh
  scripts/reserve_req_id.sh --claim short-slug

Prints the next PR id found from requirement and acceptance docs. With --claim,
adds a placeholder entry to ACTIVE_REQUIREMENTS.md so parallel Codex sessions can
see the intended id before broad edits begin.
USAGE
}

next_id() {
  cd "$ROOT"
  local max_id
  max_id="$(
    {
      rg --no-filename --only-matching 'PR-[0-9]+' REQUIREMENTS.md ACCEPTANCE_TESTS.md orderapp-remote/docs/REQUIREMENTS.md orderapp-remote/docs/ACCEPTANCE_TESTS.md docs orderapp-remote/docs 2>/dev/null || true
    } | awk '/^PR-[0-9][0-9][0-9]$/ { print substr($0, 4) }' | sort -n | tail -1
  )"
  if [ -z "$max_id" ]; then
    echo "PR-001"
  else
    printf 'PR-%03d\n' "$((10#$max_id + 1))"
  fi
}

slug=""
case "${1:-}" in
  "")
    next_id
    ;;
  --claim)
    slug="${2:-}"
    if [ -z "$slug" ]; then
      usage >&2
      exit 2
    fi
    id="$(next_id)"
    if rg -q "^### ${id}-" "$ACTIVE_FILE" 2>/dev/null; then
      echo "${id} is already present in ACTIVE_REQUIREMENTS.md" >&2
      exit 1
    fi
    tmp="$(mktemp)"
    awk -v entry="### ${id}-${slug}
- Branch:
- Owner/session:
- Status: planned
- Scope:
- Verifier:
  - Unit:
  - API:
  - Frontend/build:
  - Manual:
  - Review/acceptance:
- Deployment:
- Last update:
- Notes:
" '
      BEGIN { inserted = 0 }
      /^None\.$/ && inserted == 0 { print entry; inserted = 1; next }
      { print }
      END {
        if (inserted == 0) {
          print ""
          print entry
        }
      }
    ' "$ACTIVE_FILE" > "$tmp"
    mv "$tmp" "$ACTIVE_FILE"
    echo "$id"
    ;;
  -h|--help|help)
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
