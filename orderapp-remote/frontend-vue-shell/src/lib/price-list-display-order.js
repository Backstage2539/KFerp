import { priceListParentProductID } from './product-price-list-selection.js'

const parentKey = row => `${row.group_id || 0}:${row.parent_group_item_id || 0}`
const childKey = row => `${row.group_id || 0}:${row.group_item_id || 0}`
function ranked(rows, keys, identity) {
  const rank = new Map((Array.isArray(keys) ? keys : []).map((key, i) => [String(key), i]))
  return [...rows].sort((a, b) => (rank.get(String(identity(a))) ?? Infinity) - (rank.get(String(identity(b))) ?? Infinity))
}
export function applyPriceListDisplayOrder(rows = [], order = {}) {
  const byParent = new Map()
  const identities = new Set(rows.filter(r => r.group_item_id).map(childKey))
  for (const row of rows) {
    const key = row.parent_group_item_id && identities.has(parentKey(row)) ? parentKey(row) : 'root'
    byParent.set(key, [...(byParent.get(key) || []), row])
  }
  const out = [], seen = new Set()
  function visit(key) {
    const siblings = byParent.get(key) || []
    // Root nodes still use their actual group as the ordering boundary.
    const sorted = ranked(siblings, key === 'root' ? order.roots : order.categories?.[key], r => r.code)
    for (const row of sorted) {
      if (seen.has(row.code)) continue
      seen.add(row.code)
      out.push({ ...row, items: ranked(row.items || [], order.products?.[row.code], priceListParentProductID) })
      if (row.group_item_id) visit(childKey(row))
    }
  }
  visit('root')
  return out
}
function swap(keys, key, direction) {
  const i = keys.indexOf(key), next = i + direction
  if (i < 0 || next < 0 || next >= keys.length || ![-1, 1].includes(direction)) return null
  const result = [...keys]
  ;[result[i], result[next]] = [result[next], result[i]]
  return result
}
export function movePriceListCategory(rows, order, code, direction) {
  const sorted = applyPriceListDisplayOrder(rows, order)
  const row = sorted.find(r => r.code === code)
  if (!row) return order
  const siblings = sorted.filter(r => parentKey(r) === parentKey(row))
  const keys = swap(siblings.map(r => r.code), code, direction)
  if (!keys) return order
  if (!row.parent_group_item_id || !sorted.some(r => childKey(r) === parentKey(row))) {
    const roots = sorted.filter(r => !r.parent_group_item_id || !sorted.some(p => childKey(p) === parentKey(r))).map(r => r.code)
    let index = 0
    return { ...order, roots: roots.map(key => keys.includes(key) ? keys[index++] : key) }
  }
  return { ...order, categories: { ...order.categories, [parentKey(row)]: keys } }
}
export function movePriceListProduct(rows, order, code, productID, direction) {
  const row = applyPriceListDisplayOrder(rows, order).find(r => r.code === code)
  const keys = swap((row?.items || []).map(priceListParentProductID), Number(productID), direction)
  return keys ? { ...order, products: { ...order.products, [code]: keys } } : order
}

// Capture existing positions before reading refreshed source data. New identities
// have no rank and append to their own parent/category.
export function capturePriceListDisplayOrder(rows, order = {}) {
  const sorted = applyPriceListDisplayOrder(rows, order)
  const categories = { ...order.categories }, products = { ...order.products }
  for (const row of sorted) {
    categories[parentKey(row)] = sorted.filter(r => parentKey(r) === parentKey(row)).map(r => r.code)
    products[row.code] = (row.items || []).map(priceListParentProductID)
  }
  return { ...order, roots: sorted.filter(r => !r.parent_group_item_id || !sorted.some(p => childKey(p) === parentKey(r))).map(r => r.code), categories, products }
}
