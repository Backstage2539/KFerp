# PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION 验收记录

## 当前口径更新
- PR-460 已覆盖本记录中的 PR439 发布售价快照反推展示。当前公式展示仍保留逐节点公式行，但缺 BOM/工序成本时不再反推发布售价。
- 本记录中的 `88.5/kg` 和反推相关部署证据只表示 PR-457 当时历史验收，不作为当前试算行为。

## 范围
- 需求：PR-457-PRICING-RULE-TRIAL-FORMULA-EXPRESSION。
- 商品价格管理的 `价格计算模板试算` 在公式步骤表前展示 `计算公式` 主行和逐节点公式行。
- 公式展示覆盖标准模板和 Excel 供应售价兼容口径；PR-460 后缺 BOM/工序成本场景不再反推发布售价。
- 试算仍是只读结果，不保存到 Pricing Rule 模板、商品价格表、发布快照或订单。

## RED 证据
- 后端：`go test ./internal/application/costing -run 'TestPricingRuleTrial(UsesBomCostTemplateFormula|InfersCostFromPublishedPriceSnapshotWhenBomCostMissing)' -count=1` 曾失败，因为 `PricingRuleTrialResult` 缺少 `FormulaExpression` / `FormulaExpressionLines`。
- 前端：`node --test src/lib/product-settings.test.js` 曾失败，因为商品价格管理试算抽屉缺少 `计算公式` / `formula_expression_lines` 展示标记。

## 验收点
- API 返回 `formula_expression` 和 `formula_expression_lines`。
- 标准模板公式可见 `(BOM+工序成本 60/kg + 其他成本 2.5/kg)`、损耗率、毛利率、税率和最终售价。
- PR-460 后，缺 BOM/工序成本场景的公式不再显示发布售价反推，BOM+工序成本按 0 展示并提示警告。
- 前端结果区显示 `计算公式`，下方再显示公式步骤表。
- 前端不恢复 `重新试算` 或 `售价后附加成本`。

## 自动化验证
- 通过：`go test ./internal/application/costing -run 'TestPricingRuleTrial(UsesBomCostTemplateFormula|InfersCostFromPublishedPriceSnapshotWhenBomCostMissing)' -count=1`
- 通过：`go test ./internal/interfaces/http/costing -run TestPricingRuleTrialAPI -count=1`
- 通过：`go test ./internal/interfaces/http/support -run TestDev457PricingRuleTrialFormulaExpressionContracts -count=1`
- 通过：`go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -run 'TestPricingRuleTrial|TestDev45(2|4|6|7)|TestPricingRuleTrialPermissionIsReadOnly' -count=1`
- 通过：`node --test src/lib/product-settings.test.js`
- 通过：`npm run build`，仅有既有 Vite chunk-size warning。
- 通过：`go test ./...`
- 通过：`scripts/verify_kferp.sh changed`
- 通过：`git diff --check`

## 浏览器验收
- 通过：development 部署 `origin/develop=a1eaf2535fac1fe66483b80c61237061a68bb3d2`，备份 `/opt/stacks/erp/orderapp.backup.deploy-20260608191229`。
- 通过：容器 `erp_orderapp` 正常启动，`erp_postgres` healthy；`/app/vue-shell/?view=productPriceManagement&pr457=1` 认证访问返回 200，需求 API 可见 PR-457。
- 通过：`POST /app/api/costing/pricing-rule-trial` 使用 `pricing_rule_id=1`、`product_id=538`、`quote_unit=kg`，返回 `88.5/kg`、`formula_expression` 含 `发布售价快照反推` 和 `最终售价 = 88.5/kg`，并返回 6 行逐节点公式。
- 通过：商品价格管理模板行点击 `试算`，选择 `PR439-20260606182321 熟豆下单商品` 后报价单位自动为 `kg`；结果区显示 `88.5/kg`、`计算公式`、逐节点公式行、`发布售价快照反推` 和公式步骤表。
- 通过：页面不显示 `重新试算` / `售价后附加成本`，控制台错误 0。截图：`/tmp/pr457-deployed-pricing-rule-formula.png`。
