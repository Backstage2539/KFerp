# PR-497 工单库存主控与库存单据目的化验收

## 范围
- 工单成为生产库存主控：开始生产、生产领料、完工入库、取消生产和详情聚合。
- 库存单据公开接口使用 `/api/stock-documents` 和 `purpose`，保留 `/api/stock-entries` 与 `entry_type` 兼容。
- 生产顶部切换条移除高频 `生产中` 入口，保留旧视图兼容。
- 工序卡继续只承担工位执行、实际分钟、损耗和异常记录。

## 已覆盖测试
- 单元测试：`go test ./internal/application/production -run 'TestServiceOwnsWorkOrderInventoryControlWithStockDocumentPurpose|TestServiceOwnsManufacturingPhase2StockEntriesAndExecutionActions' -count=1`
- API 测试：`go test ./internal/interfaces/http/production -run 'TestStockDocumentPurposeAliasesUseERPNextFlowLanguage|TestWorkOrderProducePathOwnsInventoryActionsAndDetail|TestManufacturingPhase2StockEntryAndExecutionAPIs|TestWorkOrderStartAPIStartsReleasedWorkOrder' -count=1`
- 前端单测：`node --test src/lib/manufacturing-execution.test.js src/lib/production-workstation.test.js src/lib/menu-ia.test.js src/lib/work-orders.test.js`
- 包级验证：`go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`
- 前端扩展验证：`node --test src/lib/manufacturing-execution.test.js src/lib/production-workstation.test.js src/lib/menu-ia.test.js src/lib/work-orders.test.js src/lib/view-routing.test.js`
- 前端构建：`npm run build`
- 项目检查：`scripts/verify_kferp.sh changed`
- Diff 检查：`git diff --check`

## 手工验收要点
- 进入生产模块，顶部切换条应为 `生产视图 / 工位视图 / 生产计划 / 工单 / 工序卡 / 质检 / 日志 / 成本`，左侧生产主菜单不再出现 `生产中`。
- 在库存作业的 `Stock Entry单据` 页面，单据目的应显示 `生产领料 / 生产退料 / 生产消耗 / 完工入库 / 库存调整`。
- 使用 `/api/stock-documents` 创建生产领料单据时，请求 `purpose=material_transfer_for_manufacture`，返回体同时包含 `purpose` 和兼容 `entry_type=material_issue_to_wip`。
- 打开工单详情接口 `/api/produce/work-orders/:id`，应能看到工单、物料占用、工序卡、库存单据、库存流水、生产日志和成本汇总。
- 完工、取消和生产领料优先走 `/api/produce/work-orders/:id/*` 生产路径；旧 `/api/work-orders/:id/start|complete` 只作为兼容入口。
