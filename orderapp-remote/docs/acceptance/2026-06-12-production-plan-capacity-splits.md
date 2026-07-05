# PR-487 Production Plan Capacity Splits

## Scope
- 工艺路线只维护路线和工序顺序，不拥有工位、工位产能、单批标准、费率、批次数、计划分钟或计划工序成本。
- 工位产能继续作为工位下的单批标准主数据。
- 生产计划草稿拥有本次工序产能拆分：按计划行和工序选择工位产能、填写承担产量，系统自动计算批次数。
- 提交生产计划时，工单和工序卡冻结拆分行的工位产能、计划投入、计划批次数、计划分钟和计划工序成本。

## Acceptance Scenario
1. 创建工位产能 `布勒 18kg`、`智烘 4kg`。
2. 创建工艺路线，只选择 `烘焙` 工序，不选择工位产能或批次数。
3. 创建草稿生产计划。
4. 点击步骤条第 3 步 `拆分产能` 或草稿单据 `编辑拆分` 打开拆分抽屉，并添加：
   - `布勒 18kg` 承担 90kg，系统自动显示 5 批。
   - `智烘 4kg` 承担 8kg，系统自动显示 2 批。
5. 保存拆分并提交生成工单。
6. 工序卡显示对应冻结工位产能、批次数、计划投入、计划分钟和计划工序成本。
7. 修改工位产能主数据后，历史工序卡冻结值不变。

## Evidence
- Frontend: `node --test src/lib/process-routes.test.js src/lib/produce-plan.test.js` passed 26/26.
- Backend targeted: `go test ./internal/infrastructure/postgres/production ./internal/interfaces/http/production ./internal/application/production ./internal/application/manufacturing -run 'TestProductionPlanSchemaCreatesOperationCapacitySplitTable|TestProductionPlanOperationSplitsOwnCapacityBatchPlanning|TestProductionPlanOperationSplitAPIReadsAndSavesDraftCapacitySplits|TestSaveProcessRouteDropsWorkstationCapacityBatchTimeRateOwnership|TestSaveProcessRouteIgnoresCapacityWorkstationMismatch|TestSaveWorkstationCapacityNormalizesReusablePreset|TestProductionPlanCreateAndSubmitCreatesDraftThenReleasedWorkOrder' -count=1` passed.
- Backend packages: `go test ./internal/infrastructure/postgres/production ./internal/interfaces/http/production ./internal/application/production ./internal/application/manufacturing ./internal/infrastructure/postgres/manufacturing -count=1` passed.
- Support contracts: `go test ./internal/interfaces/http/support -run TestDev487ProductionPlanCapacitySplitContracts -count=1` passed.
- Full backend: `go test ./...` passed.
- Frontend build: `npm run build` in `orderapp-remote/frontend-vue-shell` passed with existing chunk-size warning.
- Repository verifier: `scripts/verify_kferp.sh changed` exited 0.
- Browser note: local Vite rendered the shell but stopped at `请求失败` because no local authenticated backend was running; run live ERP browser acceptance after merge/deploy.
