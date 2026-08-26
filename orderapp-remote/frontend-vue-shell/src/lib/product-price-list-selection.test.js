import assert from 'node:assert/strict'
import test from 'node:test'

import {
  buildPriceListProductFamilies,
  defaultPriceListProductSpecSelections,
  normalizePriceListProductSpecSelections,
  normalizePriceListPublicationGroups,
  normalizePriceListPublicationRows,
  priceListCategoryCodesForSelectedProducts,
  priceListCategoryHiddenByCollapsedAncestor,
  priceListCategoryProductIDs,
  priceListProductSpecSelectionIssue,
  priceListProductSpecSelectionCounts,
  priceListProductSpecLabel,
  priceListSelectedSkuCategoryRows,
  resolvePriceListProductSpecSelectionIssue,
  setPriceListCategorySpecSelection,
  togglePriceListProductSpecSelection,
  priceListVisibleCategoryRows,
} from './product-price-list-selection.js'

test('price-list spec label skips blank higher-priority fields before using the SKU label', () => {
  assert.equal(
    priceListProductSpecLabel({
      sku_id: 884,
      derived_sales_unit: '',
      sku_name: '227g',
      spec_label: '',
    }),
    '227g',
  )
})

const categoryRows = [
  {
    code: 'business-group-9-90',
    label: '咖啡熟豆',
    group_id: 9,
    group_item_id: 90,
    parent_group_item_id: 0,
    items: [],
  },
  {
    code: 'business-group-9-92',
    label: '意式拼配豆',
    group_id: 9,
    group_item_id: 92,
    parent_group_item_id: 90,
    items: [{ product_id: 550, name: '熟豆-红岩拼配' }],
  },
  {
    code: 'business-group-9-91',
    label: '挂耳咖啡',
    group_id: 9,
    group_item_id: 91,
    parent_group_item_id: 0,
    items: [],
  },
  {
    code: 'business-group-unclassified',
    label: '未分类',
    group_id: 9,
    group_item_id: 0,
    parent_group_item_id: 0,
    items: [],
    unclassified: true,
  },
]

test('price-list product picker hides empty categories outside the selected type but keeps ancestors', () => {
  const visibleRows = priceListVisibleCategoryRows(categoryRows)

  assert.deepEqual(visibleRows.map((row) => row.label), ['咖啡熟豆', '意式拼配豆'])
})

test('price-list parent category selection includes descendant products and preview categories', () => {
  assert.deepEqual(
    priceListCategoryProductIDs(categoryRows, 'business-group-9-90'),
    ['550'],
  )
  assert.deepEqual(
    priceListCategoryCodesForSelectedProducts(categoryRows, ['550']),
    ['business-group-9-92'],
  )
})

test('price-list collapsed parent category hides descendant category rows', () => {
  assert.equal(
    priceListCategoryHiddenByCollapsedAncestor(categoryRows, categoryRows[1], {
      'business-group-9-90': true,
    }),
    true,
  )
  assert.equal(
    priceListCategoryHiddenByCollapsedAncestor(categoryRows, categoryRows[1], {
      'business-group-9-92': true,
    }),
    false,
  )
})

const multiSpecProducts = [
  {
    product_id: 1,
    sku_id: 1,
    effective_parent_product_id: 1,
    parent_product_id: 0,
    name: '金色山脉',
    default_sku_id: 3,
    default_sales_unit: '227g袋装',
    is_default_sku: true,
    active: true,
  },
  {
    product_id: 2,
    sku_id: 2,
    effective_parent_product_id: 1,
    parent_product_id: 1,
    name: '金色山脉 磅',
    sku_name: '磅',
    derived_sales_unit: '磅',
    active: true,
  },
  {
    product_id: 3,
    sku_id: 3,
    effective_parent_product_id: 1,
    parent_product_id: 1,
    name: '金色山脉 227g袋装',
    sku_name: '227g袋装',
    derived_sales_unit: '227g袋装',
    is_default_sku: true,
    active: true,
  },
  {
    product_id: 4,
    sku_id: 4,
    effective_parent_product_id: 1,
    parent_product_id: 1,
    name: '金色山脉 旧规格',
    sku_name: '旧规格',
    derived_spec_status: 'template_removed',
    active: true,
  },
  {
    product_id: 5,
    sku_id: 5,
    effective_parent_product_id: 1,
    parent_product_id: 1,
    name: '金色山脉 停用规格',
    sku_name: '停用规格',
    derived_spec_status: 'template_disabled',
    active: true,
  },
  {
    product_id: 10,
    sku_id: 10,
    effective_parent_product_id: 10,
    parent_product_id: 0,
    name: '初晓',
    default_sku_id: 11,
    default_sales_unit: '磅',
    active: true,
  },
  {
    product_id: 11,
    sku_id: 11,
    effective_parent_product_id: 10,
    parent_product_id: 10,
    name: '初晓 磅',
    sku_name: '磅',
    derived_sales_unit: '磅',
    active: true,
  },
]

