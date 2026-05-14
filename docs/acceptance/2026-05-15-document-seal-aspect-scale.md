# 2026-05-15 Document Seal Aspect Scale Acceptance

## Scope
- 销售单 PDF 预览、出库单 PDF 预览和合同盖章工作台中的公章按原图比例显示，圆章不再被压成椭圆。
- 销售单、出库单和合同盖章预览提供公章大小滑轨。
- 合同保存盖章 PDF 时按公章图片实际宽高比写入最终 PDF。

## Evidence
- `DOCUMENT_STAMP_ASPECT_OK`: `document-pdf-stamp.js` 使用 `sealAspectRatio` 计算 PDF 公章高度，默认比例为 `1`。
- `DOCUMENT_STAMP_SIZE_SLIDER_OK`: `SalesOrderView.vue` 和 `DeliveryNoteView.vue` 提供“公章大小”滑轨，调整后调用销售单公章位置接口保存 `seal_width_mm`。
- `DOCUMENT_OUTPUT_STAMP_ASPECT_OK`: `sales_order_pdf.go` 的销售单/出库单正式 PDF 和 `sales_order_png.go` 共享方形公章基准，后续生成文件与预览比例一致。
- `CONTRACT_STAMP_ASPECT_SAVE_OK`: `contract-stamp.js` 从 `sealImage.width`/`sealImage.height` 计算比例，保存盖章 PDF 时按该比例绘制。
- `CONTRACT_STAMP_SIZE_SLIDER_OK`: `ContractsView.vue` 提供“公章大小”滑轨，调整所有待盖章页面的公章尺寸。

## Verification
- `node --test src/lib/document-pdf-stamp.test.js src/lib/contract-stamp.test.js src/lib/sales-order.test.js`
- `go test ./internal/infrastructure/pdf -count=1`
- `go test ./internal/interfaces/http/support -run TestDev277 -count=1`
