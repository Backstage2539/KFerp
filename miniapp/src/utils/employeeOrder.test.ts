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
  buildEmployeeOrderItemsPayload,
  createEmployeeOrderItem,
  employeeOrderShippingChanged,
  employeeOrderItemsTotal,
  preserveEmployeeOrderDraftItemsForMissingCustomer,
  revalidateEmployeeOrderItems,
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

  it('creates independent rows and maps every complete row into the order payload', () => {
    const first = createEmployeeOrderItem('line-1')
    const second = createEmployeeOrderItem('line-2')
    Object.assign(first, {
      product_id: 551,
      product_name: '乌拉嘎',
      product_kind: 'roasted_bean',
      spec_label: '227g',
      spec_g: 227,
      sales_unit: '袋',
      qty: 2,
      unit_price: 48,
    })
    Object.assign(second, {
      product_id: 661,
      product_name: '挂耳',
      product_kind: 'drip_bag',
      spec_label: '10g × 10袋/盒',
      spec_g: 10,
      sales_unit: '盒',
      unit_bag_count: 10,
      unit_bean_g: 10,
      qty: 3,
      unit_price: 80,
    })

    expect(buildEmployeeOrderItemsPayload([first, second])).toEqual([
      expect.objectContaining({ product_id: 551, qty: 2, spec_g: 227, unit_price: 48 }),
      expect.objectContaining({ product_id: 661, qty: 3, unit_bag_count: 10, unit_price: 80 }),
    ])
    expect(employeeOrderItemsTotal([first, second])).toBe(336)
  })

  it('clears only product rows that are no longer available after changing customer', () => {
    const publicItem = Object.assign(createEmployeeOrderItem('public'), {
      product_family_key: '0:10:0',
      product_id: 11,
      product_name: '公共商品',
      spec_label: '227g',
    })
    const unavailableItem = Object.assign(createEmployeeOrderItem('private'), {
      product_family_key: '8:20:0',
      product_id: 21,
      product_name: '客户专属商品',
      spec_label: '454g',
    })
    const families = [{
      customer_id: 0,
      parent_product_id: 10,
      name: '公共商品',
      specs: [{ product_id: 11, spec_label: '227g' }],
    }]

    const result = revalidateEmployeeOrderItems([publicItem, unavailableItem], families, 9)
    expect(result[0]).toMatchObject({ product_id: 11, product_name: '公共商品' })
    expect(result[1]).toMatchObject({ product_id: 0, product_name: '', qty: 1 })
  })

  it('preserves a manually edited unit price while restoring a valid server draft', () => {
    const item = Object.assign(createEmployeeOrderItem('draft-line'), {
      product_id: 11,
      product_name: '公共商品',
      unit_price: 72.5,
    })
    const families = [{
      customer_id: 0,
      parent_product_id: 10,
      name: '公共商品',
      specs: [{ product_id: 11, spec_label: '227g', tiers: [{ unit_price: 68 }] }],
    }]

    const [restored] = revalidateEmployeeOrderItems(
      [item],
      families,
      9,
      { preserveUnitPrice: true },
    )

    expect(restored).toMatchObject({ product_id: 11, unit_price: 72.5 })
  })

  it('keeps an unavailable draft line visible and marks it for reselection', () => {
    const item = Object.assign(createEmployeeOrderItem('retired-line'), {
      product_id: 99,
      product_name: '已停用商品',
      spec_label: '227g',
      unit_price: 66,
    })

    const [restored] = revalidateEmployeeOrderItems(
      [item],
      [],
      9,
      { preserveUnavailable: true, preserveUnitPrice: true },
    )

    expect(restored).toMatchObject({
      product_id: 99,
      product_name: '已停用商品',
      unit_price: 66,
      validation_error: '商品已失效或不适用于当前客户，请重新选择',
    })
  })

  it('keeps populated draft lines visible when their customer is no longer available', () => {
    const first = Object.assign(createEmployeeOrderItem('first'), {
      product_family_key: '9:10:201',
      customer_product_alias_id: 201,
      product_id: 11,
      product_name: '客户别名A',
      qty: 2,
      unit_price: 66,
    })
    const second = Object.assign(createEmployeeOrderItem('second'), {
      product_family_key: '9:10:202',
      customer_product_alias_id: 202,
      product_id: 11,
      product_name: '客户别名B',
      qty: 3,
      unit_price: 72,
    })

    expect(preserveEmployeeOrderDraftItemsForMissingCustomer([first, second])).toEqual([
      expect.objectContaining({ key: 'first', product_id: 11, customer_product_alias_id: 201, qty: 2, unit_price: 66, validation_error: '草稿客户已失效或不可用，请重新选择客户和商品' }),
      expect.objectContaining({ key: 'second', product_id: 11, customer_product_alias_id: 202, qty: 3, unit_price: 72, validation_error: '草稿客户已失效或不可用，请重新选择客户和商品' }),
    ])
  })

  it('restores the exact customer alias family when aliases share one SKU', () => {
    const item = Object.assign(createEmployeeOrderItem('alias-b'), {
      product_family_key: '9:10:202',
      product_id: 11,
      product_name: '客户别名B',
      spec_label: '227g',
      unit_price: 72,
    })
    const families = [
      {
        customer_id: 9,
        parent_product_id: 10,
        customer_product_alias_id: 201,
        name: '客户别名A',
        specs: [{ product_id: 11, spec_label: '227g', tiers: [{ unit_price: 60 }] }],
      },
      {
        customer_id: 9,
        parent_product_id: 10,
        customer_product_alias_id: 202,
        name: '客户别名B',
        specs: [{ product_id: 11, spec_label: '227g', tiers: [{ unit_price: 72 }] }],
      },
    ]

    const [restored] = revalidateEmployeeOrderItems(
      [item],
      families,
      9,
      { preserveUnavailable: true, preserveUnitPrice: true },
    )

    expect(restored).toMatchObject({
      product_family_key: '9:10:202',
      customer_product_alias_id: 202,
      product_name: '客户别名B',
      product_id: 11,
      unit_price: 72,
      validation_error: '',
    })

    expect(buildEmployeeOrderItemsPayload([restored])).toEqual([
      expect.objectContaining({
        product_id: 11,
        customer_product_alias_id: 202,
        name: '客户别名B',
      }),
    ])
  })

  it('detects manually changed shipping snapshots before syncing edited customer data', () => {
    const defaults = {
      receiver_name: '张三',
      receiver_phone: '13800000000',
      receiver_address: '上海市测试路1号',
      receiver_company: '客户A公司',
    }
    expect(employeeOrderShippingChanged(defaults, defaults)).toBe(false)
    expect(employeeOrderShippingChanged({ ...defaults, receiver_phone: '13900000000' }, defaults)).toBe(true)
  })
})
