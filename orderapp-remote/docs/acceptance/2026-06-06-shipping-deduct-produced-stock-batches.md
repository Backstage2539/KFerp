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

- Passed on development after deploy `33e6ceb98901de79284a2c765d7118a451d37673`.
- GoalE2E repair/replay: deleted the four incorrect `SOURCE-WH:finished_goods` shipment deduction/ledger rows from PR-424 replay in a transaction and wrote both `order_audit_logs` and generic `audit_logs`, then reran `/api/orders/1487/shipping-tracking`.
- Result: `stock_batches` show熟豆 batch `FP-0000000032` remaining `227g / 1` after shipping 2 bags and挂耳 component consumption; 生豆 `FP-0000000031`, 挂耳 `FP-0000000034`, and速溶 `FP-0000000033` are `0g / 0`.
- Duplicate replay kept batch balances unchanged and kept `order_stock_deductions` / `sales_order_shipment` ledger count at 4.
- Browser acceptance: 内置浏览器 `仓库库存` 搜索 `GoalE2E-0605-234447` only shows the remaining熟豆 batch row; shipped生豆、挂耳、速溶 no longer appear as available stock.
