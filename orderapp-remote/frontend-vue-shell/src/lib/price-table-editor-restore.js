import { clonePriceTable, createPriceTableBatch, publicationTableMetadata } from './price-table-batch.js'
import { normalizePriceListProductSpecSelections, priceListParentProductID } from './product-price-list-selection.js'

const number = value => Number(value || 0)
const own = (value, key) => Object.prototype.hasOwnProperty.call(value, key)
const releaseKey = row => publicationTableMetadata(row).release_id || 'legacy:' + row.id
export const publicationEditorBaseline = row => ({
  id: number(row.id), release_id: releaseKey(row), table_key: publicationTableMetadata(row).table_key || 'legacy:' + row.id,
  version: row.version || '', schema_version: 1,
})

function savedPriceRows(content = {}) {
  if (Array.isArray(content.price_rows)) return clonePriceTable(content.price_rows)
  return (content.groups || []).flatMap(group => (group.items || []).flatMap(item =>
    (item.tiers_snapshot || item.commercial_wholesale_tiers || []).map(tier => ({
      ...item, ...tier, final_unit_price: tier.final_unit_price ?? tier.price_per_unit,
    }))))
}

// This converts public persisted fields, without the copy workflow's global
// freeze flag. Newly added products must still support normal pricing edits.
export function publicationEditorPayload(publication = {}, families = [], groupRows = []) {
  const payload = clonePriceTable(publication)
  if (payload.draft) return payload
  const config = payload.config || {}
  const pricing = config.price_list_template_selection || {}
  const priceRows = savedPriceRows(payload.content)
  const savedSelections = own(config, 'product_spec_selections') ? config.product_spec_selections
    : priceRows.map(row => ({ ...row, parent_product_id: priceListParentProductID(row), selection_source: 'explicit' }))
  const selections = normalizePriceListProductSpecSelections(savedSelections, families, { preserveMissing: true })
  const parentSelections = {}, groupSelections = {}, productOverrides = {}, flatRowOverrides = {}, tierFixedPrices = {}
  for (const row of pricing.group_selections || []) {
    const id = number(row.group_item_id || row.parent_group_item_id)
    if (!id) continue
    if (number(row.level) === 1) parentSelections[String(id)] = clonePriceTable(row)
    else {
      const group = groupRows.find(group => number(group.group_item_id) === id)
      groupSelections[String(group?.key || row.group_key || id)] = clonePriceTable(row)
    }
  }
  for (const row of pricing.product_overrides || []) {
    const parentID = number(row.parent_product_id || row.product_id)
    const skuID = number(row.bom_spec_id || row.sku_id || row.product_id)
    const parent = row.scope === 'parent_product'
    const key = parent ? 'parent:' + parentID : 'sku:' + skuID
    productOverrides[key] = { ...row, product_key: key, parent_product_id: parentID,
      sku_id: parent ? 0 : skuID, product_id: parent ? parentID : skuID }
  }
  for (const row of priceRows) {
    const skuID = number(row.bom_spec_id || row.sku_id || row.product_id)
    const parentID = number(row.parent_product_id || row.product_id)
    const amount = number(row.final_unit_price ?? row.fixed_unit_price)
    if (row.pricing_mode === 'fixed_price' && skuID > 0 && amount > 0) {
      // The published final price includes any manual edit made before publish.
      const key = 'sku:' + skuID
      productOverrides[key] = { ...productOverrides[key], scope: 'sku', product_key: key,
        parent_product_id: parentID, product_id: skuID, sku_id: skuID, fixed_unit_price: amount }
      // Older publications may have price rows but no pricing configuration.
      if (!config.price_list_template_selection) productOverrides['parent:' + parentID] = {
        scope: 'parent_product', product_id: parentID, parent_product_id: parentID, pricing_mode: 'fixed_price',
      }
    }
    if (row.row_key && row.manual_adjusted === true && amount > 0) flatRowOverrides[row.row_key] = amount
    if (row.row_key && row.tier_pricing_mode === 'fixed_price' && amount > 0) tierFixedPrices[row.row_key] = amount
  }
  payload.editor = payload.editor || {
    pdfOptions: config, customizers: config.customizers || {},
    priceSource: number(payload.price_source_publication_id) > 0 ? { id: number(payload.price_source_publication_id), version: payload.source_version || '' } : null,
    styleSource: number(payload.style_source_publication_id),
  }
  payload.draft = {
    defaults: pricing.defaults || { pricing_mode: 'tier_template' }, parentSelections, groupSelections, productOverrides, flatRowOverrides, tierFixedPrices,
    product_spec_selections: selections, price_list_display_order: config.price_list_display_order || {},
    customerPriceSeedRows: priceRows.map(row => ({ ...row, frozen_final_price: false,
      customer_price_source_publication_id: number(publication.id), customer_price_source_version: publication.version || '' })),
    customerPriceConfiguredSources: {}, customerPriceCopyFrozen: false,
  }
  return payload
}

