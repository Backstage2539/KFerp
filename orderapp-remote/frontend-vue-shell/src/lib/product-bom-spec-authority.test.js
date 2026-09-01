import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import test from 'node:test'

const productSource = readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
const bomSource = readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')

test('product archive exposes only default BOM specifications and no legacy migration controls', () => {
  assert.ok(!productSource.includes('bom-spec-migration'))
  assert.ok(!productSource.includes('个历史规格'))
  assert.ok(!productSource.includes('row.sku_rows'))
  assert.ok(productSource.includes('未配置 BOM 规格'))
  assert.ok(productSource.includes('到 BOM 配置'))
})

test('product BOM creation owns specifications and inventory units', () => {
  assert.ok(!bomSource.includes('v-model="bomForm.specification_mode"'))
  assert.ok(bomSource.includes('规格模板（可选）'))
  assert.ok(bomSource.includes('activeUnitDefinitions'))
  assert.ok(bomSource.includes('variants: bomVariants'))
})

test('product components only expose a current published BOM specification', () => {
  assert.ok(bomSource.includes('product.bom_spec_authoritative === true'))
  assert.ok(bomSource.includes('商品组件必须引用该商品默认已发布 BOM 中的明确规格'))
  assert.ok(!bomSource.includes('直接商品组件'))
  assert.ok(!bomSource.includes('该组件使用商品 ID 与商品库存单位'))
})
