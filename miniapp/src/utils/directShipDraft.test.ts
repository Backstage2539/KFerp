import { describe, expect, it } from 'vitest'
import type { EmployeeOrderProductFamily } from '../api/customerPortal'
import {
  buildDirectShipDraftItems,
  createDirectShipDraftLine,
  directShipDraftValidation,
  selectDirectShipDraftProduct,
  selectDirectShipDraftSpec,
} from './directShipDraft'

const family: EmployeeOrderProductFamily = {
  parent_product_id: 550,
  name: '乌拉嘎',
  customer_id: 31,
  default_sku_id: 552,
  specs: [
    { product_id: 551, sku_id: 551, sku_code: 'SKU-551', spec_label: '227g', net_content_qty: 227, net_content_unit: 'g' },
    { product_id: 552, sku_id: 552, sku_code: 'SKU-552', spec_label: '454g', net_content_qty: 454, net_content_unit: 'g' },
  ],
}

describe('direct shipment draft lines', () => {
  it('starts with an empty row whose quantity is ready at one', () => {
    expect(createDirectShipDraftLine('line-a')).toEqual({
      key: 'line-a',
      product_family_key: '',
      product_id: 0,
      product_name: '',
      spec_g: 0,
      spec_label: '',
      qty: 1,
    })
  })

  it('selects the employee-order default spec and preserves quantity when the product changes', () => {
    const selected = selectDirectShipDraftProduct({ ...createDirectShipDraftLine('line-a'), qty: 3 }, family)

    expect(selected).toMatchObject({
      key: 'line-a',
      product_id: 552,
      product_name: '乌拉嘎',
      spec_g: 454,
      spec_label: '454g',
      qty: 3,
    })
    expect(selectDirectShipDraftSpec(selected!, family.specs[0])).toMatchObject({
      product_id: 551,
      spec_g: 227,
      spec_label: '227g',
      qty: 3,
    })
  })

  it('falls back from default SKU to the marked default and then the first available spec', () => {
    const markedDefault = selectDirectShipDraftProduct(createDirectShipDraftLine('marked'), {
      ...family,
      default_sku_id: 0,
      specs: [family.specs[0], { ...family.specs[1], is_default_sku: true }],
    })
    expect(markedDefault).toMatchObject({ product_id: 552, spec_label: '454g' })

    const firstAvailable = selectDirectShipDraftProduct(createDirectShipDraftLine('first'), {
      ...family,
      default_sku_id: 0,
    })
    expect(firstAvailable).toMatchObject({ product_id: 551, spec_label: '227g' })
  })

  it('drops blank rows and merges duplicate SKU/spec quantities in the request payload', () => {
    const first = selectDirectShipDraftSpec(createDirectShipDraftLine('one'), family.specs[0])
    const second = selectDirectShipDraftSpec({ ...createDirectShipDraftLine('two'), qty: 2 }, family.specs[0])

    expect(buildDirectShipDraftItems([createDirectShipDraftLine('blank'), first, second])).toEqual([
      { product_id: 551, spec_g: 227, qty: 3 },
    ])
  })

  it('rejects partial and non-positive rows but ignores a completely blank row', () => {
    expect(directShipDraftValidation([createDirectShipDraftLine('blank')])).toBe('请至少选择一个商品规格并填写数量')
    expect(directShipDraftValidation([
      { ...createDirectShipDraftLine('partial'), product_family_key: '31:550:0', product_name: '乌拉嘎' },
    ])).toBe('请完整选择每一行的商品和规格')
    expect(directShipDraftValidation([
      selectDirectShipDraftProduct(createDirectShipDraftLine('complete'), family)!,
      { ...createDirectShipDraftLine('quantity-only'), qty: 2 },
    ])).toBe('请完整选择每一行的商品和规格')
    expect(directShipDraftValidation([
      { ...selectDirectShipDraftProduct(createDirectShipDraftLine('invalid'), family)!, qty: 0 },
    ])).toBe('商品数量必须大于 0')
    expect(directShipDraftValidation([
      createDirectShipDraftLine('blank'),
      selectDirectShipDraftProduct(createDirectShipDraftLine('complete'), family)!,
    ])).toBe('')
  })
})
