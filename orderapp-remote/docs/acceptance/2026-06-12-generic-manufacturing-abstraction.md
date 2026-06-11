# PR-473-GENERIC-MANUFACTURING-ABSTRACTION

## Scope
- 生产计划和工单主流程改为通用制造口径：商品、BOM、工艺路线、工序、工位、生产计划、工单、工序卡。
- 生产计划页不再展示烘焙排产建议，不请求 /api/produce/machines。
- 创建生产计划仍生成 draft 单据，但前端不再提交 `input_by_key`，由后端默认 BOM、预期损耗率和缺口计算计划投料。
- 工单页把历史 `roast_level` 作为 `工艺参数` 兼容展示，不作为咖啡烘焙主列。

## Verification Plan
- 前端：`node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js`
- API：`go test ./internal/interfaces/http/production -run TestProductionPlanAPICreatesListsAndSubmitsFormalPlan -count=1`
- 支持合同：`go test ./internal/interfaces/http/support -run 'TestDev473GenericManufacturingAbstractionContracts|TestProducePlanCapacitySuggestionCompatibilitySourceGuard' -count=1`
- 构建：`npm run build`

## Acceptance Scenarios
- 咖啡熟豆：`BOM = 生豆 + 包材`，`工艺路线 = 烘焙 -> 包装 -> 质检`，计划和工单正常生成。
- 包装盒：`BOM = 纸板 + 油墨 + 胶水`，`工艺路线 = 印刷 -> 模切 -> 糊盒 -> 质检`，页面不出现烘焙建议、推荐机器、每锅数量或锅数。
- 童装：`BOM = 布料 + 辅料`，`工艺路线 = 裁剪 -> 缝制 -> 整烫 -> 质检`，页面不出现每锅、锅数、熟豆或生豆主流程词。
- PR439：`selected=["539-454"]` 创建生产计划时，不要求前端提交 `input_g` 或 `input_by_key`。

## Evidence
- RED frontend: pending final run log in ACTIVE_REQUIREMENTS.md.
- GREEN frontend/API/support/build: pending final run log in ACTIVE_REQUIREMENTS.md.
