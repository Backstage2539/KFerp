# PR-448 商品价格表选品区验收反馈

## 范围
- 分类和商品行摘要不再展示父类/子类字样。
- 继承态统一显示 `继承分类`；如果不继承，摘要直接显示实际计价模板或方式。
- 点击分类或商品行的 `计价 / 展示` 摘要后，在弹窗中修改配置，不在选品列表内铺开下拉。
- 没有生成价格行时隐藏 `平铺价格行` 块。
- 生成抽屉预览按当前勾选商品生成；只有下载历史发布快照时才复用该发布版本内容。

## 验收口径
- 选品列表默认只用于勾选、识别商品、查看摘要。
- 弹窗编辑不改变后端模型、不新增 API 或数据库字段。
- 价格行仍用于发布快照冻结最终价、模板来源、Pricing Rule 版本、成本来源和客户引用；没有可发布价格行时不单独显示空块。
- 预览不能因当前已发布版本内容为空而空白。

## 当前证据
- RED frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed before implementation because config dialog, inherited summary and current-selection preview behavior were missing.
- GREEN frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 16/16; `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js src/lib/product-settings.test.js` passed 165/165.
- RED support: support contracts failed before update because old PR-309/PR-445/PR-447 markers still expected current publication content, inline parent/child wording, or old panel toggles.
- GREEN support: `go test ./internal/interfaces/http/support -run 'TestDev448PriceListSelectionFeedbackContracts|TestDev447PriceListSelectionCompactContracts|TestDev445PriceListInlineSelectionConfigContracts|TestDev309BeanListVersionDownloadDocsAndWiring' -count=1` passed; `go test ./internal/interfaces/http/support -count=1` passed.
- Build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with the existing Vite chunk-size warning.
- Local browser: `http://127.0.0.1:5185/vue-shell/?view=costing` loaded current branch through development API. 生成价格表抽屉 showed `category-pricing-summary=2`、`product-compact-status=2`、inline config panels `0`、empty `flat-price-row-editor=0`；选品摘要只显示 `继承分类` 或实际摘要，不出现 `父类：`、`子类：`、`继承父类`、`继承子类`；预览从 `0 款` 修正为 `2 款`，显示两个商品；分类计价、商品计价、商品展示弹窗均可打开；当前验收 URL console errors `0`。截图：`/tmp/pr448-local-price-list-selection-feedback.png`。
- Pending: merge to `develop`, deploy development stack, and run deployed browser acceptance on `https://erp.qacoohee.com/app/vue-shell/?view=costing`.
