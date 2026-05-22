# PR-330-ORDER-ENTRY-PUBLIC-SKU-SCOPE

## 验收范围
- ERP 录单和修改订单商品下拉读取当前客户 SKU 设置的 `use_public_sku`。
- 客户关闭公共 SKU 时，商品候选不展示公共 SKU、其他客户 SKU 或其他客户豆单里的生豆 SKU。
- 芬纳咖啡关闭公共 SKU 后，岩师傅的 `孟连水洗A`、`红酒日晒-2026` 不出现在芬纳录单商品候选中；芬纳自己的定制熟豆仍可选并保留价格梯度。

## 验收证据
- 单元：`TestFilterOrderProductsForCustomerHonorsPublicSKUUsage` 覆盖后端过滤；`filterProductsForCustomer hides public products when customer disables public SKU usage` 覆盖前端下拉过滤。
- API：`TestOrderAPIFormHidesPublicGreenBeansWhenCustomerDisablesPublicSKU` 覆盖 `/api/order/form?customer_id=...` 返回 `use_public_sku=false` 且隐藏岩师傅公共生豆。
- 手册：`OP_MANUAL_ORDER_SALES.md` 记录 `use_public_sku` 关闭后的商品候选范围和排查方式。
