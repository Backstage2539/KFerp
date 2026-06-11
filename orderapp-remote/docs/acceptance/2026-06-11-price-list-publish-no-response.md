# PR-469 商品价格表发布无反馈修复

## 范围
- PR-469-PRICE-LIST-PUBLISH-NO-RESPONSE：修复商品价格表中“点了发布价格表没有反应”的交互回归。
- 当前反馈场景：页面已有 `BOM已失效` 提示，但用户滚动到预览底部点击 `发布价格表` 后没有错误、成功提示或弹窗。

## 行为要求
- `发布价格表` 不再因为空预览、缺版本号、缺客户、价格行不完整或 BOM 已失效而静默返回。
- 不可发布时，按钮附近直接显示阻断原因；点击按钮后同一阻断原因进入页面错误提示。
- BOM已失效时阻断文案提示先重新启用或修正生产 BOM 后再发布。
- 满足发布条件时继续走原商品价格表发布接口，发布快照结构不变。

## 验收
- [ ] 打开商品价格表，存在 `BOM已失效` 的商品时，预览底部 `发布价格表` 附近显示阻断原因。
- [ ] 点击 `发布价格表` 后页面显示 `BOM已失效：请重新启用 BOM 后再发布价格表`，不能静默无反馈。
- [ ] 空预览、缺版本号、缺客户或价格行不完整时，点击 `发布价格表` 显示对应阻断原因。
- [ ] 正常满足发布条件时仍能发布价格表并在已发布价格表列表中出现新版本。

## 验证记录
- RED frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` failed because `CostingView.vue` lacked `priceListPublishBlockedReason`.
- RED support: `go test ./internal/interfaces/http/support -run TestDev469PriceListPublishNoResponseContracts -count=1` failed because PR-469 markers were missing.
- GREEN frontend: `node --test src/lib/costing-bean-list-version-ui.test.js` passed 25/25.
- GREEN frontend combined: `node --test src/lib/product-settings.test.js src/lib/costing-bean-list-version-ui.test.js` passed 156/156.
- GREEN support: `go test ./internal/interfaces/http/support -run TestDev469PriceListPublishNoResponseContracts -count=1` and `go test ./internal/interfaces/http/support -count=1` passed.
- GREEN backend/build: `go test ./...` passed; `npm run build` passed with the existing Vite chunk-size/plugin timing warnings.
