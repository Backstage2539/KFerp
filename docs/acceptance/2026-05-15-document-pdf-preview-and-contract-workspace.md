# 2026-05-15 Document PDF Preview And Contract Workspace Acceptance

## Scope
- 销售单预览改为真实 PDF 预览，并在预览 PDF 上标注 `PREVIEW 预览版`。
- 出库单预览改为真实 PDF 预览，并在预览 PDF 上标注 `PREVIEW 预览版`。
- 销售单、出库单和合同盖章共用 `PDFStampPreview.vue`，公章拖动按 PDF 点位保存。
- 合同盖章工作台支持保存合同标题/备注，并支持删除合同从有效列表隐藏。

## Evidence
- `SALES_ORDER_PREVIEW_PDF_OK`: `GET /api/orders/:id/sales-order-preview.pdf` 返回 `application/pdf`，PDF 字节以 `%PDF-` 开头；预览渲染不创建销售单正式版本。
- `DELIVERY_NOTE_PREVIEW_PDF_OK`: `GET /api/orders/:id/delivery-note-preview.pdf` 返回 `application/pdf`，PDF 字节以 `%PDF-` 开头；预览渲染不创建出库单正式版本。
- `CONTRACT_METADATA_SAVE_DELETE_OK`: `PUT /api/contracts/:id` 保存标题和备注；`DELETE /api/contracts/:id` 软删除合同；`GET /api/contracts` 默认不再返回已删除合同。
- `CONTRACT_WORKSPACE_UI_OK`: `ContractsView.vue` 使用 `PDFStampPreview`，提供合同标题、合同备注、保存合同、删除合同、保存盖章 PDF 和下载已盖章 PDF。

## Verification Commands
- `go test ./internal/application/sales ./internal/infrastructure/postgres/sales ./internal/infrastructure/pdf ./internal/interfaces/http/sales -count=1`
- `go test ./internal/application/contracts ./internal/interfaces/http/contracts ./internal/infrastructure/postgres/contracts -count=1`
- `go test ./internal/interfaces/http/support -count=1`
- `cd orderapp-remote/frontend-vue-shell && node --test src/lib/*.test.js && npm run build`

## Acceptance Notes
- 预览版 PDF 只用于操作员确认，不作为正式历史版本。
- 合同删除为软删除，保留数据库审计、源文件和已盖章文件，避免破坏历史追溯。
