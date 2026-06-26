# PR-499 Production Execution Hub Phase 2 Acceptance Evidence

Date: 2026-06-21 Asia/Shanghai
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
- GREEN: `node --test src/lib/production-execution-hub.test.js src/lib/production-workstation.test.js src/lib/produce-plan.test.js src/lib/produce-running.test.js src/lib/menu-ia.test.js src/lib/view-routing.test.js src/lib/work-orders.test.js src/lib/quality-inspections.test.js src/lib/production-costs.test.js src/lib/production-logs.test.js` passed 99/99.
- GREEN: `npm run build` in `frontend-vue-shell` passed after `npm ci`; Vite reported the existing chunk-size warning.
- GREEN: `scripts/verify_kferp.sh changed`.
- GREEN: `git diff --check`.

## Development Deployment And Browser Acceptance
- Pending: feature branch push.
- Pending: merge to latest `develop`.
- Pending: deployment to development.
- Pending: ERP browser acceptance for 生产视图、工位视图、生产计划、生产中、工单、工序卡.
