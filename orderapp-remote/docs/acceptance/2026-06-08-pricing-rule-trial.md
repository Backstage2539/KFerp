# PR-452 商品价格管理模板试算

## Scope
- 商品价格管理模板列表新增 `试算`，打开 `价格计算模板试算` 抽屉。
- 试算通过 `POST /api/costing/pricing-rule-trial` 读取 Pricing Rule、所选商品当前 BOM/工序成本和临时录入口，返回试算单价、成本拆解、公式步骤和预警。
- 试算只读，不保存到模板、商品价格表、发布快照或订单。

## RED
- `node --test src/lib/product-settings.test.js`：实现前失败于缺少 `buildPricingRuleTrialPayload`、试算按钮、试算抽屉和 `/api/costing/pricing-rule-trial` 前端接线。
- `go test ./internal/application/costing -run 'TestPricingRuleTrial' -count=1`：实现前失败于缺少 Pricing Rule 试算命令、结果和服务方法。
- `go test ./internal/interfaces/http/costing -run TestPricingRuleTrialAPI -count=1`：实现前失败于缺少 `POST /api/costing/pricing-rule-trial`。
- `go test ./internal/interfaces/http/support -run TestDev452PricingRuleTrialContracts -count=1`：实现前失败于缺少 PR-452 合同、文档和 UI/API 标记。
- `go test ./internal/interfaces/http/support -run TestPricingRuleTrialPermissionIsReadOnly -count=1`：实现前失败于 `POST /api/costing/pricing-rule-trial` 被通用 costing POST 规则映射为 `costing.write`。

## Acceptance
- [x] 商品价格管理模板行显示 `试算`。
- [x] 试算抽屉可选择商品、报价单位、临时损耗率、临时利润/加价、临时税率和其他成本。
- [x] API 返回 BOM+工序成本、其他成本、损耗后成本、税前价、税额、试算单价、公式步骤和预警。
- [x] 试算结果不回写商品价格表、不生成发布快照、不改变订单，也不保存 Pricing Rule 最终价。
- [x] 本地 mock 浏览器验收覆盖当前 Vue 生产构建的模板行 `试算`、试算抽屉、商品选择、报价单位、临时覆盖项、公式步骤和无控制台错误。
- [ ] 真实 ERP / development 环境验收覆盖真实有 BOM 商品的试算结果、公式步骤和无控制台错误。

## GREEN
- `node --test src/lib/product-settings.test.js`：通过 126/126。
- `go test ./internal/application/costing -run 'TestPricingRuleTrial' -count=1`：通过。
- `go test ./internal/interfaces/http/costing -run TestPricingRuleTrialAPI -count=1`：通过。
- `go test ./internal/interfaces/http/support -run 'TestDev452PricingRuleTrialContracts|TestPricingRuleTrialPermissionIsReadOnly' -count=1`：通过。
- `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -count=1`：通过。
- `go test ./...`：通过。
- `npm run build`：通过，保留既有 Vite chunk-size warning。
- `scripts/verify_kferp.sh changed`：通过。
- `git diff --check`：通过。

## Browser
- 本地 mock ERP 服务：`http://127.0.0.1:5192/vue-shell?view=productPriceManagement`，静态文件来自当前 `frontend-vue-shell/dist`。
- 页面验收：`商品价格管理` 模板列表显示 `试算`；打开 `价格计算模板试算` 抽屉后可见模板摘要、试算商品、报价单位、临时损耗率、临时利润/加价、临时税率、其他成本、`重新试算`。
- 交互验收：选择 `真实BOM试算商品` 后自动带出 `kg`，调用 `/api/costing/pricing-rule-trial`。
- Payload 验收：`{"pricing_rule_id":77,"product_id":101,"customer_id":0,"quote_unit":"kg","overrides":{"margin_rate":0.25,"tax_rate":0.06,"other_costs":{"packaging":8,"labor_buffer":2}}}`，未包含商品价格表、发布快照、订单或最终价格记录字段。
- 结果验收：页面显示 BOM+工序成本 `48.85/kg`、其他成本 `10/kg`、损耗后成本 `66.88/kg`、试算单价 `94.52/kg`、BOM版本 `v3`、试算警告、公式步骤和“试算结果不写入商品价格表、发布快照或订单。”；浏览器 console errors 为 0。
- 限制：本机没有 `DATABASE_URL` / `APP_PASS`，本轮未启动真实后端，也未部署到 development；真实 ERP BOM 数据验收仍待集成/部署后执行。