test('price-list picker groups child SKUs under one parent and excludes the aggregate root when specs exist', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts)

  assert.equal(families.length, 2)
  assert.equal(families[0].parent_product_id, 1)
  assert.equal(families[0].name, '金色山脉')
  assert.deepEqual(families[0].sku_options.map((row) => row.sku_id), [3, 2])
  assert.equal(families[0].default_sku_id, 3)
  assert.equal(families[1].default_sku_id, 11)
})

test('price-list picker falls back to a legacy root SKU only when no active child spec exists', () => {
  const families = buildPriceListProductFamilies([{
    product_id: 20,
    sku_id: 20,
    effective_parent_product_id: 20,
    parent_product_id: 0,
    name: '历史单规格商品',
    default_sales_unit: '盒',
    active: true,
  }])

  assert.deepEqual(families[0].sku_options.map((row) => row.sku_id), [20])
  assert.equal(families[0].default_sku_id, 20)
  assert.deepEqual({
    parent_product_id: families[0].sku_options[0].parent_product_id,
    effective_parent_product_id: families[0].sku_options[0].effective_parent_product_id,
    sku_id: families[0].sku_options[0].sku_id,
  }, {
    parent_product_id: 20,
    effective_parent_product_id: 20,
    sku_id: 20,
  })
})

test('price-list default selection chooses one default SKU per parent and records its source', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts)

  assert.deepEqual(defaultPriceListProductSpecSelections(families), [
    { parent_product_id: 1, sku_id: 3, selection_source: 'product_default', default_sku_id_at_selection: 3 },
    { parent_product_id: 10, sku_id: 11, selection_source: 'product_default', default_sku_id_at_selection: 11 },
  ])
})

test('price-list product specs support multiple explicit selections without mixing parent products', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts)
  let selections = defaultPriceListProductSpecSelections(families)

  selections = togglePriceListProductSpecSelection(selections, families[0], 2, true)
  assert.deepEqual(selections.filter((row) => row.parent_product_id === 1), [
    { parent_product_id: 1, sku_id: 3, selection_source: 'product_default', default_sku_id_at_selection: 3 },
    { parent_product_id: 1, sku_id: 2, selection_source: 'explicit', default_sku_id_at_selection: 3 },
  ])

  selections = togglePriceListProductSpecSelection(selections, families[0], 3, false)
  assert.deepEqual(selections.filter((row) => row.parent_product_id === 1).map((row) => row.sku_id), [2])
})

test('price-list category select preserves existing specs and adds only each missing product default', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts)
  const categoryRows = [{ code: 'coffee', items: families }]
  const explicit = [{ parent_product_id: 1, sku_id: 2, selection_source: 'explicit', default_sku_id_at_selection: 3 }]

  const selected = setPriceListCategorySpecSelection(categoryRows, 'coffee', explicit, true)
  assert.deepEqual(selected, [
    { parent_product_id: 1, sku_id: 2, selection_source: 'explicit', default_sku_id_at_selection: 3 },
    { parent_product_id: 10, sku_id: 11, selection_source: 'product_default', default_sku_id_at_selection: 11 },
  ])
  assert.deepEqual(setPriceListCategorySpecSelection(categoryRows, 'coffee', selected, false), [])
})

test('price-list spec selection normalization preserves an invalid draft row for explicit correction', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts)
  const normalized = normalizePriceListProductSpecSelections([
    { parent_product_id: 1, sku_id: 999, selection_source: 'explicit', default_sku_id_at_selection: 999 },
    { parent_product_id: 10, sku_id: 11, selection_source: 'explicit', default_sku_id_at_selection: 11 },
  ], families, { fallbackInvalid: true })

  assert.equal(normalized[0].sku_id, 999)
  assert.equal(normalized[0].selection_issue, 'invalid_spec')
  assert.equal(normalized[0].current_default_sku_id, 3)
  assert.deepEqual(resolvePriceListProductSpecSelectionIssue(normalized, families[0], 'switch'), [
    { parent_product_id: 1, sku_id: 3, selection_source: 'product_default', default_sku_id_at_selection: 3 },
    { parent_product_id: 10, sku_id: 11, selection_source: 'explicit', default_sku_id_at_selection: 11 },
  ])
})

