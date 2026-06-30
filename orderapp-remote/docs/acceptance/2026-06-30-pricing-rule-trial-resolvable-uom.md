# PR-507-PRICING-RULE-TRIAL-RESOLVABLE-UOM 价格试算可解析销售单位

## 目标
- 商品价格管理的 `价格试算` 选择商品后，`销售单位` 继承该商品有效默认销售单位或销售规格。
- 销售单位候选只来自当前商品可解析的单位换算，不再因为全局单位字典存在就展示 `盒`、`袋`、`条`。
- 后端试算接口拒绝不可解析销售单位，避免静默按 `kg` 计算出误导价格。

## 验收场景
- SKU 有 `1 袋 = 0.227 kg` 时，试算抽屉候选包含 `袋`、`kg`、`g` 和可换算的标准重量单位，默认选中 `袋`。
- SKU 只有净含量 `227 g` 且没有显式 conversion JSON 时，试算仍能把该销售规格解析到 `kg/g`。
- 商品没有 `盒` 换算时，前端不展示 `盒`；直接调用 `/api/costing/pricing-rule-trial` 传 `quote_unit=盒` 返回错误，提示维护销售规格或单位换算。

## 证据
- RED：`node --test src/lib/product-settings.test.js` 在新增候选测试下失败，旧逻辑仍返回全局 `kg/g/盒/袋/磅/条`。
- RED：`go test ./internal/application/costing -run 'TestPricingRuleTrialRejectsUnresolvableQuoteUnit' -count=1 -v` 失败，旧服务仍接受不可解析 `盒` 并按 `kg` 兜底。
- GREEN targeted：
  - `node --test src/lib/product-settings.test.js`
  - `go test ./internal/application/costing -run 'TestPricingRuleTrialRejectsUnresolvableQuoteUnit' -count=1 -v`
  - `go test ./internal/interfaces/http/costing -run 'TestPricingRuleTrialAPIRejectsUnresolvableQuoteUnit|TestPricingRuleTrialAPI$' -count=1 -v`

## 手册
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
