# PR-517-OPERATION-STANDARD-COST-MASTER acceptance evidence

## Scope

- 工序列表维护标准工序成本，作为价格试算的标准口径。
- 工序列表维护 `标准工序成本（元/库存单位）`。
- 工位/设备统一维护 `适用工序`。
- 工位产能只维护标准批量、库存单位、标准分钟和状态，不再维护适用工序。
- 工艺路线只维护工序顺序、损耗记录和质检项，不再维护 `标准成本默认产能`。
- 商品价格管理试算按工艺路线工序读取工序列表标准成本，明细来源显示 `工序列表`。

## Evidence Targets

- Manufacturing API: operation save/read supports `standard_operation_cost`; workstation save/read supports `applicable_operation_ids`; capacity save ignores old `applicable_operation_ids`.
- Manufacturing schema/repository: `manufacturing_operations.standard_operation_cost` and `manufacturing_workstation_operations` are available; old capacity-operation rows are backfilled to workstation-operation rows.
- Process route: route save clears old `standard_cost_capacity_id`; Vue route page no longer loads or renders workstation capacities.
- Costing: standard operation details use `operation_master` / `per_inventory_unit`; old default-capacity warnings no longer block price publish.
- Frontend: 工序页显示标准工序成本；工位页在工位表单维护适用工序；价格试算明细 displays `成本来源` / `工序列表`.

## Verification

- `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`
- `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js src/lib/produce-plan.test.js`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- Browser acceptance after deployment: open `生产管理 -> 工序`, save a standard operation cost; open `工位/设备`, set applicable operations at workstation level; open `工艺路线` and verify no `标准成本默认产能`; run product price trial and verify operation detail source is `工序列表`.

## Current Evidence

- Targeted backend GREEN: manufacturing/costing focused tests passed after implementation.
- Targeted frontend GREEN: `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js` passed after implementation.
- Full backend/support GREEN: `go test ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/manufacturing ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`.
- Frontend/build GREEN: `node --test src/lib/process-routes.test.js src/lib/product-settings.test.js src/lib/produce-plan.test.js` passed with 192 tests; `npm run build` passed after `npm ci`, with the existing large-chunk warning.
- Review GREEN: `git diff --check` and `scripts/verify_kferp.sh changed` passed.
