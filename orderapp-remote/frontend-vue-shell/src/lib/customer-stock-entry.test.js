import test from 'node:test'
import assert from 'node:assert/strict'
import { buildOrderPayload } from './order-entry.js'
import { customerStockWarehouses, recipientAddressSnapshot, stockSupplySummary } from './customer-stock-entry.js'

test('留厂购货把交付方式与目标客户仓提交到销售接口', () => {
  const payload = buildOrderPayload({ form: { customer_id: 77, delivery_mode: 'customer_stock', delivery_warehouse_code: 'XI-QIN' }, rows: [] })
  assert.equal(payload.delivery_mode, 'customer_stock')
  assert.equal(payload.delivery_warehouse_code, 'XI-QIN')
})

test('留厂目标只能选当前客户启用的成品仓', () => {
  assert.deepEqual(customerStockWarehouses([{ code: 'ok', customer_id: 77, kind: 'finished', active: true }, { code: 'other', customer_id: 88, kind: 'finished' }, { code: 'raw', customer_id: 77, kind: 'raw' }, { code: 'off', customer_id: 77, kind: 'finished', active: false }], 77).map(row => row.code), ['ok'])
})

test('留厂客户仓种类与服务端一致，含客户成品仓和客户加工仓', () => {
  const kinds = ['finished', 'customer_finished', 'customer_processing', 'customer', 'raw', 'finished_product']
  const rows = kinds.map(kind => ({ code: kind, customer_id: 77, active: true, kind }))
  assert.deepEqual(customerStockWarehouses(rows, 77).map(row => row.code), ['finished', 'customer_finished', 'customer_processing', 'customer'])
})

test('共享地址选择生成订单快照，修改快照不改变地址库', () => {
  const address = { recipient_name: '测试收件人', phone: '13800000000', company: '测试门店', province: '云南省', city: '大理市', detail_address: '测试路1号' }
  const snapshot = recipientAddressSnapshot(address)
  assert.equal(snapshot.receiver_address, '云南省大理市测试路1号')
  snapshot.receiver_name = '本单修改'
  assert.equal(address.recipient_name, '测试收件人')
})

test('未入库权益扣除已预订量；待入库不显示为实物', () => {
  assert.deepEqual(stockSupplySummary({ physical_available_qty: 4, pending_inbound_qty: 20, pending_reserved_qty: 12 }, 14), { physical: 4, pending: 20, reserved: 12, gap: 2 })
  assert.equal(stockSupplySummary({ physical_available_qty: 0, pending_inbound_qty: 24 }, 12).physical, 0)
})

test('普通发货清除上一次留厂订单的目标仓，防止误入客户库存', () => {
  const payload = buildOrderPayload({ form: { customer_id: 77, delivery_mode: 'ship', delivery_warehouse_code: 'XI-QIN' }, rows: [] })
  assert.equal(payload.delivery_mode, 'ship')
  assert.equal(payload.delivery_warehouse_code, '')
})
