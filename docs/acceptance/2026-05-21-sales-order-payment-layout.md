# 2026-05-21 销售单收款码与文本框版式验收

## 需求
- PR-307-SALES-ORDER-PAYMENT-LAYOUT
- 销售单收款码默认固定在第 1 页右侧并放大。
- 销售单预览支持直接拖动/拉伸收款码框，以及收款方式、个性化说明、公账收款文本框。
- 个性化说明优先显示在公账收款前面，公账信息太长时不得把个性化说明挤没。
- 预览 PDF、正式 PDF 和 PNG 图片使用同一套布局设置。

## 验收清单
- [x] 单元测试覆盖默认第 1 页右侧收款码布局和可配置文本框/收款码框。
- [x] 单元测试覆盖个性化说明在销售单付款文字区内优先于公账收款信息显示。
- [x] 单元测试覆盖 PNG 图片按配置位置和尺寸绘制放大的收款码。
- [x] API/仓储测试覆盖销售单设置保存/读取新增布局字段，独立保存收款布局时不覆盖收款方式、个性化说明、收款码和公章设置。
- [x] 前端源码守卫覆盖销售单预览页的文本框和收款码拖动/拉伸入口，销售单设置页不再出现坐标输入项。
- [x] 操作手册、REQUIREMENTS 和 ACCEPTANCE_TESTS 已同步。

## 证据
- `go test ./internal/infrastructure/pdf -run 'TestSalesOrderPaymentLayout|TestRenderSalesOrderPNGUsesConfiguredPaymentCodeLayout|TestRenderSalesOrderPNGUsesHighResolutionCanvasAndLargePaymentCode' -count=1`
- `go test ./internal/infrastructure/pdf -run TestSalesOrderPaymentTextSectionsPrioritizePersonalNote -count=1`
- `go test ./internal/infrastructure/postgres/sales -run TestSalesOrderSettingsRoundTrip -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestSalesOrderSettingsAPI|TestSalesOrderSealPositionAPIOnlyUpdatesCoordinates' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev307SalesOrderPaymentLayoutControls -count=1`
- `go test ./internal/interfaces/http/sales -run TestSalesOrderPaymentLayoutAPIOnlyUpdatesPaymentBoxes -count=1`
- `node --test src/lib/document-pdf-stamp.test.js`
