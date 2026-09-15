import assert from 'node:assert/strict'
import { test } from 'node:test'
import {
  customerProcessingProgress,
  customerProcessingReceiptResult,
  customerProcessingSourceLabel,
  customerProcessingTimeline,
} from './customer-processing-trace.js'

test('customer processing trace uses customer request as source and preserves quantities', () => {
  assert.equal(customerProcessingSourceLabel('customer_processing'), '客户工单')
  assert.deepEqual(customerProcessingProgress({
    target_qty: 100,
    actual_inbound_qty: 10,
    output_reserved_qty: 20,
    output_converted_qty: 10,
  }), {
    requestedQty: 100,
    inboundQty: 10,
    orderOccupiedQty: 10,
    remainingReservableQty: 80,
  })

  assert.deepEqual(customerProcessingProgress({
    target_qty: 100,
    actual_inbound_qty: 10,
    output_reserved_qty: 20,
    output_converted_qty: 10,
    output_released_qty: 5,
  }), {
    requestedQty: 100,
    inboundQty: 10,
    orderOccupiedQty: 5,
    remainingReservableQty: 85,
  })
})

test('customer processing trace never invents missing timestamps', () => {
  assert.deepEqual(customerProcessingTimeline({
    status: 'released',
    created_at: '2026-09-16 09:00',
    accepted_at: '2026-09-16 10:00',
  }).map((item) => [item.label, item.time]), [
    ['待接单', '2026-09-16 09:00'],
    ['已接单', '2026-09-16 10:00'],
    ['开始生产', '暂无记录'],
    ['生产完成', '暂无记录'],
  ])
})

test('customer processing receipt result separates this receipt, occupied stock and available stock', () => {
  assert.deepEqual(customerProcessingReceiptResult({
    document: {
      id: 701,
      entry_no: 'SE-0000000701',
      items: [{ qty_units: 10, batch_code: 'FG-20260916-01', to_warehouse: 'CUST-454' }],
    },
    workOrder: {
      execution_hub: {
        header: { work_order_id: 88, work_order_no: 'WO-0000000088' },
        quality_status: { status: 'pass', result: '合格' },
      },
    },
    request: {
      request_no: 'CPR-0000000067',
      items: [{
        id: 6671,
        actual_inbound_qty: 100,
        output_reserved_qty: 20,
        output_converted_qty: 20,
        output_released_qty: 0,
        related_orders: [
          { order_id: 31, order_no: 'SO-0000000031', status: 'reserved', reserved_qty: 20, converted_qty: 20 },
        ],
      }],
    },
    processingRequestItemID: 6671,
  }), {
    stockEntryID: 701,
    stockEntryNo: 'SE-0000000701',
    workOrderID: 88,
    workOrderNo: 'WO-0000000088',
    requestNo: 'CPR-0000000067',
    receiptQty: 10,
    inboundQty: 100,
    occupiedQty: 20,
    availableQty: 80,
    warehouse: 'CUST-454',
    batchCode: 'FG-20260916-01',
    qualityLabel: '合格',
    convertedOrders: [{ order_id: 31, order_no: 'SO-0000000031', status: 'reserved', reserved_qty: 20, converted_qty: 20 }],
  })
})
