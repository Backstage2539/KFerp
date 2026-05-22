# PR-320-BEAN-LIST-PREVIEW-STYLE-PDF

## 验收目标
- 生成豆单抽屉的“生成 PDF”不再调用 `window.print()` 或系统打印窗口。
- 生成 PDF 时保存当前预览快照为草稿，并由后端生成、保存、下载预览卡片样式 PDF。
- 发布豆单、保存草稿、版本列表下载都使用同一套预览卡片样式。
- 已存在的旧文本版 PDF 缓存因 cache key 升级为 `bean-list-preview-style-v2` 而重新生成覆盖。

## 证据
- 单元：`go test ./internal/infrastructure/pdf -run TestRenderBeanListPDF -count=1`
- 单元：`go test ./internal/application/costing -run TestGenerateBeanListPublicationPDF -count=1`
- API：`go test ./internal/interfaces/http/costing -run 'TestBeanListPublication(PDFAPI|PublishAndDraftGenerateStoredPreviewPDF)' -count=1`
- 前端：`node --test src/lib/costing-bean-list-version-ui.test.js --test-name-pattern "generate PDF saves"`
- 手册：`OP_MANUAL_COSTING.md`
