import test from 'node:test'
import assert from 'node:assert/strict'
import { createPriceTableBatch, savePriceTableBatchDraft, readPriceTableBatchDraft } from './price-table-batch.js'

const tick = () => new Promise(resolve => setImmediate(resolve))
const quota = () => new DOMException('Setting the value exceeded the quota.', 'QuotaExceededError')
const scope = 'kferp:price-list-generation:factory:official:all:product-catalog:56'

// Model request success separately from transaction completion: a successful
// put is not yet durable, and the editor must not announce a saved draft early.
function draftDatabase({ autoCommit = true } = {}) {
  const stores = new Map(), transactions = []
  const db = {
    objectStoreNames: { contains: name => stores.has(name) },
    createObjectStore(name) { stores.set(name, new Map()) },
    close() {},
    transaction(name, mode) {
      if (!stores.has(name)) throw new Error('missing store: ' + name)
      const changes = [], tx = {
        mode,
        commit() {
          for (const change of changes) change()
          tx.oncomplete?.()
        },
        objectStore() {
          return {
            put(value, key) {
              changes.push(() => stores.get(name).set(key, structuredClone(value)))
              const request = {}
              queueMicrotask(() => request.onsuccess?.())
              return request
            },
            get(key) {
              const request = {}
              queueMicrotask(() => {
                request.result = structuredClone(stores.get(name).get(key))
                request.onsuccess?.()
              })
              return request
            },
            delete(key) {
              changes.push(() => stores.get(name).delete(key))
              const request = {}
              queueMicrotask(() => request.onsuccess?.())
              return request
            },
          }
        },
      }
      transactions.push(tx)
      if (autoCommit || mode === 'readonly') setImmediate(() => tx.commit())
      return tx
    },
  }
  return {
    transactions,
    indexedDB: { open() {
      const request = { result: db }
      queueMicrotask(() => { request.onupgradeneeded?.(); request.onsuccess?.() })
      return request
    } },
  }
}

function browserStorage() {
  const values = new Map()
  return { values, full: false,
    getItem(key) { return values.get(key) ?? null },
    setItem(key, value) { if (this.full) throw quota(); values.set(key, value) },
    removeItem(key) { values.delete(key) },
  }
}

test('a full browser cache saves the complete named batch durably and restores it after reload', async () => {
  const storage = browserStorage(), database = draftDatabase({ autoCommit: false })
  const stale = createPriceTableBatch({ config: { version: 'V3.0.66' } }, '零售价格表')
  storage.setItem(`kferp:named-price-tables:${scope}`, JSON.stringify(stale))
  assert.deepEqual(await readPriceTableBatchDraft(scope, storage, database.indexedDB), stale)
  const batch = structuredClone(stale)
  batch.tables[0].payload = { draft: { productOverrides: { 'sku:309': { fixed_unit_price: 38.5 } }, product_spec_selections: [] } }
  batch.tables[0].source_publication = { id: 152, version: 'V3.0.67' }
  storage.full = true
  let settled = false
  const save = Promise.resolve().then(() => savePriceTableBatchDraft(scope, batch, storage, database.indexedDB))
  save.then(() => { settled = true }, () => { settled = true })
  await tick()
  assert.equal(settled, false, 'must wait for fallback transaction commit')
  const write = database.transactions.find(tx => tx.mode === 'readwrite')
  assert.ok(write, 'full localStorage must use durable fallback')
  write.commit()
  await save
  assert.deepEqual(await readPriceTableBatchDraft(scope, storage, database.indexedDB), batch)
  assert.equal(await readPriceTableBatchDraft(scope + ':other-customer', storage, database.indexedDB), null)
})

test('new batch saves do not consume the small legacy browser cache', async () => {
  const storage = browserStorage(), database = draftDatabase()
  const batch = createPriceTableBatch({ draft: { marker: 'durable' } })
  await savePriceTableBatchDraft(scope, batch, storage, database.indexedDB)
  assert.equal(storage.values.size, 0)
  assert.deepEqual(await readPriceTableBatchDraft(scope, storage, database.indexedDB), batch)
})

test('repeated durable saves retain the newest draft after cache space becomes available', async () => {
  const storage = browserStorage(), database = draftDatabase()
  const batch = createPriceTableBatch({ draft: { marker: 'fallback' } })
  storage.full = true
  await savePriceTableBatchDraft(scope, batch, storage, database.indexedDB)
  storage.full = false
  batch.tables[0].payload.draft.marker = 'newer-local-save'
  await savePriceTableBatchDraft(scope, batch, storage, database.indexedDB)
  assert.deepEqual(await readPriceTableBatchDraft(scope, storage, database.indexedDB), batch)
})

test('overlapping edits save snapshots in order and reload waits for the final transaction', async () => {
  const storage = browserStorage(), database = draftDatabase({ autoCommit: false })
  const batch = createPriceTableBatch({ draft: { marker: 'first' } })
  const first = savePriceTableBatchDraft(scope, batch, storage, database.indexedDB)
  batch.tables[0].payload.draft.marker = 'second'
  const second = savePriceTableBatchDraft(scope, batch, storage, database.indexedDB)
  batch.tables[0].payload.draft.marker = 'not-saved'
  await tick()
  assert.equal(database.transactions.length, 1)
  database.transactions[0].commit()
  await first
  await tick()
  assert.equal(database.transactions.length, 2)
  let readSettled = false
  const read = readPriceTableBatchDraft(scope, storage, database.indexedDB)
  read.then(() => { readSettled = true })
  await tick()
  assert.equal(readSettled, false)
  database.transactions[1].commit()
  await second
  assert.equal((await read).tables[0].payload.draft.marker, 'second')
})

test('a failed durable write retains the legacy draft without claiming it was saved', async () => {
  const storage = browserStorage(), database = draftDatabase({ autoCommit: false })
  const original = createPriceTableBatch({ draft: { marker: 'retained' } })
  storage.setItem(`kferp:named-price-tables:${scope}`, JSON.stringify(original))
  const changed = structuredClone(original)
  changed.tables[0].payload.draft.marker = 'edited'
  const save = savePriceTableBatchDraft(scope, changed, storage, database.indexedDB)
  const rejected = assert.rejects(save, /本地草稿保存失败/)
  await tick()
  database.transactions[0].error = quota()
  database.transactions[0].onabort()
  await rejected
  assert.deepEqual(await readPriceTableBatchDraft(scope, storage, database.indexedDB), original)
})

test('fallback failure keeps the existing browser draft and reports a readable saving error', async () => {
  const storage = browserStorage()
  const original = createPriceTableBatch({ draft: { marker: 'retained' } })
  await savePriceTableBatchDraft(scope, original, storage, null)
  storage.full = true
  const changed = structuredClone(original)
  changed.tables[0].payload.draft.marker = 'edited'
  await assert.rejects(Promise.resolve().then(() => savePriceTableBatchDraft(scope, changed, storage, null)),
    err => err.name !== 'QuotaExceededError' && /草稿|保存|存储/.test(err.message))
  assert.deepEqual(await readPriceTableBatchDraft(scope, storage, null), original)
})
