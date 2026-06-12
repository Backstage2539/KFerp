# PR-489 Production Plan Preview Layout

## Scope

- 生产计划页当前计划工作台左右两栏支持收起和展开。
- 待生产需求、计划预览、物料需求和生产计划单据等宽表支持横向拖拽滚动。
- 未创建草稿生产计划前，当前计划区展示 `创建草稿生产计划后可填写工序产能拆分`，说明拆分需要先创建 draft 计划。

## Acceptance

- 勾选库存不足商品后，计划预览表格展示 `库存(g)`、`缺口(g)`、`BOM摘要`、`计划投料(g)` 和 `工艺路线摘要`。
- 按住计划预览表格空白区域左右拖动，可以查看超出屏幕的 BOM 摘要、计划投料和工艺路线摘要列。
- 点击 `收起待生产需求` 后，当前生产计划区域变宽；再次点击 `展开待生产需求` 后左侧恢复。
- 点击 `收起当前生产计划` 后，待生产需求区域变宽；再次点击 `展开当前生产计划` 后右侧恢复。
- 未创建草稿时能看到 `创建草稿生产计划后可填写工序产能拆分`。
- 点击 `创建生产计划` 成功生成草稿后，当前生产计划显示实际 `工序产能拆分` 编辑区。

## Evidence

- RED frontend: `node --test src/lib/produce-plan.test.js` failed before implementation because collapse, drag-scroll and split hint markers were missing.
- GREEN frontend: `node --test src/lib/produce-plan.test.js` passed after implementation.
- RED support/docs: `go test ./internal/interfaces/http/support -run TestDev489ProductionPlanPreviewLayoutContracts -count=1 -v` failed before PR-489 docs/seed markers were added.
