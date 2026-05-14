const importTypeCatalog = [
  { value: 'processing_workbook', label: '代加工工单', capability: 'processing' },
  { value: 'direct_ship_workbook', label: '代发清单', capability: 'direct_ship' },
  { value: 'settlement_workbook', label: '结算单', capability: 'settlement' },
]

export function importTypeOptions(capabilities = null) {
  const rows = Array.isArray(capabilities)
    ? importTypeCatalog.filter((option) => hasCustomerCapability(capabilities, option.capability))
    : importTypeCatalog
  return rows.map(({ value, label }) => ({ value, label }))
}

export function hasCustomerCapability(capabilities = [], code) {
  if (!code) return false
  const allowed = new Set((Array.isArray(capabilities) ? capabilities : []).map((item) => String(item || '').trim()).filter(Boolean))
  return allowed.has(code)
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
    imports: processing || directShip || settlement,
    orders,
  }
}

export function visibleCustomerFulfillmentImports(imports = [], capabilities = []) {
  const allowedTypes = new Set(importTypeOptions(capabilities).map((option) => option.value))
  return (Array.isArray(imports) ? imports : []).filter((row) => allowedTypes.has(row?.import_type))
}

export function importSummaryCards(summary = {}) {
  const cards = []
  addPositiveCard(cards, '有效行', summary.valid_rows)
  addPositiveCard(cards, '错误行', summary.invalid_rows)
  addPositiveCard(cards, '代发订单', summary.direct_ship_orders)
  addPositiveCard(cards, '加工工单', summary.processing_orders)
  addPositiveCard(cards, '费用明细', summary.fee_items)
  return cards
}

export function latestParsedBatchForType(imports = [], latestBatch = null, importType = '') {
  if (isParsedBatchForType(latestBatch, importType)) return latestBatch
  const rows = Array.isArray(imports) ? imports : []
  return rows.find((row) => isParsedBatchForType(row, importType)) || null
}

export function groupInvalidImportRows(rows = []) {
  const groups = new Map()
  for (const row of Array.isArray(rows) ? rows : []) {
    const sheet = String(row?.sheet_name || '-').trim() || '-'
    const rowType = String(row?.row_type || '-').trim() || '-'
    const error = String(row?.error || '校验未通过').trim() || '校验未通过'
    const key = `${sheet}|${rowType}|${error}`
    const current = groups.get(key) || { key, sheet_name: sheet, row_type: rowType, error, count: 0 }
    current.count += 1
    groups.set(key, current)
  }
  return [...groups.values()]
}

export function buildImportPreviewEffects(summary = {}) {
  const effects = []
  addPositiveEffect(effects, '将应用有效行', summary.valid_rows)
  addPositiveEffect(effects, '需先处理错误行', summary.invalid_rows)
  addPositiveEffect(effects, '托管生豆入库', summary.raw_bean_receipts)
  addPositiveEffect(effects, '托管生豆出库', summary.raw_bean_issues)
  addPositiveEffect(effects, '托管生豆盘点', summary.raw_bean_balances)
  addPositiveEffect(effects, '客户 SKU', summary.customer_skus)
  addPositiveEffect(effects, '包材库存', summary.packaging_balances)
  addPositiveEffect(effects, '加工工单', summary.processing_orders)
  addPositiveEffect(effects, '包装任务', summary.packaging_jobs)
  addPositiveEffect(effects, '库存转换', summary.conversion_jobs)
  addPositiveEffect(effects, '代发订单', summary.direct_ship_orders)
  addPositiveEffect(effects, '代发明细', summary.direct_ship_items)
  addPositiveEffect(effects, '费用明细', summary.fee_items)
  addPositiveEffect(effects, '结算批次', summary.settlement_batches)
  return effects
}

export function rowStatusLabel(status) {
  return {
    valid: '有效',
    invalid: '错误',
    applied: '已应用',
    parsed: '已解析',
  }[status] || status || '-'
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
  return [
    { label: '商品', value: orderMoneyValue(row?.total_amount) },
    { label: '运费', value: orderMoneyValue(row?.shipping_amount) },
    { label: '优惠', value: orderMoneyValue(row?.discount_amount) },
    { label: '应收', value: orderMoneyValue(row?.grand_total), emphasized: true },
  ]
}

function isParsedBatchForType(batch, importType) {
  return Boolean(batch && batch.status === 'parsed' && batch.import_type === importType)
}

function addPositiveCard(cards, label, value) {
  const n = Number(value || 0)
  if (n > 0) cards.push({ label, value: n })
}

function addPositiveEffect(effects, label, value) {
  const n = Number(value || 0)
  if (n > 0) effects.push({ label, value: n })
}

function orderMoneyValue(value) {
  const text = String(value ?? '').trim()
  return text || '0.00'
}
