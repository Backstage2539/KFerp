# 2026-05-20 产品豆单生成区收敛验收

## 范围

- 产品豆单预览按商用批发、挂耳、零售、生豆四个类型支持收起和展开。
- 生豆豆单在产品豆单页面有独立预览区。
- 豆单范围放在产品豆单页面顶部，作为页面全局范围。
- 生豆豆单预览区可直接修改每个档位销售价，并可保存为当前豆单范围下的生豆豆单草稿。
- 生豆 SKU 没有挂到带生豆模板的分类时，生豆卡片必须提示无法生成生豆价格，并引导去 SKU设置 调整分类。
- 外层删除“生成挂耳豆单”按钮，挂耳通过生成抽屉的豆单类型选择。
- 生成抽屉不再二次选择发布归属或客户，归属来自当前“豆单范围”。
- 生成抽屉不再提供“复制已有豆单配置”。

## 验收点

- [x] `CostingView.vue` 有 `beanListPreviewCollapsed` 和四个类型的 `toggleBeanListPreviewSection(...)`。
- [x] 版本列表和生成抽屉的豆单类型均包含“挂耳豆单”。
- [x] 页面包含“生豆豆单”预览区，读取 `green_bean_list` 和 `green_bean_sale_tiers`。
- [x] 页面顶部包含全局 `versionListScope`，豆单版本列表内不再包含范围下拉。
- [x] 生豆豆单预览区包含 `green-inline-price-editor`，修改价格复用 `setGreenBeanTierPrice(...)`。
- [x] 页面包含“保存生豆价格”和 `saveGreenBeanPriceDraft`，保存时调用 `/api/costing/bean-list/drafts` 并固定 `listType: 'green'`。
- [x] `missing_green_bean_template` 会展示“未挂到带生豆模板的分类，无法生成生豆价格”的提示。
- [x] 页面源码不再包含“生成挂耳豆单”“复制已有豆单配置”“selectedCopyPublicationID”“applyCopiedBeanListPublicationConfig”。
- [x] 生成抽屉显示“当前归属”，不再显示“发布归属”下拉或客户选择。

## 证据

- `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/product-bean-list-split.test.js`
- `go test ./internal/interfaces/http/costing -run 'TestCostingViewSupportsConfigurableBeanListPublishingWorkflow|TestCostingViewPDFSupportsDripBeanListPricing|TestCostingViewHasCollapsibleBeanListPreviewSections' -count=1`
