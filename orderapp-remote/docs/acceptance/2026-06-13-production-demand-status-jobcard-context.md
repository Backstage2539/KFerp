# PR-491-PRODUCTION-DEMAND-STATUS-JOBCARD-CONTEXT

## Summary
- 待生产需求新增 `待计划 / 生产中 / 生产完成` 状态和状态过滤。
- 已进入生产计划或工单的需求不可重复勾选生成生产计划，前后端都做保护。
- 生产计划页内层宽表释放滚动边界，滚到列表顶/底后页面继续滚动。
- 工序卡主表展示商品和 `BOM/配方`，工单号链接打开右侧工单详情抽屉，抽屉展示配方物料快照。

## Acceptance
- 在生产计划页选择一个 `待计划` 需求创建草稿生产计划；刷新后该需求显示 `生产中`，复选框置灰并提示 `已进入生产计划的需求不可重复生成计划`。
- 用需求状态筛选 `待计划 / 生产中 / 生产完成` 时，列表按状态返回。
- 在待生产需求、当前生产计划、库存充足和生产计划单据宽表内滚动到顶/底后，继续滚轮页面仍能滚动。
- 进入工序卡页，行内能看到商品和 `BOM/配方`；点击工单号打开 `工单详情` 抽屉，并看到商品、规格、订单号、计划数量、BOM/配方和 `配方物料`。

## Evidence
- RED frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` failed before implementation because production demand status helpers and job-card product/BOM drawer markers were missing.
- RED backend/API: `go test ./internal/interfaces/http/production -run 'TestParseUnprodSummaryQueryIncludesDemandStatusFilter|TestProducePlanSummaryAPIMarksPlannedDemandAsInProductionAndFiltersIt|TestJobCardAPIIncludesActualLossFields' -count=1 -v` failed before implementation because `DemandStatus` and job-card context fields were missing.
- RED support: `go test ./internal/interfaces/http/support -run TestDev491ProductionDemandStatusJobCardContextContracts -count=1 -v` failed before PR-491 docs/seed markers existed.
- GREEN frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` passed 33/33.
- GREEN backend/API: `go test ./internal/interfaces/http/production -run 'TestParseUnprodSummaryQueryIncludesDemandStatusFilter|TestProducePlanSummaryAPIMarksPlannedDemandAsInProductionAndFiltersIt|TestJobCardAPIIncludesActualLossFields' -count=1 -v` passed with the database-backed summary scenario skipped when `ORDERAPP_TEST_DATABASE_URL`/`DATABASE_URL` was not configured.
- GREEN support: `go test ./internal/interfaces/http/support -run TestDev491ProductionDemandStatusJobCardContextContracts -count=1 -v` passed.
- GREEN broader: `go test ./internal/application/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/production ./internal/interfaces/http/support -count=1`; `go test ./...`; `npm run build` in `frontend-vue-shell`; `scripts/verify_kferp.sh changed`; `git diff --check` passed.
- GREEN browser/local: local production Vue build + mock API + headless Chrome opened `http://127.0.0.1:5204/vue-shell/?view=producePlan` and `?view=jobCards`. Verified `待计划` demand checkbox enabled, `生产中` and `生产完成` rows disabled with `已进入生产计划的需求不可重复生成计划`, table wrappers use `overscroll-behavior: auto`, job card row shows `榛巧拼配` and `BOM版本 #723`, and clicking `WO-PR491-001` opens `工单详情` with `配方物料 / 孟连水洗`.
