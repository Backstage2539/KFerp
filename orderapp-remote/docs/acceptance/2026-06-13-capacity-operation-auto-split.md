# PR-494-CAPACITY-OPERATION-AUTO-SPLIT

## Scope
- 工位产能可配置适用工序。
- 生产计划当前拆分区、草稿计划拆分抽屉、released 工单拆分抽屉按当前工序自动拆分产能。
- 包装类 `件/袋/盒/个` 产能按 `planned_g/spec_g` 换算计划数量。

## Acceptance
1. 在 `生产管理 -> 工位/设备` 创建或编辑 `布勒 10kg`、`智烘 3kg`，适用工序选择 `烘焙`。
2. 创建一个未配置适用工序的旧产能，确认下拉可手工选择，但点击 `自动拆分` 不会选中它。
3. 创建 23kg 草稿生产计划，进入烘焙工序的 `工序产能拆分`，点击 `自动拆分`。
4. 页面显示 `布勒 10kg` 承担 20kg、`智烘 3kg` 承担 3kg，并显示对应批次卡片、计划分钟和成本。
5. 手工新增拆分后点击 `分配剩余产量`，该行按当前产能最大整批数填入剩余量，仍可人工修改。
6. 打开生产计划单据列表或详情中的草稿 `编辑拆分`，确认抽屉行为与当前计划拆分区一致。
7. 提交生成 released 工单后，在生产工单页点击 `编辑拆分`，确认自动拆分和 `分配剩余产量` 行为一致，保存后 pending 工序卡重建。
8. 包装工序配置 `10袋`、`3袋` 产能；计划 10442g 且规格 454g 时，自动拆为 20袋 + 3袋，保存后工单/工序卡冻结正确批次数、分钟和成本。

## Evidence
- Frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js src/lib/process-routes.test.js`
- Backend: manufacturing capacity API/service/schema tests and production count-unit split metrics tests.
- Support: `go test ./internal/interfaces/http/support -run TestDev494CapacityOperationAutoSplitContracts -count=1`
