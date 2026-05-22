# PR-332-BEAN-LIST-PDF-DENSE-PAGINATION 验收

## 范围
- 产品豆单公开页和版本列表下载的预览样式 PDF。
- 服务器端 `bean_list_publication_assets` PDF 缓存。

## 验收点
- PDF 生成器在卡片行放不下当前页时，先尝试 normal、compact、dense 三档密度，压缩卡片内边距、字号、行距和报价块高度，必要时截断过长风味/特点文案。
- 只有 dense 密度仍无法容纳时才换页，避免分类标题后留下大面积空白。
- 截图形态的“工厂量单 1 个卡片 + 庄园精品豆 2 个卡片”测试数据生成 1 页 PDF。
- PDF 缓存键升级为 `bean-list-preview-style-v3`；旧 `bean-list-preview-style-v2` 缓存下载时必须重新生成。

## 自动化证据
- `go test ./internal/infrastructure/pdf -run TestRenderBeanListPDFCompactsCardRowsBeforeAddingBlankPage -count=1`
- `go test ./internal/application/costing -run 'TestGenerateBeanListPublicationPDF(SavesAndReusesAsset|RegeneratesStaleCacheKey)' -count=1`
- `go test ./internal/interfaces/http/costing ./internal/application/costing -count=1`
