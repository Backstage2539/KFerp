import { describe, expect, it } from 'vitest'
import {
  emptyDirectShipForm,
  emptyFulfillmentForm,
  emptyOrderSearch,
  emptyProcessingForm,
} from './serviceForms'

describe('service form factories', () => {
  it('builds fresh default forms for service workflows', () => {
    expect(emptyDirectShipForm()).toEqual({ source_name: '', total_rows: 0, note: '' })
    expect(emptyProcessingForm()).toEqual({
      input_material_id: 0,
      input_qty_g: 0,
      target_product_id: 0,
      target_spec_g: 454,
      target_qty: 1,
      note: '',
    })
    expect(emptyFulfillmentForm()).toEqual({
      recipient_name: '',
      recipient_phone: '',
      recipient_address: '',
      recipient_company: '',
      product_id: 0,
      bom_spec_id: 0,
      bom_variant_id: 0,
      inventory_unit: '',
      product_name: '',
      spec_g: 454,
      qty: 1,
      sales_unit: '',
      unit_bag_count: 0,
      unit_bean_g: 0,
      note: '',
    })
    expect(emptyOrderSearch()).toEqual({
      keyword: '',
      date_from: '',
      date_to: '',
      process_status: '',
      pay_status: '',
      ship_status: '',
    })
  })

  it('returns independent objects for repeated calls', () => {
    const first = emptyFulfillmentForm()
    const second = emptyFulfillmentForm()
    first.recipient_name = '张三'

    expect(second.recipient_name).toBe('')
  })
})
