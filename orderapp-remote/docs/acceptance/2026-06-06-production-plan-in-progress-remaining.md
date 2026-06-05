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

- Passed on development after deploy `c3acd2c2894927deda34d1d8203019d246e45e6b`.
- `/api/produce/unproduced?from=2026-06-05&to=2026-06-05&customer_id=164` returned remaining GoalE2E rows after the order had entered `生产中`, including `GoalE2E-0605-234447 咖啡挂耳` `534-10`.
- Follow-up PR-423 then allowed `/api/produce/start` and `/api/produce/running/finish` to complete the remaining挂耳 production.
