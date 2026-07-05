# PR-493-PLAN-WORKORDER-SPLIT-EDIT

## Scope
- 新建生产计划/工单的工艺路线快照按 `operation_id` 回查最新工序主数据名称。
- 步骤条第 3 步和生产计划单据列表/详情中的草稿计划支持 `编辑拆分`，可打开拆分抽屉维护工序产能拆分。
- `released` 且工序卡仍为 `pending` 的工单支持 `编辑拆分`，保存后重建 pending 工序卡并写操作日志。

## Acceptance
- 修改工序主数据名称后，新建草稿生产计划的 `工序产能拆分` 展示新工序名称。
- 创建草稿计划时不维护拆分，点击步骤条第 3 步 `拆分产能`，或回到生产计划单据列表点击 `编辑拆分` 后，可在拆分抽屉补充分配 `布勒 18kg`、`智烘 4kg` 等工位产能并看到自动批次卡片。
- 草稿计划提交工单后，在生产工单页点击 `编辑拆分`，保存拆分后工序摘要和工序卡按新拆分刷新。
- 工单开始生产后不再显示 `编辑拆分` 入口；后端也拒绝存在非 pending 工序卡的工单拆分保存。

## Evidence
- RED backend: `go test ./internal/infrastructure/postgres/production -run TestProcessRouteSnapshotUsesLatestOperationMasterNames` failed before implementation because route snapshots did not join `manufacturing_operations`.
- RED API: `go test ./internal/interfaces/http/production -run TestWorkOrderOperationSplitAPISavesReleasedWorkOrderCapacitySplits` failed before implementation because `SaveWorkOrderOperationSplitsCommand` and `/api/work-orders/:id/operation-splits` did not exist.
- RED frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` failed before implementation because work-order split helpers and UI markers were missing.
- GREEN targeted: `go test ./internal/infrastructure/postgres/production -run 'TestProcessRouteSnapshotUsesLatestOperationMasterNames|TestProductionPlanOperationSplitsOwnCapacityBatchPlanning'`; `go test ./internal/interfaces/http/production -run TestWorkOrderOperationSplitAPISavesReleasedWorkOrderCapacitySplits`; `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js`.
