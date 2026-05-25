# PR-374-COMPOSABLE-PRODUCT-PRICING 验收记录

## 范围
- 不新增固定 `cost_model` 枚举。
- 商品价格由商品配置、单位模板、BOM 用料、阶梯价模板和价格表生成规则组合生成。
- 速溶盒装按“每盒 10 条、每条 10g”的 BOM 件数成本生成 `元/盒` 价格。
- 产品价格表、发布快照、PDF 和录单阶梯保留自定义单位。

## 测试数据场景
1. 全局单位字典存在 `条` 和 `盒`。
2. 单位模板：成品库存单位 `盒`，报价单位 `盒`，录单单位 `盒`，换算 `1 盒 = 10 条` 和/或 `1 盒 = 0.1 kg`，整数单位开启。
3. 物料：`速溶咖啡条装原料`，单位 `条`，采购价按每条维护，备注 `1 条 10g`。
4. SKU：`速溶盒装`，挂到 `速溶咖啡 / 盒装速溶`。
5. BOM：`速溶咖啡条装原料` 消耗单位 `个/盒`，用量 `10`；盒子、标签等包材也按 `个/盒` 配置。
6. 商品配置：绑定盒报价阶梯模板，价格表生成规则选成本加成，展示单位继承报价单位。

## 验收点
- 成本核算返回的 `commercial_wholesale_tiers[].display_unit` 为 `盒`。
- `commercial_wholesale_tiers[].price_per_unit` 来自每盒 BOM 成本和阶梯利润率，不来自熟豆烘焙公式。
- PDF 价格单位展示为 `盒`。
- 录单阶梯展示为 `元/盒`，不会换算成 `元/磅`。

## 自动验证
- `go test ./internal/domain/costing -run 'TestComposableProductPricingUsesBomUnitCostAndCustomQuoteUnit|TestCustomGradientDisplayUnitDoesNotFallbackToLb' -count=1`
- `go test ./internal/infrastructure/postgres/costing -run TestLoadProductInputsReadsComposablePriceRulesAndBomUnitCosts -count=1`
- `node --test src/lib/bean-list-pdf.test.js src/lib/order-entry.test.js`

## 浏览器验收
- 使用测试数据进入 SKU设置、BOM 配方维护、产品价格表、录单页面。
- 验证“速溶咖啡”产品类型下生成的盒装价格表显示 `元/盒`。
- 发布后进入录单，选择速溶盒装 SKU，确认阶梯价显示 `元/盒` 且数量按盒计算。
