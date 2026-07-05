# PR-519 生产计划拆分产能差距预览验收记录

## Scope

- 生产计划 `工序产能拆分` 抽屉新增只读 `产能安排总览` 和 `用料需求差距`。
- 新增 `POST /api/production-plans/:id/operation-splits/preview`，复用保存 payload 计算预览，不写库、不写操作日志。
- 保存拆分和提交工单规则不变；提交仍由后端覆盖校验阻断不足。

## Acceptance Criteria

- 20kg 实际需求安排 12kg 时返回并展示 `short`，差距为负。
- 20kg 实际需求安排 20kg 时返回并展示 `matched`。
- 20kg 实际需求安排 24kg 时返回并展示 `over`。
- 多工序同一计划行的物料折算按各工序已安排量的最小值计算，避免重复叠加。
- BOM 原料损耗、包装件数和比例用料继续由后端聚合逻辑计算，前端不复制 BOM 公式。

## Verification

- RED: `go test ./internal/application/production -run TestPreviewProductionPlanOperationSplitsIsReadOnlyAndRequiresPositiveQuantity -count=1` 在新增 service/API 前失败。
- RED: `go test ./internal/interfaces/http/production -run TestProductionPlanOperationSplitPreviewAPIReturnsDemandGapWithoutSaving -count=1` 在新增 handler 前失败。
- RED: `go test ./internal/infrastructure/postgres/production -run TestPreviewProductionPlanOperationSplits -count=1` 在新增 repository 预览前失败。
- RED: `node --test src/lib/produce-plan.test.js` 在前端 preview helper/UI 标记前失败。
- GREEN targeted: `go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production -run 'TestPreviewProductionPlanOperationSplits|TestProductionPlanOperationSplitPreviewAPIReturnsDemandGapWithoutSaving' -count=1` passed.
- GREEN frontend targeted: `node --test src/lib/produce-plan.test.js` passed.

## Browser Smoke Target

- 打开 `/vue-shell?view=producePlan&demand_status=unplanned&plan=1&selected=550-454`。
- 打开 `PP-0000000059` 的 `拆分产能` 抽屉。
- 切换 20kg、12kg、2kg 等产能档或修改承担产量，确认总览和用料差距颜色实时变化。
