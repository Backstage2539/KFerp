import test from 'node:test'
import assert from 'node:assert/strict'

import {
  parseProductionMaterialSummary,
  productionLogMaterialBatchCodes,
  productionLogMaterialSummaryText,
} from './production-logs.js'

test('production log helpers expose finished trace material batch evidence', () => {
  const raw = JSON.stringify([
    { material_id: 1, material_name: '卡蒂姆水洗', unit: 'g', deduct_g: 600, batch_code: 'MB-RAW-001' },
    { material_id: 9, material_name: '豆袋', unit: '个', deduct_units: 2, material_batch_code: 'MB-BAG-001' },
    { material_id: 1, material_name: '卡蒂姆水洗', unit: 'g', deduct_g: 200, batch_code: 'MB-RAW-001' },
  ])

  assert.equal(parseProductionMaterialSummary(raw).length, 3)
  assert.deepEqual(productionLogMaterialBatchCodes(raw), ['MB-RAW-001', 'MB-BAG-001'])
  assert.equal(
    productionLogMaterialSummaryText(raw),
    '卡蒂姆水洗(MB-RAW-001): 600g\n豆袋(MB-BAG-001): 2个\n卡蒂姆水洗(MB-RAW-001): 200g',
  )
  assert.deepEqual(productionLogMaterialBatchCodes('not-json'), [])
})
