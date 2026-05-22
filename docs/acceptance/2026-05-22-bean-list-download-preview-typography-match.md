# PR-323-BEAN-LIST-DOWNLOAD-MATCH-PREVIEW-TYPOGRAPHY

## 验收目标
- 产品豆单生成抽屉中的预览和下载 PDF 使用同一套卡片视觉语言。
- 下载 PDF 的版本标题、分类条、商品卡、商品名称、出品建议、风味/特点、绿色/蓝色报价块在字体大小、粗字重和排版上与预览一致。
- 已保存的 `bean-list-preview-style-v1` 缓存视为旧排版，重新下载时以 `bean-list-preview-style-v2` 生成并覆盖。

## 证据
- RED：`go test ./internal/infrastructure/pdf -run TestRenderBeanListPDFPreviewTypographyMatchesVuePreview -count=1` 曾失败，缺少预览粗字重/字号标记。
- RED：`go test ./internal/application/costing -run TestGenerateBeanListPublicationPDFRegeneratesStaleCacheKey -count=1` 曾失败，旧 v1 缓存被复用。
- 单元：`go test ./internal/infrastructure/pdf -run 'TestRenderBeanListPDFPreviewTypographyMatchesVuePreview|TestRenderBeanListPDFUsesPreviewCardStyle' -count=1`
- 单元：`go test ./internal/application/costing -run 'TestGenerateBeanListPublicationPDFSavesAndReusesAsset|TestGenerateBeanListPublicationPDFRegeneratesStaleCacheKey' -count=1`
- 手册：`OP_MANUAL_COSTING.md`
