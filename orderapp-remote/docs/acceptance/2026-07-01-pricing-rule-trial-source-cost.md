# PR-512-PRICING-RULE-TRIAL-SOURCE-COST 验收记录

## Scope
- 商品价格管理 `价格试算` 不再默认把已含 BOM 原料损耗的 `BOM+工序成本` 再套一次 `损耗增加`。
- 试算展示取整规则来源、税率来源和当前工艺路线；工序成本读取工艺路线工序的 `计划工序成本`。
- 工艺路线页面可保存和读取 `计划工序成本`，用于价格试算标准成本。

## RED
- `go test ./internal/application/costing ./internal/application/manufacturing ./internal/interfaces/http/costing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/costing -count=1`：实现前失败，缺少 `PricingRuleTrialDefaultTaxRate`、税率/取整/工艺路线响应字段，且工艺路线保存会清空 `planned_operation_cost`。
- `node --test frontend-vue-shell/src/lib/product-settings.test.js frontend-vue-shell/src/lib/process-routes.test.js`：实现前失败，试算 UI 仍使用旧工序语义，且工艺路线页面没有 `计划工序成本` 输入。

## GREEN
- `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -count=1`
- `go test ./internal/interfaces/http/support -count=1`
- `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/process-routes.test.js`
- `cd orderapp-remote/frontend-vue-shell && npm ci`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- `git diff --check`

## Browser
- 待部署后验收：打开 `商品价格管理 -> 价格试算`，未填写 `临时损耗率` 时确认 `损耗增加` 不显示；填写临时损耗率后确认卡片出现。
- 待部署后验收：确认试算顶部显示 `取整规则`，税额卡片显示税率来源，`工艺路线` 下拉默认来自所选 BOM 版本。
- 待部署后验收：进入 `生产管理 -> 工艺路线` 保存某道工序的 `计划工序成本`，回到价格试算后 `BOM+工序成本` 明细读取该成本。
