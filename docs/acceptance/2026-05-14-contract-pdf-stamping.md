# 2026-05-14 Contract PDF Stamping Acceptance

## Prompt-To-Artifact Checklist

- 上传一个合同：`ContractsView.vue` 提供 `/api/contracts` multipart 上传；`contractsapp.UploadContract` 保存合同元数据和文件。
- DOCX 转 PDF：`contractsapp.UploadContract` 对 `ContractSourceDOCX` 调用 `docconvert.LibreOfficeConverter`，运行 `soffice --headless --convert-to pdf`。
- PDF 可直接盖章：PDF 上传后 `pdf_object_key` 复用源 PDF，前端直接渲染 `/contracts/:id/pdf`。
- 参考销售单公章设置：`GET /api/settings/sales-order/seals` 列出 `sales_order_assets.kind='seal'`；合同页选择同一批公章资产。
- 可以选择公章：`ContractsView.vue` 的公章下拉框绑定 `selectedSealID`。
- 多页拖动公章位置：`ContractsView.vue` 对每页维护 `StampPlacement.page_number/x/y/width/height`，`moveContractStampPlacement` 按显示比例转换拖动距离。
- 保存盖章后的 PDF：`createStampedContractPDF` 使用 `pdf-lib` 写入公章，`POST /api/contracts/:id/stamped` 保存版本，`/contracts/:id/stamped-latest.pdf` 下载最新版。

## Evidence

- `CONTRACT_PDF_UPLOAD_OK`: `go test ./internal/application/contracts -run TestUploadPDFContractStoresOriginalAsStampablePDF -count=1 -v`
- `CONTRACT_DOCX_CONVERT_OK`: `go test ./internal/application/contracts -run TestUploadDOCXContractConvertsToPDFBeforePersistence -count=1 -v`; `go test ./internal/infrastructure/docconvert -count=1 -v`
- `CONTRACT_MULTI_PAGE_SEAL_DRAG_OK`: `node --test src/lib/contract-stamp.test.js`
- `CONTRACT_STAMPED_PDF_SAVE_OK`: `go test ./internal/application/contracts -run TestSaveStampedPDFStoresVersionWithPlacements -count=1 -v`; `go test ./internal/interfaces/http/contracts -count=1 -v`
- `CONTRACT_MANUAL_UPDATED_OK`: `OP_MANUAL_ORDER_SALES.md`; `orderapp-remote/docs/OP_MANUAL_ORDER_SALES.md`

## Review Status

- Backend unit/API evidence is automated.
- Frontend helper evidence is automated.
- Browser click evidence remains manual unless a later smoke test starts the app with seeded contract/seal fixtures.

