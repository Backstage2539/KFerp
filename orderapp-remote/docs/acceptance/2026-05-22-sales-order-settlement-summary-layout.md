# 2026-05-22 销售单结算汇总排版验收

## 范围
- PR-329-SALES-ORDER-SETTLEMENT-SUMMARY-LAYOUT
- 销售单预览、PDF 和 PNG 图片的商品明细列顺序、优惠后价和结算区末两行。

## 验收要点
- [x] 商品明细“单价”列显示原价；最右列显示优惠后价，条目备注放在优惠后价前一列并继续换行。
- [x] 销售单结算区倒数第二行显示订单备注。
- [x] 销售单结算区最后一行同排显示商品合计、优惠合计、运费和应收，减少左侧大面积空白。
- [x] 预览、正式 PDF 和 PNG 图片共用同一套商品列和结算汇总逻辑。

## 证据
- `go test ./internal/infrastructure/pdf -run 'TestSalesOrderItemRowsShowOriginalPriceAndDiscountedFinalColumn|TestSalesOrderFinancialRowsPutNoteBeforeFinalSummary' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev329SalesOrderSettlementSummaryLayout -count=1`
- 手册：`OP_MANUAL_ORDER_SALES.md`
