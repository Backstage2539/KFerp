export function hasCustomerCapability(capabilities = [], code) {
  if (!code) return false
  const allowed = new Set((Array.isArray(capabilities) ? capabilities : []).map((item) => String(item || '').trim()).filter(Boolean))
  return allowed.has(code)
}

export function customerFulfillmentPublicSKUOrderMode(capabilities = []) {
  return hasCustomerCapability(capabilities, 'product_order')
}

export function customerFulfillmentSubmitCopy(capabilities = []) {
  if (customerFulfillmentPublicSKUOrderMode(capabilities)) {
    return {
      formTitle: '提交订单信息',
      submitButton: '提交订单',
      successPrefix: '已提交订单',
      errorFallback: '提交订单失败',
      notePlaceholder: '订单要求',
    }
  }
  return {
    formTitle: '提交代发信息',
    submitButton: '提交代发',
    successPrefix: '已提交代发',
    errorFallback: '提交代发失败',
    notePlaceholder: '发货要求',
  }
}

export function customerFulfillmentWorkbenchSections(capabilities = []) {
  const processing = hasCustomerCapability(capabilities, 'processing')
  const directShip = hasCustomerCapability(capabilities, 'direct_ship')
  const inventory = hasCustomerCapability(capabilities, 'inventory_custody')
  const settlement = hasCustomerCapability(capabilities, 'settlement')
  const orders = processing
    || directShip
    || hasCustomerCapability(capabilities, 'product_order')
    || hasCustomerCapability(capabilities, 'mall')
  return {
    processing,
    directShip,
    inventory,
    settlement,
    orders,
  }
}

export function activeCustomerFulfillmentCustomers(payload = {}) {
  const rows = Array.isArray(payload.customers) ? payload.customers : (Array.isArray(payload.rows) ? payload.rows : [])
  return rows.filter((row) => row?.active !== false)
}

export function customerFulfillmentCustomerOptionLabel(customer) {
  const fallback = Number(customer?.id || 0) > 0 ? `客户 ${customer.id}` : ''
  return String(customer?.name || customer?.company_name || fallback).trim()
}

export function customerFulfillmentCustomerOptionMeta(customer) {
  const label = customerFulfillmentCustomerOptionLabel(customer)
  const parts = []
  if (customer?.company_name && customer.company_name !== label) parts.push(customer.company_name)
  if (customer?.contact) parts.push(customer.contact)
  if (customer?.phone || customer?.company_phone) parts.push(customer.phone || customer.company_phone)
  return parts.join(' / ')
}

export function customerFulfillmentOrderFees(row = {}) {
  const f=row.freight
  if(f?.final_receivable_cents!=null) return [
    {label:'商品',value:orderMoneyValue(row.total_amount)},
    {label:'最终运费',value:orderMoneyValue(f.final_receivable_cents/100)},
    {label:'物流成本',value:orderMoneyValue(f.actual_cost_cents/100)},
    {label:'运费已收 / 待收',value:`${orderMoneyValue(f.received_freight_cents/100)} / ${orderMoneyValue(f.due_freight_cents/100)}`},
    {label:'优惠',value:orderMoneyValue(row.discount_amount)},
    {label:'应收',value:orderMoneyValue(Number(row.grand_total||0)-Number(row.shipping_amount||0)+f.final_receivable_cents/100),emphasized:true},
  ]
  return [
    { label: '商品', value: orderMoneyValue(row?.total_amount) },
    { label: '运费', value: orderMoneyValue(row?.shipping_amount) },
    { label: '优惠', value: orderMoneyValue(row?.discount_amount) },
    { label: '应收', value: orderMoneyValue(row?.grand_total), emphasized: true },
  ]
}

function orderMoneyValue(value) {
  const text = String(value ?? '').trim()
  return text || '0.00'
}
