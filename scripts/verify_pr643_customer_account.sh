#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
: "${ORDERAPP_TEST_DATABASE_URL:?Set a disposable local test database URL; never use a live business database}"
cd "$ROOT/orderapp-remote"
go test ./internal/application/customerfulfillment ./internal/interfaces/http/sales ./internal/infrastructure/postgres/customerfulfillment -run 'TestAccount|TestCustomerAccount|TestCustomerWorkspaceContinuousOrdersAndRecipientIsolation' -count=1
go test ./internal/infrastructure/pdf ./internal/domain/sales ./internal/interfaces/http/support ./internal/interfaces/http/finance -count=1
"$ROOT/scripts/verify_kferp.sh" frontend
