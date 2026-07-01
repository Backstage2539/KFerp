# PR-512-PRICING-RULE-TRIAL-SOURCE-COST 验收记录

## Scope
- 商品价格管理 `价格试算` 不再默认把已含 BOM 原料损耗的 `BOM+工序成本` 再套一次 `损耗增加`。
- 试算展示取整规则来源、税率来源和当前工艺路线；工艺路线只作为生产路径模板说明。
- 后续 PR-514 已纠正路线成本边界：工艺路线页面不保存或读取 `计划工序成本`，真实工序成本只在生产计划/未开工工单拆分选择工位产能后冻结。

## RED
- `go test ./internal/application/costing ./internal/application/manufacturing ./internal/interfaces/http/costing ./internal/interfaces/http/manufacturing ./internal/infrastructure/postgres/costing -count=1`：实现前失败，缺少 `PricingRuleTrialDefaultTaxRate`、税率/取整/工艺路线响应字段。
- `node --test frontend-vue-shell/src/lib/product-settings.test.js frontend-vue-shell/src/lib/process-routes.test.js`：实现前失败，试算 UI 仍使用旧工序语义。

## GREEN
- `go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/application/manufacturing ./internal/interfaces/http/manufacturing ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -count=1`
- `go test ./internal/interfaces/http/support -count=1`
- `node --test orderapp-remote/frontend-vue-shell/src/lib/product-settings.test.js orderapp-remote/frontend-vue-shell/src/lib/process-routes.test.js`
- `cd orderapp-remote/frontend-vue-shell && npm ci`
- `npm run build`
- `scripts/verify_kferp.sh changed`
- `git diff --check`

## Deployment Smoke
- `./deploy_orderapp.sh` 在干净 `develop` checkout 中完成，Vue shell build、小程序 typecheck/build 和 Docker build 内 `go test ./...` 均通过。
- 服务器容器 `erp_orderapp`、`erp_postgres`、`erp_caddy`、`erp_docconvert` 运行正常，`erp_orderapp` 日志显示监听 `:8080`。
- `GET /app/vue-shell?view=productPriceManagement` 和 `GET /app/vue-shell?view=processRoutes` 返回 `200`。
- 认证后 `GET /app/api/product-settings?limit=1`、`GET /app/api/production-boms?status=all&limit=1`、`GET /app/api/product-pricing-rules` 返回 `200`。
- 认证后空 id `POST /app/api/costing/pricing-rule-trial` 返回 `400`，确认路由和业务校验生效。
- `req/product` 和 `req/dev` API 可见 `PR-512-PRICING-RULE-TRIAL-SOURCE-COST` 及三条 `DEV-512` 跟踪记录。
