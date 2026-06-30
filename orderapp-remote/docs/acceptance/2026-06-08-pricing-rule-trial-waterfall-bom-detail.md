# PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL 验收记录

## 目标
- 商品价格管理的价格计算模板试算展示价格瀑布，让 `BOM+工序成本 + 其他成本 + 加价增加 + 税额 + 取整调整 = 试算单价` 可直接查看；只有显式输入临时损耗率时才显示 `损耗增加`。
- 展示 `BOM+工序成本明细`，包含物料成本明细、工序成本明细、物料合计、工序合计和总计。
- `PR439-20260606182321 工厂量单商品` 这类由生产 BOM 通过 `output_product_id` 声明产出的商品，试算必须读取该生产 BOM 的已发布版本，例如 `BOM-000539 / V002`，不得因没有旧商品 BOM 绑定而显示全 0。
- 缺 BOM/工序成本时显示 0 和警告，不反推发布售价快照。

## 本地验证
- RED 服务/API：`go test ./internal/application/costing -run 'TestPricingRuleTrial(UsesBomCostTemplateFormula|DoesNotInferCostFromPublishedPriceSnapshotWhenBomCostMissing)' -count=1`、`go test ./internal/interfaces/http/costing -run TestPricingRuleTrialAPI -count=1` 在实现前因 `PricingRuleTrialBaseCostDetail`、`base_cost_details`、`yield_loss_amount` 等字段缺失失败。
- RED 前端：`node --test src/lib/product-settings.test.js` 在实现前因缺少 `BOM+工序成本明细`、`损耗增加`、`加价增加`、`pricing-rule-trial-waterfall` 失败。
- RED PR439 工厂量单商品回归：`go test ./internal/application/costing -run 'TestPricingRuleTrialUsesBaseCostDetailsWhenProductInputSummaryMissing' -count=1` 在修复前显示明细已有 `42+8`，但 `base_cost=0`；`go test ./internal/infrastructure/postgres/costing -run 'TestLoadProductInputsUsesProductionBomOutputProductFallback|TestPricingRuleTrialDetailsUseProductionBomOutputProductFallback' -count=1` 在修复前缺少 `production_boms.output_product_id` fallback。
- GREEN 服务/API：目标服务测试和 `TestPricingRuleTrialAPI` 已通过，覆盖 BOM/工序明细、价格瀑布字段和缺 BOM 不反推。
- GREEN PR439 工厂量单商品回归：服务层会把 BOM/工序明细合计反填为 `base_cost`；仓储层 `loadProductInputs` 和 `LoadPricingRuleTrialBaseCostDetails` 均会在无旧绑定时通过 `production_boms.output_product_id` 找到已发布生产 BOM 版本。
- GREEN 前端：`node --test src/lib/product-settings.test.js` 已通过，覆盖瀑布、明细、加号/等号、删除英文来源/状态和反推文案。
- GREEN 完整本地验证：`go test ./internal/application/costing ./internal/interfaces/http/costing ./internal/infrastructure/postgres/costing ./internal/interfaces/http/support -run 'TestPricingRuleTrial|TestDev4(5(2|4|6|7|9)|60|61)|TestDev(449|461)|TestLoadProductInputs|TestPricingRuleTrialPermissionIsReadOnly' -count=1`、`node --test src/lib/costing-bean-list-version-ui.test.js src/lib/product-bean-list-split.test.js src/lib/product-settings.test.js`、`go test ./...`、`npm run build`、`scripts/verify_kferp.sh changed`、`git diff --check` 均已通过。
- GREEN deploy build：`./deploy_orderapp.sh` 完成 Vue shell build、miniapp typecheck/build、miniapp `build:mp-weixin`、Docker build 和容器内 `go test ./...`。

## 待浏览器验收
- 已合并到 `develop` 并部署到 development：`origin/develop=47d90a37590c92d439fffdbcfc48873aca99f2ff`。部署备份：`root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260608213219`。
- Smoke 已通过：`erp_orderapp`、`erp_docconvert`、`erp_caddy` 正常，`erp_postgres` healthy；`/app/` 返回 303 到 `/app/orders`；认证访问 `/app/vue-shell/?view=productPriceManagement&merge=pricing-20260608` 返回 200；需求 API 暴露 `PR-460-PRICING-RULE-TRIAL-WATERFALL-BOM-DETAIL`，状态 `review`，负责人 `VA`。
- 按 Van 本次分工，业务浏览器验收由 Van 在 deployed ERP 手工执行。
- 浏览器进入 `商品价格管理`，打开任意模板 `试算`，选择有 BOM/工序成本的商品，确认价格瀑布、物料成本明细、工序成本明细和公式步骤可见。
- 选择 `PR439-20260606182321 工厂量单商品`，确认 `BOM-000539 / V002` 的 BOM/工序成本进入试算，价格瀑布不再全 0。
- 选择缺 BOM/工序成本的商品，确认 `BOM+工序成本` 为 0、有感叹号和警告，不出现 `发布售价快照反推`。
- 重复试算前后确认商品价格表、发布快照、Pricing Rule 模板、订单和操作日志业务写记录没有被改动。
