# PR-447 商品价格表选品区降噪

## 范围
- 商品价格表生成抽屉的“选择分类和产品”区域默认不常驻显示分类计价、商品行计价、标签和标红词配置。
- 分类头 A 位置保留分类计价入口，但默认显示摘要和覆盖状态，点击“计价”后弹窗编辑配置。
- 商品勾选行 B 位置保留商品行计价、标签和标红词，但默认显示计价/展示摘要，点击“计价”或“展示”后展开配置。
- 不新增 API、数据库字段、Pricing Rule 字段、阶梯模板字段或发布快照字段。

## 验收口径
- 默认选品列表只服务勾选、识别名称、查看状态摘要。
- 分类计价、商品行计价、标签和标红词只在用户点开对应操作后出现。
- 已覆盖分类或商品显示轻量状态标识。
- 发布快照仍按 `商品 > 子类 > 父类 > 价格表` 解析。

## 当前证据
- RED frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed before implementation because `category-pricing-summary` was missing.
- GREEN frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 15/15; `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js src/lib/product-settings.test.js` passed 164/164.
- RED support: `go test ./internal/interfaces/http/support -run TestDev447PriceListSelectionCompactContracts -count=1` failed before docs/req seeds because `PR-447-PRICE-LIST-SELECTION-COMPACT` was missing from `req_store.go`.
- GREEN support: `go test ./internal/interfaces/http/support -run TestDev447PriceListSelectionCompactContracts -count=1` passed; `go test ./internal/interfaces/http/support -run 'TestDev44(5|7)PriceList' -count=1` passed.
- Build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing Vite chunk-size warning; `scripts/verify_kferp.sh changed`; `git diff --check`.
- Browser: local mocked Vue shell `http://127.0.0.1:5182/vue-shell/?view=costing` rendered 商品价格表生成抽屉。默认状态：`category-pricing-summary=1`、`product-compact-status=1`、分类计价配置 0、商品计价配置 0、商品展示配置 0，页面不常驻显示 `父类计价`、`子类计价`、`商品行计价`。点开分类 `计价` 后出现父类/子类计价；点开商品 `计价` 后出现商品行计价；点开商品 `展示` 后出现标签和 `标红词，用逗号分隔` 输入框。浏览器控制台错误 0，截图 `/tmp/pr447-price-list-selection-compact.png`。
- Deployed browser: `https://erp.qacoohee.com/app/vue-shell/?view=costing` 的商品价格表生成抽屉默认状态显示 2 个分类计价摘要和 2 个商品摘要，分类/商品计价/展示配置面板计数均为 0；点开分类 `计价`、商品 `计价`、商品 `展示` 后分别出现对应配置，浏览器控制台错误 0，截图 `/tmp/pr447-deployed-price-list-selection-compact.png`。
