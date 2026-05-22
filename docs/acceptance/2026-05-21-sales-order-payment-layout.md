# 2026-05-21 销售单收款码与文本框版式验收

## 需求
- PR-307-SALES-ORDER-PAYMENT-LAYOUT
- 销售单收款码默认固定在第 1 页右侧并放大。
- 销售单设置支持调整收款码 X/Y/宽/高，以及收款方式、公账收款、说明文本框 X/Y/宽/高。
- 预览 PDF、正式 PDF 和 PNG 图片使用同一套布局设置。

## 验收清单
- [x] 单元测试覆盖默认第 1 页右侧收款码布局和可配置文本框/收款码框。
- [x] 单元测试覆盖 PNG 图片按配置位置和尺寸绘制放大的收款码。
- [x] API/仓储测试覆盖销售单设置保存/读取新增布局字段，保存公章坐标不覆盖收款布局。
- [x] 前端源码守卫覆盖销售单设置页的文本框和收款码位置/尺寸输入项。
- [x] 操作手册、REQUIREMENTS 和 ACCEPTANCE_TESTS 已同步。

## 证据
- `go test ./internal/infrastructure/pdf -run 'TestSalesOrderPaymentLayout|TestRenderSalesOrderPNGUsesConfiguredPaymentCodeLayout|TestRenderSalesOrderPNGUsesHighResolutionCanvasAndLargePaymentCode' -count=1`
- `go test ./internal/infrastructure/postgres/sales -run TestSalesOrderSettingsRoundTrip -count=1`
- `go test ./internal/interfaces/http/sales -run 'TestSalesOrderSettingsAPI|TestSalesOrderSealPositionAPIOnlyUpdatesCoordinates' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev307SalesOrderPaymentLayoutControls -count=1`

