# PR-488 Production Plan Split Quantity Auto Batch

## Scope
- 生产计划 `工序产能拆分` 不再要求用户填写批次数。
- 拆分行填写本次工位产能承担的产量，系统按工位产能标准批量自动计算批次数。
- 计划数量保持等于承担产量；最后一批不满批时，只把批次数、计划分钟和工序成本向上取整，不放大计划数量。

## Acceptance Scenario
1. 创建工位产能 `布勒 18kg`、`智烘 4kg`。
2. 创建草稿生产计划并进入 `工序产能拆分`。
3. 添加 `布勒 18kg` 拆分行，填写承担产量 90kg。
4. 页面自动显示 5 批、90000g、计划分钟和计划工序成本。
5. 添加 `智烘 4kg` 拆分行，填写承担产量 8kg。
6. 页面自动显示 2 批、8000g、计划分钟和计划工序成本。
7. 若 `布勒 18kg` 承担产量改为 20kg，页面显示计划数量 20000g、自动批次数 2，计划数量不变成 36000g。
8. 保存拆分并提交生成工单后，工序卡冻结对应工位产能、计划批次数、计划投入、计划分钟和计划工序成本。

## Evidence
- RED frontend: `node --test src/lib/produce-plan.test.js` failed before implementation because capacity split helper ignored `planned_qty` and the page still displayed manual `批次数`.
- RED service/API/repository: targeted Go tests failed before implementation with `planned_batch_count required` and `PlannedBatchCount=0, want 2`.
- GREEN frontend: `node --test src/lib/produce-plan.test.js` passed after changing the UI and helper to `承担产量` + `自动批次数`.
- GREEN service/API/repository: targeted Go tests passed after `planned_qty` became the save contract and repository metrics used `ceil(planned_qty / batch_size_qty)`.
