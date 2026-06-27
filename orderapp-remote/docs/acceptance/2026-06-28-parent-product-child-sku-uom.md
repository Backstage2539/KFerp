# PR-504 父商品 + 子 SKU + 单位模板验收记录

## Scope
- 商品档案作为父商品入口，子 SKU 表达具体销售/库存规格。
- `袋/227g` 不作为销售单位；`227g袋装` 是子 SKU，销售单位仍来自单位模板。
- 商品价格表、BOM、生产、库存和订单后续以具体 `sku_id` 为业务对象。

## RED Evidence
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/costing ./internal/application/bom -count=1` 初始失败：`CreateSKUCommand`、商品读取/API 响应和价格表发布快照缺少子 SKU 字段。
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/bom.test.js` 初始失败：前端缺少 `buildChildSkuCreatePayload`、`productSkuRowsForParent`，商品价格表平铺行按 `product_id` 去重导致同父商品不同 SKU 被覆盖。

## GREEN Evidence
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/costing ./internal/application/bom -count=1` passed.
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/bom.test.js` passed.
- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/application/production ./internal/interfaces/http/production ./internal/application/bom -count=1` passed.
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/bean-list-pdf.test.js src/lib/order-entry.test.js src/lib/produce-plan.test.js src/lib/bom.test.js` passed.
- `npm ci` and `npm run build` passed; Vite reported the existing large-chunk warning.
- `scripts/verify_kferp.sh changed` passed.
- `git diff --check` passed.

## Acceptance Notes
- `/api/product-settings` 返回 `sku_id/parent_product_id/effective_parent_product_id/sku_name/sku_code/barcode/spec_label/net_content_qty/net_content_unit/is_default_sku`。
- 商品档案配置抽屉新增 `销售规格 / SKU` 区块，可在父商品下新增子 SKU；新增子 SKU 必须选择单位模板，不直接维护销售单位换算。
- 商品价格表平铺行优先按 `sku_id` 识别唯一商品行，并在发布快照中补 `sku_id/parent_product_id/sku_snapshot`。
- `/api/costing` 商品输入返回子 SKU 元数据，价格表从真实商品输入生成平铺行时可以冻结父商品、子 SKU 和规格快照。
- 生产 BOM 仍按产出商品的有效库存单位写入产出单位；当产出商品是子 SKU 时，产出单位即该子 SKU 的库存单位。

## Pending Deployment Smoke
- Browser smoke after development deployment: 商品档案新增子 SKU，商品价格表同父商品多 SKU 价格行，生产 BOM 选择子 SKU 后产出单位来自该 SKU 单位模板。