test('price-list default changes require an explicit keep-or-switch decision while explicit selections remain frozen', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts).map((family) => (
    family.parent_product_id === 1 ? { ...family, default_sku_id: 2 } : family
  ))

  const staleDefault = normalizePriceListProductSpecSelections([
    { parent_product_id: 1, sku_id: 3, selection_source: 'product_default', default_sku_id_at_selection: 3 },
  ], families)
  assert.equal(staleDefault[0].sku_id, 3)
  assert.equal(staleDefault[0].selection_issue, 'default_changed')
  assert.equal(staleDefault[0].current_default_sku_id, 2)
  assert.equal(priceListProductSpecSelectionIssue(families[0], staleDefault)?.type, 'default_changed')
  assert.deepEqual(resolvePriceListProductSpecSelectionIssue(staleDefault, families[0], 'keep'), [
    { parent_product_id: 1, sku_id: 3, selection_source: 'explicit', default_sku_id_at_selection: 2 },
  ])
  assert.deepEqual(resolvePriceListProductSpecSelectionIssue(staleDefault, families[0], 'switch'), [
    { parent_product_id: 1, sku_id: 2, selection_source: 'product_default', default_sku_id_at_selection: 2 },
  ])
  assert.deepEqual(normalizePriceListProductSpecSelections([
    { parent_product_id: 1, sku_id: 3, selection_source: 'explicit', default_sku_id_at_selection: 3 },
  ], families), [
    { parent_product_id: 1, sku_id: 3, selection_source: 'explicit', default_sku_id_at_selection: 3 },
  ])
})

test('price-list selected SKU rows and counters keep product and spec totals distinct', () => {
  const families = buildPriceListProductFamilies(multiSpecProducts)
  const selections = [
    { parent_product_id: 1, sku_id: 2, selection_source: 'explicit', default_sku_id_at_selection: 3 },
    { parent_product_id: 1, sku_id: 3, selection_source: 'explicit', default_sku_id_at_selection: 3 },
  ]
  const rows = priceListSelectedSkuCategoryRows([{ code: 'coffee', items: families }], selections)

  assert.deepEqual(priceListProductSpecSelectionCounts(selections), { productCount: 1, specCount: 2 })
  assert.deepEqual(rows[0].items.map((row) => row.sku_id), [3, 2])
  assert.ok(rows[0].items.every((row) => row.__price_list_category_code === 'coffee'))
})

test('price-list publication rows repair a stale parent SKU when one child spec is selected', () => {
  const rows = normalizePriceListPublicationRows([
    { product_id: 550, sku_id: 550, parent_product_id: 550, final_unit_price: 68 },
    { product_id: 552, sku_id: 552, parent_product_id: 550, final_unit_price: 88 },
  ], [
    { parent_product_id: 550, sku_id: 551, selection_source: 'product_default' },
  ])

  assert.deepEqual(rows.map((row) => [row.product_id, row.sku_id, row.parent_product_id]), [
    [551, 551, 550],
    [552, 552, 550],
  ])
})

test('price-list publication rows keep ambiguous parent rows unchanged', () => {
  const rows = normalizePriceListPublicationRows([
    { product_id: 550, sku_id: 550, parent_product_id: 550 },
    { product_id: 600, sku_id: 600, parent_product_id: 600 },
  ], [
    { parent_product_id: 550, sku_id: 551 },
    { parent_product_id: 550, sku_id: 552 },
    { parent_product_id: 600, bom_spec_id: 701, bom_variant_id: 702 },
  ])

  assert.equal(rows[0].sku_id, 550)
  assert.equal(rows[1].sku_id, 600)
})

test('price-list publication rows normalize stale parent identity to one selected BOM spec', () => {
  const rows = normalizePriceListPublicationRows([
    { product_id: 600, sku_id: 600, parent_product_id: 600, price_unit: '1Kg' },
  ], [{
    parent_product_id: 600,
    sku_id: 701,
    bom_id: 90,
    bom_version_id: 901,
    bom_spec_id: 701,
    bom_variant_id: 702,
    selection_source: 'product_default',
  }])

  assert.deepEqual(rows[0], {
    product_id: 600,
    parent_product_id: 600,
    bom_id: 90,
    bom_version_id: 901,
    bom_spec_id: 701,
    bom_variant_id: 702,
    price_unit: '1Kg',
  })
})

test('price-list publication rows normalize a BOM spec id stored in the legacy sku field', () => {
  const rows = normalizePriceListPublicationRows([
    { product_id: 1063, sku_id: 7, parent_product_id: 1063, price_unit: '454g' },
  ], [{
    parent_product_id: 1063,
    sku_id: 7,
    bom_id: 6589,
    bom_version_id: 65891,
    bom_spec_id: 7,
    bom_variant_id: 422,
    selection_source: 'product_default',
  }])

  assert.deepEqual(rows[0], {
    product_id: 1063,
    parent_product_id: 1063,
    bom_id: 6589,
    bom_version_id: 65891,
    bom_spec_id: 7,
    bom_variant_id: 422,
    price_unit: '454g',
  })
})

