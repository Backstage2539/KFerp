# PR-475-PRODUCTION-PLAN-ERPNEXT-WORKBENCH 验收记录

## Summary
生产计划页改为当前生产计划工作台：左侧待生产需求用于选择库存不足商品，右侧当前生产计划工作台自动展示计划预览、BOM 摘要、工艺路线摘要和物料需求汇总。创建草稿后留在当前计划区，草稿可直接 `提交当前计划生成工单`；历史生产计划列表继续保留状态/时间过滤和批量提交。

## Verification
- RED frontend：`node --test src/lib/produce-plan.test.js` 在实现前失败，缺少当前计划工作台、自动预览和当前计划提交 helper。
- GREEN frontend：`node --test src/lib/produce-plan.test.js` 通过，覆盖 `当前生产计划`、`planning-workbench`、自动预览、创建 payload 不带 `input_by_key`、当前计划单 id 提交和烘焙排产词禁用。
- RED support/docs：`go test ./internal/interfaces/http/support -run TestDev475ProductionPlanERPNextWorkbenchContracts -count=1 -v` 在文档/种子补齐前失败。
- GREEN support/docs：`go test ./internal/interfaces/http/support -run TestDev475ProductionPlanERPNextWorkbenchContracts -count=1 -v` 通过。
- GREEN packages/build/check：`node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` 通过 20/20；`go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1` 通过；`go test ./...` 通过；`npm run build` 通过并仅保留既有 Vite chunk-size warning；`scripts/verify_kferp.sh changed` 和 `git diff --check` 通过。
- GREEN browser/local：本地生产 Vue build + mock API at `http://127.0.0.1:5191/vue-shell/?view=producePlan` 通过。初始页顶部只有筛选/刷新，左侧 `待生产需求`、右侧 `当前生产计划`、下方 `库存充足（只提示）` 和 `生产计划单据` 可见；勾选 `539-454` 后自动请求 `/api/produce/unproduced?plan=1&selected=539-454` 并显示计划预览和物料需求；创建草稿后显示 `PP-PR475-001` / `草稿`；点击 `提交当前计划生成工单` 后走 `POST /api/production-plans/submit`，状态变为 `已提交工单` 且当前提交按钮置灰；历史状态/时间过滤和批量 `提交生成工单` 仍可见。390px 窄屏下左/右工作台上下堆叠，页面无整体横向溢出。

## Acceptance Scenarios
- 进入生产计划页，顶部只有筛选和刷新；主区显示左侧待生产需求和右侧当前生产计划工作台。
- 勾选库存不足商品后，右侧自动显示计划预览和物料需求汇总，请求包含 `plan=1` 和 `selected`。
- 点击 `创建生产计划` 后，右侧显示草稿计划号和状态 `草稿`，不会自动提交工单。
- 点击 `提交当前计划生成工单` 后，复用 `POST /api/production-plans/submit` 且 payload 为单个计划 id，成功后状态变为 `已提交工单`。
- 历史生产计划单据列表仍可按状态/时间过滤，并可批量提交多个草稿计划。
- 页面不出现 `生产建议`、`推荐机器`、`每锅数量`、`锅数`、`预计成品`，也不请求 `/api/produce/machines`。
