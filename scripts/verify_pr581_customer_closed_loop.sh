#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT"
git diff --check

cd "$ROOT/orderapp-remote"
go test \
  ./internal/application/customer \
  ./internal/application/customerfulfillment \
  ./internal/application/customerportal \
  ./internal/application/production \
  ./internal/application/sales \
  ./internal/infrastructure/postgres/customerfulfillment \
  ./internal/infrastructure/postgres/customerportal \
  ./internal/infrastructure/postgres/production \
  ./internal/infrastructure/postgres/sales \
  ./internal/interfaces/http/customerfulfillment \
  ./internal/interfaces/http/customerportal \
  ./internal/interfaces/http/sales \
  ./internal/interfaces/http/support

cd "$ROOT/orderapp-remote/frontend-vue-shell"
vue_tests=()
while IFS= read -r test_file; do
  vue_tests+=("$test_file")
done < <(find src -name '*.test.js' -type f | sort)
node --test "${vue_tests[@]}"
npm run build

cd "$ROOT/miniapp"
npm test -- --run
npm run typecheck
npm run build:mp-weixin:development

artifact="$ROOT/miniapp/dist/build/mp-weixin"
node "$ROOT/scripts/verify_mp_weixin_artifact.mjs" "$artifact" "$artifact/PAGE_FILE_MANIFEST"
"$ROOT/scripts/verify_mp_weixin_manifest.sh" "$artifact"
