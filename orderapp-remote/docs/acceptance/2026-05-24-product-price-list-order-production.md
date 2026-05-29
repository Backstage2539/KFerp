# PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION

## 范围
- 录单 `/api/order/form` 在价格表版本选项中返回 `product_type_category_id` 和 `product_type_name`。
- 前端保留旧熟豆/生豆/挂耳豆单版本选择，并新增按产品类型寻找最新版价格表的 helper。
- 录单商品选项返回产品类型、产品子类型和轻量单位规则；前端可按 `unit_conversion_json` 推出盒装等录单单位的默认克重规格。
- 生产计划行携带产品类型、产品子类型和工序模板 ID。
- 成本核算/产品价格表预览可带 `customer_id` 加载客户规则作用后的产品输入。
- 客户产品规则模板或客户专属覆盖指定的阶梯价模板、单位规则和工序模板，会进入客户范围产品价格表生成与生产计划工序解析。
- 速溶咖啡等无 BOM 产品可通过产品类型/产品子类型识别默认原料，不再只依赖 `product_kind` 或商品名称。

## 验收
- 旧 `list_type=commercial/green/drip` 的录单版本下拉仍可用。
- 速溶咖啡盒装 SKU 配置 `order_unit=盒`、`unit_conversion_json={"盒":{"kg":0.2}}` 后，录单选择商品时数量栏显示“数量（盒）”，规格默认可选“盒（200g）”。
- 产品类型为“速溶咖啡”的产品，即使历史 `product_kind` 仍是熟豆，也能在生产计划无 BOM 原料中使用“速溶咖啡”。
- 产品子类型绑定工序模板后，生产计划行带出 `operation_template_id`，供后续工序流转使用。
- 给客户绑定“大客户速溶规则模板”后，产品价格表客户范围预览使用客户规则；再设置客户专属覆盖时，覆盖值优先生效。订单进入生产计划后工序模板也按客户覆盖/模板优先级解析。

## 证据
- `go test ./internal/infrastructure/postgres/sales -run TestOrderFormBeanListVersionOptionsExposeProductTypeFields -count=1`
- `node --test src/lib/order-entry.test.js`
- `go test ./internal/infrastructure/postgres/production -run 'TestNoBomRawMaterialUsesProductTypeNameForInstantCoffee|TestBuildRoastPlanRowsCarriesOperationTemplateID' -count=1`
- `go test ./internal/interfaces/http/costing -run TestBeanListAPIPassesCustomerIDForCustomerProductRules -count=1`
- `go test ./internal/interfaces/http/sales -run TestAPIProductsCarriesProductTypeAndUnitRule -count=1`
- `node --test frontend-vue-shell/src/lib/order-entry.test.js --test-name-pattern "order entry derives default order-unit spec"`
- `node --test frontend-vue-shell/src/lib/product-bean-list-split.test.js --test-name-pattern "loads customer rule scoped prices"`
- `go test ./internal/interfaces/http/support -run TestDev358 -count=1`
