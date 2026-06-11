# PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX

## Scope
- 商品价格表生成草稿按工作台、价格表归属、客户和商品类型保存，刷新后恢复计价方式、价格模板、阶梯模板、分类覆盖、商品覆盖和手工价格覆盖。
- 商品价格表预览同时接收 `按价格模板计算` 和 `按阶梯模板价计算` 的价格计算模板试算结果。
- 商品价格表和商品档案只使用 `product_catalog` 用途的商品分组模板，优先选择已有商品归类的模板，避免 `熟豆-红岩拼配` 从 `咖啡熟豆 / 意式拼配豆` 看起来回到未分类。

## Verification
- RED support: `go test ./internal/interfaces/http/support -run TestDev467PriceListGenerationPersistencePreviewGroupFixContracts -count=1` failed because `req_store.go` missed `PR-467-PRICE-LIST-GENERATION-PERSISTENCE-PREVIEW-GROUP-FIX`.
- GREEN frontend: `node --test src/lib/product-price-list-draft.test.js src/lib/costing-bean-list-version-ui.test.js`.
- GREEN frontend: `node --test src/lib/product-price-list-selection.test.js src/lib/product-price-list-types.test.js src/lib/business-grouping.test.js src/lib/product-settings.test.js`.
- GREEN support/API contract: `go test ./internal/interfaces/http/support -run TestDev467PriceListGenerationPersistencePreviewGroupFixContracts -count=1`.
- 2026-06-11 follow-up API: `POST /api/costing/pricing-rule-trial` with `pricing_rule_id=11`, `product_id=550` returned `熟豆-红岩拼配`, default BOM `BOM-004014 / V002 / 723`, `42.64/lb` and `92.03/kg`.
- 2026-06-11 follow-up browser read: deployed 商品价格表 showed 红岩平铺行 had inherited `按价格计算模板计算：价格表` and `咖啡熟豆磅装模板` with unit `/kg`, but the final price cell was empty.
- 2026-06-11 RED/GREEN frontend: `node --test orderapp-remote/frontend-vue-shell/src/lib/costing-bean-list-version-ui.test.js` failed before the price-list preview could clear current-row pricing-rule trial error cache or derive product-level quote units, then passed 24/24 after the fix.
- 2026-06-11 deployed follow-up still failed before the second patch: the browser flat row input for 红岩 stayed `0`, while PR439 showed `39.9` from an existing snapshot. `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/costing-bean-list-version-ui.test.js` then failed before numeric `product_key` was accepted as a trial payload fallback, and passed 155/155 after the fallback.
- 2026-06-11 CDP browser root cause: with `按价格计算模板计算 / 咖啡熟豆磅装模板` selected, independent browser reproduction showed zero `/api/costing/pricing-rule-trial` requests. The flat row had `tier_pricing_rule_id=0`, and the payload helper used nullish fallback so it stopped before reading `pricing_rule_id=11`.
- 2026-06-11 RED frontend: `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js` failed because a `pricing_rule` flat row with `tier_pricing_rule_id: 0` returned `null` trial payload instead of falling back to `pricing_rule_id`.
- 2026-06-11 GREEN frontend: `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js` passed 131/131 after pricing-rule id selection changed to first positive id fallback.
- 2026-06-11 GREEN frontend combined: `node --test orderapp-remote/frontend-vue-shell/src/lib/costing-bean-list-version-ui.test.js orderapp-remote/frontend-vue-shell/src/lib/bean-list-pdf.test.js` passed 51/51.

## 浏览器验收
- 打开部署后的商品价格表，选择 `咖啡熟豆`。
- 确认 `选择分类和产品` 中 `熟豆-红岩拼配` 位于 `意式拼配豆` 下，不在未分类。
- 将 `熟豆-红岩拼配` 计价设为 `按价格模板计算` / `咖啡熟豆磅装模板`，确认价格表预览显示价格。
- 刷新页面，确认计价方式和模板仍保留。
- 将 `熟豆-红岩拼配` 计价设为 `按阶梯模板价计算` / `咖啡熟豆`，确认价格表预览显示两个阶梯价格。
- 重新部署后再次打开同一页面，确认商品仍在 `咖啡熟豆 / 意式拼配豆`，没有回到未分类。
