import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'
import {
  clampNotificationWindowStart,
  dedupeNotifications,
  filterDismissedNotifications,
  notificationBackendIDs,
  notificationWindow,
} from './global-notifications.js'

function appSource() {
  return readFileSync(new URL('../App.vue', import.meta.url), 'utf8')
}

function sourceAfter(source, marker) {
  const index = source.indexOf(marker)
  return index >= 0 ? source.slice(index) : ''
}

test('global notifications render as a stack instead of a single active banner', () => {
  const source = appSource()

  assert.match(source, /<div[^>]*v-if="allNotifications\.length"[^>]*class="global-notification-stack"/s)
  assert.match(source, /class="notification-window-toolbar"/)
  assert.match(source, /aria-label="上一条通知"/)
  assert.match(source, /aria-label="下一条通知"/)
  assert.match(source, /@click="clearAllNotifications"/)
  assert.match(source, /v-for="\(\s*item,\s*idx\s*\)\s+in\s+visibleNotifications"/)
  assert.doesNotMatch(source, /v-if="activeNotification"/)
  assert.doesNotMatch(source, /activeNotification\s*=\s*computed/)
})

test('mobile notification stack reserves space for page-level error toasts', () => {
  const source = appSource()
  const mobileStyles = sourceAfter(source, '@media (max-width: 900px)')

  assert.match(source, /import \{[^}]*notificationWindow[^}]*\} from '\.\/lib\/global-notifications\.js'/s)
  assert.match(source, /const allNotifications = computed\(\(\) => dedupeNotifications/)
  assert.match(source, /const visibleNotifications = computed\(\(\) => notificationWindow\(allNotifications\.value,\s*notificationWindowStart\.value,\s*notificationWindowSize\)\)/)
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

test('notification window keeps a three item viewport while allowing later notices', () => {
  const rows = [
    { id: 1, title: '新订单 SO-001' },
    { id: 2, title: '新订单 SO-002' },
    { id: 3, title: '新订单 SO-003' },
    { id: 4, title: '新订单 SO-004' },
    { id: 5, title: '新订单 SO-005' },
  ]

  assert.deepEqual(notificationWindow(rows, 0, 3).map((row) => row.id), [1, 2, 3])
  assert.deepEqual(notificationWindow(rows, 2, 3).map((row) => row.id), [3, 4, 5])
  assert.deepEqual(notificationWindow(rows, 99, 3).map((row) => row.id), [3, 4, 5])
  assert.equal(clampNotificationWindowStart(99, rows.length, 3), 2)
})

test('bulk clearing only sends backend notification ids once', () => {
  const rows = [
    { id: 11, title: '新订单 SO-001' },
    { id: 11, title: '重复投递 SO-001' },
    { id: 'local-1', local_notice: true, title: '本地提示' },
    { id: 12, title: '新订单 SO-002' },
    { id: 0, title: '无后端 ID' },
  ]

  assert.deepEqual(notificationBackendIDs(rows), [11, 12])
})

test('closing a backend notification records the dismissal before server read sync', () => {
  const source = appSource()
  const dismissSource = sourceAfter(source, 'async function dismissNotification(item)')

  assert.match(source, /const dismissedNotificationIDs = ref/)
  assert.match(dismissSource, /rememberDismissedNotification\(item\)[\s\S]*markNotificationRead\(item\.id\)/)
  assert.match(dismissSource, /notifications\.value = filterDismissedNotifications\(notifications\.value,\s*dismissedNotificationIDs\.value\)/)
  assert.doesNotMatch(dismissSource, /The next poll will reconcile read state/)
})

test('clearing notification center dismisses every visible backend notice before read sync', () => {
  const source = appSource()
  const clearSource = sourceAfter(source, 'async function clearAllNotifications()')

  assert.match(source, /const notificationFetchLimit = 100/)
  assert.match(source, /fetchERPNotifications\(notificationFetchLimit\)/)
  assert.match(clearSource, /notificationBackendIDs\(allNotifications\.value\)/)
  assert.match(clearSource, /rememberDismissedNotificationIDs\(ids\)/)
  assert.match(clearSource, /localNotifications\.value = \[\]/)
  assert.match(clearSource, /notifications\.value = \[\]/)
  assert.match(clearSource, /Promise\.allSettled\(ids\.map\(\(id\) => markNotificationRead\(id\)\)\)/)
})
