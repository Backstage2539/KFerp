#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

usage() {
  cat <<'USAGE'
Usage: scripts/verify_kferp.sh <mode>

Modes:
  changed          Run whitespace/conflict-marker checks.
  backend          Run all Go tests under orderapp-remote.
  frontend-tests   Run all Vue shell node --test files.
  frontend-build   Run Vue shell production build.
  frontend         Run frontend-tests and frontend-build.
  all              Run changed, backend, and frontend.
USAGE
}

changed() {
  cd "$ROOT"
  git diff --check
  if rg -n '^(<<<<<<<|=======|>>>>>>>)' . \
    -g '!orderapp-remote/frontend-vue-shell/node_modules/**' \
    -g '!orderapp-remote/tmp/**'; then
    echo "conflict markers found" >&2
    return 1
  fi
}

backend() {
  cd "$ROOT/orderapp-remote"
  go test ./...
}

frontend_tests() {
  cd "$ROOT/orderapp-remote/frontend-vue-shell"
  mapfile -t tests < <(find src -name '*.test.js' -type f | sort)
  if [ "${#tests[@]}" -eq 0 ]; then
    echo "no frontend test files found" >&2
    return 1
  fi
  node --test "${tests[@]}"
}

frontend_build() {
  cd "$ROOT/orderapp-remote/frontend-vue-shell"
  npm run build
}

mode="${1:-}"
case "$mode" in
  changed)
    changed
    ;;
  backend)
    backend
    ;;
  frontend-tests)
    frontend_tests
    ;;
  frontend-build)
    frontend_build
    ;;
  frontend)
    frontend_tests
    frontend_build
    ;;
  all)
    changed
    backend
    frontend_tests
    frontend_build
    ;;
  -h|--help|help|"")
    usage
    ;;
  *)
    usage >&2
    exit 2
    ;;
esac
