# PR-520 库存单位保存后锁定

## Scope

- 物料档案库存单位只在新建时选择，编辑已有物料时只读展示。
- 销售规格模板库存单位只在新建模板时选择，编辑已有模板时只读展示。
- 后端拒绝绕过前端提交的库存单位变化；单位选错时通过新建档案或模板迁移，不回改历史库存、BOM、价格表、订单、工单和库存流水。

## Evidence

- RED：`go test ./internal/infrastructure/postgres/catalog -run TestProductUnitTemplateInventoryUnitIsLockedAfterCreate -count=1` 在模板锁定 helper/marker 不存在时失败。
- RED：`go test ./internal/infrastructure/postgres/materials -run TestMaterialInventoryUnitIsLockedAfterCreate -count=1` 在物料锁定 helper/marker 不存在时失败。
- RED：`node --test src/lib/materials-ui.test.js src/lib/product-settings.test.js` 在前端锁定 marker 和 disabled select 不存在时失败。
- GREEN：`go test ./internal/infrastructure/postgres/materials ./internal/infrastructure/postgres/catalog -run 'TestMaterialInventoryUnitIsLockedAfterCreate|TestProductUnitTemplateInventoryUnitIsLockedAfterCreate' -count=1` 通过。
- GREEN：`go test ./internal/interfaces/http/materials -run 'TestMaterialsAPIUpdateAllowsBaseFieldsAndWritesAudit|TestMaterialsAPIUpdateRejectsInventoryUnitChange|TestMaterialsAPIUpdateAllowsOmittedInventoryUnit|TestMaterialsAPIUpdateRejectsInlineStockChange' -count=1` 通过。
- GREEN：`go test ./internal/infrastructure/postgres/materials ./internal/interfaces/http/materials ./internal/infrastructure/postgres/catalog ./internal/interfaces/http/catalog ./internal/application/catalog -count=1` 通过。
- GREEN：`node --test src/lib/materials-ui.test.js src/lib/product-settings.test.js` 通过。
- GREEN：`npm ci` 后 `npm run build` 通过，保留既有 Vite large-chunk warning。
- GREEN：`scripts/verify_kferp.sh changed` 和 `git diff --check` 通过。
