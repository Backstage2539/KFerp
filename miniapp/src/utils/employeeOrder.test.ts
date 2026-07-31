import { describe, expect, it } from 'vitest'
import {
  customerProductFamilies,
  customerShippingDefaults,
  defaultProductSpec,
  productSpecLabel,
  productSpecWeightG,
} from './employeeOrder'

describe('employee mini order entry', () => {
  it('fills the customer shipping snapshot', () => {
    expect(customerShippingDefaults({
      id: 8,
      name: '客户A',
      receiver_name: '张三',
      receiver_phone: '13800000000',
      receiver_address: '上海市测试路1号',
      receiver_company: '客户A公司',
    })).toEqual({
      receiver_name: '张三',
      receiver_phone: '13800000000',
      receiver_address: '上海市测试路1号',
      receiver_company: '客户A公司',
    })
  })

  it('keeps product name separate and chooses an available default spec', () => {
    const family = {
      parent_product_id: 550,
      name: '乌拉嘎',
      customer_id: 0,
      default_sku_id: 552,
      product_kind: 'roasted_bean',
      specs: [
        { product_id: 551, sku_id: 551, spec_label: '227g', net_content_qty: 227, net_content_unit: 'g' },
        { product_id: 552, sku_id: 552, spec_label: '454g', net_content_qty: 454, net_content_unit: 'g' },
      ],
    }
    const selected = defaultProductSpec(family)
    expect(family.name).toBe('乌拉嘎')
    expect(selected?.product_id).toBe(552)
    expect(productSpecLabel(selected)).toBe('454g')
    expect(productSpecWeightG(selected)).toBe(454)
  })

  it('shows public and selected-customer products only', () => {
    const families = [
      { parent_product_id: 1, name: '公共商品', customer_id: 0, specs: [] },
      { parent_product_id: 2, name: '客户A商品', customer_id: 8, specs: [] },
      { parent_product_id: 3, name: '客户B商品', customer_id: 9, specs: [] },
    ]
    expect(customerProductFamilies(families, 8).map((row) => row.name)).toEqual(['公共商品', '客户A商品'])
  })
})
