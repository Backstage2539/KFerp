# PR-425-SHIPPING-DEDUCT-PRODUCED-STOCK-BATCHES

## Scope

- 发货扣减生产完成订单时，默认成品仓无分配订单必须优先扣减 `stock_batches` 成品批次。
- 只有不存在可用成品批次时，才回退旧 `finished_inventory` 汇总库存兼容路径。

## Evidence

- RED browser/live: PR-424 部署后，GoalE2E 订单 `SO-20260605-0001` 的 `finished_inventory` 和 `sales_order_shipment` 流水已更新，但内置浏览器 `仓库库存` 仍显示成品批次 `FP-0000000031/32/33/34` 保持原始数量，说明仓库库存页仍认为成品可用。
- Root cause: 无分配发货兜底只调用 `deductLegacyFinishedInventoryAllocationTx`，没有复用成品批次 FIFO 分配和 `deductFinishedBatchAllocationTx`。
- RED local: `go test ./internal/interfaces/http/support -run TestDev425ShippingNoAllocationFallbackDeductsFinishedBatches -count=1` failed before implementation because no-allocation fallback did not call `previewOrderStockBatches` or `deductFinishedBatchAllocationTx`.
- GREEN local: `go test ./internal/interfaces/http/support -run 'TestDev425ShippingNoAllocationFallbackDeductsFinishedBatches|TestDev424ShippingDeductsDefaultFinishedInventoryWithoutAllocation' -count=1` passed.
- API behavior test present: `TestOrdersShippingTrackingAPIDeductsDefaultFinishedBatchWithoutAllocation` covers a `生产完成` order with default `finished_goods`, no allocation, and two available成品批次; it requires `ORDERAPP_TEST_DATABASE_URL`.

## Deployment Acceptance

- Pending: deploy to development, repair/replay GoalE2E order `SO-20260605-0001`, and verify `仓库库存` no longer shows those produced成品批次 as available.
- Expected: `stock_batches.remaining_g/remaining_units` for the shipped GoalE2E quantities are deducted and repeat tracking update does not double-deduct.
