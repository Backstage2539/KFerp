# PR-499 Production Execution Hub Phase 2 Acceptance Evidence

Date: 2026-06-26 Asia/Shanghai
Branch: `codex/production-execution-hub-phase2-20260617`

## Scope
- 生产管理高频视图优化二期：新增 `工单执行枢纽`，从生产视图、工位视图、生产工单和工序卡进入同一工单上下文。
- `GET /api/produce/work-orders/:id` 兼容原详情，并新增 `execution_hub`，包含工单头、BOM/路线、工序进度、工位分配、WIP、质检、Stock Entry、完工入库、成本和 `trace_timeline`。
- readiness 结构化返回 `can_start`、`can_complete`、`blocking_reasons`、`next_handler`、`suggested_action`、`severity` 和 `related_links`。
- 枢纽跳转库存作业、质检、成本和日志时保留 `work_order_id`、`job_card_id`、`running_item_id`、`material_id`、`shortage_g`、`batch_id` 等上下文。

## Business Rule Guard
- 本期只扩展 read model、上下文链接和 Vue/Vite 展示，不改变完工入库、Stock Entry、WIP 扣减/占用、质检冻结、工序卡状态流转等核心写规则。
- 完工入库聚合只认 `purpose=manufacture` 或明确 finished entry type，避免把 `material_transfer_for_manufacture` 生产领料误算入完工入库。
- 未发现需要业务确认的核心口径冲突。

## RED Evidence
- Frontend RED: `node --test src/lib/production-execution-hub.test.js` failed before implementation because `production-execution-hub.js` and shared drawer/page hooks were missing.
- Backend/API RED: targeted production tests failed before implementation because `WorkOrderDetail.ExecutionHub`、`ProductionExecutionReadiness`、enhanced workstation load fields and API JSON contract were missing.

## GREEN Evidence
- Targeted frontend: `node --test src/lib/production-execution-hub.test.js src/lib/production-workstation.test.js src/lib/view-routing.test.js` passed 21/21.
- Targeted backend/API: `go test ./internal/application/production ./internal/interfaces/http/production -run 'TestWorkOrderExecutionHubReadModelAndTraceTimeline|TestProductionWorkstationOverviewAnswersProductionAndStationQuestions|TestWorkOrderProducePathOwnsInventoryActionsAndDetail' -count=1` passed.

## Implementation Evidence
- Read model: `orderapp-remote/internal/application/production/service.go` adds `WorkOrderExecutionHub` and `ProductionExecutionReadiness`, plus task `readiness_detail` and enhanced workstation load fields.
- API contract: `orderapp-remote/internal/interfaces/http/production/work_order_api_test.go` verifies work-order detail JSON exposes `execution_hub/readiness/trace_timeline`.
- Frontend: `ProductionExecutionHubDrawer.vue` renders readiness, WIP, quality, operation progress, context actions and timeline filters.
- Entry points: `ProductionOverviewView.vue`、`WorkstationView.vue`、`WorkOrdersView.vue` and `JobCardsView.vue` open the same execution hub.
- Context links: `ProductionCostsView.vue`、`ProductionLogsView.vue`、`QualityInspectionsView.vue` and `App.vue` preserve production context parameters.
- Production logs context: `ProductionLogsView.vue` applies `running_item_id` and `batch_id` from execution hub context, and `/api/produce/logs` forwards `running_item_id` to the production log query.

## Release Verification
- GREEN: `go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/support -count=1`.
- GREEN: `go test ./...`.
- GREEN after merging `origin/develop=12e2fb70`: `node --test src/lib/production-execution-hub.test.js src/lib/production-workstation.test.js src/lib/produce-plan.test.js src/lib/produce-running.test.js src/lib/menu-ia.test.js src/lib/view-routing.test.js src/lib/work-orders.test.js src/lib/quality-inspections.test.js src/lib/production-costs.test.js src/lib/production-logs.test.js` passed 100/100.
- GREEN: `npm run build` in `frontend-vue-shell` passed after `npm ci`; Vite reported the existing chunk-size warning.
- GREEN: `scripts/verify_kferp.sh changed`.
- GREEN: `git diff --check`.

## Development Deployment And Browser Acceptance
- GREEN: feature branch pushed to `origin/codex/production-execution-hub-phase2-20260617`.
- GREEN: merged/fast-forwarded into `origin/develop` at `96ac56800772651a30caeca8573f7eaad9bd648b`.
- GREEN: development deployed from `96ac56800772651a30caeca8573f7eaad9bd648b`.
  - Backup: `root@1.12.242.58:/opt/stacks/erp/orderapp.backup.deploy-20260626214842`.
  - Deployment used the documented manual deployment shape because the local clean `develop` worktree was occupied by another dirty workflow and `deploy_orderapp.sh` enforces branch guards.
  - Docker build ran `go test ./...` inside the `orderapp` image build and completed.
  - Containers: `erp_orderapp`, `erp_caddy`, `erp_postgres`, `erp_docconvert` running after deployment.
- GREEN server smoke:
  - Unauthenticated `GET https://erp.qacoohee.com/app/` returned `303`.
  - Authenticated `GET /app/vue-shell?view=productionOverview` returned `200`.
  - Authenticated `GET /app/vue-shell?view=workstationView` returned `200`.
  - Authenticated `GET /app/vue-shell?view=producePlan` returned `200`.
  - Authenticated `GET /app/vue-shell?view=produceRunning` returned `200`.
  - Authenticated `GET /app/vue-shell?view=workOrders` returned `200`.
  - Authenticated `GET /app/vue-shell?view=jobCards` returned `200`.
  - Authenticated `GET /app/api/production/workstation-overview` returned `200`.
  - Requirement API exposes `PR-499-PRODUCTION-EXECUTION-HUB-PHASE2`.
- GREEN ERP browser acceptance, Chrome/browser runtime, 2026-06-26 22:18-22:19 Asia/Shanghai:
  - Rendered pages: `productionOverview`、`workstationView`、`producePlan`、`produceRunning`、`workOrders`、`jobCards`、`stockOperations`、`qualityInspections`、`productionCosts`、`produceLogs`.
  - For production Vue pages, `.production-top-nav` rendered in exact order: `生产视图 / 工位视图 / 生产计划 / 生产中 / 工单 / 工序卡 / 质检 / 日志 / 成本`; active state matched the current page. Current live nav badge data was `待0 阻0 中10` for 生产视图、工位视图、生产中.
  - Browser page checks saw no Vite/framework error overlay and no application console errors after filtering local browser-extension warnings.
  - From `生产视图` task-row `工单`, `工位视图` task-row `详情`, `生产工单` row `执行枢纽`, and `工序卡` work-order link, the same `生产执行枢纽` drawer opened.
  - The drawer rendered `执行 readiness`, `WIP 状态`, `质检状态`, `工序进度`, context actions, cost and `追溯 timeline` with filters `全部 / 工序 / 库存 / 质检 / 成本 / 日志`.
  - Browser data examples proved blocking/readiness text is API/read-model driven: workstation entry showed `前序工序未完成` with next handler `现场主管`; work-order entry showed abnormal operations with `未分配工位` and next handler `生产负责人`.
  - Context action proof: in the execution hub action row the buttons were `开始生产` (disabled), `打开/创建 WIP 领料`, `打开工序卡`, `打开质检`, `完工入库`, `成本`, `日志`; clicking `打开/创建 WIP 领料` navigated to `view=stockOperations&tab=wip&work_order_id=34&job_card_id=51`, and the target page headings were `库存作业 / WIP在制仓 / WIP批次库存`.
