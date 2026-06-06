import type { CustomerPriceTableGroup } from '../api/customerPortal'

export type CollapsedPriceTableSummary = {
  list_type: string
  title: string
  latest_version_no: string
  total_versions: number
  expanded: boolean
}

export function priceTableGroupLabel(group: CustomerPriceTableGroup): string {
  const title = group.list_type_label || group.list_type || '商品'
  return `${title} · ${Number(group.product_count || 0)} 个商品 · ${Number(group.price_table_count || 0)} 个价格表`
}

export function collapsedPriceTableSummaries(groups: CustomerPriceTableGroup[] = []): CollapsedPriceTableSummary[] {
  return groups.map((group) => ({
    list_type: group.list_type,
    title: group.list_type_label || group.list_type || '商品',
    latest_version_no: group.latest_version?.version_no || '',
    total_versions: group.versions?.length || group.price_table_count || 0,
    expanded: false,
  }))
}
