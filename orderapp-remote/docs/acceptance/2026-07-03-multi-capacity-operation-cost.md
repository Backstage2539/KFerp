# PR-518-MULTI-CAPACITY-OPERATION-COST acceptance evidence

## Scope

- 工位产能档作为候选成本能力，不自动代表某个产品报价。
- 工艺路线工序显式选择 `标准成本产能档`；多个候选时不能按排序、最便宜或第一条自动猜。
- BOM 发布时冻结工序成本快照：工序、工位、产能档、小时费率、标准分钟、标准批量和折算后的 `元/库存单位`。
- 商品价格管理试算和商品价格表发布读取 BOM 冻结的工序成本快照。
- BOM 绑定工艺路线但没有工序成本快照时，价格试算返回警告，价格表发布失败并提示 `请先发布包含标准成本产能档快照的 BOM`。

## Evidence Targets

- Manufacturing: process route operations preserve and validate `standard_cost_capacity_id`; candidates must be active and applicable to the operation.
- BOM: `production_bom_version_operation_costs` stores the frozen operation cost snapshot when a BOM version is published.
- Costing: pricing trial details read `bom_operation_snapshot`; missing snapshots use `bom_operation_snapshot_missing` and are blocked during price list publish.
- Frontend: process route editor shows `标准成本产能档`, candidate formula, and the warning that it is only for BOM/price standard cost, not production scheduling.
- Docs: requirements, acceptance checklist, production manual, costing manual, and PR/DEV/API/REV seed rows describe the new boundary.

## Verification

- `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`
- `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- Browser acceptance after deployment: configure workstation hourly cost and capacity, select that capacity as route operation `标准成本产能档`, publish a BOM, run price trial, and verify operation detail source is `BOM工序成本快照`.

## Current Evidence

- RED: `go test ./internal/application/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/costing -count=1` failed before implementation because route save cleared `standard_cost_capacity_id`, BOM had no operation cost snapshot table, and costing did not read BOM operation snapshots.
- RED: `go test ./internal/application/costing -run TestPublishBeanListBlocksMissingBomOperationCostSnapshot -count=1` failed before implementation because price list publish allowed `bom_operation_snapshot_missing`.
- GREEN targeted backend: `go test ./internal/application/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/bom ./internal/infrastructure/postgres/costing -count=1`.
- GREEN targeted costing publish guard: `go test ./internal/application/costing -run 'TestPublishBeanListBlocksMissingBomOperationCostSnapshot|TestPublishBeanListDoesNotBlockRetiredStandardCostDefaultCapacityWarning' -count=1`.
- GREEN costing packages: `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing -count=1`.
- GREEN frontend targeted: `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js`.
- GREEN support contracts: `go test ./internal/interfaces/http/support -run 'TestDev515StandardManufacturingCostPricingContracts|TestDev517OperationStandardCostMasterContracts|TestDev518MultiCapacityOperationCostContracts' -count=1`.
- GREEN full touched backend/support: `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`.
- GREEN frontend/build: `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js` passed with 157 tests; first `npm run build` failed because fresh worktree lacked `vite`, then `npm ci` succeeded and `npm run build` passed with the existing large-chunk warning.
- GREEN review: `scripts/verify_kferp.sh changed`; `git diff --check`.

## Deployment Smoke

- Pending.
