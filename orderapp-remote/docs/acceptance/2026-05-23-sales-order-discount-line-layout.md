# 2026-05-23 销售单商品优惠列排版验收

## 范围
- PR-330-SALES-ORDER-DISCOUNT-LINE-LAYOUT
- 销售单预览、PDF 和 PNG 图片的商品规格、商品优惠折扣列、无优惠隐藏规则和商品备注列位置。

## 验收要点
- [x] 商品明细“规格”显示为 `规格/销售单位`，例如 `1000g/件`。
- [x] 有行级优惠时显示“优惠折扣”列，每个商品行展示 `￥-28元` 这类优惠金额。
- [x] 商品表不再显示“优惠后价”列。
- [x] 无优惠订单不显示“优惠折扣”列，也不显示“优惠合计”。
- [x] 每个商品的条目备注放在商品表最后一列。
- [x] 销售单快照从订单明细读取每行 `discount_amount`，预览、正式 PDF 和 PNG 共用同一套商品列逻辑。

## 证据
- `go test ./internal/infrastructure/pdf ./internal/infrastructure/postgres/sales -run 'TestSalesOrderItemRowsShowSpecPerUnitDiscountAndFinalNote|TestSalesOrderFinancialRowsHideDiscountTotalWhenNoDiscount|TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev330SalesOrderDiscountLineLayout -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`
