# 2026-05-22 销售单长商品名、优惠运费分行和备注验收

## 范围
- PR-326-SALES-ORDER-LAYOUT-DISCOUNT-NOTE
- 销售单预览、PDF 和 PNG 图片的商品明细行、金额区和末行备注。

## 验收要点
- [x] 长商品名在商品列内自动换行，行高按最高列内容撑开，不遮挡规格、数量、单价、小计和备注列。
- [x] 金额区分行展示商品合计、运费、优惠和应收；优惠按类别显示并用粗体标明类别和金额。
- [x] 销售单页面支持保存“销售单备注”，该备注只进入销售单最后一行，不替代订单内部备注。
- [x] 预览、正式 PDF 和 PNG 图片共用同一套快照字段和金额分行逻辑。

## 证据
- `go test ./internal/infrastructure/pdf -run 'TestSalesOrderItemRowsWrapLongNamesAndNotes|TestSalesOrderFinancialRowsSeparateDiscountShippingAndNote' -count=1`
- `go test ./internal/application/sales -run TestServiceOwnsSalesOrderDocumentUseCases -count=1`
- `go test ./internal/infrastructure/postgres/sales -run TestSalesOrderPreviewIncludesNoteAndDiscountBreakdowns -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestSalesOrderDocumentRoutesRegisterPreviewPDF|TestSalesOrderNoteAPISavesAndReturnsPreviewSnapshot' -count=1`
- `node --test src/lib/sales-order.test.js`
- 手册：`OP_MANUAL_ORDER_SALES.md`
