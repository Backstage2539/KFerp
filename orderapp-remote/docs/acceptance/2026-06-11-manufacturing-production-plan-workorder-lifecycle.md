# PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE

## Scope
- 一期生产链路：`生产计划 -> 生产工单 -> 工序卡 -> 开始生产`。
- 新增正式 `production_plans`、`production_plan_items`，计划提交生成 `released` 工单和 `pending` 工序卡。
- 工单通过 `POST /api/work-orders/:id/start` 开始生产后，才创建 running item、WIP 占用并进入现有生产中/完工链路。
- 旧 `POST /api/produce/start` 保留兼容，内部走临时计划 -> 工单 -> 开始生产。
- 不包含 Stock Entry 单据化、甘特图、产能排程、班组计时、工序暂停。

## IDs
- PR: `PR-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE`
- DEV:
  - `DEV-472-PRODUCTION-PLAN-SCHEMA-REPOSITORY`
  - `DEV-472-PLAN-SUBMIT-WORKORDER-JOBCARDS`
  - `DEV-472-WORKORDER-START-LIFECYCLE`
  - `DEV-472-LEGACY-PRODUCE-START-COMPAT`
  - `DEV-472-VUE-PRODUCTION-PLAN-WORKORDERS`
  - `DEV-472-DOCS-ACCEPTANCE`
- UT: `UT-472-MANUFACTURING-PRODUCTION-PLAN-LIFECYCLE`
- API: `API-472-MANUFACTURING-PRODUCTION-PLAN-LIFECYCLE`
- REV: `REV-472-MANUFACTURING-PRODUCTION-PLAN-WORKORDER-LIFECYCLE`

## RED Evidence
- Schema RED: `go test ./internal/infrastructure/postgres/production -run 'TestProductionPlanSchemaCreatesFormalPlanTables|TestWorkOrderSchemaAllowsReleasedOrdersBeforeRunningItem' -count=1` failed before implementation because `production_plans` / `production_plan_items` were missing and `work_orders.running_item_id` was still `BIGINT NOT NULL UNIQUE`.
- Service RED: `go test ./internal/application/production -run 'TestServiceOwnsFormalProductionPlanWorkOrderLifecycle|TestServiceRejectsInvalidProductionPlanAndWorkOrderCommands' -count=1` failed before implementation because `CreateProductionPlanCommand`, `SubmitProductionPlanCommand`, `WorkOrderStartCommand` and plan detail DTOs did not exist.
- API RED: `go test ./internal/interfaces/http/production -run 'TestProductionPlanAPICreatesListsAndSubmitsFormalPlan|TestWorkOrderStartAPIStartsReleasedWorkOrder|TestProductionPlanRepositoryCreatesSubmitsAndStartsFormalLifecycle|TestLegacyProduceStartAPIUsesTemporaryPlanAndStillStartsProduction' -count=1` failed before implementation because production-plan and work-order start application types/routes were missing.
- Frontend RED: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` failed before implementation because `buildProductionPlanCreatePayload` was not exported and `src/lib/work-orders.js` did not exist.
- Support/docs RED: `go test ./internal/interfaces/http/support -run TestDev472ManufacturingProductionPlanLifecycleContracts -count=1` failed because PR-472 manual/docs markers were not wired.

## GREEN Evidence
- Backend application: `go test ./internal/application/production -count=1` passed.
- Backend schema/repository source tests: `go test ./internal/infrastructure/postgres/production -count=1` passed.
- Backend API: `go test ./internal/interfaces/http/production -count=1` passed. DB-backed repository flow cases skip when `ORDERAPP_TEST_DATABASE_URL` / `DATABASE_URL` is unavailable.
- Support/docs contract: `go test ./internal/interfaces/http/support -run TestDev472ManufacturingProductionPlanLifecycleContracts -count=1` passed.
- Frontend node tests: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` passed 15/15.
- Full Go: `go test ./...` passed.
- Vue build: `npm run build` passed with existing Vite plugin-timing and chunk-size warnings.
- Changed verifier: `scripts/verify_kferp.sh changed` exited 0.
- Whitespace: `git diff --check` exited 0.

## Browser Acceptance
- Local Vue shell acceptance with mocked API passed at `http://127.0.0.1:5177/vue-shell/?view=producePlan`.
- 生产计划/开始生产：selected `咖啡豆订单-红岩拼配`, generated plan preview, posted `POST /api/production-plans`, displayed `PP-0000000001 · draft`, submitted via `POST /api/production-plans/1/submit`, and did not call old `/api/produce/start` from the new UI.
- 生产工单：submitted plan navigated to 生产工单, displayed released `WO-PP-0000000001-0000000001`, froze `咖啡烘焙包装路线`, clicked `开始生产`, posted `POST /api/work-orders/77/start`, and reloaded running batch `PB-PR472-001`.
- 工序卡：displayed coffee route `烘焙/包装`, packaging route `印刷/模切/糊盒`, and clothing route `裁剪/缝制/质检`.
- Screenshots:
  - `/tmp/kferp-pr472-browser-evidence/01-production-plan-created.png`
  - `/tmp/kferp-pr472-browser-evidence/02-work-order-started.png`
  - `/tmp/kferp-pr472-browser-evidence/03-job-cards-routes.png`

## Manual Updates
- `orderapp-remote/docs/REQUIREMENTS.md`
- `orderapp-remote/docs/ACCEPTANCE_TESTS.md`
- `orderapp-remote/docs/OP_MANUAL_PRODUCTION.md`
- `orderapp-remote/docs/OP_MANUAL_COSTING.md`
