import assert from 'node:assert/strict'
import { test } from 'node:test'

import {
  PRICE_LIST_PAGE_PREFERENCES_KEY,
  readPriceListPagePreferences,
  resolvePriceListScopePreference,
  resolveProductTypePreference,
  writePriceListPagePreferences,
} from './price-list-page-preferences.js'

function memoryStorage(seed = {}) {
  const values = new Map(Object.entries(seed))
  return {
    getItem(key) { return values.has(key) ? values.get(key) : null },
    setItem(key, value) { values.set(key, String(value)) },
  }
}

test('price-list page preferences persist the selected owner and product type', () => {
  const storage = memoryStorage()
  writePriceListPagePreferences({ scope: 'customer:27', productTypeCategoryID: 91 }, storage)

  assert.deepEqual(readPriceListPagePreferences(storage), {
    scope: 'customer:27',
    productTypeCategoryID: 91,
  })
  assert.ok(storage.getItem(PRICE_LIST_PAGE_PREFERENCES_KEY))
})

test('price-list page preferences restore the second product-catalog template after reload', () => {
  const storage = memoryStorage()
  const options = [
    { id: -2000128, label: '商品咖啡豆模板' },
    { id: -2000129, label: '商品挂耳模板' },
  ]

  writePriceListPagePreferences({ scope: 'official', productTypeCategoryID: options[1].id }, storage)
  const reloaded = readPriceListPagePreferences(storage)

  assert.equal(reloaded.productTypeCategoryID, options[1].id)
  assert.equal(resolveProductTypePreference(reloaded.productTypeCategoryID, options), options[1].id)
  assert.equal(resolveProductTypePreference(-2000130, options), options[0].id)
})

test('price-list page preferences ignore malformed browser values', () => {
  const storage = memoryStorage({
    [PRICE_LIST_PAGE_PREFERENCES_KEY]: JSON.stringify({ scope: 'customer:bad', productTypeCategoryID: -4 }),
  })
  assert.deepEqual(readPriceListPagePreferences(storage), {
    scope: 'official',
    productTypeCategoryID: 0,
  })
})

test('remembered owner and product type fall back when no longer available', () => {
  assert.equal(resolvePriceListScopePreference('customer:27', [{ id: 27 }]), 'customer:27')
  assert.equal(resolvePriceListScopePreference('customer:27', [{ id: 28 }]), 'official')
  assert.equal(resolveProductTypePreference(91, [{ id: 90 }, { id: 91 }]), 91)
  assert.equal(resolveProductTypePreference(91, [{ id: 90 }]), 90)
  assert.equal(resolveProductTypePreference(91, []), 91)
})