export async function loadPublishedEditorBatch({ loadSummary, loadDetail, isCurrent = () => true }) {
  let page = 1, first, current, rows
  do {
    const result = await loadSummary(page)
    if (!isCurrent()) return undefined
    if (!first) { first = result; current = result.current }
    if (!current || current.status !== 'published') return null
    rows = (result.rows || []).filter(row => row.status === 'published' && releaseKey(row) === releaseKey(current))
    // Summary pagination groups by release: a release is never split between pages.
    if (rows.some(row => number(row.id) === number(current.id))) break
    if (page * number(result.page_size || 10) >= number(result.total)) throw new Error('未找到完整已发布价格表，请重试')
    page += 1
  } while (true)
  const details = await Promise.all(rows.map(loadDetail))
  if (!isCurrent()) return undefined
  if (details.some(row => row.status !== 'published' || releaseKey(row) !== releaseKey(current))) throw new Error('价格表发布状态已变化，请重试')
  const tables = details.map(row => {
    const meta = publicationTableMetadata(row)
    return { key: meta.table_key || 'legacy:' + row.id, name: meta.table_name || row.version || '价格表',
      direct_ship_enabled: Boolean(meta.direct_ship_enabled), source_publication: publicationEditorBaseline(row), payload: clonePriceTable(row) }
  })
  const defaultRow = details.find(row => publicationTableMetadata(row).is_default_table) || details[0]
  const defaultKey = publicationTableMetadata(defaultRow).table_key || 'legacy:' + defaultRow.id
  return { schema_version: 1, version: first.suggested_version || current.version, changelog: '',
    default_table_key: defaultKey, active_table_key: defaultKey, tables }
}

export function reconcileEditorBatch(local, published, legacyDraft = null) {
  if (!local && legacyDraft) {
    if (!published) return createPriceTableBatch({ draft: legacyDraft })
    local = clonePriceTable(published)
    const table = local.tables.find(row => row.key === local.active_table_key) || local.tables[0]
    table.payload.draft = clonePriceTable(legacyDraft)
    delete table.source_publication
  }
  if (!local) return published ? clonePriceTable(published) : null
  const batch = clonePriceTable(local)
  // Old browser-only keys predate published table identities. Reassociate only
  // an unambiguous table, keep its whole payload, and require a user decision.
  for (const table of batch.tables) {
    if (table.source_publication || table.is_new_table || published?.tables.some(row => row.key === table.key)) continue
    const candidates = (published?.tables || []).filter(row => !batch.tables.some(localRow => localRow.key === row.key))
    const matches = candidates.filter(row => row.name === table.name)
    const match = matches.length === 1 ? matches[0] : batch.tables.length === 1 && candidates.length === 1 ? candidates[0] : null
    if (!match) continue
    const oldKey = table.key
    table.key = match.key
    if (batch.active_table_key === oldKey) batch.active_table_key = match.key
    if (batch.default_table_key === oldKey) batch.default_table_key = match.key
  }
  for (const table of published?.tables || []) {
    if (!batch.tables.some(row => row.key === table.key) && !batch.removed_table_keys?.includes(table.key)) batch.tables.push(clonePriceTable(table))
  }
  return batch
}

export function editorTableConflict(local, published) {
  if (!local || !published) return false
  const expected = published.source_publication
  return local.source_publication?.id !== expected.id && local.acknowledged_publication_id !== expected.id
}

function backupInIndexedDB(indexedDB, key, value) {
  return new Promise((resolve, reject) => {
    const request = indexedDB.open('kferp-price-table-backups', 1)
    let blocked = false
    request.onupgradeneeded = () => {
      if (!request.result.objectStoreNames.contains('backups')) request.result.createObjectStore('backups')
    }
    request.onerror = () => reject(request.error)
    request.onblocked = () => { blocked = true; reject(new Error('备份存储被占用')) }
    request.onsuccess = () => {
      const db = request.result
      if (blocked) { db.close(); return }
      db.onversionchange = () => db.close()
      try {
        const transaction = db.transaction('backups', 'readwrite')
        transaction.oncomplete = () => { db.close(); resolve() }
        transaction.onabort = transaction.onerror = () => { db.close(); reject(transaction.error) }
        transaction.objectStore('backups').put(value, key)
      } catch (err) { db.close(); reject(err) }
    }
  })
}

export async function resolveEditorTableConflict(batch, published, choice, { scope = '', storage = globalThis.window?.localStorage, indexedDB = globalThis.indexedDB } = {}) {
  const next = clonePriceTable(batch)
  const index = next.tables.findIndex(table => table.key === published.key)
  if (index < 0) throw new Error('当前价格表已变化，请重试')
  if (choice === 'published') {
    if (!storage) throw new Error('无法备份本地草稿，请检查浏览器存储')
    const key = 'kferp:price-table-backup:' + scope + ':' + published.key + ':' + Date.now()
    const value = JSON.stringify(next.tables[index])
    try { storage.setItem(key, value) } catch (err) {
      if (!['QuotaExceededError', 'NS_ERROR_DOM_QUOTA_REACHED'].includes(err?.name) && ![22, 1014].includes(err?.code)) throw err
      try {
        if (!indexedDB) throw err
        // Wait for the transaction to commit before replacing the only draft.
        await backupInIndexedDB(indexedDB, key, value)
      } catch (cause) {
        throw new Error('无法备份本地草稿，未载入最新发布版；当前草稿已保留。请先保存草稿后重试。', { cause })
      }
    }
    next.tables[index] = clonePriceTable(published)
  } else if (choice === 'local') {
    next.tables[index].acknowledged_publication_id = published.source_publication.id
  } else throw new Error('请选择草稿处理方式')
  return next
}
