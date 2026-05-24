# PR-358-PRODUCT-PRICE-LIST-ORDER-PRODUCTION

## 范围
- 录单 `/api/order/form` 在价格表版本选项中返回 `product_type_category_id` 和 `product_type_name`。
- 前端保留旧熟豆/生豆/挂耳豆单版本选择，并新增按产品类型寻找最新版价格表的 helper。
- 生产计划行携带产品类型、产品子类型和工序模板 ID。
- 速溶咖啡等无 BOM 产品可通过产品类型/产品子类型识别默认原料，不再只依赖 `product_kind` 或商品名称。

## 验收
- 旧 `list_type=commercial/green/drip` 的录单版本下拉仍可用。
- 产品类型为“速溶咖啡”的产品，即使历史 `product_kind` 仍是熟豆，也能在生产计划无 BOM 原料中使用“速溶咖啡”。
- 产品子类型绑定工序模板后，生产计划行带出 `operation_template_id`，供后续工序流转使用。

## 证据
- `go test ./internal/infrastructure/postgres/sales -run TestOrderFormBeanListVersionOptionsExposeProductTypeFields -count=1`
- `node --test src/lib/order-entry.test.js`
- `go test ./internal/infrastructure/postgres/production -run 'TestNoBomRawMaterialUsesProductTypeNameForInstantCoffee|TestBuildRoastPlanRowsCarriesOperationTemplateID' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev358 -count=1`
