export function orderCatalogKey(customerID, ids = []) {
  return `${Number(customerID || 0)}:${[...new Set(ids.map(Number).filter(id => id > 0))].sort((a,b) => a-b).join(',')}`
}

export function staffPriceTableGroups(options = [], selectedIDs = []) {
  const groups = new Map()
  for (const row of options) {
    const id = Number(row.classification_template_id || row.product_type_category_id || 0)
    const legacy = row.list_type === 'retail' ? 'commercial' : row.list_type || 'commercial'
    const key = id > 0 ? `classification:${id}` : `legacy:${legacy}`
    if (!groups.has(key)) groups.set(key, { key, label: row.classification_template_name || row.product_type_name || (legacy === 'green' ? '生豆' : '商品'), classified: id > 0, classificationTemplateID: id, autoSelect: true, options: [] })
    groups.get(key).options.push(row)
  }
  return [...groups.values()].map(group => {
    const selected = group.options.find(row => selectedIDs.includes(Number(row.id))) || group.options.find(row => row.is_default) || group.options[0]
    return { ...group, listType: selected.list_type || 'commercial' }
  })
}

export function createOrderCatalogSession({ state, load }) {
  let revision = 0
  Object.assign(state, { customerID: 0, key: '', loading: false, error: '', families: [], options: [] })
  return {
    async refresh(customerID, ids = []) {
      const current = ++revision
      Object.assign(state, { customerID: Number(customerID || 0), loading: customerID > 0, error: '', families: [], key: '' })
      if (!(customerID > 0)) { state.options = []; return null }
      try {
        const data = await load(customerID, ids)
        if (current !== revision) return null
        if (Number(data.customer_id) !== Number(customerID)) throw Error('商品目录客户不匹配，请重试')
        state.families = data.product_families || []
        state.options = data.price_table_options || []
        state.key = orderCatalogKey(customerID, data.selected_price_table_ids || [])
        return data
      } catch (cause) {
        if (current === revision) { state.error = cause.message || '商品目录读取失败'; state.families = [] }
        return null
      } finally { if (current === revision) state.loading = false }
    },
  }
}
