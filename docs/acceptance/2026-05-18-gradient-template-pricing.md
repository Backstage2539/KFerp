# 2026-05-18 梯度模板与价格来源验收

## 范围
- 产品设置新增梯度模板维护。
- 二级分类绑定梯度模板，产品自动继承。
- 成本试算和商用豆单按分类模板生成梯度价格。
- 价格来源解释覆盖快速成本参数，并展示当前公式步骤。

## 验收点
- [x] 梯度模板可设置名称、展示单位、区间名、最小/最大数量和利润率；后台换算并保存克重区间，系统匹配使用克重区间。
- [x] 同一模板统一一个展示单位，支持元/磅、元/kg、元/227g、元/100g、元/250g；元/磅按 454g 换算。
- [x] 二级分类可绑定启用模板，第一版不提供产品级覆盖。
- [x] 成本试算加载产品时读取分类绑定模板，未绑定分类继续使用默认商用梯度。
- [x] 价格来源解释展示模板、当前价格和公式步骤。
- [x] 公式步骤覆盖生豆成本、出成率、kg/lb 换算、生产成本、包装成本、损耗、税费、利润率和最终价格。
- [x] 价格来源抽屉不提供临时试算表单；交易价格仍需发布后生效。
- [x] 已发布豆单保持快照，不随模板或快速成本参数静默变更。

## 验证证据
- `go test ./internal/domain/costing ./internal/application/costing ./internal/interfaces/http/catalog ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing -run 'TestGradientTemplate|TestCommercialPriceExplanation|TestBeanListAppliesCategoryGradientTemplate|TestProductSettingsAPI.*Gradient|TestProductSettingsAPISupportsCategoryTree|TestCostingPriceExplanationAPI|TestLoadProductInputsReadsCategoryGradientTemplates'`
- `node --test src/lib/gradient-templates.test.js`
- `npm run build`

## 手册
- `OP_MANUAL_COSTING.md`
- `OP_MANUAL_INVENTORY_MATERIALS.md`
