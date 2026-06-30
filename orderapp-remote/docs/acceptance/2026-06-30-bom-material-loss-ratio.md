# PR-508 BOM Material Loss Ratio Acceptance Evidence

Date: 2026-06-30

## Scope

- Production BOM material rows using `比例 %` can maintain `原料损耗比`.
- Non-material rows and non-ratio rows force `material_loss_rate=0`.
- Production planning, work order material snapshots, WIP/consumption paths, and BOM costing use the loss-adjusted demand.

## RED Evidence

- `go test ./internal/application/bom ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/costing -count=1`
  - Failed before implementation because `MaterialLossRate`, loss-adjusted material consumption helpers, BOM persistence markers, and costing markers were missing.
- `node --test src/lib/bom.test.js`
  - Failed before implementation because the BOM UI and payload did not expose `原料损耗比`.

## GREEN Evidence

- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/production ./internal/interfaces/http/production ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing -count=1`
  - Passed after implementation.
- `node --test src/lib/bom.test.js`
  - Passed after implementation.
- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/production ./internal/interfaces/http/production ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
  - Passed after documentation and PR/DEV seed updates.
- `node --test src/lib/bom.test.js src/lib/produce-plan.test.js src/lib/product-settings.test.js`
  - Passed 203/203.
- `npm ci`
  - Installed the fresh worktree frontend dependencies; npm reported one existing high severity audit item.
- `npm run build`
  - Passed with the existing Vite large-chunk warning.
- `scripts/verify_kferp.sh changed`
  - Passed.
- `git diff --check`
  - Passed.

## Acceptance Checklist

- [x] BOM draft save accepts `ratio_pct=40` with `material_loss_rate=0.2` and returns the field in detail API.
- [x] Product components and non-ratio rows force `material_loss_rate=0`.
- [x] Production material demand uses `1kg / (1 - 0.2) * 40% = 0.5kg`.
- [x] BOM costing treats `40% + 20% 原料损耗比` as effective material ratio `50%`.
- [x] BOM UI shows `原料损耗比`, `损耗比例 %`, and `合计比例（不含原料损耗）`.
- [x] Targeted Go/API tests, frontend tests, Vue build, changed verifier, and diff check passed locally.
- [ ] Browser acceptance to be refreshed after development deploy.

## Manual Updates

- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
