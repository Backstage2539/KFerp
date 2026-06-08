# PR-459-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL 验收记录

## 目标
- 商品价格管理的价格计算模板试算展示价格瀑布，让 `BOM+工序成本 + 其他成本 + 损耗增加 + 加价增加 + 税额 + 取整调整 = 试算单价` 可直接查看。
- 展示 `BOM+工序成本明细`，包含物料成本明细、工序成本明细、物料合计、工序合计和总计。
- 缺 BOM/工序成本时显示 0 和警告，不反推发布售价快照。

## 本地验证
- RED 服务/API：`go test ./internal/application/costing -run 'TestPricingRuleTrial(UsesBomCostTemplateFormula|DoesNotInferCostFromPublishedPriceSnapshotWhenBomCostMissing)' -count=1`、`go test ./internal/interfaces/http/costing -run TestPricingRuleTrialAPI -count=1` 在实现前因 `PricingRuleTrialBaseCostDetail`、`base_cost_details`、`yield_loss_amount` 等字段缺失失败。
- RED 前端：`node --test src/lib/product-settings.test.js` 在实现前因缺少 `BOM+工序成本明细`、`损耗增加`、`加价增加`、`pricing-rule-trial-waterfall` 失败。
- GREEN 服务/API：目标服务测试和 `TestPricingRuleTrialAPI` 已通过，覆盖 BOM/工序明细、价格瀑布字段和缺 BOM 不反推。
- GREEN 前端：`node --test src/lib/product-settings.test.js` 已通过，覆盖瀑布、明细、加号/等号、删除英文来源/状态和反推文案。
- GREEN 完整本地验证：`go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -run 'TestPricingRuleTrial|TestDev45(2|4|6|7|9)|TestLoadProductInputs' -count=1`、`npm run build`、`go test ./...`、`scripts/verify_kferp.sh changed`、`git diff --check` 均已通过。

## 待浏览器验收
- 本次按 Van 要求仅完成开发，不合并、不部署；浏览器验收保留到后续合并部署后执行。
- 浏览器进入 `商品价格管理`，打开任意模板 `试算`，选择有 BOM/工序成本的商品，确认价格瀑布、物料成本明细、工序成本明细和公式步骤可见。
- 选择缺 BOM/工序成本的商品，确认 `BOM+工序成本` 为 0、有感叹号和警告，不出现 `发布售价快照反推`。
- 重复试算前后确认商品价格表、发布快照、Pricing Rule 模板、订单和操作日志业务写记录没有被改动。
