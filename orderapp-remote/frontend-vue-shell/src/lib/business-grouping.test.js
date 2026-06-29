import test from 'node:test'
import assert from 'node:assert/strict'

import {
  businessGroupRowsForUsage,
  businessGroupControlOptions,
  businessGroupMoveAssignmentPayload,
  groupRowsByBusinessGroupTemplate,
  preferredBusinessGroupTemplateID,
} from './business-grouping.js'

const productGroup = {
  id: 9,
  name: '商品分组',
  active: true,
  usages: [{ usage_key: 'product_catalog', active: true }],
  items: [
    { id: 90, group_id: 9, parent_id: 0, name: '咖啡熟豆', active: true, sort_order: 10 },
    { id: 91, group_id: 9, parent_id: 0, name: '挂耳咖啡', active: true, sort_order: 20 },
    { id: 92, group_id: 9, parent_id: 90, name: '精品意式', active: true, sort_order: 30 },
  ],
}

test('business group template rows include empty large and small categories plus unclassified', () => {
  const groups = groupRowsByBusinessGroupTemplate([
    { id: 1, name: '熟豆商品' },
    { id: 2, name: '挂耳商品' },
    { id: 3, name: '未分类商品' },
  ], {
    template: productGroup,
    usageKey: 'product_catalog',
    objectKey: 'product',
    assignments: [
      { usage_key: 'product_catalog', object_key: 'product', object_id: 1, group_id: 9, group_item_id: 90 },
      { usage_key: 'product_catalog', object_key: 'product', object_id: 2, group_id: 9, group_item_id: 91 },
      { usage_key: 'product_catalog', object_key: 'product', object_id: 3, group_id: 8, group_item_id: 80 },
    ],
  })

  assert.deepEqual(groups.map((group) => ({
    key: group.key,
    label: group.label,
    path: group.path_label,
    depth: group.depth,
    count: group.rows.length,
  })), [
    { key: 'business-group-9-90', label: '咖啡熟豆', path: '咖啡熟豆', depth: 0, count: 1 },
    { key: 'business-group-9-92', label: '精品意式', path: '咖啡熟豆 / 精品意式', depth: 1, count: 0 },
    { key: 'business-group-9-91', label: '挂耳咖啡', path: '挂耳咖啡', depth: 0, count: 1 },
    { key: 'business-group-unclassified', label: '未分类', path: '未分类', depth: 0, count: 1 },
  ])
})

test('business group template rows prefer current template assignment over legacy default residue', () => {
  const groups = groupRowsByBusinessGroupTemplate([
    { id: 1, name: '熟豆-红岩拼配' },
  ], {
    template: productGroup,
    usageKey: 'product_catalog',
    objectKey: 'product',
    assignments: [
      { usage_key: 'product_catalog', object_key: 'product', object_id: 1, group_id: 6, group_item_id: 61 },
      { usage_key: 'product_catalog', object_key: 'product', object_id: 1, group_id: 9, group_item_id: 92 },
    ],
  })

  assert.deepEqual(groups.map((group) => [group.label, group.rows.map((row) => row.name)]), [
    ['咖啡熟豆', []],
    ['精品意式', ['熟豆-红岩拼配']],
    ['挂耳咖啡', []],
    ['未分类', []],
  ])
})

test('business group controls expose template and move options for any usage', () => {
  const options = businessGroupControlOptions([productGroup], {
    usageKey: 'production_bom',
    selectedTemplateID: 9,
  })

  assert.deepEqual(options.templateOptions.map((option) => option.label), ['商品分组'])
  assert.deepEqual(options.moveOptions.map((option) => ({
    label: option.label,
    depth: option.depth,
    parent: option.parent_group_item_id,
  })), [
    { label: '咖啡熟豆', depth: 0, parent: 0 },
    { label: '咖啡熟豆 / 精品意式', depth: 1, parent: 90 },
    { label: '挂耳咖啡', depth: 0, parent: 0 },
  ])
})

test('preferred business group template keeps warehouse inventory on stock grouping after refresh', () => {
  const groups = [
    { id: 128, name: '商品分组', code: 'product_catalog', active: true, sort_order: 1, usages: [{ usage_key: 'product_catalog', active: true }] },
    { id: 222, name: '库存分组', active: true, sort_order: 2, usages: [] },
  ]

  assert.equal(preferredBusinessGroupTemplateID(groups, {
    selectedTemplateID: 0,
    usageKey: 'warehouse_inventory',
    preferredNames: ['库存分组'],
    preferredNameIncludes: ['库存', '仓库'],
  }), 222)

  assert.equal(preferredBusinessGroupTemplateID(groups, {
    selectedTemplateID: 128,
    usageKey: 'warehouse_inventory',
    preferredNames: ['库存分组'],
    preferredNameIncludes: ['库存', '仓库'],
  }), 128)
})

test('product catalog business group rows ignore legacy defaults and non-product templates', () => {
  const rows = businessGroupRowsForUsage([
    { id: 6, name: '商品默认分组', code: 'default_product_catalog', active: true, sort_order: 10, usages: [{ usage_key: 'product_catalog', active: true }] },
    { id: 221, name: 'BOM分组', active: true, sort_order: 5, usages: [{ usage_key: 'production_bom', active: true }] },
    { id: 222, name: '库存分组', active: true, sort_order: 6, usages: [] },
    { id: 128, name: '商品分组', code: 'product_catalog', active: true, sort_order: 10, usages: [{ usage_key: 'product_catalog', active: true }] },
  ], 'product_catalog')

  assert.deepEqual(rows.map((row) => ({ id: row.id, name: row.name })), [
    { id: 128, name: '商品分组' },
  ])
})

test('business group move payload supports product BOM and warehouse object identities', () => {
  assert.deepEqual(businessGroupMoveAssignmentPayload({
    usageKey: 'product_catalog',
    objectKey: 'product',
    objectID: 88,
    option: { group_id: 9, group_item_id: 90 },
  }), {
    id: 0,
    usage_key: 'product_catalog',
    object_key: 'product',
    object_id: 88,
    object_ref: '',
    group_id: 9,
    group_item_id: 90,
    sort_order: 100,
  })

  assert.deepEqual(businessGroupMoveAssignmentPayload({
    usageKey: 'warehouse_inventory',
    objectKey: 'warehouse',
    objectRef: ' finished_goods ',
    option: { group_id: 9, group_item_id: 91 },
  }), {
    id: 0,
    usage_key: 'warehouse_inventory',
    object_key: 'warehouse',
    object_id: 0,
    object_ref: 'finished_goods',
    group_id: 9,
    group_item_id: 91,
    sort_order: 100,
  })
})
