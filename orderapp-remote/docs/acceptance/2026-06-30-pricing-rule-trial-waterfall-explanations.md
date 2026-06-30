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
- Pending: development 部署后，在 `productPriceManagement` 打开价格试算，选择 `榛巧拼配227g袋装` / `SKU-000573`。
- Pending: 分别用 `kg` 和 `袋` 试算，确认 BOM 明细金额和单位随销售单位变化。
- Pending: 点击 `BOM+工序成本`、`其他成本`、`加价增加`，确认说明面板可读、无重叠、无控制台错误。
