# PR-492-PRODUCTION-DEMAND-ADDON-ORDER-SPLIT

## Summary
- 修复同商品同规格已有老订单进入生产计划后，新加订单被整行判为 `生产中`、无法继续勾选生成计划的问题。
- 待生产需求按订单号拆分生产计划覆盖状态：已计划订单显示 `生产中`，未计划加单订单显示 `待计划`。
- 加单计划预览只包含未计划订单号，并按未计划订单数量计算缺口。

## Acceptance
- `榛巧拼配 454g` 中 `SO-20260612-0004 / SO-20260612-0005` 已进入生产计划时，后续 `SO-20260613-0003` 仍能作为 `待计划` 行勾选。
- 勾选加单行后，当前生产计划预览只展示未计划订单号和未计划订单缺口，不重复包含已进入生产计划的老订单。
- 状态筛选 `生产中` 可查看已计划订单，状态筛选 `待计划` 可查看加单订单。

## Evidence
- RED/API intent: `TestProducePlanSummaryAPILeavesAddOnOrdersSelectableWhenOlderOrdersPlanned` records the mixed old-order/add-on scenario; local DB-backed production API tests skip when `ORDERAPP_TEST_DATABASE_URL`/`DATABASE_URL` is not configured.
- GREEN unit: `go test ./internal/infrastructure/postgres/production -run TestSplitProductionDemandRowByPartsKeepsAddOnSelectable -count=1 -v` passed.
- GREEN frontend: `node --test src/lib/produce-plan.test.js` passed.
- GREEN targeted packages: `go test ./internal/infrastructure/postgres/production ./internal/interfaces/http/production -count=1` passed.
- SQL/live data check: development DB order-level query shows `SO-20260612-0004 / SO-20260612-0005` match existing production plan `PP-0000000040`, while `SO-20260613-0001 / SO-20260613-0003` for `榛巧拼配` have no production plan and must remain add-on demand.
