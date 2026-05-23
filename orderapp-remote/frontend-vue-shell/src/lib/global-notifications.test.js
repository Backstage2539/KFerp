import test from 'node:test'
import assert from 'node:assert/strict'
import { readFileSync } from 'node:fs'

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

  assert.match(source, /const visibleNotifications = computed\(\(\) => notifications\.value\.slice\(0,\s*3\)\)/)
  assert.match(source, /ref="notificationStack"/)
  assert.match(source, /notificationStackSpace/)
  assert.match(source, /getBoundingClientRect\(\)\.bottom/)
  assert.match(source, /const notificationStackStyle = computed/)
  assert.match(source, /'--kferp-notice-stack-space'/)
  assert.match(mobileStyles, /\.global-notification-stack\s*\{[^}]*position:\s*relative/s)
  assert.match(mobileStyles, /\.global-notification-stack\s+\.global-notification\s*\+ \.global-notification\s*\{[^}]*margin-top:\s*-10px/s)
})
