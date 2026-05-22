import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

import {
  clearFormDraft,
  FORM_DRAFT_SCOPES,
  hasFormDraft,
  readFormDraft,
  saveFormDraft,
} from './form-draft-cache.js'

test('form draft cache keeps in-memory cloned drafts until the page is refreshed', () => {
  const key = `${FORM_DRAFT_SCOPES.orderEntry}:factory:new`
  clearFormDraft(key)

  const draft = {
    form: { customer_id: 7, notes: '发票随货' },
    rows: [{ product_id: 9, product_query: '曲奇拼配', qty: 2 }],
  }
  saveFormDraft(key, draft)
  draft.form.notes = 'mutated after save'
  draft.rows[0].qty = 99

  assert.equal(hasFormDraft(key), true)
  assert.deepEqual(readFormDraft(key), {
    form: { customer_id: 7, notes: '发票随货' },
    rows: [{ product_id: 9, product_query: '曲奇拼配', qty: 2 }],
  })

  const restored = readFormDraft(key)
  restored.form.notes = 'mutated after read'
  assert.equal(readFormDraft(key).form.notes, '发票随货')

  clearFormDraft(key)
  assert.equal(hasFormDraft(key), false)
})

test('form draft cache does not persist drafts through browser storage', () => {
  const source = readFileSync(new URL('./form-draft-cache.js', import.meta.url), 'utf8')

  assert.match(source, /const drafts = new Map\(\)/)
  assert.doesNotMatch(source, /localStorage|sessionStorage/)
})

test('order entry, BOM, and SKU settings wire form draft cache to component unmount and mount', () => {
  const orderEntry = readFileSync(new URL('../views/OrderEntryView.vue', import.meta.url), 'utf8')
  const bom = readFileSync(new URL('../views/BomView.vue', import.meta.url), 'utf8')
  const productSettings = readFileSync(new URL('../views/ProductSettingsView.vue', import.meta.url), 'utf8')

  for (const marker of [
    'ORDER_ENTRY_DRAFT_SCOPE',
    'orderEntryDraftKey',
    'saveOrderEntryDraft',
    'restoreOrderEntryDraft',
    'onBeforeUnmount(saveOrderEntryDraft)',
    'clearFormDraft(orderEntryDraftKey())',
  ]) {
    assert.ok(orderEntry.includes(marker), `OrderEntryView.vue should include ${marker}`)
  }

  for (const marker of [
    'BOM_FORM_DRAFT_SCOPE',
    'bomFormDraftKey',
    'saveBomFormDraft',
    'restoreBomFormDraft',
    'onBeforeUnmount(saveBomFormDraft)',
  ]) {
    assert.ok(bom.includes(marker), `BomView.vue should include ${marker}`)
  }

  for (const marker of [
    'SKU_SETTINGS_FORM_DRAFT_SCOPE',
    'productSettingsDraftKey',
    'saveProductSettingsDraft',
    'restoreProductSettingsDraft',
    'restoringProductSettingsDraft',
    'onBeforeUnmount(saveProductSettingsDraft)',
  ]) {
    assert.ok(productSettings.includes(marker), `ProductSettingsView.vue should include ${marker}`)
  }
})
