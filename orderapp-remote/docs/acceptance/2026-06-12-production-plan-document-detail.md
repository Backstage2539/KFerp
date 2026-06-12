# PR-478-PRODUCTION-PLAN-DOCUMENT-DETAIL

## Scope
- 增强生产计划单据详情，不改变生产计划创建流程、历史列表批量提交、状态过滤或时间过滤。
- 生产计划单据列表保持紧凑；点击计划号或详情打开生产计划单据详情抽屉。
- 详情抽屉展示单据头、计划行、BOM 版本、工艺路线摘要、物料需求汇总、工艺参数/商品生产配置快照和生成结果。
- `GET /api/production-plans/:id` 新增只读派生字段 `material_summary`、`related_work_orders` 和 `job_card_count`。

## Acceptance
- [ ] 点击计划号或详情打开生产计划单据详情抽屉，不离开生产计划页。
- [ ] 草稿计划详情显示计划行、物料需求汇总和“尚未生成工单”。
- [ ] 已提交计划详情显示关联工单、工单状态和工序卡数量，并能跳转到生产工单或工序卡页面。
- [ ] 咖啡烘焙度等行业字段只在 `工艺参数 / 商品生产配置快照` 中展示；页面不出现生产建议、推荐机器、每锅数量、锅数或预计成品。

## Verification
- RED frontend: `node --test src/lib/produce-plan.test.js` failed before implementation because production plan detail endpoint and drawer markers were missing.
- RED backend/API: `go test ./internal/interfaces/http/production -run TestProductionPlanAPIDetailIncludesDocumentSummary -count=1 -v` failed before detail response fields existed.
- RED repository: `go test ./internal/infrastructure/postgres/production -run 'TestAggregateProductionPlanMaterialSummary' -count=1 -v` failed before material snapshot aggregation existed.
- RED support/docs: `go test ./internal/interfaces/http/support -run TestDev478ProductionPlanDetailDrawerContracts -count=1 -v` failed before PR-478 docs and seed markers existed.
- GREEN frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` passed 22/22.
- GREEN backend/API: `go test ./internal/application/production ./internal/interfaces/http/production ./internal/infrastructure/postgres/production ./internal/interfaces/http/support ./internal/architecture -count=1` passed.
- GREEN build/check: `npm run build` passed in `frontend-vue-shell` with the existing Vite chunk-size warning; `scripts/verify_kferp.sh changed` and `git diff --check` passed.
- GREEN browser/local: local production Vue build + mock API at `http://127.0.0.1:5192/vue-shell/?view=producePlan` rendered the production plan page. Clicking `PP-PR478-SUBMITTED` opened the production plan document detail drawer with `单据头`、`计划行`、`物料需求汇总`、`工艺路线摘要`、`工艺参数 / 商品生产配置快照`、`生成结果`、`WO-PR478-001` and `工序卡 4 张`. Page text did not include `生产建议`、`推荐机器`、`每锅数量`、`锅数` or `预计成品`. Mobile viewport 390x844 had no page-level horizontal overflow.
