# PR-462-PRICING-RULE-TRIAL-OUTPUT-BOM-OPERATION-SELECT 商品价格管理试算 BOM 版本与工序选择验收记录

## 范围
- 商品价格管理的价格计算模板试算支持选择 `试算BOM版本` 和 `工序`。
- BOM 版本只按 `production_boms.output_product_id=product_id` 查找产出当前商品的 active BOM / published 版本，不读商品绑定 BOM，不读旧 `product_bom_sources` 或 `product_bom_items` 兜底。
- 缺产出 BOM 明细时 `BOM+工序成本` 为 0 并提示维护生产 BOM；试算仍不写商品、BOM、Pricing Rule、商品价格表、发布快照或订单。

## 验收要点
- `PR439-20260606182321 工厂量单商品` 应通过 `BOM-000539 ... V002` 这类产出当前商品的生产 BOM 版本计算出非 0 BOM/工序成本。
- 选择其他 active/published BOM 版本或 active 工序模板后，本次试算的 BOM/工序明细、价格瀑布、公式步骤和试算单价随选择重算。
- 没有产出当前商品的生产 BOM 时，只显示 0 和警告，不按发布售价或旧商品 BOM 反推。

## 自动化证据
- RED：`go test ./internal/application/costing -run 'TestPricingRuleTrialUsesSelectedOutputBomVersionAndOperationTemplate' -count=1` 先因结果缺少 `bom_version_options` / `operation_template_options` 和选择字段失败。
- RED：`go test ./internal/infrastructure/postgres/costing -run TestPricingRuleTrialProductionCostUsesOutputProductBomOnly -count=1` 先因仓储没有 `LoadPricingRuleTrialProductionOptions` 和产出商品 BOM 版本选项失败。
- RED：`node --test src/lib/product-settings.test.js` 先因试算 payload 和抽屉缺少 `bom_version_id` / `operation_template_id` 失败。
- GREEN：服务层按选择的 BOM 版本和工序模板加载明细，并在无产出 BOM 明细时忽略旧商品成本汇总。
- GREEN：仓储试算明细函数不包含 `product_bom_sources`、`product_bom_items`、`inherit_current` 或 `inherit_version` 兜底。

## 部署状态
- 按 Van 要求，本次只提交功能分支，不合并、不部署。
