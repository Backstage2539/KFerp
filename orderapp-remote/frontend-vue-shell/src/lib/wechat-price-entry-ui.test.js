import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { test } from 'node:test'

const view = readFileSync(new URL('../views/UISettingsView.vue', import.meta.url), 'utf8')
const portal = readFileSync(new URL('../views/CustomerPortalSettingsView.vue', import.meta.url), 'utf8')
const settings = readFileSync(new URL('../components/WechatOfficialSettings.vue', import.meta.url), 'utf8')
const menu = readFileSync(new URL('../components/WechatMenuButton.vue', import.meta.url), 'utf8')
const costing = readFileSync(new URL('../views/CostingView.vue', import.meta.url), 'utf8')

test('official account management is a system settings tab and is removed from customer portal', () => {
  assert.match(view, /公众号管理/)
  assert.match(view, /WechatOfficialSettings/)
  assert.match(view, /props\.viewParams\?\.tab/)
  assert.doesNotMatch(portal, /WechatOfficialSettings/)
})

test('menu target choices refresh when returning from price-entry settings', () => {
  assert.match(settings, /async function selectTab\(key\)/)
  assert.match(settings, /if \(key === 'menu'\) await run\(loadMenuEntries\)/)
  assert.match(settings, /@click="selectTab\(key\)"/)
})

test('restoring a historical menu refreshes current price-entry targets first', () => {
  const restoreStart = settings.indexOf('async function restore(row)')
  const restoreEnd = settings.indexOf('\nfunction openManual()', restoreStart)
  assert.ok(restoreStart >= 0 && restoreEnd > restoreStart, 'async restore handler not found')
  const restore = settings.slice(restoreStart, restoreEnd)
  assert.match(restore, /await loadMenuEntries\(\)/)
  assert.ok(restore.indexOf('await loadMenuEntries()') < restore.indexOf("tab.value = 'menu'"))
})

test('menu offers manual published pages and preserves imported destinations', () => {
  assert.match(settings, /entry\.enabled && entry\.published && !entry\.deleted/)
  assert.match(settings, /\/api\/admin\/page-entries/)
  assert.match(menu, /选择页面入口/)
  assert.match(menu, /保留已导入的/)
})
test('price table shortcut prefills a new manually created page', () => {
  assert.match(costing, /新建页面入口/)
  assert.match(costing, /publication_id:publicationId/)
  assert.match(view, /PageEntryManager/)
})
