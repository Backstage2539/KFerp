export type OrderSearchForm = {
  keyword: string
  date_from: string
  date_to: string
  process_status: string
  pay_status: string
  ship_status: string
}

export type OrderStatusField = 'process_status' | 'pay_status' | 'ship_status'

export type DirectShipForm = {
  source_name: string
  total_rows: number
  note: string
}

export type ProcessingForm = {
  input_material_id: number
  input_qty_g: number
  target_product_id: number
  target_spec_g: number
  target_qty: number
  note: string
}

export type FulfillmentForm = {
  recipient_name: string
  recipient_phone: string
  recipient_address: string
  recipient_company: string
  product_id: number
  product_name: string
  spec_g: number
  qty: number
  sales_unit: string
  unit_bag_count: number
  unit_bean_g: number
  note: string
}

export function emptyOrderSearch(): OrderSearchForm {
  return { keyword: '', date_from: '', date_to: '', process_status: '', pay_status: '', ship_status: '' }
}

export function emptyDirectShipForm(): DirectShipForm {
  return { source_name: '', total_rows: 0, note: '' }
}

export function emptyProcessingForm(): ProcessingForm {
  return {
    input_material_id: 0,
    input_qty_g: 0,
    target_product_id: 0,
    target_spec_g: 454,
    target_qty: 1,
    note: '',
  }
}

export function emptyFulfillmentForm(): FulfillmentForm {
  return {
    recipient_name: '',
    recipient_phone: '',
    recipient_address: '',
    recipient_company: '',
    product_id: 0,
    product_name: '',
    spec_g: 454,
    qty: 1,
    sales_unit: '',
    unit_bag_count: 0,
    unit_bean_g: 0,
    note: '',
  }
}
