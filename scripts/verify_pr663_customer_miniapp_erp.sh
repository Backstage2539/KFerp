#!/usr/bin/env bash
set -euo pipefail

ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

cd "$ROOT"
git diff --check

cd "$ROOT/orderapp-remote"
go test \
  ./internal/application/catalog \
  ./internal/application/costing \
  ./internal/application/customerfulfillment \
  ./internal/application/customerportal \
  ./internal/application/sales \
  ./internal/infrastructure/excel \
  ./internal/infrastructure/pdf \
  ./internal/infrastructure/postgres/catalog \
  ./internal/infrastructure/postgres/costing \
  ./internal/infrastructure/postgres/customerfulfillment \
  ./internal/infrastructure/postgres/customerportal \
  ./internal/infrastructure/postgres/sales \
  ./internal/interfaces/http/catalog \
  ./internal/interfaces/http/customerportal \
  ./internal/interfaces/http/support

cd "$ROOT/orderapp-remote/frontend-vue-shell"
node --test src/lib/*.test.js
npm run build

cd "$ROOT/miniapp"
npm test -- --run
npm run typecheck
npm run build:mp-weixin:development
