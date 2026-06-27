# PR-501 商品 UOM 与销售换算

## 范围

- 商品档案维护库存单位、默认销售单位、销售单位到库存单位换算和整数销售单位规则。
- 商品价格管理不维护单位换算；商品价格表发布时按商品档案单位规则固化价格单位、库存单位和换算。
- 录单和生产计划读取已发布价格表/订单快照中的单位换算，生产、BOM、WIP 和库存流水只使用库存单位数量。

## RED

- `go test ./internal/infrastructure/postgres/catalog -run TestProductConfigOverridesRemainReadableButProductUpdateOnlyWritesUnitRule -count=1`：商品操作日志缺少默认销售单位、销售换算和整数销售规则审计字段。
- `go test ./internal/infrastructure/postgres/costing -run TestResolveProductSalesUnitRuleUsesProductMasterAndLegacyFallbacks -count=1`：商品价格表发布的单位解析只读直接模板，缺少商品档案和历史兼容回退。
- `go test ./internal/infrastructure/postgres/costing -run TestProductSalesUnitConversionMapAcceptsLegacyFlatConversions -count=1`：历史单位模板的 flat 换算 JSON（如 `{"盒":0.2}`）无法解析到库存单位。
- `go test ./internal/application/costing -run TestPublishBeanListUsesCustomerAliasUnitRuleWhenPresent -count=1`：客户价目表发布时会用普通商品单位规则覆盖客户 alias/template/rule 的单位规则。
- `go test ./internal/infrastructure/postgres/production -run TestDripProductionPlanNeedsUseOrderPriceSnapshotUnitConversion -count=1`：生产计划仍按 `box` 旧硬编码计算挂耳需求，未读取订单价格快照换算。
- `go test ./internal/infrastructure/postgres/sales -run TestOrderFormProductsExposeProductTypeAndUnitRuleFields -count=1`：录单商品选项把整块 `unit_rule_override_json` 当作 `unit_conversion_json`。
- `node --test src/lib/product-settings.test.js`：商品新增/配置抽屉缺少默认销售单位、销售单位换算和整数销售单位字段。
- `node --test src/lib/product-settings.test.js --test-name-pattern 'product production config save does not turn inherited sales units into product overrides'`：商品配置保存会把继承兜底的销售单位写成商品覆盖。
- `node --test src/lib/costing-price-list-workflow.test.js`：商品价格表平铺价格行未读取商品档案销售单位换算。

## GREEN

- `go test ./internal/application/catalog ./internal/interfaces/http/catalog ./internal/infrastructure/postgres/catalog ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/sales ./internal/interfaces/http/sales ./internal/infrastructure/postgres/sales ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -count=1`
- `node --test src/lib/product-settings.test.js src/lib/costing-price-list-workflow.test.js src/lib/bom.test.js src/lib/order-entry.test.js src/lib/produce-plan.test.js`
- `npm ci`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- `git diff --check`

## 部署验证

- 部署：feature branch fast-forward merged to `develop`; development stack deployed by `./deploy_orderapp.sh`.
- Docker build gate: container build ran `go test ./...` and passed.
- API smoke: authenticated `/api/product-settings`、`/api/bom/products`、`/api/production-boms?status=all`、`/api/order/form`、`/api/costing/bean-list`、`/api/costing/bean-list/publications`、`/api/produce/unproduced` returned `200`.
- Field smoke: deployed `/api/product-settings` exposes `inventory_unit`、`default_sales_unit`、`unit_conversion_json`、`sales_unit_rules`; deployed `/api/bom/products` exposes `inventory_unit`.
- Browser smoke: deployed 商品档案页面 opened; 创建新商品档案抽屉 shows `库存单位`、`整数库存`、`默认销售单位`、`销售单位换算`、`整数销售单位`.
