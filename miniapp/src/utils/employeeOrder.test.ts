import { describe, expect, it } from 'vitest'
import {
  employeeOrderProductCategory,
  employeeOrderProductFamilyKey,
  customerProductFamilies,
  customerShippingDefaults,
  defaultProductSpec,
  filterEmployeeOrderCustomers,
  filterEmployeeOrderProductFamilies,
  productSpecLabel,
  productSpecWeightG,
  salesUnitLabel,
  shanghaiToday,
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

  it('fills every shipping field with the documented customer fallbacks', () => {
    expect(customerShippingDefaults({
      id: 9,
      name: '客户B',
      contact: '李四',
      phone: '13900000000',
      address: '上海市测试路2号',
      company_name: '客户B公司',
    })).toEqual({
      receiver_name: '李四',
      receiver_phone: '13900000000',
      receiver_address: '上海市测试路2号',
      receiver_company: '客户B公司',
    })
  })

  it('initializes the order date from Shanghai time even near a UTC date boundary', () => {
    expect(shanghaiToday(new Date('2026-07-30T16:30:00.000Z'))).toBe('2026-07-31')
    expect(shanghaiToday(new Date('2026-07-31T15:59:59.000Z'))).toBe('2026-07-31')
  })

  it('searches customers by name, full pinyin and initials and limits results', () => {
    const customers = Array.from({ length: 25 }, (_, index) => ({
      id: index + 1,
      name: index === 22 ? '上海咖啡' : `客户${index + 1}`,
      py: index === 22 ? 'shanghai kafei' : `kehu${index + 1}`,
      pyi: index === 22 ? 'shkf' : `kh${index + 1}`,
    }))
    expect(filterEmployeeOrderCustomers(customers, 'shanghai')).toHaveLength(1)
    expect(filterEmployeeOrderCustomers(customers, 'shkf')[0]?.name).toBe('上海咖啡')
    expect(filterEmployeeOrderCustomers(customers, '')).toHaveLength(20)
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

  it('prefers default_sku_id before an inconsistent is_default_sku flag', () => {
    const selected = defaultProductSpec({
      parent_product_id: 550,
      name: '乌拉嘎',
      default_sku_id: 552,
      specs: [
        { product_id: 551, spec_label: '227g', is_default_sku: true },
        { product_id: 552, spec_label: '454g' },
      ],
    })
    expect(selected?.product_id).toBe(552)
  })

  it('shows public and selected-customer products only', () => {
    const families = [
      { parent_product_id: 1, name: '公共商品', customer_id: 0, specs: [] },
      { parent_product_id: 2, name: '客户A商品', customer_id: 8, specs: [] },
      { parent_product_id: 3, name: '客户B商品', customer_id: 9, specs: [] },
    ]
    expect(customerProductFamilies(families, 8).map((row) => row.name)).toEqual(['公共商品', '客户A商品'])
    expect(customerProductFamilies(families, 0)).toEqual([])
  })

  it('uses the selected customer alias instead of the public family for the same product', () => {
    const families = [
      {
        parent_product_id: 550,
        parent_product_name: '乌拉嘎',
        name: '乌拉嘎',
        customer_id: 0,
        specs: [{ product_id: 551, sku_id: 551, spec_label: '227g' }],
      },
      {
        parent_product_id: 550,
        parent_product_name: '乌拉嘎',
        name: '客户专属豆',
        alias_name: '客户专属豆',
        customer_id: 8,
        customer_product_alias_id: 81,
        specs: [{ product_id: 551, sku_id: 551, spec_label: '227g' }],
      },
    ]

    expect(customerProductFamilies(families, 8).map((row) => row.name)).toEqual(['客户专属豆'])
    expect(filterEmployeeOrderProductFamilies(families, 8, '乌拉嘎', 'all').map((row) => row.name)).toEqual(['客户专属豆'])
  })

  it('searches one row per product family by name, alias, pinyin, code and spec', () => {
    const families = [
      {
        parent_product_id: 550,
        customer_id: 0,
        parent_product_name: '乌拉嘎',
        name: '客户专属豆',
        alias_name: '客户专属豆',
        py: 'kehuzhuanshudou wulaga',
        pyi: 'khzsd wlg',
        product_code: 'COF-550',
        product_kind: 'roasted_bean',
        specs: [
          { product_id: 551, sku_code: 'COF-550-227', spec_label: '227g' },
          { product_id: 552, sku_code: 'COF-550-454', spec_label: '454g' },
        ],
      },
      {
        parent_product_id: 660,
        customer_id: 8,
        name: '另一款熟豆',
        product_kind: 'roasted_bean',
        specs: [{ product_id: 661, spec_label: '1Kg' }],
      },
    ]

    expect(filterEmployeeOrderProductFamilies(families, 8, '454', 'all').map((row) => row.name)).toEqual(['客户专属豆'])
    expect(filterEmployeeOrderProductFamilies(families, 8, 'COF-550-227', 'all')).toHaveLength(1)
    expect(filterEmployeeOrderProductFamilies(families, 8, 'wlg', 'all')).toHaveLength(1)
    expect(filterEmployeeOrderProductFamilies(families, 8, '客户专属豆', 'roasted')).toHaveLength(1)
    expect(filterEmployeeOrderProductFamilies(families, 8, '乌拉嘎', 'roasted')).toHaveLength(1)
  })

  it('filters the five business product categories and caps product results at 30', () => {
    const families = [
      { parent_product_id: 1, name: '熟豆', product_kind: 'roasted_bean', specs: [] },
      { parent_product_id: 2, name: '挂耳', product_kind: 'drip_bag', specs: [] },
      { parent_product_id: 3, name: '生豆', product_kind: 'green_bean', specs: [] },
      { parent_product_id: 4, name: '速溶', product_kind: 'instant_coffee', specs: [] },
    ]
    expect(families.map(employeeOrderProductCategory)).toEqual(['roasted', 'drip_bag', 'green_bean', 'instant_coffee'])
    expect(filterEmployeeOrderProductFamilies(families, 8, '', 'drip_bag').map((row) => row.name)).toEqual(['挂耳'])

    const many = Array.from({ length: 35 }, (_, index) => ({
      parent_product_id: index + 10,
      name: `商品${index + 1}`,
      product_kind: 'roasted_bean',
      specs: [],
    }))
    expect(filterEmployeeOrderProductFamilies(many, 8, '', 'all')).toHaveLength(30)
  })

  it('uses customer, parent product and alias to identify a family and localizes sales units', () => {
    expect(employeeOrderProductFamilyKey({
      customer_id: 8,
      parent_product_id: 550,
      customer_product_alias_id: 66,
      name: '乌拉嘎',
      specs: [],
    })).toBe('8:550:66')
    expect(salesUnitLabel('bag')).toBe('袋')
    expect(salesUnitLabel('box')).toBe('盒')
    expect(salesUnitLabel('公斤')).toBe('公斤')
  })
})
