# PR-424-SHIPPING-DEDUCT-PRODUCED-FINISHED-STOCK

## Scope

- 生产完成订单在发货回填快递单号时，默认成品仓 `finished_goods` 也必须扣减订单行成品库存。
- 无 `order_stock_batch_allocations` 分配记录时，发货扣减按订单来源仓和订单行规格/数量兜底计算。

## Evidence

- RED live: GoalE2E 订单 `SO-20260605-0001` 生产完成后调用 `/api/orders/1487/shipping-tracking` 返回成功，订单变为 `已发货` 且快递号为 `SF-0605234447`，但 `finished_inventory` 未变化，`order_stock_deductions` 和 `sales_order_shipment` 库存流水均为 0。
- Root cause: 发货扣库存只在有 `order_stock_batch_allocations` 时执行；无分配时仅非默认来源仓走订单行兜底扣减，默认 `finished_goods` 被跳过。
- RED local: `go test ./internal/interfaces/http/support -run TestDev424ShippingDeductsDefaultFinishedInventoryWithoutAllocation -count=1` failed before implementation because source contained `len(allocations) == 0 && warehouse != "finished_goods"`。
- GREEN local: `go test ./internal/interfaces/http/support -run TestDev424ShippingDeductsDefaultFinishedInventoryWithoutAllocation -count=1` passed after making no-allocation fallback apply to default `finished_goods` as well.
- API behavior test present: `TestOrdersShippingTrackingAPIDeductsDefaultFinishedInventoryWithoutAllocation` covers a `生产完成` order with default `finished_goods` and no allocation; it requires `ORDERAPP_TEST_DATABASE_URL`.

## Deployment Acceptance

- Passed on development after deploy `149cb31d41741f745d89bc0279af50baa6a7449e`.
- Replayed `/api/orders/1487/shipping-tracking` for `SO-20260605-0001`: order remained `已发货`, `ship_tracking_no=SF-0605234447`, four `sales_order_shipment` rows were written, and duplicate replay did not double-deduct.
- Follow-up PR-425 fixed the batch-stock UI side so `仓库库存` also reflects the shipment deduction.
