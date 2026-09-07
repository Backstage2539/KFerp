export type PriceTableOption = {
  is_customer_owned?: boolean
  published_at?: string
  id: number
  list_type: string
  classification_template_id?: number
  classification_template_name?: string
  product_type_category_id?: number
  product_type_name?: string
  table_name?: string
  table_key?: string
  release_id?: string
  version_no: string
  is_default?: boolean
  is_default_table?: boolean
}
export type PriceTableGroup = { key: string; label: string; options: PriceTableOption[] }
export function priceTableGroups(options: PriceTableOption[] = []): PriceTableGroup[] {
  const groups = new Map<string, PriceTableGroup>()
  for (const row of options) {
    const id = Number(row.classification_template_id || row.product_type_category_id || 0)
    const key = id ? `classification:${id}` : `legacy:${row.list_type}`
    if (!groups.has(key)) groups.set(key, { key, label: row.classification_template_name || row.product_type_name || (row.list_type === 'green' ? '生豆' : '商品'), options: [] })
    groups.get(key)!.options.push(row)
  }
  return [...groups.values()]
}
export function priceTableLabel(table?: PriceTableOption): string {
  return table ? `${table.is_customer_owned === undefined ? '' : table.is_customer_owned ? '客户 · ' : '公共 · '}${table.table_name || table.product_type_name || '价格表'} · ${table.version_no}${table.is_default ? '（默认）' : ''}` : '选择价格表'
}
export function replaceSelectedPriceTable(ids: number[], group: PriceTableGroup, id: number): number[] {
  if (!group.options.some(row => row.id === id)) throw new Error('所选价格表不属于该商品类型')
  const groupIDs = new Set(group.options.map(row => row.id))
  return [...ids.filter(value => !groupIDs.has(value)), id]
}
