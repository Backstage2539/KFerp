# PR-445 价格表计价配置内联到选品位置

## Scope
- 商品价格表生成抽屉不再单独展示父类计价配置、子类计价配置或商品行配置表。
- 分类层计价配置直接放在“选择分类和产品”的分类头部 A 位置。
- 商品覆盖计价配置直接放在商品勾选行 B 位置。
- 继承和发布快照逻辑不变，仍按 `商品 > 子类 > 父类 > 价格表` 解析。

## RED
- `node --test src/lib/costing-bean-list-version-ui.test.js`：实现前失败，因为旧独立配置表仍在 `Price List / Item Price 生成规则` 区，A/B 位置没有计价控件。

## Acceptance
- [x] `Price List / Item Price 生成规则` 区只维护价格表默认计价，不再列出独立的父类/子类/商品配置表。
- [x] 分类头 A 位置可设置父类计价和子类计价。
- [x] 商品勾选行 B 位置可设置商品行计价覆盖。
- [x] 平铺价格行仍按原继承顺序生成，不改变发布快照字段。

## GREEN
- `node --test src/lib/costing-bean-list-version-ui.test.js` passed.
- `node --test src/lib/costing-bean-list-version-ui.test.js src/lib/bean-list-pdf.test.js src/lib/product-settings.test.js` passed.
- `go test ./internal/interfaces/http/support -run TestDev445PriceListInlineSelectionConfigContracts -count=1` passed.
- `go test ./internal/interfaces/http/support -count=1` passed.
- `npm run build` in `frontend-vue-shell` passed with existing Vite chunk-size warning.
- Local browser: mocked Vue shell on `http://127.0.0.1:5178/vue-shell/?view=costing` opened the 生成价格表 drawer. `Price List / Item Price 生成规则` only showed price-list default config; 分类头 A 位置 showed 父类计价/子类计价; 商品勾选行 B 位置 showed 商品行计价; no console/page errors. Screenshot: `/tmp/pr445-price-list-inline-selection-config.png`.
