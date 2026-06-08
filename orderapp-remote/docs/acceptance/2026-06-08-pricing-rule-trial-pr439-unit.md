# PR-456-PRICING-RULE-TRIAL-PR439-UNIT 验收记录

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
- GREEN post-merge：`node --test src/lib/product-settings.test.js src/lib/materials-ui.test.js src/lib/bom.test.js src/lib/menu-ia.test.js src/lib/product-bean-list-split.test.js` 通过 176/176；`go test ./...`、`npm run build`、`scripts/verify_kferp.sh changed`、`git diff --check` 通过。

## 开发环境证据
- Deploy：`origin/develop=3e83ab9ea33b239d20cd2ffe80f070c695b7a44e` 已部署到开发环境；备份路径 `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608175237`。
- Smoke：`erp_orderapp` running，`erp_postgres` healthy；未登录 `/app/` 返回 `303` 到 `/app/orders`，登录后 `/app/vue-shell/?view=productPriceManagement&pr456=1` 返回 `200`。
- PR/DEV：`/app/api/req/product?limit=500` 可见 `PR-456-PRICING-RULE-TRIAL-PR439-UNIT`。
- API：`POST /app/api/costing/pricing-rule-trial` 使用 `pricing_rule_id=1`、`product_id=538`、`quote_unit=kg`，返回 `试算单价 88.5/kg`、`base_cost 50.12/kg`、`发布售价快照` 公式步骤和 `未找到BOM/工序成本，已按发布售价快照反推成本基数` 预警。
- Read-only：重复试算前后 `product_price_records`、`product_price_tiers`、`bean_list_publications`、`orders`、`order_items`、`order_audit_logs` 计数不变。
- Browser：商品价格管理模板表 10 行均有 `试算`；试算抽屉选择 `PR439-20260606182321 熟豆下单商品` 后，报价单位下拉来自 `条/盒/袋/g/kg/磅` 且值为 `kg`，页面不显示 `重新试算` / `售价后附加成本`，显示 `88.5/kg`、`发布售价快照`、公式步骤和只读提示，控制台错误 0。截图：`/tmp/pr456-deployed-pricing-rule-trial-pr439.png`。

## 验收口径
- 商品价格管理每个价格计算模板行仍显示 `试算`。
- 打开试算抽屉后，页面不显示 `重新试算`，不显示 `售价后附加成本`。
- 报价单位选项来自全局单位字典，例如 `kg`、`lb`、`袋`。
- 选择 `PR439-20260606182321 熟豆下单商品` 和 `kg` 后自动试算成功，试算单价为 `88.5/kg`。
- 公式步骤包含 `发布售价快照`、成本基数、损耗率、利润/加价、税费、取整和试算单价。
- 试算结果不写入 Pricing Rule 模板、商品价格表、发布快照或订单。
