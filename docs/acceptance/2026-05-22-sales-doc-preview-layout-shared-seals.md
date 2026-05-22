# 2026-05-22 销售单预览版式与共享公章验收

## 需求
- PR-308-SALES-DOC-PREVIEW-LAYOUT-AND-SEALS
- 销售单预览中直接拖动/拉伸“文字位置和大小”“收款码位置和大小”，不再在销售单设置页输入坐标。
- 公章设置只维护共享公章资产：上传公章、选择公章、去除背景。
- 销售单、出库单和合同盖章共用同一套公章资产，上传或选择后三个功能都能使用。

## 验收清单
- [x] 销售单预览页出现“文字位置和大小”“收款码位置和大小”两个可拖动/拉伸框，松开后保存到收款布局接口。
- [x] 收款布局独立保存，不覆盖收款方式、个性化说明、收款码资产、公章资产和公章坐标。
- [x] 销售单设置页不再展示收款码/文字框坐标输入，也不再展示公章位置画布或公章坐标保存按钮。
- [x] 公章设置只保留上传公章、选择公章、去除背景；销售单、出库单和合同盖章复用销售单公章资产列表。
- [x] 操作手册、REQUIREMENTS 和 ACCEPTANCE_TESTS 已同步。

## 证据
- `go test ./internal/interfaces/http/sales -run 'TestSalesOrderSettingsRegistersSealToolRoutes|TestSalesOrderPaymentLayoutAPIOnlyUpdatesPaymentBoxes|TestSalesOrderSealSelectAPIChoosesReusableSealAsset' -count=1`
- `go test ./internal/interfaces/http/support -run 'TestDev307SalesOrderPaymentLayoutControls|TestDeliveryNoteViewSupportsSharedSealSettings' -count=1`
- `node --test src/lib/document-pdf-stamp.test.js`
- `npm run build`
