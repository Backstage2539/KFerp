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
- Deployment gate fix: first Docker build reached image `go test ./...` and failed only on historical support static markers after the UI wording changed from unit template to sales spec template. Updated those support markers to PR-505 wording; `go test ./internal/interfaces/http/support -count=1`, `go test ./...`, `scripts/verify_kferp.sh changed`, and `git diff --check` passed.
- Merge/deploy: feature branch `codex/sales-spec-template-derived-sku-20260628` was pushed and fast-forwarded into `develop` at `fe55bcff921528e410a01233147995e8f1e3b5a1`.
- Development deploy: deployed from clean temp checkout `/tmp/kferp-pr505-deploy` with `deploy_orderapp.sh`; successful backup path `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260628113551`.
- Development deploy gate: Docker build ran `go test ./...` successfully, Vue build passed with the existing Vite large-chunk warning, miniapp typecheck/build passed, and `erp_orderapp` restarted.
- API route smoke after deploy: `erp_orderapp`, `erp_postgres`, `erp_caddy`, and `erp_docconvert` were running; `/app/` returned `303`; authenticated `/app/vue-shell?view=productMaster`, `/app/api/product-settings`, `/app/api/bom/products`, and `/app/api/production-boms?status=all` returned `200`.
- API write smoke after deploy: created sales spec template `PR505验收销售规格20260628114412` and parent SKU `PR505验收父SKU20260628114412`; `/api/product-settings` returned two derived child SKUs (`227g袋装`, `100g袋装`) with sales unit `袋`, inherited inventory unit `kg`, unit rule source `derived_sales_spec`, and status `active`.
- BOM product smoke after deploy: `/api/bom/products` returned the two derived child SKUs with `inventory_unit=kg`, proving BOM candidates read the parent SKU inventory unit through the child SKU.
- Browser smoke after deploy: `/vue-shell?view=productMaster` showed `销售规格模板`、`库存单位`、`派生子 SKU`、`销售规格`; old `不引用单位模板`、`高级单位覆盖`、`销售单位换算` text was absent and console errors were 0.

## Acceptance Notes
- `/api/product-settings` 的销售规格模板返回 `sales_specs`，并继续兼容旧 `quote_unit/order_unit/unit_conversion_json` 字段。
- 父 SKU 保存后会按模板规格行自动派生子 SKU；模板新增规格会补派生，模板停用/移除规格只标记历史派生 SKU，不删除。
- 删除销售规格模板时，已派生子 SKU 不删除，统一标记为 `template_removed`。
- 默认销售规格必须是启用规格；全停用的模板保存会被后端拒绝。
- 商品档案新增和配置抽屉显示 `库存单位` + `销售规格模板`，并展示模板明细和已派生子 SKU 编号。
- 子 SKU 的销售单位来自销售规格模板，库存单位来自父 SKU；价格表/BOM/录单/生产查询均读取该有效单位边界。

## Result
- PR-505 has been merged into `develop`, deployed to development, and verified with targeted tests, full deploy-gate tests, API write smoke, API route smoke, BOM candidate smoke, and browser UI smoke.