test('price-list publication groups repair stale parent item identity before preview applies prices', () => {
  const groups = normalizePriceListPublicationGroups([
    { category: '咖啡豆', items: [{ product_id: 550, sku_id: 550, parent_product_id: 550, prices: [] }] },
  ], [{ parent_product_id: 550, sku_id: 551, selection_source: 'product_default' }])

  assert.deepEqual(groups[0].items[0], {
    product_id: 551,
    sku_id: 551,
    parent_product_id: 550,
    prices: [],
  })
})

test('selected SKU projection keeps customer alias and parent product name separate from the sales spec', () => {
  const families = buildPriceListProductFamilies([
    {
      product_id: 100,
      sku_id: 100,
      effective_parent_product_id: 100,
      parent_product_id: 0,
      name: 'Karen 白月光',
      product_name: '白月光瑰夏',
      customer_product_display_name: 'Karen 白月光',
      default_sku_id: 101,
      active: true,
      product_attributes: [{ key: 'roast_level', label: '烘焙度', value: '浅烘' }],
    },
    {
      product_id: 101,
      sku_id: 101,
      effective_parent_product_id: 100,
      parent_product_id: 100,
      name: '白月光瑰夏227g袋装',
      sku_name: '227g袋装',
      spec_label: '227g',
      derived_sales_unit: '227g袋装',
      active: true,
      effective_sales_spec: {
        sku_id: 101,
        spec_key: 'bag-227g',
        spec_label: '227g',
        sales_unit: '袋',
      },
    },
    {
      product_id: 102,
      sku_id: 102,
      effective_parent_product_id: 100,
      parent_product_id: 100,
      name: '白月光瑰夏454g袋装',
      sku_name: '454g袋装',
      spec_label: '454g',
      derived_sales_unit: '454g袋装',
      active: true,
      effective_sales_spec: {
        sku_id: 102,
        spec_key: 'bag-454g',
        spec_label: '454g',
        sales_unit: '袋',
      },
    },
  ])

  const rows = priceListSelectedSkuCategoryRows([{ code: 'coffee', items: families }], [
    {
      parent_product_id: 100,
      sku_id: 101,
      selection_source: 'product_default',
      default_sku_id_at_selection: 101,
    },
    {
      parent_product_id: 100,
      sku_id: 102,
      selection_source: 'explicit',
      default_sku_id_at_selection: 101,
    },
  ])
  const selected = rows[0].items[0]

  assert.equal(selected.name, 'Karen 白月光')
  assert.equal(selected.product_name, '白月光瑰夏')
  assert.equal(selected.__price_list_display_name, 'Karen 白月光')
  assert.equal(selected.__price_list_product_name, '白月光瑰夏')
  assert.equal(selected.__price_list_sales_spec_label, '227g')
  assert.equal(selected.sku_id, 101)
  assert.equal(selected.parent_product_id, 100)
  assert.equal(selected.effective_sales_spec.sku_id, 101)
  assert.deepEqual(selected.product_attributes, [{ key: 'roast_level', label: '烘焙度', value: '浅烘' }])
  assert.deepEqual(rows[0].items.map((item) => ({
    name: item.name,
    product_name: item.product_name,
    sales_spec: item.__price_list_sales_spec_label,
    sku_id: item.sku_id,
  })), [
    { name: 'Karen 白月光', product_name: '白月光瑰夏', sales_spec: '227g', sku_id: 101 },
    { name: 'Karen 白月光', product_name: '白月光瑰夏', sales_spec: '454g', sku_id: 102 },
  ])
})

test('selected SKU projection ignores a generated child display name when no customer alias exists', () => {
  const families = buildPriceListProductFamilies([
    {
      product_id: 100,
      sku_id: 100,
      effective_parent_product_id: 100,
      parent_product_id: 0,
      name: '白月光瑰夏',
      product_name: '白月光瑰夏',
      default_sku_id: 101,
      active: true,
    },
    {
      product_id: 101,
      sku_id: 101,
      effective_parent_product_id: 100,
      parent_product_id: 100,
      name: '白月光瑰夏 227g',
      customer_product_display_name: '白月光瑰夏 227g',
      spec_label: '227g',
      active: true,
    },
  ])

  const rows = priceListSelectedSkuCategoryRows([{ code: 'coffee', items: families }], [{
    parent_product_id: 100,
    sku_id: 101,
    selection_source: 'product_default',
    default_sku_id_at_selection: 101,
  }])

  assert.equal(rows[0].items[0].name, '白月光瑰夏')
  assert.equal(rows[0].items[0].__price_list_display_name, '白月光瑰夏')
  assert.equal(rows[0].items[0].__price_list_sales_spec_label, '227g')
})
