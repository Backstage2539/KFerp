# PR-455-PRICING-RULE-TRIAL-PR439-UNIT 验收记录

## 范围
- 商品价格管理 `价格计算模板试算` 抽屉删除 `重新试算` 按钮。
- 试算抽屉和结果区删除 `售价后附加成本`。
- 报价单位改为全局单位字典下拉。
- `PR439-20260606182321 熟豆下单商品` 无 BOM/工序成本但存在已发布 `88.5/kg` 价格快照时，试算 API 按模板公式反推本次成本基数，并展示 `发布售价快照` 公式节点。

## 自动化证据
- RED 前端：`node --test src/lib/product-settings.test.js` 曾失败，原因是 `buildPricingRuleTrialPayload` 仍提交 `post_markup_costs`，Vue 抽屉仍缺少 `pricingRuleTrialQuoteUnitOptions` / 自动试算。
- RED 后端：`go test ./internal/application/costing -run 'TestPricingRuleTrial(UsesBomCostTemplateFormula|InfersCostFromPublishedPriceSnapshotWhenBomCostMissing)' -count=1` 曾失败，原因是服务返回 `product cost required`。
- GREEN 前端：`node --test src/lib/product-settings.test.js` 通过 126/126。
- GREEN 后端：`go test ./internal/application/costing -run 'TestPricingRuleTrial(UsesBomCostTemplateFormula|InfersCostFromPublishedPriceSnapshotWhenBomCostMissing)' -count=1` 通过。

## 验收口径
- 商品价格管理每个价格计算模板行仍显示 `试算`。
- 打开试算抽屉后，页面不显示 `重新试算`，不显示 `售价后附加成本`。
- 报价单位选项来自全局单位字典，例如 `kg`、`lb`、`袋`。
- 选择 `PR439-20260606182321 熟豆下单商品` 和 `kg` 后自动试算成功，试算单价为 `88.5/kg`。
- 公式步骤包含 `发布售价快照`、成本基数、损耗率、利润/加价、税费、取整和试算单价。
- 试算结果不写入 Pricing Rule 模板、商品价格表、发布快照或订单。
