import assert from 'node:assert/strict'
import fs from 'node:fs'
import test from 'node:test'

import { buildOrderPayload, normalizeOrderProductFamilies } from './order-entry.js'
import { producePlanKey } from './produce-plan.js'
import {
  buildPriceListProductFamilies,
  defaultPriceListProductSpecSelections,
} from './product-price-list-selection.js'
import {
  buildProductSpecWriteIdentity,
  legacyProductTemplateWriteTargets,
  normalizeProductBomSpecs,
  productSpecSelectionsForWrite,
  visibleRowsForProductSpecMigration,
} from './product-spec-cutover.js'

test('legacy sales-spec template writes exclude cutover products', () => {
  const products = [
    { id: 10, migration_state: 'cutover' },
    { id: 20, migration_state: 'ready' },
    { id: 30, migration_state: 'legacy' },
  ]

  assert.deepEqual(
    legacyProductTemplateWriteTargets(products, [10, 20, 30]).map((row) => row.id),
    [20, 30],
  )
})

test('cutover catalogs hide legacy derived SKUs while legacy catalogs retain them', () => {
  const rows = [
    { id: 10, name: '已切换商品', migration_state: 'cutover' },
    { id: 11, parent_product_id: 10, auto_derived_sku: true, name: '旧 227g SKU' },
    { id: 20, name: '旧商品', migration_state: 'legacy' },
    { id: 21, parent_product_id: 20, auto_derived_sku: true, name: '仍在使用的旧 SKU' },
  ]

  assert.deepEqual(
    visibleRowsForProductSpecMigration(rows).map((row) => row.id),
    [10, 20, 21],
  )
})

test('published BOM variants normalize into read-only sales specification choices', () => {
  const specs = normalizeProductBomSpecs({
    migration_state: 'cutover',
    variants: [
      { bom_spec_id: 91, bom_variant_id: 191, spec_code: 'BOM-SPEC-000091', barcode: '6900000000091', name: '227g 袋装', inventory_unit: '袋', is_default: true },
      { bom_spec_id: 92, bom_variant_id: 192, spec_code: 'BOM-SPEC-000092', name: '454g 袋装', inventory_unit: '袋' },
    ],
  })

  assert.deepEqual(specs, [
    { bom_spec_id: 91, bom_variant_id: 191, code: 'BOM-SPEC-000091', barcode: '6900000000091', name: '227g 袋装', unit: '袋', is_default: true, sort_order: 0 },
    { bom_spec_id: 92, bom_variant_id: 192, code: 'BOM-SPEC-000092', barcode: '', name: '454g 袋装', unit: '袋', is_default: false, sort_order: 0 },
  ])
})

test('cutover writes use parent product plus BOM spec while legacy writes retain child SKU', () => {
  assert.deepEqual(buildProductSpecWriteIdentity({
    migration_state: 'cutover',
    product_id: 11,
    parent_product_id: 10,
    bom_spec_id: 91,
    bom_variant_id: 191,
    qty: 2,
    unit: '袋',
  }), {
    product_id: 10,
    bom_spec_id: 91,
    bom_variant_id: 191,
    qty: 2,
    unit: '袋',
  })

  assert.deepEqual(buildProductSpecWriteIdentity({
    migration_state: 'legacy',
    product_id: 21,
    parent_product_id: 20,
    spec_g: 454,
    qty: 3,
    unit: '袋',
  }), {
    product_id: 21,
    qty: 3,
    unit: '袋',
  })
})

test('production demand selection keys use parent product plus BOM spec after cutover', () => {
  assert.equal(producePlanKey(10, 227, 91), 'product:10:bom_spec:91')
  assert.equal(producePlanKey(21, 454), '21-454')
})

test('order payload dual-writes only the identity allowed by each migration state', () => {
  const payload = buildOrderPayload({
    form: {},
    rows: [
      {
        migration_state: 'cutover', product_id: 11, parent_product_id: 10,
        bom_spec_id: 91, bom_variant_id: 191, product_name: '商品 A',
        product_kind: 'roasted_bean', spec_g: 227, qty: 2, unit: '袋', unit_price: 48,
      },
      {
        migration_state: 'legacy', product_id: 21, parent_product_id: 20,
        product_name: '商品 B', product_kind: 'roasted_bean', spec_g: 454,
        qty: 3, unit: '袋', unit_price: 60,
      },
    ],
  })

  assert.deepEqual(payload.product_id, ['10', '21'])
  assert.deepEqual(payload.parent_product_id, ['10', '20'])
  assert.deepEqual(payload.bom_spec_id, ['91', '0'])
  assert.deepEqual(payload.bom_variant_id, ['191', '0'])
  assert.deepEqual(payload.qty, ['2', '3'])
  assert.deepEqual(payload.unit, ['袋', '袋'])
})

