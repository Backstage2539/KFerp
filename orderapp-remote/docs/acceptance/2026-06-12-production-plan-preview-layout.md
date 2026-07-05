# PR-489 Production Plan Preview Layout

## Scope

- 生产计划页当前计划工作台左右两栏支持收起和展开。
- 待生产需求、计划预览、物料需求和生产计划单据等宽表支持横向拖拽滚动。
- 当前计划工作台只展示计划预览、物料和提交动作，不内嵌工序产能拆分表；拆分入口在步骤条第 3 步和草稿单据 `编辑拆分`。

## Acceptance

- 勾选库存不足商品后，计划预览表格展示 `库存(g)`、`缺口(g)`、`BOM摘要`、`计划投料(g)` 和 `工艺路线摘要`。
- 按住计划预览表格空白区域左右拖动，可以查看超出屏幕的 BOM 摘要、计划投料和工艺路线摘要列。
- 点击 `收起待生产需求` 后，当前生产计划区域变宽；再次点击 `展开待生产需求` 后左侧恢复。
- 点击 `收起当前生产计划` 后，待生产需求区域变宽；再次点击 `展开当前生产计划` 后右侧恢复。
- 当前计划工作台不显示 `工序产能拆分` 表。
- 点击 `创建生产计划` 成功生成草稿后，点击步骤条第 3 步 `拆分产能` 可打开拆分抽屉。

## Evidence

- RED frontend: `node --test src/lib/produce-plan.test.js` failed before implementation because collapse, drag-scroll and split hint markers were missing.
- GREEN frontend: `node --test src/lib/produce-plan.test.js` passed after implementation.
- RED support/docs: `go test ./internal/interfaces/http/support -run TestDev489ProductionPlanPreviewLayoutContracts -count=1 -v` failed before PR-489 docs/seed markers were added.
