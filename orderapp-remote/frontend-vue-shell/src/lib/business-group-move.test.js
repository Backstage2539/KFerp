import test from 'node:test'
import assert from 'node:assert/strict'

import {
  beginBusinessGroupMoveState,
  businessGroupCategoryBreadcrumb,
  businessGroupCategoryTreeNodes,
  businessGroupGroupsForCategorySelection,
  restoreBusinessGroupMoveState,
} from './business-grouping.js'

const displayGroups = [
  {
    key: 'business-template-9',
    label: '咖啡豆',
    group_id: 9,
    group_item_id: 0,
    is_template_group: true,
    template_total: 3,
    rows: [],
  },
  {
    key: 'business-group-9-90',
    label: '意式咖啡',
    path_label: '意式咖啡',
    group_id: 9,
    group_item_id: 90,
    parent_group_item_id: 0,
    depth: 0,
    total: 1,
    rows: [{ id: 1 }],
  },
  {
    key: 'business-group-9-92',
    label: '深度烘焙',
    path_label: '意式咖啡 / 深度烘焙',
    group_id: 9,
    group_item_id: 92,
    parent_group_item_id: 90,
    depth: 1,
    total: 2,
    rows: [{ id: 2 }, { id: 3 }],
  },
  {
    key: 'business-group-9-91',
    label: '手冲咖啡',
    path_label: '手冲咖啡',
    group_id: 9,
    group_item_id: 91,
    parent_group_item_id: 0,
    depth: 0,
    total: 0,
    rows: [],
  },
  {
    key: 'business-group-unclassified',
    label: '未分类',
    path_label: '未分类',
    group_id: 0,
    group_item_id: 0,
    unclassified: true,
    depth: 0,
    total: 1,
    rows: [{ id: 4 }],
  },
]

test('category tree keeps template, major, minor and unclassified hierarchy with filtered counts', () => {
  const nodes = businessGroupCategoryTreeNodes(displayGroups)
  const byKey = new Map(nodes.map((node) => [node.key, node]))

  assert.deepEqual(nodes.map((node) => node.key), [
    'business-group-all',
    'business-template-9',
    'business-group-9-90',
    'business-group-9-92',
    'business-group-9-91',
    'business-group-unclassified',
  ])
  assert.deepEqual({
    parent: byKey.get('business-group-9-92').parent_key,
    depth: byKey.get('business-group-9-92').tree_depth,
    direct: byKey.get('business-group-9-90').direct_count,
    subtree: byKey.get('business-group-9-90').count,
    all: byKey.get('business-group-all').count,
  }, {
    parent: 'business-group-9-90',
    depth: 3,
    direct: 1,
    subtree: 3,
    all: 4,
  })
  assert.equal(byKey.get('business-template-9').targetable, false)
  assert.equal(byKey.get('business-group-9-90').targetable, true)
  assert.equal(byKey.get('business-group-unclassified').targetable, true)
  assert.equal(businessGroupCategoryBreadcrumb(nodes, 'business-group-9-92'), '全部分类 / 咖啡豆 / 意式咖啡 / 深度烘焙')
})

test('category selection filters the right list without changing page-specific row filtering', () => {
  assert.deepEqual(
    businessGroupGroupsForCategorySelection(displayGroups, 'business-template-9').map((group) => group.key),
    ['business-template-9', 'business-group-9-90', 'business-group-9-92', 'business-group-9-91'],
  )
  assert.deepEqual(
    businessGroupGroupsForCategorySelection(displayGroups, 'business-group-9-90').map((group) => group.key),
    ['business-group-9-90', 'business-group-9-92'],
  )
  assert.deepEqual(
    businessGroupGroupsForCategorySelection(displayGroups, 'business-group-unclassified').map((group) => group.key),
    ['business-group-unclassified'],
  )
  assert.equal(businessGroupGroupsForCategorySelection(displayGroups, 'business-group-all'), displayGroups)
})

test('flat lists keep their filtered total on the synthetic all-categories node', () => {
  const nodes = businessGroupCategoryTreeNodes([{
    key: 'all-products',
    label: '全部商品',
    all: true,
    rows: [{ id: 1 }, { id: 2 }],
  }])
  assert.equal(nodes[0].count, 2)
})

test('multiple templates are emitted in tree preorder so every category stays below its own template', () => {
  const nodes = businessGroupCategoryTreeNodes([
    ...displayGroups.slice(0, -1),
    {
      key: 'business-template-10',
      label: '包装形式',
      group_id: 10,
      group_item_id: 0,
      is_template_group: true,
      rows: [],
    },
    {
      key: 'business-group-10-100',
      label: '袋装',
      group_id: 10,
      group_item_id: 100,
      parent_group_item_id: 0,
      depth: 0,
      rows: [{ id: 5 }],
    },
    displayGroups.at(-1),
  ])

  assert.deepEqual(nodes.map((node) => node.key), [
    'business-group-all',
    'business-template-9',
    'business-group-9-90',
    'business-group-9-92',
    'business-group-9-91',
    'business-template-10',
    'business-group-10-100',
    'business-group-unclassified',
  ])
})

test('move mode expands every branch then restores expansion, browse selection and scroll snapshot', () => {
  const nodes = businessGroupCategoryTreeNodes(displayGroups)
  const started = beginBusinessGroupMoveState({
    expandedKeys: ['business-group-all', 'business-template-9'],
    selectedKey: 'business-group-9-92',
    scrollTop: 184,
  }, nodes)

  assert.equal(started.active, true)
  assert.deepEqual(started.snapshot, {
    expandedKeys: ['business-group-all', 'business-template-9'],
    selectedKey: 'business-group-9-92',
    scrollTop: 184,
  })
  assert.deepEqual(started.expandedKeys, [
    'business-group-all',
    'business-template-9',
    'business-group-9-90',
  ])

  assert.deepEqual(restoreBusinessGroupMoveState(started), {
    active: false,
    expandedKeys: ['business-group-all', 'business-template-9'],
    selectedKey: 'business-group-9-92',
    scrollTop: 184,
    snapshot: null,
  })
})
