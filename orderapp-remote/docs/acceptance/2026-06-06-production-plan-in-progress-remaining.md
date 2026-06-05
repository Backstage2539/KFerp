# PR-422-PRODUCTION-PLAN-IN-PROGRESS-REMAINING

## Scope

- 生产计划继续纳入 `生产中` 订单的剩余未完成商品。
- 主生产缺口查询和挂耳专用缺口查询共用开放状态过滤器。

## Evidence

- RED live: GoalE2E 订单 `SO-20260605-0001` 完成熟豆、生豆、速溶后，挂耳仍无成品库存和生产日志，但 `/api/produce/unproduced` 返回 0 行，直接启动 `534-10` 返回 `没有可开始生产的数据`。
- RED local: `go test ./internal/interfaces/http/support -run TestDev422ProductionPlanIncludesInProgressOrders -count=1` failed before implementation because plan queries did not use a shared filter including `生产中`.
- GREEN local: `go test ./internal/interfaces/http/support -run TestDev422ProductionPlanIncludesInProgressOrders -count=1` passed after adding `productionPlanOpenStatusFilter`.
- Integration test present: `TestProducePlanIncludesInProgressOrdersWithRemainingItems` covers a `生产中` order with remaining roasted and drip-bag items; it requires `ORDERAPP_TEST_DATABASE_URL`.

## Deployment Acceptance

- Pending: deploy to development and replay GoalE2E remaining drip-bag production.
- Expected: `/api/produce/unproduced?from=2026-06-05&to=2026-06-05&customer_id=164` returns the remaining `GoalE2E-0605-234447 咖啡挂耳` row for `SO-20260605-0001`.
- Expected: `/api/produce/start` accepts selected `534-10`, and `/api/produce/running/finish` writes a production log for product `534`.
