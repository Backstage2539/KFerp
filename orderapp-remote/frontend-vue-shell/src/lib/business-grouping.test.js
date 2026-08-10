import test from 'node:test'
import assert from 'node:assert/strict'

import {
  businessGroupFeatureSelectionIDs,
  businessGroupFeatureSelectionPayload,
  businessGroupRowsForFeatureSelection,
  businessGroupRowsForUsage,
  businessGroupControlOptions,
  businessGroupMoveAssignmentPayload,
  groupRowsByBusinessGroupTemplate,
  groupRowsByBusinessGroupTemplates,
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

test('multiple referenced business group templates form one non-duplicating product list and one unclassified group', () => {
  const dripGroup = {
    id: 10,
    name: '商品挂耳模板',
    active: true,
    usages: [{ usage_key: 'product_catalog', active: true }],
    items: [
      { id: 100, group_id: 10, parent_id: 0, name: '挂耳咖啡', active: true, sort_order: 10 },
      { id: 101, group_id: 10, parent_id: 100, name: '盒装挂耳', active: true, sort_order: 20 },
    ],
  }
  const rows = [
    { id: 1, name: '熟豆商品' },
    { id: 2, name: '挂耳商品' },
    { id: 3, name: '未分类商品' },
  ]
  const groups = groupRowsByBusinessGroupTemplates(rows, {
    templates: [productGroup, dripGroup],
    usageKey: 'product_catalog',
    objectKey: 'product',
    assignments: [
      { usage_key: 'product_catalog', object_key: 'product', object_id: 1, group_id: 9, group_item_id: 92 },
      { usage_key: 'product_catalog', object_key: 'product', object_id: 2, group_id: 10, group_item_id: 101 },
      { usage_key: 'product_catalog', object_key: 'product', object_id: 3, group_id: 8, group_item_id: 80 },
    ],
  })

  assert.deepEqual(groups.map((group) => group.key), [
    'business-template-9',
    'business-group-9-90',
    'business-group-9-92',
    'business-group-9-91',
    'business-template-10',
    'business-group-10-100',
    'business-group-10-101',
    'business-group-unclassified',
  ])
  assert.deepEqual(
    groups.filter((group) => group.is_template_group).map((group) => ({
      key: group.key,
      label: group.label,
      template_total: group.template_total,
      rows: group.rows.length,
    })),
    [
      { key: 'business-template-9', label: '商品分组', template_total: 1, rows: 0 },
      { key: 'business-template-10', label: '商品挂耳模板', template_total: 1, rows: 0 },
    ],
  )
  assert.deepEqual(
    groups.filter((group) => !group.is_template_group).map((group) => ({
      key: group.key,
      template: group.template_label,
      count: group.rows.length,
    })),
    [
      { key: 'business-group-9-90', template: '商品分组', count: 0 },
      { key: 'business-group-9-92', template: '商品分组', count: 1 },
      { key: 'business-group-9-91', template: '商品分组', count: 0 },
      { key: 'business-group-10-100', template: '商品挂耳模板', count: 0 },
      { key: 'business-group-10-101', template: '商品挂耳模板', count: 1 },
      { key: 'business-group-unclassified', template: '', count: 1 },
    ],
  )
  assert.deepEqual(groups.flatMap((group) => group.rows).map((row) => row.id).sort(), [1, 2, 3])

  assert.deepEqual(groupRowsByBusinessGroupTemplates(rows, { templates: [] }), [{
    key: 'all-products',
    label: '全部商品',
    path_label: '全部商品',
    depth: 0,
    parent_group_item_id: 0,
    group_id: 0,
    group_item_id: 0,
    template_label: '',
    rows,
    all: true,
    unclassified: false,
    sort_order: 0,
  }])
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

test('legacy usage-filtered business group lookup still requires explicit active usages', () => {
  const rows = businessGroupRowsForUsage([
    { id: 6, name: '商品默认分组', code: 'default_product_catalog', active: true, sort_order: 10, usages: [{ usage_key: 'product_catalog', active: true }] },
    { id: 221, name: 'BOM分组', active: true, sort_order: 5, usages: [{ usage_key: 'production_bom', active: true }] },
    { id: 222, name: '通用分组模板', active: true, sort_order: 6, usages: [] },
    { id: 128, name: '商品分组', code: 'product_catalog', active: true, sort_order: 10, usages: [{ usage_key: 'product_catalog', active: true }] },
    { id: 129, name: '商品挂耳模板', code: 'product_drip', active: true, sort_order: 11, usages: [{ usage_key: 'product_catalog', active: true }] },
    { id: 130, name: '停用商品引用', active: true, sort_order: 12, usages: [{ usage_key: 'product_catalog', active: false }] },
  ], 'product_catalog')

  assert.deepEqual(rows.map((row) => ({ id: row.id, name: row.name })), [
    { id: 128, name: '商品分组' },
    { id: 129, name: '商品挂耳模板' },
  ])
})

test('feature-owned business group selection resolves templates in the selected order without template usages', () => {
  const rows = businessGroupRowsForFeatureSelection([
    { id: 6, name: '商品默认分组', code: 'default_product_catalog', active: true, sort_order: 1 },
    { id: 221, name: '咖啡豆分组', active: true, sort_order: 20, usages: [] },
    { id: 222, name: '挂耳分组', active: true, sort_order: 10, usages: [{ usage_key: 'production_bom', active: true }] },
    { id: 223, name: '停用分组', active: false, sort_order: 5 },
  ], [222, 221, 222, 223, 999])

  assert.deepEqual(rows.map((row) => row.id), [222, 221])
})

test('feature-owned business group selection normalizes GET and PUT contracts', () => {
  assert.deepEqual(businessGroupFeatureSelectionIDs({ group_template_ids: [9, '10', 9, 0, -1] }), [9, 10])
  assert.deepEqual(businessGroupFeatureSelectionIDs({ feature_key: 'product_catalog' }), [])
  assert.deepEqual(businessGroupFeatureSelectionPayload(' product_catalog ', [10, '9', 10, 0]), {
    feature_key: 'product_catalog',
    group_template_ids: [10, 9],
  })
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
    objectKey: 'warehouse_inventory_item',
    objectRef: ' finished_goods:finished_product:88:250 ',
    option: { group_id: 9, group_item_id: 91 },
  }), {
    id: 0,
    usage_key: 'warehouse_inventory',
    object_key: 'warehouse_inventory_item',
    object_id: 0,
    object_ref: 'finished_goods:finished_product:88:250',
    group_id: 9,
    group_item_id: 91,
    sort_order: 100,
  })
})
