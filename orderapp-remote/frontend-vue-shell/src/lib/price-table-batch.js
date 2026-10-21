export const clonePriceTable = value => JSON.parse(JSON.stringify(value ?? {}))

function tableKey() {
  return globalThis.crypto?.randomUUID?.() || `table-${Date.now()}-${Math.random().toString(36).slice(2)}`
}

export function createPriceTableBatch(payload = {}, name = '价格表') {
  const key = tableKey()
  return { version: payload.version || payload.config?.version || '', changelog: payload.changelog || '', default_table_key: key, active_table_key: key, tables: [{ key, name, is_new_table: true, direct_ship_enabled: false, payload: clonePriceTable(payload) }] }
}

export function addPriceTable(batch, copyKey = '') {
  const next = clonePriceTable(batch)
  const source = next.tables.find(table => table.key === copyKey)
  const key = tableKey()
  let index = 1; let name = ''
  do { name = source ? `${source.name}（副本${index++}）` : `价格表${index++}` } while (next.tables.some(table => table.name.trim() === name))
  next.tables.push({ key, name, is_new_table: true, direct_ship_enabled: Boolean(source?.direct_ship_enabled), payload: clonePriceTable(source?.payload || {}) })
  next.active_table_key = key
  return next
}

export function removePriceTable(batch, key) {
  if (batch.tables.length <= 1) throw new Error('至少保留一张价格表')
  const next = clonePriceTable(batch)
  if (next.tables.some(table => table.key === key)) next.removed_table_keys = [...new Set([...(next.removed_table_keys || []), key])]
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
const pendingDraftWrites = new WeakMap()

function draftStore(indexedDB, mode, key, value) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('kferp-price-table-drafts', 1)
    let blocked = false
    request.onupgradeneeded = () => {
      if (!request.result.objectStoreNames.contains('drafts')) request.result.createObjectStore('drafts')
    }
    request.onerror = () => reject(request.error)
    request.onblocked = () => { blocked = true; reject(new Error('草稿存储被占用')) }
    request.onsuccess = () => {
      const db = request.result
      if (blocked) { db.close(); return }
      db.onversionchange = () => db.close()
      try {
        const transaction = db.transaction('drafts', mode)
        const store = transaction.objectStore('drafts')
        const operation = mode === 'readwrite' ? store.put(value, key) : store.get(key)
        transaction.oncomplete = () => { db.close(); resolve(operation.result) }
        transaction.onabort = transaction.onerror = () => { db.close(); reject(transaction.error) }
      } catch (err) { db.close(); reject(err) }
    }
  })
}

export async function savePriceTableBatchDraft(scope, batch, storage, indexedDB = globalThis.indexedDB) {
  try {
    // Snapshot reactive data now, before a queued write can observe later edits.
    const value = JSON.stringify(batch)
    const key = `kferp:named-price-tables:${scope}`
    if (!indexedDB) {
      const target = storageOrDefault(storage)
      if (!target) throw new Error('浏览器存储不可用')
      target.setItem(key, value)
      return
    }
    let writes = pendingDraftWrites.get(indexedDB)
    if (!writes) { writes = new Map(); pendingDraftWrites.set(indexedDB, writes) }
    const previous = writes.get(key) || Promise.resolve()
    const write = previous.catch(() => {}).then(() => draftStore(indexedDB, 'readwrite', key, value))
    writes.set(key, write)
    try { await write } finally { if (writes.get(key) === write) writes.delete(key) }
  } catch (cause) {
    throw new Error('本地草稿保存失败，当前编辑内容仍保留在页面中；请勿刷新，可重试保存或发布到服务器。', { cause })
  }
}

export async function readPriceTableBatchDraft(scope, storage, indexedDB = globalThis.indexedDB) {
  const key = `kferp:named-price-tables:${scope}`
  let raw
  if (indexedDB) {
    try {
      await pendingDraftWrites.get(indexedDB)?.get(key)
      raw = await draftStore(indexedDB, 'readonly', key)
    } catch (cause) {
      // An inaccessible draft must not be mistaken for a missing draft.
      throw new Error('本地草稿读取失败，未替换当前内容，请重试或检查浏览器存储。', { cause })
    }
  }
  try {
    // Existing localStorage drafts remain a migration source, never a second writer.
    if (raw === undefined || raw === null) raw = storageOrDefault(storage)?.getItem(key)
    const batch = raw ? JSON.parse(raw) : null
    return Array.isArray(batch?.tables) && batch.tables.length ? batch : null
  } catch { return null }
}

export function publicationTableMetadata(row) {
  return { ...(row?.config?.publication_batch || {}), ...Object.fromEntries(['release_id','table_key','table_name','is_default_table','direct_ship_enabled'].filter(key => row?.[key] !== undefined).map(key => [key,row[key]])) }
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
