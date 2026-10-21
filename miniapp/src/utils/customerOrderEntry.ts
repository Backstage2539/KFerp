import type { CustomerRecipientAddress, EmployeeOrderProductTier } from '../api/customerPortal'

function normalized(value: unknown): string {
  return String(value || '').trim().toLowerCase().replace(/\s+/g, '')
}

export function customerOrderCatalogError(error: unknown, productOrder = false): string {
  const message = error instanceof Error ? error.message : String(error || '')
  if (/capability not enabled|capability .* unavailable/i.test(message)) return `当前客户尚未开通${productOrder ? '商品下单' : '所选代发服务'}，请联系工厂开通`
  return message || '商品与价格表加载失败，请重试'
}

export function customerPriceTableEmptyMessage(state: { loaded: boolean; loading: boolean; failed: boolean; error?: string; stock?: boolean }): string {
  if (state.loading || !state.loaded && !state.failed) return '价格表加载中…'
  if (state.failed) return /capability not enabled|尚未开通/i.test(state.error || '') ? '当前下单服务尚未开通，无法加载价格表' : '价格表暂时无法加载，请重试'
  return state.stock ? '库存代发使用已购库存，不重复收取商品货款' : 'ERP 尚未绑定可用价格表'
}

export function filterRecipientAddresses(rows: CustomerRecipientAddress[] = [], query = ''): CustomerRecipientAddress[] {
  const keyword = normalized(query)
  if (!keyword) return rows
  return rows.filter((row) => normalized([
    row.recipient_name, row.phone, row.company, row.province, row.city, row.district, row.detail_address,
  ].join(' ')).includes(keyword))
}

export function recipientAddressSummary(row?: Partial<CustomerRecipientAddress> | null): string {
  if (!row) return ''
  const contact = [row.recipient_name, row.phone].filter(Boolean).join(' ')
  const address = [row.province, row.city, row.district, row.detail_address].filter(Boolean).join('')
  return [contact, row.company, address].filter(Boolean).join(' · ')
}

export function tierQuantityLabel(tier: Partial<EmployeeOrderProductTier>): string {
  const min = Number(tier.min_qty ?? tier.min ?? 1)
  const maxValue = tier.max_qty ?? tier.max
  const max = maxValue === undefined || maxValue === null ? 0 : Number(maxValue)
  return `${min}${max > 0 ? `-${max}` : '+'} ${String(tier.sales_unit || '件').trim() || '件'}`
}
