# PR-466-PRICE-LIST-TIER-TEMPLATE-TRIAL-PREVIEW

## Scope
- 商品价格表预览中，`按阶梯模板价计算` 的平铺价格行也要使用档位引用的价格计算模板试算价。
- `红岩拼配` 引用 `咖啡熟豆` 阶梯价模板时，两个阶梯都引用 `咖啡熟豆磅装模板`，预览应显示两个阶梯价格。
- 两个阶梯引用同一个价格计算模板且公式相同时，两个阶梯单价一致。

## Verification
- RED: `node --test src/lib/product-settings.test.js` failed because `priceTablePricingRuleTrialPayload` returned `null` for `pricing_mode=tier_template`.
- GREEN: `node --test src/lib/product-settings.test.js`.

## Manual Acceptance
- 打开 商品价格表，选择包含 `红岩拼配` 的商品类型。
- 确认 `红岩拼配` 计价为 `按阶梯模板价计算`，阶梯价模板为 `咖啡熟豆`。
- 确认 `咖啡熟豆` 两个阶梯都引用 `咖啡熟豆磅装模板`。
- 查看价格表预览，确认 `红岩拼配` 显示两个阶梯价格，且两个阶梯单价一致。
