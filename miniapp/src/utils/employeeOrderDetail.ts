import type {
  EmployeeOrderDetailItem,
  EmployeeOrderDocumentAsset,
  EmployeeOrderDocumentFormat,
  EmployeeOrderDocumentKind,
  EmployeeOrderDocuments,
  EmployeeOrderTrace,
} from '../api/customerPortal'

let rememberedListQuery = ''

export function employeeOrderDetailPagePath(orderID: unknown): string {
  const id = Number(orderID)
  if (!Number.isSafeInteger(id) || id <= 0) return ''
  return `/pages/employee-order-detail/employee-order-detail?id=${id}`
}

export type EmployeeOrderNavigationRow<T extends { id?: unknown }> = T & {
  detail_url: string
}

export function employeeOrderNavigationRows<T extends { id?: unknown }>(
  rows: readonly T[] | null | undefined,
): EmployeeOrderNavigationRow<T>[] {
  const result: EmployeeOrderNavigationRow<T>[] = []
  for (const row of Array.isArray(rows) ? rows : []) {
    const detailURL = employeeOrderDetailPagePath(row.id)
    result.push({ ...row, detail_url: detailURL })
  }
  return result
}

export function rememberEmployeeOrderListQuery(query: string) {
  rememberedListQuery = String(query || '').trim()
}

export function employeeOrderListQuery(): string {
  return rememberedListQuery
}

function documentGroupKey(kind: EmployeeOrderDocumentKind): 'sales_order' | 'delivery_note' {
  return kind === 'sales-order' ? 'sales_order' : 'delivery_note'
}

export function employeeOrderDocumentAsset(
  documents: EmployeeOrderDocuments | undefined,
  kind: EmployeeOrderDocumentKind,
  format: EmployeeOrderDocumentFormat,
): EmployeeOrderDocumentAsset | undefined {
  const grouped = documents?.[documentGroupKey(kind)]?.[format]
  if (grouped) return grouped
  const flatKey = `${kind === 'sales-order' ? 'sales_order' : 'delivery_note'}_${format}` as const
  return documents?.[flatKey]
}

export function employeeOrderItemDisplayName(item: Partial<EmployeeOrderDetailItem> = {}): string {
  return String(
    item.customer_product_display_name_snapshot
      || item.product_name_snapshot
      || item.product_name
      || '-',
  ).trim() || '-'
}

export function employeeOrderItemSpecLabel(item: Partial<EmployeeOrderDetailItem> = {}): string {
  if (item.product_kind === 'drip_bag') {
    if (String(item.unit_conversion_label || '').trim()) return String(item.unit_conversion_label).trim()
    if (item.sales_unit === 'box') return `${Number(item.unit_bag_count || 10)}袋/盒`
    return `${String(item.unit_bean_g || '10').trim()}g/袋`
  }
  const spec = String(item.spec || '').trim()
  if (!spec) return '-'
  return /g$/i.test(spec) ? spec : `${spec}g`
}

type PriceSourceSnapshot = {
  source?: string
  source_label?: string
  template_name?: string
  version?: string
  version_no?: string
  template_version?: string
  price_version?: string
  price_list_version?: string
}

function itemPriceSourceSnapshot(item: Partial<EmployeeOrderDetailItem>): PriceSourceSnapshot {
  try {
    return JSON.parse(String(item.price_source_json || '{}')) as PriceSourceSnapshot
  } catch {
    return {}
  }
}

export function employeeOrderItemPriceTableVersion(item: Partial<EmployeeOrderDetailItem> = {}): string {
  const snapshot = itemPriceSourceSnapshot(item)
  return String(
    item.bean_list_version_no
      || snapshot.version
      || snapshot.version_no
      || snapshot.template_version
      || snapshot.price_version
      || snapshot.price_list_version
      || '',
  ).trim()
}

