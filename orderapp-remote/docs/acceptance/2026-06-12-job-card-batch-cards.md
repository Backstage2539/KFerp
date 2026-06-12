# PR-490 Job Card Batch Cards

## Scope

- PR-490-JOB-CARD-BATCH-CARDS：生产计划 `工序产能拆分` 继续按一条拆分记录保存，但按自动批次数渲染批次卡片。
- `布勒 18kg` 承担 72kg 时显示 4 个批次卡片；承担 20kg 时显示 2 个批次卡片，最后一批显示 2kg 并标记不足标准批量。
- 工序卡主表隐藏 `计划投入 / 实际投入 / 实际产出` 三列和输入框；后端字段和保存实际接口兼容保留。

## Acceptance

- 创建草稿生产计划。
- 在某道工序下选择 `布勒 18kg`，填写承担产量 `72kg`。
- 页面显示 `自动批次数 4`，并渲染 `第1批`、`第2批`、`第3批`、`第4批`。
- 将承担产量改为 `20kg` 时，页面显示 2 个批次卡片，最后一批显示 `2kg` 和 `不足标准批量`。
- 提交当前计划生成工单后进入工序卡页，看不到 `计划投入 / 实际投入 / 实际产出` 表头和输入框。
- 工序卡仍可录入 `实际分钟`、`损耗原因`、`异常原因`，并执行开始、暂停、继续、完成和保存实际。

## Evidence

- RED frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` failed before implementation because `productionPlanSplitBatchCards` was missing and JobCardsView still exposed the hidden fields.
- GREEN frontend: `node --test src/lib/produce-plan.test.js src/lib/work-orders.test.js` passed after implementation.
- RED support/docs: `go test ./internal/interfaces/http/support -run TestDev490JobCardBatchCardsContracts -count=1 -v` failed before PR-490 docs and seed markers were added.
