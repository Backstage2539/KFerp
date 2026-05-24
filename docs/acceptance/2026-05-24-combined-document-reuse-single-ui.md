# PR-354-COMBINED-DOCUMENT-REUSE-SINGLE-UI 验收记录

## 范围
- 组合销售单入口复用单张销售单抽屉，保留销售单设置、客户信息、销售单备注、预览、PDF版本和图片版本栏目。
- 组合出库单入口复用单张出库单抽屉，保留刷新预览、下载最新版、公章设置、出库维护、预览和历史版本栏目。
- 组合模式只改变单据内容和版本接口，不再维护另一套简化组合弹窗逻辑。

## 验收点
- 订单列表点击“组合销售单”时，页面打开 `SalesOrderView` 并传入所选 `orderIds`；预览、生成 PDF 和 PDF 历史版本走组合销售单接口。
- 订单列表点击“组合出库单”时，页面打开 `DeliveryNoteView` 并传入所选 `orderIds`；预览、生成 PDF 和历史版本走组合出库单接口。
- 组合出库单的出库维护区保留在同一界面中，但组合内容读取各订单已保存的出库信息，修改出库信息仍回单个订单维护。

## 证据
- 支持测试：`go test ./internal/interfaces/http/support -run TestDev354 -count=1`
- API 测试：`go test ./internal/interfaces/http/sales -run 'TestCombinedDocumentRoutesRegisterPreviewGenerateAndDownload|TestCombinedSalesOrderDocumentAPI|TestCombinedDeliveryNoteDocumentAPIRequiresShippedOrders' -count=1`
- 前端构建：`npm --prefix orderapp-remote/frontend-vue-shell run build`
