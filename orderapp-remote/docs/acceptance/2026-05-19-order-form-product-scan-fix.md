# 2026-05-19 录单初始化商品字段扫描修复验收

## 范围
- 修复打开录单页时 `/api/order/form` 返回 `number of field descriptions must equal number of destinations, got 16 and 15`。
- 保持录单商品下拉继续返回烘焙度、产品形态、客户归属、挂耳袋/盒元数据和阶梯价。

## 根因
- `fetchOrderProducts` 的商品查询在 `name` 后多选了一列 `product_kind`，但 `Scan` 仍按 15 个目标接收；该位置原本应是 `roast_level`。

## 验收点
- [x] 商品查询第三列恢复为 `roast_level`。
- [x] `product_kind` 仍写入 `ProductOption.ProductKind`，不会丢失生豆或挂耳形态。
- [x] 开发环境 `/app/api/order/form` 重新返回 200。

## 验证证据
- `go test ./internal/infrastructure/postgres/sales -run TestOrderFormProductQueryKeepsRoastLevelAndProductKindScanShape -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestOrderAPIFormReturnsRetailSpecs|TestOrderAPIFormReturnsCustomerDefaultsForOrderEntry|TestOrderAPIFormFiltersProductsForSelectedCustomer' -count=1`
- `go test ./...`
- `npm run build`
- `git diff --check`
- 部署后认证请求：`GET /app/api/order/form` 返回 200。
