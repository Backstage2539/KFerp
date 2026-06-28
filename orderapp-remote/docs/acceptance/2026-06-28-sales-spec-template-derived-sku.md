# PR-505 父 SKU 库存单位 + 销售规格模板派生子 SKU 验收记录

## Scope
- 原 `单位模板` UI 改为 `销售规格模板`，只维护销售规格、销售单位、净含量、默认规格和启用状态。
- 父 SKU/商品档案维护唯一库存单位，并引用销售规格模板。
- 子 SKU 由父 SKU 引用的模板规格自动派生，不手工新增，不配置库存单位。
- BOM、价格表、录单、生产和库存仍使用具体子 SKU；子 SKU 有效库存单位继承父 SKU。

## RED Evidence
- Catalog Go tests 初始失败：`ProductSalesSpec`、`SalesSpecs`、`sales_specs_json` 和派生 SKU 同步字段缺失。
- Frontend `node --test src/lib/product-settings.test.js` 初始失败：缺少销售规格模板行归一化和模板明细解析。

## GREEN Evidence
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog -count=1` passed.
- `go test ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1` passed.
- `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing -count=1` passed.
- `go test ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -count=1` passed.
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/order-entry.test.js src/lib/produce-plan.test.js src/lib/bom.test.js` passed.
- `npm run build` passed in `orderapp-remote/frontend-vue-shell` with the existing Vite large-chunk warning.
- `scripts/verify_kferp.sh changed` passed.
- `git diff --check` passed.
- Post-review patch: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog -count=1` passed after adding derived SKU API mapping, inactive default sales spec validation, and template delete status handling.
- Post-review patch: `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/application/bom ./internal/interfaces/http/bom ./internal/infrastructure/postgres/bom -count=1` passed.
- Post-review patch: `scripts/verify_kferp.sh changed` and `git diff --check` passed.

## Acceptance Notes
- `/api/product-settings` 的销售规格模板返回 `sales_specs`，并继续兼容旧 `quote_unit/order_unit/unit_conversion_json` 字段。
- 父 SKU 保存后会按模板规格行自动派生子 SKU；模板新增规格会补派生，模板停用/移除规格只标记历史派生 SKU，不删除。
- 删除销售规格模板时，已派生子 SKU 不删除，统一标记为 `template_removed`。
- 默认销售规格必须是启用规格；全停用的模板保存会被后端拒绝。
- 商品档案新增和配置抽屉显示 `库存单位` + `销售规格模板`，并展示模板明细和已派生子 SKU 编号。
- 子 SKU 的销售单位来自销售规格模板，库存单位来自父 SKU；价格表/BOM/录单/生产查询均读取该有效单位边界。

## Pending
- 浏览器验收：新建销售规格模板，新建父 SKU 引用后自动派生两个子 SKU，BOM 选择子 SKU 后产出单位显示父 SKU 库存单位。
- development merge/deploy and smoke.
