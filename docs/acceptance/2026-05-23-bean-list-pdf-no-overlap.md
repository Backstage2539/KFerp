# PR-333-BEAN-LIST-PDF-NO-OVERLAP 验收

## 需求

- 豆单 PDF 紧凑排版不得让分类标题、商品卡和报价块互相覆盖。
- 卡片行高度不能低于该密度下的实际可渲染高度；不能为了塞进当前页把报价块画到卡片外。
- 分类标题必须与第一行商品一起预留安全高度；当前组最后一行应尽量为下一组标题和首行商品预留空间。
- 已生成的 v3 问题缓存必须按 `bean-list-preview-style-v4` 重新生成。

## 证据

- RED：`go test ./internal/infrastructure/pdf -run 'Test(CardRowLayoutDoesNotSqueezeBelowRenderableHeight|RenderGroupKeepsTitleWithFirstCardRow)' -count=1` 先失败，分别暴露卡片行被压低和标题未与首行同页。
- GREEN：`TestCardRowLayoutDoesNotSqueezeBelowRenderableHeight`、`TestRenderGroupKeepsTitleWithFirstCardRow` 和 `TestRenderBeanListPDFCompactsCardRowsBeforeAddingBlankPage` 通过。
- API/cache：`TestGenerateBeanListPublicationPDFRegeneratesStaleCacheKey` 覆盖旧 v3 缓存按 v4 重新生成。

## 验收点

- 用户截图中的“2、庄园精品豆...”分类标题不能再被两张商品卡的“报价”文字或价格块压住。
- 若当前页空间不足，标题和第一行商品整体换页；若可以通过 compact/dense 安全放下，则保留在当前页。
