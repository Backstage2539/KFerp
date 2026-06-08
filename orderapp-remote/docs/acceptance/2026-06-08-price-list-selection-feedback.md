# PR-449 商品价格表选品区验收反馈

## 范围
- 商品价格表主页面删除旧产品类型预览卡，直接展示 `Price List / Item Price 生成规则`、选品、平铺价格行、预览和发布/PDF 动作。
- 顶部按钮从 `生成价格表` 改为 `价格表配置`，用于维护版本、样式、归属和来源配置。
- `模板继承规则` 改名为 `计价模式规则`，只通过按钮弹窗展示，不在页面常驻。
- 分类和商品行摘要不再展示父类/子类字样。
- 继承态统一显示 `继承分类`；如果不继承，摘要直接显示实际计价模板或方式。
- 点击分类或商品行的 `计价 / 展示` 摘要后，在弹窗中修改配置，不在选品列表内铺开下拉。
- 没有生成价格行时隐藏 `平铺价格行` 块。
- 生成抽屉预览按当前勾选商品生成；只有下载历史发布快照时才复用该发布版本内容。

## 验收口径
- 主页面旧价格表预览卡不再出现；用户直接在页面配置价格表规则和选品。
- `价格表配置` 抽屉只承载版本、样式、归属、官方来源复制等配置，不再重复展示 `Price List / Item Price 生成规则` 以下内容。
- `计价模式规则` 弹窗展示 `商品 > 子类 > 父类 > 价格表` 和 `group_source=price_list` 快照规则。
- 选品列表默认只用于勾选、识别商品、查看摘要。
- 弹窗编辑不改变后端模型、不新增 API 或数据库字段。
- 价格行仍用于发布快照冻结最终价、模板来源、Pricing Rule 版本、成本来源和客户引用；没有可发布价格行时不单独显示空块。
- 预览不能因当前已发布版本内容为空而空白。

## 当前证据
- RED frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed before implementation because config dialog, inherited summary and current-selection preview behavior were missing.
- GREEN frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 16/16; `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js src/lib/product-settings.test.js` passed 165/165.
- RED follow-up frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed after browser feedback because `price-list-page-config` and `计价模式规则` modal behavior were missing.
- GREEN follow-up frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 17/17; after drawer label tightening, `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js src/lib/product-settings.test.js` passed 166/166.
- RED support: support contracts failed before update because old PR-309/PR-445/PR-447 markers still expected current publication content, inline parent/child wording, or old panel toggles.
- GREEN support: `go test ./internal/interfaces/http/support -run 'TestDev449PriceListSelectionFeedbackContracts|TestDev447PriceListSelectionCompactContracts|TestDev445PriceListInlineSelectionConfigContracts|TestDev309BeanListVersionDownloadDocsAndWiring' -count=1` passed; `go test ./internal/interfaces/http/support -count=1` passed.
- Build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with the existing Vite chunk-size warning.
- Local browser: `http://127.0.0.1:5185/vue-shell/?view=costing` loaded current branch through development API. 生成价格表抽屉 showed `category-pricing-summary=2`、`product-compact-status=2`、inline config panels `0`、empty `flat-price-row-editor=0`；选品摘要只显示 `继承分类` 或实际摘要，不出现 `父类：`、`子类：`、`继承父类`、`继承子类`；预览从 `0 款` 修正为 `2 款`，显示两个商品；分类计价、商品计价、商品展示弹窗均可打开；当前验收 URL console errors `0`。截图：`/tmp/pr448-local-price-list-selection-feedback.png`。
- Local browser follow-up: mocked local Vue shell on `http://127.0.0.1:5187/vue-shell/?view=costing` with 599x752 viewport rendered `price-list-page-config=1`, old `.collapsible-bean-section=0`, `.price-list-model-panel=0`, `Price List / Item Price 生成规则` appeared once on the main page, `价格表配置` button count `1`, and `计价模式规则` button count `1`. Clicking `价格表配置` opened a drawer with `aria-label="价格表配置"` and no duplicate generation rules. Clicking `计价模式规则` opened a modal with `商品 > 子类 > 父类 > 价格表` and `group_source=price_list`; page text no longer contained `模板继承规则`; console errors `0`. Screenshot: `/tmp/pr449-price-list-page-config-followup.png`.
- Previous deploy before browser-comment follow-up: feature branch was pushed and merged to `develop`; `origin/develop=f15d99508044c0b90dc85c4ef3c272f039a61644` deployed to development. Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608113859`. Deployed browser showed compact selection summaries, no `父类/子类` wording, no empty `平铺价格行` block, preview rendered 2 selected products with prices, dialogs opened, console errors `0`. Screenshot: `/tmp/pr449-deployed-price-list-selection-feedback.png`.
- Post-merge follow-up: after merging latest `origin/develop`, `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js src/lib/product-settings.test.js` passed 166/166; `go test ./internal/interfaces/http/support -count=1` passed; `scripts/verify_kferp.sh changed` passed; `scripts/verify_kferp.sh frontend-build` passed with the existing Vite chunk-size warning; `git diff --check` passed.
- Deploy hardening: first follow-up deploy build failed inside Docker `go test ./...` because `internal/interfaces/http/costing` still expected old `productPriceListPreviewSections` / `.collapsible-bean-section` markers. Updated those source contracts to `price-list-page-config` and current inline configuration markers; `go test ./internal/interfaces/http/costing -count=1`, `go test ./internal/interfaces/http/support -count=1`, and local `go test ./...` passed.
- Pending: merge the browser-comment follow-up to `develop`, deploy development stack, and run deployed browser acceptance on `https://erp.qacoohee.com/app/vue-shell/?view=costing`.