export function employeeOrderItemPriceSourceLabel(item: Partial<EmployeeOrderDetailItem> = {}): string {
  const snapshot = itemPriceSourceSnapshot(item)
  const source = String(snapshot.source || snapshot.source_label || snapshot.template_name || '').trim()
  const version = employeeOrderItemPriceTableVersion(item)
  if (source && version) return `${source} · ${version}`
  if (version) return version
  if (source) return source
  return String(item.price_source_json || '').trim() ? '价格来源快照' : '-'
}

export function employeeOrderInvoiceStatusLabel(status: unknown): string {
  switch (String(status || '').trim()) {
    case 'requested':
      return '已申请'
    case 'uploaded':
      return '已上传'
    default:
      return '未申请'
  }
}

export type EmployeeOrderDetailLine = {
  key: string
  label: string
  value: string
  emphasized?: boolean
}

function money(value: unknown): string {
  const text = String(value ?? '').trim() || '0.00'
  return `¥${text}`
}

function hasMoney(value: unknown): boolean {
  return Math.abs(Number.parseFloat(String(value ?? '0'))) > 0.0001
}

export function employeeOrderFeeLines(order: Record<string, unknown> = {}): EmployeeOrderDetailLine[] {
  const lines: EmployeeOrderDetailLine[] = [
    { key: 'total_amount', label: '商品金额', value: money(order.total_amount) },
    { key: 'shipping_amount', label: '运费', value: money(order.shipping_amount) },
    { key: 'discount_amount', label: '优惠', value: money(order.discount_amount) },
    { key: 'grand_total', label: '应收', value: money(order.grand_total), emphasized: true },
  ]
  if (String(order.express_fee || '').trim()) {
    lines.push({ key: 'express_fee', label: '快递费', value: String(order.express_fee).trim() })
  }
  if (hasMoney(order.outsource_total_fee)) {
    lines.push({ key: 'outsource_total_fee', label: '委外合计', value: money(order.outsource_total_fee) })
  }
  const outsourceFields = [
    ['outsource_material_fee', '委外物料'],
    ['outsource_roast_fee', '委外烘焙'],
    ['outsource_packaging_fee', '委外包装'],
    ['outsource_manual_fee', '委外人工'],
    ['outsource_tax_fee', '委外税费'],
    ['outsource_other_fee', '委外其他'],
  ] as const
  for (const [key, label] of outsourceFields) {
    if (hasMoney(order[key])) lines.push({ key, label, value: money(order[key]) })
  }
  return lines
}

export function employeeOrderTraceLineLabel(row: EmployeeOrderTrace = {}): string {
  const name = String(row.product_name || row.productName || '-').trim() || '-'
  const tier = String(row.tier_label || row.tierLabel || '').trim()
  return tier ? `${name} · ${tier}` : name
}

export function employeeOrderTraceSourceLines(
  row: EmployeeOrderTrace = {},
  type: 'quote' | 'production' = 'quote',
): string[] {
  if (type === 'production') {
    return [
      row.bom_version_no ? `BOM：${row.bom_version_no}` : '',
      row.process_route_name ? `工艺：${row.process_route_name}` : '',
      row.process_card_no ? `工序卡：${row.process_card_no}` : '',
      row.work_order_no ? `工单：${row.work_order_no}` : '',
      row.material_batch_no ? `物料批次：${row.material_batch_no}` : '',
      row.source_label || '',
    ].filter(Boolean)
  }
  const priceText = row.final_unit_price
    ? `${String(row.final_unit_price).trim()}/${String(row.price_unit || '-').trim() || '-'}`
    : ''
  return [
    row.price_list_version
      ? `价格表：${row.price_list_version}`
      : (row.price_list_publication_id ? `价格表：#${row.price_list_publication_id}` : ''),
    priceText ? `最终价：${priceText}` : '',
    row.pricing_rule_version ? `Pricing Rule：${row.pricing_rule_version}` : '',
    row.manual_adjusted ? '人工调整' : '',
    row.source_label || '',
  ].filter(Boolean)
}
