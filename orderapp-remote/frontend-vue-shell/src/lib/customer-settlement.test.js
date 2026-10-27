import test from 'node:test'
import assert from 'node:assert/strict'
import { settlementPeriodRange, settlementChargeOptions } from './customer-settlement.js'

test('月结上月、当月使用上海时区完整自然月并正确跨年', () => {
  const now = new Date('2026-01-01T00:30:00+08:00')
  assert.deepEqual(settlementPeriodRange('previous', now), { period_from: '2025-12-01', period_to: '2025-12-31' })
  assert.deepEqual(settlementPeriodRange('current', now), { period_from: '2026-01-01', period_to: '2026-01-31' })
  assert.equal(settlementPeriodRange('custom', now), null)
})

test('月结费用选择包含核心三类与已有其他费用类型', () => {
  const options = settlementChargeOptions([{ fee_type: 'storage', fee_name: '仓储费' }, { fee_type: 'custom_cost', fee_name: '展会耗材' }])
  assert.deepEqual(options.slice(0, 3).map(option => option.value), ['order_goods', 'shipping', 'direct_ship_service'])
  assert.ok(options.some(option => option.value === 'storage' && option.label === '仓储费'))
  assert.ok(options.some(option => option.value === 'custom_cost' && option.label === '展会耗材'))
})
