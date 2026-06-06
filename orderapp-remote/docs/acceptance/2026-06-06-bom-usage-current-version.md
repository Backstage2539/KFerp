# PR-430 BOM usage current version

## Scope
- 商品档案“被哪些 BOM 使用”和生产 BOM 详情上层使用关系的组件反查只读取每个生产 BOM 当前有效版本。
- 当前有效版本口径：有草稿时读取草稿，否则读取最新发布版本。
- 产出关系继续使用 `production_boms.output_product_id`；本修复不改变产出商品主关系。

## Reproduction
- Live data showed product `532` `GoalE2E-0605-234447 咖啡熟豆` was listed as a component of `BOM-001435 GoalE2E-0605-234447 咖啡挂耳 BOM`.
- Database check confirmed historical V001 row `production_bom_version_items.id=61` had `component_type=product` and `component_product_id=532`.
- The old lookup scanned every draft/published BOM version, so an old component row could continue to pollute current reverse lookup after a corrected version exists.

## Verifier
- RED: `go test ./internal/infrastructure/postgres/bom -run TestProductionBomUsageLookupsUseCurrentVersionOnly -count=1` failed before implementation because `current_usage_versions` was missing.
- RED: `go test ./internal/interfaces/http/support -run TestDev430BomUsageCurrentVersion -count=1` failed before requirement seed and docs were added.
- GREEN target: `go test ./internal/infrastructure/postgres/bom -run 'TestProductionBomUsageLookupsUseCurrentVersionOnly|TestProductionBomOutputProductAndMultiLevelPublishValidationMarkers' -count=1`.
- GREEN support: `go test ./internal/interfaces/http/support -run TestDev430BomUsageCurrentVersion -count=1`.
- GREEN broader: `go test ./internal/infrastructure/postgres/bom ./internal/interfaces/http/bom ./internal/interfaces/http/support -count=1`; `go test ./...`; `npm run build`; `scripts/verify_kferp.sh changed`; `git diff --check`.

## Source Evidence
- `listProductionBomUsageByProduct` now uses `current_usage_versions` before matching component rows.
- `listProductionBomComponentUsedByBoms` now uses `current_component_versions` before matching component rows.
- Both paths keep output/product relationships separate and avoid reinterpreting old published component lines as current usage.

## Live Acceptance
- After deploy, create or repair the GoalE2E 挂耳 BOM current version so it no longer uses product `532`.
- Verify `/api/production-bom-product-usage/532` does not contain `BOM-001435` with `relation_type=component`.
- Verify the 商品档案配置 drawer for `GoalE2E-0605-234447 咖啡熟豆` no longer shows `BOM-001435 GoalE2E-0605-234447 咖啡挂耳 BOM · 作为组件`.
