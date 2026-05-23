# PR-342-ORDER-ENTRY-CUSTOMER-COMMON-PRODUCT-SORT

## 需求

- 录单选择客户后，如果该客户有历史订单，商品下拉列表默认把常用商品排在前面。
- 常用商品按该客户有效历史订单中的下单次数优先，明细次数和最近下单时间作为辅助排序。
- 作废订单不参与常用商品统计。
- 搜索、客户 SKU 范围、公共 SKU 开关和豆单范围过滤继续先正常生效。

## 验收

- `/api/order/form` 返回 `customer_product_usages`，包含客户、商品、订单次数、明细次数和最近订单日期。
- 选择有历史订单的客户后，商品候选中该客户常用商品置顶。
- 没有历史订单的客户保持原商品顺序。
- 客户输入搜索词后，只在搜索命中的商品内按常用商品排序。

## 证据

- `go test ./internal/interfaces/http/sales -run TestOrderAPIFormReturnsCustomerProductUsageForCommonProductSorting -count=1`
- `node --test src/lib/order-entry.test.js`
