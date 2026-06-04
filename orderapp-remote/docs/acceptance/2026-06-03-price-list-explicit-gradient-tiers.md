# PR-400 产品价格表显式阶梯价模板

## 背景
- Van 反馈：`初晓2.5kg装` 没有绑定阶梯价模板，但产品价格表仍显示阶梯价。
- PR-399 已去掉旧分类/父分类阶梯价 fallback，但成本引擎和发布保存层仍会在无模板时自动生成旧 Excel 默认商业阶梯价。

## 需求
- 产品价格表商业阶梯价必须来自明确阶梯价模板：客户商品规则、客户规则模板、商品级覆盖或商品档案引用的商品配置模板。
- 无明确模板时，成本试算可保留内部 kg/lb 基础价格，但产品价格表预览、发布快照和 `product_price_tiers` 不得自动补默认商业阶梯价。
- 历史已发布价格表、旧订单、旧 PDF 和旧 `product_price_tiers` 不回改。

## RED Evidence
- `go test ./internal/domain/costing -run TestProductWithoutGradientTemplateDoesNotPublishCommercialTiers -count=1`
  - 失败原因：无 `GradientTemplate` 的 `初晓2.5kg装` 仍生成 `2包-13包`、`14包-23包`、`24包-47包`、`48包+`。
- `go test ./internal/infrastructure/postgres/costing -run TestCommercialTiersForPublishDoesNotInventDefaultTiers -count=1`
  - 失败原因：发布保存函数根据 `WholesaleKgPrices/WholesaleLbPrices` 重新拼出默认商业阶梯价。

## GREEN Evidence
- `go test ./internal/domain/costing ./internal/application/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/costing ./internal/interfaces/http/support -count=1`
  - 通过：成本引擎、应用服务、仓储发布保存、HTTP API 和需求管理种子均按显式阶梯价模板规则运行。
- `go test ./...`
  - 通过：全量 Go 包通过。
- `scripts/verify_kferp.sh changed`
  - 通过：命令退出码 0。
- `./deploy_orderapp.sh development`
  - 通过：Vue build 通过并带既有 chunk-size warning；Docker build 内 `go test ./...` 通过；部署到 `origin/develop=9d7d3e5dfcdbd84b574c72ef0d493e291924b432`。

## Deploy Smoke
- 容器：`erp_orderapp`、`erp_caddy`、`erp_postgres`、`erp_docconvert` 运行，`erp_postgres` healthy。
- 未认证 `GET /app/`：303 到 `/app/orders`。
- BasicAuth `GET /app/vue-shell`：200。
- 需求 API：`PR-400-PRICE-LIST-EXPLICIT-GRADIENT-TIERS` 可查到。
- 价格表只读 API：Karen 范围 `初晓2.5kg装` 返回 `tiers 0`、`gradient_template False`。

## 手册
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
