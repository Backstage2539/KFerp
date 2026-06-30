# PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS 验收记录

## Scope
- 商品价格管理 `价格试算` 结果区中，`BOM+工序成本`、`其他成本`、`加价增加` 三张瀑布卡片可点击打开只读 `试算说明`。
- 后端 `PricingRuleTrialResult` 返回 `other_cost_details` 和 `profit_explanation`，前端解释当前试算结果，不保存任何数据。

## RED
- `go test ./internal/application/costing -run 'TestPricingRuleTrial' -count=1`：在实现前失败，原因是试算结果缺少 `other_cost_details` 和 `profit_explanation` 字段。
- `go test ./internal/interfaces/http/costing -run 'TestPricingRuleTrial' -count=1`：在实现前失败，原因是 API JSON 不返回新解释字段。
- `cd frontend-vue-shell && node --test src/lib/product-settings.test.js`：在实现前失败，原因是瀑布卡片没有点击入口和 `试算说明` 面板。
- `go test ./internal/interfaces/http/support -run TestDev510PricingRuleTrialWaterfallExplanationsContracts -count=1`：在补文档前失败，原因是 PR-510 文档和 seed 标记缺失。

## GREEN
- `go test ./internal/application/costing -run 'TestPricingRuleTrial' -count=1`
- `go test ./internal/interfaces/http/costing -run 'TestPricingRuleTrial' -count=1`
- `go test ./internal/interfaces/http/support -run TestDev510PricingRuleTrialWaterfallExplanationsContracts -count=1`
- `cd frontend-vue-shell && node --test src/lib/product-settings.test.js`
- `scripts/verify_kferp.sh frontend-build`

## Browser
- Development smoke: authenticated `GET /app/vue-shell?view=productPriceManagement` returned `200`; `/app/api/req/product?limit=1000` exposed `PR-510-PRICING-RULE-TRIAL-WATERFALL-EXPLANATIONS`; deployed source and Vue bundle contain `other_cost_details` / `profit_explanation` / `试算说明` markers.
- API smoke: `pricing_rule_id=11` with `SKU-000573` returned `BOM+工序成本 164.51/kg` and `37.35/袋`; first material row changed from `40.82/kg` to `9.27/袋`, confirming the amount and unit both follow the selected sales unit.
- Browser smoke: in `productPriceManagement`, opened `价格试算`, selected `榛巧拼配 227g袋装 / SKU-000573`, and confirmed sales unit inherited as `袋`. Switching to `kg` updated waterfall and BOM detail amounts. Clicking `BOM+工序成本`, `其他成本`, and `加价增加` opened readable `试算说明` panels with expected details and no console errors.
- Narrow viewport smoke: at `599x752`, the `加价增加` explanation panel had no detected text overflow and no console errors.
