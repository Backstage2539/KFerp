export const priceEntryPurposeLabels = Object.freeze({
  wholesale: '批发',
  direct_ship: '一件代发',
})

export function groupPriceEntries(rows = []) {
  const groups = new Map()
  for (const row of rows) {
    if (!row?.type_key) continue
    let group = groups.get(row.type_key)
    if (!group) {
      group = { key: row.type_key, name: row.type_name || '未命名商品类型', entries: [] }
      groups.set(row.type_key, group)
    }
    group.entries.push(row)
  }
  return [...groups.values()]
    .map((group) => ({
      ...group,
      entries: group.entries.sort(
        (a, b) =>
          Object.keys(priceEntryPurposeLabels).indexOf(a.purpose) -
          Object.keys(priceEntryPurposeLabels).indexOf(b.purpose),
      ),
    }))
    .sort((a, b) => a.name.localeCompare(b.name, 'zh-CN'))
}
