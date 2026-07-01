# PR-515-STANDARD-MANUFACTURING-COST-PRICING acceptance evidence

## Scope

- 标准制造成本成为商品价格试算的默认成本来源。
- 价格表不直接绑定生产计划里的产能和批次数。
- 生产计划/工单真实工位、产能和批次只影响实际成本追溯，不回改历史价格。

## Evidence Targets

- Go/API: costing service returns `cost_source=standard_manufacturing_cost`, `material_unit_cost`, `operation_unit_cost`, `standard_manufacturing_unit_cost`, and standard-cost snapshots.
- Repository: standard operation cost is derived from process route operations, workstation capacity standard minutes/output, and workstation hourly rate; it does not read old route planned operation cost as the price source.
- Frontend: 商品价格管理试算瀑布展示 `标准制造成本`、`物料单位成本`、`标准工序成本` 和 `标准制造成本折算明细`。
- Manuals: 成本手册 and 生产手册 describe the boundary between standard cost pricing and actual production execution.

## Verification

- `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/production ./internal/interfaces/http/production -count=1`
- `node --test src/lib/product-settings.test.js src/lib/process-routes.test.js`
- `npm run build`
- `scripts/verify_kferp.sh changed`
