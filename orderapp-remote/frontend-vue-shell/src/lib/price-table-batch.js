export const clonePriceTable = value => JSON.parse(JSON.stringify(value ?? {}))

function tableKey() {
  return globalThis.crypto?.randomUUID?.() || `table-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function createPriceTableBatch(payload = {}, name = '价格表') {
  const key = tableKey()
  return { version: payload.version || payload.config?.version || '', changelog: payload.changelog || '', default_table_key: key, active_table_key: key, tables: [{ key, name, payload: clonePriceTable(payload) }] }
}

export function addPriceTable(batch, copyKey = '') {
  const next = clonePriceTable(batch)
  const source = next.tables.find(table => table.key === copyKey)
  const key = tableKey()
  let index = 1; let name = ''
  do { name = source ? `${source.name}（副本${index++}）` : `价格表${index++}` } while (next.tables.some(table => table.name.trim() === name))
  next.tables.push({ key, name, payload: clonePriceTable(source?.payload || {}) })
  next.active_table_key = key
  return next
}

export function removePriceTable(batch, key) {
  if (batch.tables.length <= 1) throw new Error('至少保留一张价格表')
  const next = clonePriceTable(batch)
  next.tables = next.tables.filter(table => table.key !== key)
  if (next.default_table_key === key) next.default_table_key = next.tables[0].key
  if (next.active_table_key === key) next.active_table_key = next.tables[0].key
  return next
}

export function validatePriceTableBatch(batch) {
  if (!batch?.tables?.length) return '至少保留一张价格表'
  const names = new Set(); const keys = new Set()
  for (const table of batch.tables) {
    const name = String(table.name || '').trim()
    if (!name) return '请填写价格表名称'
    if (names.has(name)) return `价格表名称重复：${name}`
    if (!table.key || keys.has(table.key)) return '价格表标识重复，请重新添加'
    names.add(name); keys.add(table.key)
  }
  return keys.has(batch.default_table_key) ? '' : '请选择本版本的默认价格表'
}

const storageOrDefault = storage => storage || (typeof window !== 'undefined' ? window.localStorage : null)
export function savePriceTableBatchDraft(scope, batch, storage) {
  storageOrDefault(storage)?.setItem(`kferp:named-price-tables:${scope}`, JSON.stringify(batch))
}
export function readPriceTableBatchDraft(scope, storage) {
  try {
    const raw = storageOrDefault(storage)?.getItem(`kferp:named-price-tables:${scope}`)
    const batch = raw ? JSON.parse(raw) : null
    return Array.isArray(batch?.tables) && batch.tables.length ? batch : null
  } catch { return null }
}

export function publicationTableMetadata(row) {
  return { ...(row?.config?.publication_batch || {}), ...Object.fromEntries(['release_id','table_key','table_name','is_default_table'].filter(key => row?.[key] !== undefined).map(key => [key,row[key]])) }
}

export function publicationBatchGroups(rows = []) {
  const groups = new Map()
  for (const row of rows) {
    const meta = publicationTableMetadata(row)
    const key = meta.release_id || `legacy:${row.id}`
    if (!groups.has(key)) groups.set(key, { key, version: row.version, tables: [] })
    groups.get(key).tables.push({ ...row, ...meta })
  }
  return [...groups.values()]
}

export function publicationBatchListState(rows = [], {query = '', page = 1, pageSize = 10} = {}) {
  const q = String(query).trim().toLowerCase()
  const groups = publicationBatchGroups(rows).filter(group => !q || group.tables.some(row => [row.version,row.table_name,row.owner_key,row.status,row.changelog,row.product_type_name].join(' ').toLowerCase().includes(q)))
  const total = groups.length
  const currentPage = Math.max(1, Math.min(Number(page) || 1, Math.max(1, Math.ceil(total / pageSize))))
  const batches = groups.slice((currentPage - 1) * pageSize, currentPage * pageSize)
  return { total, page: currentPage, pageSize, batches, rows: batches.flatMap(group => group.tables) }
}
