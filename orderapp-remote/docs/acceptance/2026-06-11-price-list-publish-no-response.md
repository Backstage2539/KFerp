# PR-469 商品价格表发布无反馈修复

## 范围
- PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE：修复商品价格表中“点了发布价格表没有反应”的交互回归。
- 当前反馈场景：页面已有 `BOM已失效` 提示，但用户滚动到预览底部点击 `发布价格表` 后没有错误、成功提示或弹窗。
- 追加反馈场景：`BOM已失效` 不应在商品价格表顶部用汇总提示展示，应显示在具体商品行下，并提供到商品档案的跳转；BOM 不支持重新启用，只能复制成新 BOM 后再选择。

## 行为要求
- `发布价格表` 不再因为空预览、缺版本号、缺客户、价格行不完整或 BOM 已失效而静默返回。
- 空预览、缺版本号、缺客户或价格行不完整时，按钮附近直接显示阻断原因；点击按钮后同一阻断原因进入页面错误提示。
- BOM已失效时，页面顶部不展示汇总 banner；具体商品行下显示 `BOM已失效`，提示到商品档案重新选择可用 BOM，并提供 `去商品档案重新选择 BOM` 跳转。
- 点击 `发布价格表` 遇到 BOM已失效时，页面滚动定位到第一条商品行提示，不在顶部重复展示 BOM 阻断文案。
- 失效 BOM 不能重新启用；如需沿用旧结构，先在生产 BOM 复制成新 BOM，再回商品档案选择。
- 满足发布条件时继续走原商品价格表发布接口，发布快照结构不变。

## 验收
- [ ] 打开商品价格表，存在 `BOM已失效` 的商品时，顶部不出现 `BOM已失效：1 款产品依赖...` 汇总 banner。
- [ ] 具体商品行下显示 `BOM已失效`，文案提示到商品档案重新选择可用 BOM，并显示 `去商品档案重新选择 BOM` 按钮。
- [ ] 点击商品行 `去商品档案重新选择 BOM` 后进入商品档案配置抽屉，并提供返回商品价格表入口。
- [ ] 点击 `发布价格表` 遇到 BOM已失效时，页面定位到商品行提示，不能静默无反馈，也不能提示“重新启用 BOM”。
- [ ] 空预览、缺版本号、缺客户或价格行不完整时，点击 `发布价格表` 显示对应阻断原因。
- [ ] 正常满足发布条件时仍能发布价格表并在已发布价格表列表中出现新版本。

## 验证记录
- RED frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed because `CostingView.vue` lacked `priceListPublishBlockedReason`.
- RED support: `go test ./internal/interfaces/http/support -run TestDev469PriceListPublishNoResponseContracts -count=1` failed because PR-469 markers were missing.
- RED frontend follow-up: `node --test src/lib/costing-bean-list-version-ui.test.js` failed while the page still used top-level `inactiveBomWarningCount` and the old `重新启用 BOM` wording.
- RED support follow-up: `go test ./internal/interfaces/http/support -run TestDev469PriceListPublishNoResponseContracts -count=1` failed because docs did not include the product-row BOM warning and 商品档案 jump behavior.
- GREEN frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 26/26.
- GREEN frontend combined: `node --test src/lib/product-settings.test.js src/lib/costing-bean-list-version-ui.test.js src/lib/product-bean-list-split.test.js` passed 174/174.
- GREEN support: `go test ./internal/interfaces/http/support -run TestDev469PriceListPublishNoResponseContracts -count=1` and `go test ./internal/interfaces/http/support -count=1` passed.
- GREEN backend/build: `go test ./...` passed; `npm run build` passed with the existing Vite chunk-size warning; `scripts/verify_kferp.sh changed` and `git diff --check` passed.
