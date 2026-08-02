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
  employeeOrderItemFromSpec,
  employeeOrderShippingChanged,
  employeeOrderItemsTotal,
  employeeOrderGrandTotal,
  employeeOrderItemDiscountAmount,
  employeeOrderEditableOrderDiscount,
  employeeOrderOutsourceTotal,
  employeeOrderTierForQuantity,
  repriceEmployeeOrderItemForQuantity,
  isEmployeeOrderNonNegativeMoney,
  hydrateEmployeeOrderEditItems,
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
      price_source_json: '{"quantity_basis":"sales_spec_count"}',
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
    expect(employeeOrderGrandTotal([first, second], 15, 6)).toBe(345)
  })

  it('hydrates an editable order through the exact customer alias before falling back to the SKU', () => {
    const families = [
      {
        customer_id: 9,
        parent_product_id: 10,
        customer_product_alias_id: 201,
        name: '客户别名A',
        specs: [{
          product_id: 11,
          spec_label: '227g',
          tiers: [{ unit_price: 60, publication_id: 81, publication_version_no: 'V8' }],
        }],
      },
      {
        customer_id: 9,
        parent_product_id: 10,
        customer_product_alias_id: 202,
        name: '客户别名B',
        specs: [{
          product_id: 11,
          spec_label: '227g',
          tiers: [{ unit_price: 72, publication_id: 82, publication_version_no: 'V9' }],
        }],
      },
    ]

    const [item] = hydrateEmployeeOrderEditItems([{
      item_id: 31,
      product_id: 11,
      customer_product_alias_id: 202,
      product_name: '历史名称',
      spec: '227',
      qty: '3',
      unit: '袋',
      unit_price: '75.50',
      line_total: '226.50',
      price_override: true,
      bean_list_publication_id: 82,
      bean_list_version_no: 'V9',
    }], families, 9)

    expect(item).toMatchObject({
      item_id: 31,
      product_family_key: '9:10:202',
      customer_product_alias_id: 202,
      product_id: 11,
      product_name: '客户别名B',
      qty: 3,
      unit_price: 75.5,
      price_override: true,
      bean_list_publication_id: 82,
      bean_list_version_no: 'V9',
      validation_error: '',
    })
    expect(buildEmployeeOrderItemsPayload([item])).toEqual([
      expect.objectContaining({
        product_id: 11,
        item_id: 31,
        parent_product_id: 10,
        customer_product_alias_id: 202,
        bean_list_publication_id: 82,
        bean_list_version_no: 'V9',
        price_override: true,
      }),
    ])
  })

  it('keeps an unavailable historical edit line visible and blocks it until a current spec is selected', () => {
    const [item] = hydrateEmployeeOrderEditItems([{
      item_id: 32,
      product_id: 99,
      customer_product_alias_id: 909,
      product_name: '已从价格表下架的商品',
      spec: '454g',
      qty: '2',
      unit: '袋',
      unit_price: '88.00',
      line_total: '176.00',
      bean_list_publication_id: 70,
      bean_list_version_no: 'V7',
    }], [], 9)

    expect(item).toMatchObject({
      item_id: 32,
      product_id: 99,
      customer_product_alias_id: 909,
      product_name: '已从价格表下架的商品',
      spec_label: '454g',
      qty: 2,
      unit_price: 88,
      validation_error: '该历史商品或规格已不在当前价格表，请重新选择当前可售规格',
    })
  })

  it('hydrates only the order-level discount and never counts preserved line discounts twice', () => {
    const items = [
      { discount_amount: '6.50' },
      { discount_amount: '2.00' },
    ]

    expect(employeeOrderEditableOrderDiscount('16.50', items)).toBe(8)
    expect(employeeOrderEditableOrderDiscount('5.00', items)).toBe(0)
    expect(employeeOrderEditableOrderDiscount('99.00', items, '3.50')).toBe(3.5)
    expect(employeeOrderEditableOrderDiscount('16.50', items, '0')).toBe(0)
  })

  it('selects the current quantity tier and keeps automatic prices automatic while quantity changes', () => {
    const family = {
      customer_id: 0,
      parent_product_id: 10,
      name: '分档商品',
      specs: [{
        product_id: 11,
        spec_label: '227g',
        tiers: [
          { unit_price: 68, min_qty: 1, max_qty: 9, publication_id: 91 },
          { unit_price: 60, min_qty: 10, publication_id: 91 },
        ],
      }],
    }
    const automatic = Object.assign(createEmployeeOrderItem('auto'), {
      product_family_key: '0:10:0',
      product_family_id: 10,
      product_id: 11,
      qty: 12,
      unit_price: 68,
      price_override: false,
    })
    const manual = { ...automatic, key: 'manual', unit_price: 72, price_override: true }

    expect(repriceEmployeeOrderItemForQuantity(automatic, family)).toMatchObject({
      qty: 12,
      unit_price: 60,
      price_override: false,
    })
    expect(repriceEmployeeOrderItemForQuantity(manual, family)).toMatchObject({
      qty: 12,
      unit_price: 72,
      price_override: true,
    })

    const [hydrated] = hydrateEmployeeOrderEditItems([{
      item_id: 88,
      product_id: 11,
      product_name: '分档商品',
      qty: '12',
      spec: '227g',
      unit: '袋',
      unit_price: '68',
      line_total: '816',
      price_override: false,
    }], [family], 8)
    expect(hydrated).toMatchObject({ unit_price: 60, price_override: false })
  })

  it('marks an item unavailable when its quantity falls into a price-tier gap', () => {
    const family = {
      customer_id: 0,
      parent_product_id: 10,
      name: '断档商品',
      specs: [{
        product_id: 11,
        spec_label: '227g',
        tiers: [
          { unit_price: 68, min_qty: 1, max_qty: 1 },
          { unit_price: 60, min_qty: 3, max_qty: 9 },
        ],
      }],
    }
    const automatic = Object.assign(createEmployeeOrderItem('gap'), {
      product_family_key: '0:10:0',
      product_family_id: 10,
      product_id: 11,
      qty: 2,
      unit_price: 68,
      price_override: false,
    })

    expect(employeeOrderTierForQuantity(family.specs[0], 2)).toBeUndefined()
    expect(repriceEmployeeOrderItemForQuantity(automatic, family)).toMatchObject({
      qty: 2,
      unit_price: 0,
      price_override: false,
      validation_error: '当前数量没有匹配的价格档，请调整数量',
    })
  })

  it('includes hidden line discounts in totals and recalculates them after quantity changes', () => {
    const family = {
      customer_id: 0,
      parent_product_id: 10,
      name: '优惠商品',
      specs: [{
        product_id: 11,
        spec_label: '227g',
        sales_unit: 'bag',
        tiers: [{
          unit_price: 50,
          min_qty: 1,
          price_source_json: '{"quantity_basis":"sales_spec_count"}',
        }],
      }],
    }
    const [hydrated] = hydrateEmployeeOrderEditItems([{
      item_id: 90,
      product_id: 11,
      product_name: '优惠商品',
      qty: '2',
      spec: '227g',
      unit: '袋',
      unit_price: '50',
      line_total: '94',
      price_override: false,
      discount_type: 'unit_amount',
      discount_value: '3',
      discount_amount: '6',
    }], [family], 8)

    expect(hydrated).toMatchObject({
      discount_type: 'unit_amount',
      discount_value: 3,
      discount_amount: 6,
    })
    expect(employeeOrderItemsTotal([hydrated])).toBe(100)
    expect(employeeOrderGrandTotal([hydrated], 10, 4)).toBe(100)

    const repriced = repriceEmployeeOrderItemForQuantity({ ...hydrated, qty: 4 }, family)
    expect(repriced).toMatchObject({ qty: 4, unit_price: 50, discount_amount: 12 })
    expect(employeeOrderItemsTotal([repriced])).toBe(200)
    expect(employeeOrderGrandTotal([repriced])).toBe(188)

    const manual = repriceEmployeeOrderItemForQuantity({
      ...hydrated,
      qty: 4,
      unit_price: 70,
      price_override: true,
    }, family)
    expect(manual).toMatchObject({ qty: 4, unit_price: 70, discount_amount: 12 })
    expect(employeeOrderItemsTotal([manual])).toBe(280)
    expect(employeeOrderGrandTotal([manual])).toBe(268)

    const base = { ...hydrated, qty: 2, unit_price: 50 }
    expect(employeeOrderItemDiscountAmount({ ...base, discount_type: 'amount', discount_value: 120 })).toBe(100)
    expect(employeeOrderItemDiscountAmount({ ...base, discount_type: 'percent', discount_value: 80 })).toBe(20)
    expect(employeeOrderItemDiscountAmount({ ...base, discount_type: 'free', discount_value: 0 })).toBe(100)

    const legacyWeightDiscount = {
      ...base,
      product_kind: 'roasted_bean',
      sales_unit: 'lb',
      spec_g: 227,
      price_source_json: '',
      discount_type: 'unit_amount',
      discount_value: 3,
    }
    expect(employeeOrderItemDiscountAmount({ ...legacyWeightDiscount, retail_order: false })).toBe(3)
    expect(employeeOrderItemDiscountAmount({ ...legacyWeightDiscount, retail_order: true })).toBe(6)
    expect(employeeOrderItemsTotal([{ ...legacyWeightDiscount, retail_order: false }])).toBe(50)
    expect(employeeOrderGrandTotal([{ ...legacyWeightDiscount, retail_order: false }], 10, 4)).toBe(53)
    expect(employeeOrderItemDiscountAmount({
      ...legacyWeightDiscount,
      price_source_json: '{"quantity_basis":"sales_spec_count"}',
    })).toBe(6)
    expect(employeeOrderItemsTotal([{
      ...legacyWeightDiscount,
      price_source_json: '{"quantity_basis":"sales_spec_count"}',
    }])).toBe(100)
    expect(employeeOrderItemDiscountAmount({
      ...legacyWeightDiscount,
      product_kind: 'drip_bag',
      sales_unit: 'box',
    })).toBe(6)
    expect(employeeOrderItemDiscountAmount({
      ...legacyWeightDiscount,
      spec_g: 1000,
      unit_price: 50,
    })).toBe(6)
    expect(employeeOrderGrandTotal([{
      ...legacyWeightDiscount,
      spec_g: 1000,
      unit_price: 50,
    }])).toBe(94)

    const replacementFamily = {
      customer_id: 0,
      parent_product_id: 20,
      name: '换购商品',
      specs: [{ product_id: 21, spec_label: '227g', sales_unit: 'bag', tiers: [{ unit_price: 40, min_qty: 1 }] }],
    }
    const replaced = employeeOrderItemFromSpec(hydrated, replacementFamily, replacementFamily.specs[0])
    expect(replaced).toMatchObject({
      product_id: 21,
      discount_type: 'unit_amount',
      discount_value: 3,
      discount_amount: 6,
    })
    const [payloadItem] = buildEmployeeOrderItemsPayload([replaced])
    expect(payloadItem).not.toHaveProperty('discount_type')
    expect(payloadItem).not.toHaveProperty('discount_value')
    expect(payloadItem).not.toHaveProperty('discount_amount')
  })

  it('includes preserved outsource fees and applies the backend round-down rule in edit previews', () => {
    const item = Object.assign(createEmployeeOrderItem('outsource'), {
      product_id: 11,
      product_kind: 'drip_bag',
      qty: 2,
      unit_price: 50,
      discount_type: 'amount',
      discount_value: 6,
    })
    const outsourceTotal = employeeOrderOutsourceTotal({
      outsource_material_fee: '1.20',
      outsource_roast_fee: '2.30',
      outsource_packaging_fee: '3.40',
      outsource_manual_fee: '4.50',
      outsource_tax_fee: '5.60',
      outsource_other_fee: '6.70',
    })

    expect(outsourceTotal).toBe(23.7)
    expect(employeeOrderGrandTotal([item], 12, 4, outsourceTotal, false)).toBe(125.7)
    expect(employeeOrderGrandTotal([item], 12, 4, outsourceTotal, true)).toBe(125)
  })

  it('does not guess among multiple customer aliases when a historical line has no alias id', () => {
    const aliases = [
      {
        customer_id: 9,
        parent_product_id: 10,
        customer_product_alias_id: 201,
        name: '别名A',
        specs: [{ product_id: 11, spec_label: '227g', tiers: [{ unit_price: 60 }] }],
      },
      {
        customer_id: 9,
        parent_product_id: 10,
        customer_product_alias_id: 202,
        name: '别名B',
        specs: [{ product_id: 11, spec_label: '227g', tiers: [{ unit_price: 60 }] }],
      },
    ]
    const [ambiguous] = hydrateEmployeeOrderEditItems([{
      item_id: 89,
      product_id: 11,
      product_name: '历史商品',
      qty: '1',
      spec: '227g',
      unit_price: '60',
      line_total: '60',
      price_override: false,
    }], aliases, 9)

    expect(ambiguous).toMatchObject({
      item_id: 89,
      product_name: '历史商品',
      validation_error: '该历史商品对应多个客户别名，请重新选择当前可售商品',
    })
  })

  it('accepts only finite non-negative shipping and order-discount amounts', () => {
    expect(isEmployeeOrderNonNegativeMoney(0)).toBe(true)
    expect(isEmployeeOrderNonNegativeMoney('12.50')).toBe(true)
    expect(isEmployeeOrderNonNegativeMoney(-1)).toBe(false)
    expect(isEmployeeOrderNonNegativeMoney('abc')).toBe(false)
    expect(isEmployeeOrderNonNegativeMoney(Number.POSITIVE_INFINITY)).toBe(false)
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

  it('reprices restored automatic drafts while preserving explicit manual prices', () => {
    const automatic = Object.assign(createEmployeeOrderItem('draft-auto'), {
      product_id: 11,
      product_name: '公共商品',
      qty: 12,
      unit_price: 68,
      price_override: false,
    })
    const manual = { ...automatic, key: 'draft-manual', unit_price: 72.5, price_override: true }
    const families = [{
      customer_id: 0,
      parent_product_id: 10,
      name: '公共商品',
      specs: [{
        product_id: 11,
        spec_label: '227g',
        tiers: [
          { unit_price: 68, min_qty: 1, max_qty: 9 },
          { unit_price: 60, min_qty: 10 },
        ],
      }],
    }]

    const [restoredAutomatic, restoredManual] = revalidateEmployeeOrderItems(
      [automatic, manual],
      families,
      9,
      { preserveManualPrice: true },
    )

    expect(restoredAutomatic).toMatchObject({ product_id: 11, unit_price: 60, price_override: false })
    expect(restoredManual).toMatchObject({ product_id: 11, unit_price: 72.5, price_override: true })
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
