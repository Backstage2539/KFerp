# PR-341-ORDER-BACKFILL-DATES 验收证据

## 范围
- 录单页新增并保留 `单据日期` 与 `订单日期` 两个可编辑字段。
- `orders.document_date` 作为 ERP 单据日期，`orders.order_date` 保持客户真实订单日期。
- 订单号按单据日期生成，旧请求未传 `document_date` 时按 `order_date` 兼容。
- 订单列表、销售单 PDF/PNG、出库单 PDF 展示双日期；出库单继续展示出库日期。

## 本地验证
- `go test ./internal/infrastructure/pdf -run 'TestSalesOrderHeaderMetaRowsShowsDocumentAndOrderDates|TestDeliveryNoteHeaderMetaRowsShowsDocumentOrderAndPostingDates'`：通过。
- `go test ./internal/interfaces/http/sales -run TestOrderAPISaveCarriesDocumentAndOrderDates`：通过。
- `node --test src/lib/order-entry.test.js`：通过。
- `go test ./...`：通过。
- `node --test src/lib/*.test.js src/api/*.test.js`：299 项通过。
- `npm run build`：通过。
- `git diff --check`：通过。
