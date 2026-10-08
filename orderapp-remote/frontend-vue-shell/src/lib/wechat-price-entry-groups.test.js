import assert from 'node:assert/strict'
import { test } from 'node:test'
import { groupPriceEntries, priceEntryPurposeLabels } from './wechat-price-entry-groups.js'

test('price entries group by stable product type and retain two independent uses', () => {
  const rows = ['coffee', 'drip', 'instant'].flatMap((type_key, index) => [
    { key: type_key + '-ship', type_key, type_name: `类型${index}`, purpose: 'direct_ship' },
    { key: type_key + '-wholesale', type_key, type_name: `类型${index}`, purpose: 'wholesale' },
  ])
  const groups = groupPriceEntries(rows)
  assert.equal(groups.length, 3)
  assert.deepEqual(
    groups.map((group) => group.entries.map((entry) => entry.purpose)),
    Array.from({ length: 3 }, () => ['wholesale', 'direct_ship']),
  )
  assert.deepEqual(priceEntryPurposeLabels, { wholesale: '批发', direct_ship: '一件代发' })
})

test('legacy per-table rows are excluded from the new entry list', () => {
  assert.deepEqual(groupPriceEntries([{ key: 'legacy', name: '历史价格表' }]), [])
})
