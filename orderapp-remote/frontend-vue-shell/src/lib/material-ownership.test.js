import test from 'node:test'
import assert from 'node:assert/strict'
import { buildMaterialCreatePayload, filterMaterialsByOwnership, materialOwnerLabel } from './material-ownership.js'

test('material create payload requires one explicit owner', () => {
  assert.deepEqual(buildMaterialCreatePayload({ code: 'MAT-F' }, 'factory', 74), { code: 'MAT-F', owner_type: 'factory', owner_customer_id: 0 })
  assert.deepEqual(buildMaterialCreatePayload({ code: 'MAT-A' }, 'customer', 74), { code: 'MAT-A', owner_type: 'customer', owner_customer_id: 74 })
  assert.throws(() => buildMaterialCreatePayload({ code: 'MAT-X' }, '', 0), /物料归属/)
  assert.throws(() => buildMaterialCreatePayload({ code: 'MAT-X' }, 'customer', 0), /客户/)
})

test('ownership label and filter use the material master owner only', () => {
  const rows = [
    { id: 1, owner_type: 'factory', owner_customer_id: 0, owner_name: '棵凡咖啡' },
    { id: 2, owner_type: 'customer', owner_customer_id: 74, owner_name: '芬纳咖啡' },
    { id: 3, owner_type: 'customer', owner_customer_id: 75, owner_name: '另一客户' },
  ]
  assert.equal(materialOwnerLabel(rows[0]), '棵凡咖啡')
  assert.equal(materialOwnerLabel(rows[1]), '芬纳咖啡')
  assert.deepEqual(filterMaterialsByOwnership(rows, 'factory').map((row) => row.id), [1])
  assert.deepEqual(filterMaterialsByOwnership(rows, 'customer:74').map((row) => row.id), [2])
  assert.deepEqual(filterMaterialsByOwnership(rows, '芬纳').map((row) => row.id), [2])
})

test('materials view has one-owner wording and no sharing controls', async () => {
  const fs = await import('node:fs')
  const source = fs.readFileSync(new URL('../views/MaterialsView.vue', import.meta.url), 'utf8')
  assert.match(source, /物料归属/)
  assert.match(source, />复制</)
	assert.match(source, /:option-meta="customerOptionMeta"/)
	assert.doesNotMatch(source, /option-label="name"/)
	assert.match(source, /String\(row\.name \|\| ''\)\.trim\(\) !== '工厂自营'/)
  assert.doesNotMatch(source, /使用范围|档案用途|关联一个或多个客户|客户关联|复制到客户/)
})