test('order form replaces cutover child SKUs with its BOM spec option contract', () => {
  const families = normalizeOrderProductFamilies([
    {
      parent_product_id: 10,
      parent_product_name: '商品 A',
      specs: [
        { sku_id: 11, product_id: 11, spec_label: '旧 227g', tiers: [{ id: 1 }] },
        { sku_id: 12, product_id: 12, spec_label: '旧 454g', tiers: [{ id: 2 }] },
      ],
    },
  ], [], [
    {
      parent_product_id: 10, legacy_child_product_id: 11,
      bom_spec_id: 91, bom_variant_id: 191, spec_name: '227g 袋装',
      inventory_unit: '袋', migration_state: 'cutover', tiers: [{ id: 101 }],
    },
    {
      parent_product_id: 10, legacy_child_product_id: 12,
      bom_spec_id: 92, bom_variant_id: 192, spec_name: '454g 袋装',
      inventory_unit: '袋', migration_state: 'cutover', tiers: [{ id: 102 }],
    },
  ])

  assert.equal(families.length, 1)
  assert.equal(families[0].migration_state, 'cutover')
  assert.deepEqual(families[0].specs.map((row) => [row.sku_id, row.bom_spec_id, row.product_id]), [
    [91, 91, 10],
    [92, 92, 10],
  ])
  assert.deepEqual(families[0].specs[0].tiers, [{ id: 101 }])
})

test('price publication selects BOM specs but writes parent product identities after cutover', () => {
  const families = buildPriceListProductFamilies([
    {
      product_id: 10, parent_product_id: 10, migration_state: 'cutover',
      bom_spec_id: 91, bom_variant_id: 191, spec_label: '227g 袋装',
      is_default_sku: true, active: true,
    },
    {
      product_id: 10, parent_product_id: 10, migration_state: 'cutover',
      bom_spec_id: 92, bom_variant_id: 192, spec_label: '454g 袋装', active: true,
    },
  ])

  assert.equal(families.length, 1)
  assert.equal(families[0].product_id, 10)
  assert.deepEqual(families[0].sku_options.map((row) => row.bom_spec_id), [91, 92])
  assert.deepEqual(
    productSpecSelectionsForWrite(defaultPriceListProductSpecSelections(families)),
    [{
      parent_product_id: 10,
      product_id: 10,
      bom_spec_id: 91,
      bom_variant_id: 191,
      migration_state: 'cutover',
      selection_source: 'product_default',
    }],
  )
})

test('product settings renders cutover BOM specs read-only and routes editing to BOM', () => {
  const source = fs.readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')
  assert.match(source, /productProductionConfigUsesBomSpecs/)
  assert.match(source, /BOM 规格（只读）/)
  assert.match(source, /productProductionBomSpecs/)
  assert.match(source, /到 BOM 维护规格/)
  assert.match(source, /\/api\/products\/\$\{productID\}\/bom-spec-migration/)
  assert.match(source, /BOM 规格迁移/)
  assert.match(source, /mutateProductBomSpecMigration\('prepare'/)
  assert.match(source, /mutateProductBomSpecMigration\('readiness'/)
  assert.match(source, /mutateProductBomSpecMigration\('cutover'/)
  assert.match(source, /旧损耗、备注、BOM 与路线/)
  assert.match(source, /window\.confirm\('确认切换到默认已发布 BOM 的规格组/)
  assert.match(source, /不会自动切换商品/)
  assert.match(source, /旧商品销售规格模板（迁移期）/)
  assert.doesNotMatch(source, /selectedLegacyProductUnitTemplateTargets/)
  assert.doesNotMatch(source, /v-if="productProductionConfigUsesBomSpecs" class="sales-spec-template-detail bom-spec-readonly-panel"/)
  assert.match(source, /productProductionBomSpecsSummary/)
})

test('cutover identity is wired through price, inventory, stock-entry, and production pages', () => {
  const views = ['CostingView.vue', 'InventoryView.vue', 'StockEntriesView.vue', 'ProducePlanView.vue']
    .map((name) => fs.readFileSync(new URL(`../views/${name}`, import.meta.url), 'utf8'))
    .join('\n')
  assert.match(views, /productSpecSelectionsForWrite/)
  assert.match(views, /buildProductSpecWriteIdentity/)
  assert.match(views, /bom_spec_id/)
  assert.match(views, /producePlanKey\([^)]*bom_spec_id/)
})
