import type { CustomerRecipientAddress, EmployeeOrderProductTier } from '../api/customerPortal'

function normalized(value: unknown): string {
  return String(value || '').trim().toLowerCase().replace(/\s+/g, '')
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
