import test from 'node:test'
import assert from 'node:assert/strict'
import { dripUnitOptions, validateDripProduct } from './drip-product.js'

test('drip unit options expose bag and box', () => {
  assert.deepEqual(dripUnitOptions({ drip_box_bag_count: 10 }), [
    { value: 'bag', label: '袋', spec: '10g/袋' },
    { value: 'box', label: '盒', spec: '10袋/盒' }
  ])
})

test('validate drip product requires positive bag grams and box count', () => {
  assert.deepEqual(validateDripProduct({ product_kind: 'drip_bag', drip_bag_grams: 0, drip_box_bag_count: 10 }), ['每袋熟豆克重必须大于 0'])
})
