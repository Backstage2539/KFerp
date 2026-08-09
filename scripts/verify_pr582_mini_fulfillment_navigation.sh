#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT"
git diff --check
test -s "$ROOT/orderapp-remote/docs/acceptance/2026-08-07-mini-fulfillment-search-inventory-navigation.md"
for contract_file in \
  "$ROOT/ACTIVE_REQUIREMENTS.md" \
  "$ROOT/orderapp-remote/docs/REQUIREMENTS.md" \
  "$ROOT/orderapp-remote/docs/ACCEPTANCE_TESTS.md" \
  "$ROOT/orderapp-remote/internal/interfaces/http/support/req_store.go"
do
  rg -q 'PR-582-MINI-FULFILLMENT-LIST-INVENTORY-NAVIGATION' "$contract_file"
done
rg -q 'pages/customer-inventory-detail/customer-inventory-detail' "$ROOT/miniapp/src/pages.json"
rg -q '多包裹申请中任一包裹命中' "$ROOT/orderapp-remote/docs/REQUIREMENTS.md"
rg -q '跨搜索/跨页多选' "$ROOT/ACTIVE_REQUIREMENTS.md"
rg -q '多包裹申请任一包裹命中' "$ROOT/orderapp-remote/docs/OP_MANUAL_CUSTOMER_FULFILLMENT.md"
rg -q '独立库存详情' "$ROOT/orderapp-remote/docs/OP_MANUAL_CUSTOMER_PORTAL.md"
rg -q '切换搜索词或库存页码不会清除已勾选规格' "$ROOT/orderapp-remote/docs/OP_MANUAL_STOCK.md"
rg -q '真实 PostgreSQL' "$ROOT/orderapp-remote/docs/acceptance/2026-08-07-mini-fulfillment-search-inventory-navigation.md"

cd "$ROOT/orderapp-remote"
go test \
  ./internal/application/customerfulfillment \
  ./internal/infrastructure/postgres/customerfulfillment \
  ./internal/interfaces/http/customerportal \
  ./internal/interfaces/http/support \
  -count=1

cd "$ROOT/miniapp"
npm test -- --run
npm run typecheck
npm run build:mp-weixin:development

artifact="$ROOT/miniapp/dist/build/mp-weixin"
node "$ROOT/scripts/verify_mp_weixin_artifact.mjs" "$artifact" "$artifact/PAGE_FILE_MANIFEST"
"$ROOT/scripts/verify_mp_weixin_manifest.sh" "$artifact"
