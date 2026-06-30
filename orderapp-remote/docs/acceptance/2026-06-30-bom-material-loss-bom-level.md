# PR-510 BOM Material Loss BOM-Level Acceptance Evidence

Date: 2026-06-30

## Scope

- PR-511-BOM-MATERIAL-LOSS-BOM-LEVEL
- BOM 版本级 `原料损耗比` is the source of truth for new draft saves.
- Move `原料损耗比` from the component row UI to BOM version settings.
- Persist the version-level rate in `production_bom_versions.material_loss_rate`.
- When the switch is enabled, component consume units must use `比例 %`.
- Keep `production_bom_version_items.material_loss_rate` as the runtime snapshot consumed by production, inventory, and costing flows.

## RED Evidence

- `go test ./internal/application/bom ./internal/infrastructure/postgres/bom -count=1`
  - Failed before implementation because `UpdateProductionBomVersionDraftCommand.MaterialLossRate` and version-level persistence markers were missing.
- `node --test src/lib/bom.test.js`
  - Failed before implementation because `BomView.vue` did not expose `versionMaterialLossRateEnabled` or the ratio-only BOM-level explanation.

## GREEN Evidence

- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1`
  - Passed after implementation.
- `node --test src/lib/bom.test.js`
  - Passed after implementation.
- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/production ./internal/interfaces/http/production ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
  - Passed after PR/DEV seed and documentation updates.
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

- [x] BOM version settings show `原料损耗比` and `损耗比例 %`.
- [x] The switch explains `开启后组件消耗单位只能使用比例 %`.
- [x] When the switch is enabled, the consume unit options are limited to `比例 %`.
- [x] Draft save payload includes version-level `material_loss_rate`.
- [x] Backend rejects non-ratio component units when version-level material loss is enabled.
- [x] `ratio_pct=40%` and `material_loss_rate=20%` still produce effective material demand `1kg / (1 - 20%) * 40% = 0.5kg`.
- [x] Targeted Go/API tests, frontend tests, Vue build, changed verifier, and diff check passed locally.
- [ ] Merge, deploy, and browser smoke.

## Manual Updates

- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_INVENTORY_MATERIALS.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
