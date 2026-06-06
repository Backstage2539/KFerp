import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import { dedupeNotifications, filterDismissedNotifications } from './global-notifications.js'

function appSource() {
  return readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
}

function sourceAfter(source, marker) {
  const index = source.indexOf(marker)
  return index >= 0 ? source.slice(index) : ''
}

test('global notifications render as a stack instead of a single active banner', () => {
  const source = appSource()

  assert.match(source, /<div[^>]*v-if="visibleNotifications\.length"[^>]*class="global-notification-stack"/s)
  assert.match(source, /v-for="\(\s*item,\s*idx\s*\)\s+in\s+visibleNotifications"/)
  assert.doesNotMatch(source, /v-if="activeNotification"/)
  assert.doesNotMatch(source, /activeNotification\s*=\s*computed/)
})

test('mobile notification stack reserves space for page-level error toasts', () => {
  const source = appSource()
  const mobileStyles = sourceAfter(source, '@media (max-width: 900px)')

  assert.match(source, /import \{[^}]*dedupeNotifications[^}]*filterDismissedNotifications[^}]*\} from '\.\/lib\/global-notifications\.js'/s)
  assert.match(source, /const visibleNotifications = computed\(\(\) => dedupeNotifications\(\[\.{3}localNotifications\.value,\s*\.{3}filterDismissedNotifications\(notifications\.value,\s*dismissedNotificationIDs\.value\)\]\)\.slice\(0,\s*3\)\)/)
  assert.match(source, /ref="notificationStack"/)
  assert.match(source, /notificationStackSpace/)
  assert.match(source, /getBoundingClientRect\(\)\.bottom/)
  assert.match(source, /const notificationStackStyle = computed/)
  assert.match(source, /'--kferp-notice-stack-space'/)
  assert.match(mobileStyles, /\.global-notification-stack\s*\{[^}]*position:\s*relative/s)
  assert.match(mobileStyles, /\.global-notification-stack\s+\.global-notification\s*\+ \.global-notification\s*\{[^}]*margin-top:\s*-10px/s)
})

test('global notifications collapse duplicate order-created events', () => {
  const rows = dedupeNotifications([
    { id: 11, event_type: 'order.created', source_type: 'order', source_id: 71, title: '新订单 SO-001' },
    { id: 11, event_type: 'order.created', source_type: 'order', source_id: 71, title: '新订单 SO-001' },
    { id: 12, event_type: 'order.created', source_type: 'order', source_id: 72, title: '新订单 SO-002' },
  ])

  assert.deepEqual(rows.map((row) => row.id), [11, 12])
})

test('dismissed backend notifications are hidden from later polling results', () => {
  const rows = filterDismissedNotifications([
    { id: 11, event_type: 'order.created', title: '新订单 SO-001' },
    { id: 12, event_type: 'order.created', title: '新订单 SO-002' },
  ], [11])

  assert.deepEqual(rows.map((row) => row.id), [12])
})

test('closing a backend notification records the dismissal before server read sync', () => {
  const source = appSource()
  const dismissSource = sourceAfter(source, 'async function dismissNotification(item)')

  assert.match(source, /const dismissedNotificationIDs = ref/)
  assert.match(dismissSource, /rememberDismissedNotification\(item\)[\s\S]*markNotificationRead\(item\.id\)/)
  assert.match(dismissSource, /notifications\.value = filterDismissedNotifications\(notifications\.value,\s*dismissedNotificationIDs\.value\)/)
  assert.doesNotMatch(dismissSource, /The next poll will reconcile read state/)
})
