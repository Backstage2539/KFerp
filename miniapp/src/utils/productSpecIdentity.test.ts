import { describe, expect, it } from 'vitest'

import {
  buildEmployeeOrderItemsPayload,
  createEmployeeOrderItem,
  defaultProductSpec,
  employeeOrderItemFromSpec,
  employeeOrderItemsTotal,
} from './employeeOrder'
import {
  buildMiniappProductSpecIdentity,
  visibleMiniappProductFamilies,
} from './productSpecIdentity'

describe('BOM specification cutover compatibility', () => {
  it('hides legacy child SKU families after their parent has cut over', () => {
    const families = visibleMiniappProductFamilies([
      { parent_product_id: 10, name: '商品 A', migration_state: 'cutover', specs: [] },
      { parent_product_id: 11, name: '旧 SKU', parent_id: 10, auto_derived_sku: true, specs: [] },
      { parent_product_id: 20, name: '商品 B', migration_state: 'legacy', specs: [] },
    ])
    expect(families.map((row) => row.parent_product_id)).toEqual([10, 20])
  })

  it('uses parent product and BOM spec for cutover writes', () => {
    expect(buildMiniappProductSpecIdentity({
      migration_state: 'cutover',
      product_id: 11,
      product_family_id: 10,
      bom_spec_id: 91,
      bom_variant_id: 191,
      qty: 2,
      sales_unit: '袋',
    })).toEqual({
      product_id: 10,
      bom_spec_id: 91,
      bom_variant_id: 191,
      qty: 2,
      unit: '袋',
    })
  })

  it('employee order payload keeps legacy identity and switches cutover identity', () => {
    const cutover = {
      ...createEmployeeOrderItem('cutover'),
      migration_state: 'cutover',
      product_family_id: 10,
      product_id: 11,
      bom_spec_id: 91,
      bom_variant_id: 191,
      product_name: '商品 A',
      spec_label: '227g 袋装',
      spec_g: 227,
      sales_unit: '袋',
      qty: 2,
      unit_price: 48,
    }
    const legacy = {
      ...createEmployeeOrderItem('legacy'),
      migration_state: 'legacy',
      product_family_id: 20,
      product_id: 21,
      product_name: '商品 B',
      spec_label: '454g 袋装',
      spec_g: 454,
      sales_unit: '袋',
      qty: 3,
      unit_price: 60,
    }

    expect(buildEmployeeOrderItemsPayload([cutover, legacy])).toEqual([
      expect.objectContaining({ product_id: 10, parent_product_id: 10, bom_spec_id: 91, bom_variant_id: 191, qty: 2, unit: '袋' }),
      expect.objectContaining({ product_id: 21, parent_product_id: 20, qty: 3, unit: '袋' }),
    ])
    expect(buildEmployeeOrderItemsPayload([cutover, legacy])[1]).not.toHaveProperty('bom_spec_id')
  })

  it('uses the BOM specification inventory unit directly without weight conversion', () => {
    const family = {
      parent_product_id: 10,
      name: '商品 A',
      migration_state: 'cutover',
      default_bom_spec_id: 102,
      specs: [
        {
          product_id: 10,
          migration_state: 'cutover',
          bom_spec_id: 101,
          bom_variant_id: 1001,
          spec_key: 'bag-227',
          spec_label: '227g袋',
          inventory_unit: '袋',
          sales_unit: '袋',
          tiers: [{ min_qty: 1, unit_price: 68, sales_unit: '袋' }],
        },
        {
          product_id: 10,
          migration_state: 'cutover',
          bom_spec_id: 102,
          bom_variant_id: 1002,
          spec_key: 'gift-2',
          spec_label: '双袋礼盒',
          inventory_unit: '盒',
          sales_unit: '盒',
          tiers: [{ min_qty: 1, unit_price: 138, sales_unit: '盒' }],
        },
      ],
    }
    expect(defaultProductSpec(family)?.bom_spec_id).toBe(102)
    const selected = employeeOrderItemFromSpec(createEmployeeOrderItem('spec'), family, family.specs[0])
    expect(selected).toEqual(expect.objectContaining({
      product_id: 10,
      bom_spec_id: 101,
      bom_variant_id: 1001,
      spec_g: 0,
      sales_unit: '袋',
      qty: 1,
      unit_price: 68,
    }))
    expect(employeeOrderItemsTotal([{ ...selected, qty: 2 }])).toBe(136)
  })
})
