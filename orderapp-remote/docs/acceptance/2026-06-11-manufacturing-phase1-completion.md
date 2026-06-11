# PR-470 Manufacturing Phase 1 Completion

## Scope
- 商品档案按商品设置默认生产 BOM，并把产出 BOM 与组件 where-used 反查拆开。
- 工序与工作中心成为主数据；工艺模板/路线操作行引用主数据并保留名称快照。
- 成本试算、生产计划和新工单统一 BOM 取数优先级：商品默认 BOM、旧 binding、最新 published 产出 BOM fallback。
- 新工单冻结 BOM 版本、工艺路线、工序和工作中心。

## Acceptance
- 商品档案打开任一有产出 BOM 的商品，配置抽屉显示 `可生产该商品的 BOM` 和 `作为组件被哪些 BOM 使用` 两块。
- 在 `可生产该商品的 BOM` 点击 `设为默认`，刷新后该 BOM 仍显示为 `默认 BOM`；组件反查列表没有 `设为默认`。
- 商品价格管理试算同一商品时，`试算BOM版本` 默认选择商品显式默认 BOM；没有显式默认时只 fallback 最新 published 版本，不标记为默认。
- 工艺模板页面可新增工序、工作中心；路线行可从主数据选择并保存 `operation_id`、`workstation_id`，同时保留工序/工作中心名称快照。
- 生产计划开始生产后，工单显示冻结 BOM 版本和冻结工艺来源；工序卡来自冻结路线/模板的工序与工作中心。之后修改商品默认 BOM 或路线，不回改历史工单。

## Verification
- RED support: `go test ./internal/interfaces/http/support -run TestDev470ManufacturingPhase1CompletionContracts -count=1`
- RED backend source: `go test ./internal/infrastructure/postgres/manufacturing ./internal/infrastructure/postgres/production ./internal/infrastructure/postgres/costing -run 'TestManufacturingSchemaAddsOperationAndWorkstationMasterData|TestWorkOrderFreezesProcessRouteAndUsesDefaultBomPriority|TestPricingRuleTrialProductionCostUsesProductDefaultBomBeforeOutputFallback' -count=1`
- RED frontend: `node --test src/lib/product-settings.test.js`
- GREEN backend: `go test ./...`
- GREEN frontend: `node --test src/lib/work-orders.test.js src/lib/product-settings.test.js src/lib/bom.test.js src/lib/menu-ia.test.js`
- GREEN build/check: `npm run build`; `scripts/verify_kferp.sh changed`
- GREEN browser/local: production Vue build + mock API + Chrome DevTools Protocol opened 商品档案、工艺模板、生产工单. Verified split BOM lists and `设为默认` request payload, operation/workstation master-data selectors, frozen BOM, `工艺路线 #30`, and frozen operation/workstation text on the work order page.
