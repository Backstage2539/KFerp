import test from 'node:test'
import assert from 'node:assert/strict'
import { readFile } from 'node:fs/promises'
import { createShellUpdateMonitor } from './shell-update-monitor.js'

function fixture() {
  let tick, visibilityHandler, timerCleared = false, calls = 0, noticed = 0
  let html = '<script type="module" src="/vue-shell/assets/main-old.js"></script>'
  let status = 200
  const document = {
    visibilityState: 'visible',
    querySelector: () => ({ src: 'https://erp.test/vue-shell/assets/main-old.js' }),
    addEventListener: (_, cb) => { visibilityHandler = cb },
    removeEventListener: (_, cb) => { if (visibilityHandler === cb) visibilityHandler = null },
  }
  const monitor = createShellUpdateMonitor({
    document, location: { pathname: '/app/vue-shell', origin: 'https://erp.test' },
    fetch: async (url, opts) => { calls++; assert.equal(url, '/app/vue-shell'); assert.equal(opts.cache, 'no-store'); return { ok: status === 200, text: async () => html } },
    setInterval: (cb, ms) => { tick = cb; assert.equal(ms, 60000); return 7 },
    clearInterval: id => { assert.equal(id, 7); timerCleared = true },
    onUpdate: () => { noticed++ },
  })
  return { monitor, document, tick: () => tick(), visible: () => visibilityHandler?.(), setHtml: h => { html = h }, setStatus: s => { status = s }, calls: () => calls, noticed: () => noticed, cleaned: () => timerCleared && !visibilityHandler }
}

test('detects a new shell without refreshing, and never treats a failed or login response as an update', async () => {
  const f = fixture(); await f.monitor.start(); await f.monitor.check()
  assert.equal(f.noticed(), 0)
  f.setStatus(401); await f.monitor.check(); assert.equal(f.noticed(), 0)
  f.setStatus(200); f.setHtml('<html>登录</html>'); await f.monitor.check(); assert.equal(f.noticed(), 0)
  f.setHtml('<script type="module" src="/vue-shell/assets/main-new.js"></script>')
  await f.monitor.check(); assert.equal(f.noticed(), 1)
  await f.monitor.check(); assert.equal(f.noticed(), 1)
  f.monitor.stop(); assert.equal(f.cleaned(), true)
})

test('polls only while visible and checks when returning to the tab', async () => {
  const f = fixture(); await f.monitor.start(); const before = f.calls()
  f.document.visibilityState = 'hidden'; await f.tick(); assert.equal(f.calls(), before)
  f.document.visibilityState = 'visible'; await f.visible(); assert.equal(f.calls(), before + 1)
  f.monitor.stop()
})

test('shell displays a non-destructive update reminder', async () => {
  const app = await readFile(new URL('../App.vue', import.meta.url), 'utf8')
  assert.match(app, /createShellUpdateMonitor/)
  assert.match(app, /系统已更新/)
  assert.match(app, /保存.*刷新/)
  assert.doesNotMatch(app, /location\.reload\(/)
})

test('network failure is ignored and an in-flight response after stop cannot notify', async () => {
  let resolveResponse, notifications = 0
  const document = { visibilityState: 'visible', querySelector: () => ({ src: 'https://erp.test/vue-shell/assets/main-old.js' }), addEventListener() {}, removeEventListener() {} }
  let fail = true
  const monitor = createShellUpdateMonitor({ document, location: { pathname: '/vue-shell', origin: 'https://erp.test' },
    fetch: () => fail ? Promise.reject(new Error('offline')) : new Promise(resolve => { resolveResponse = resolve }),
    setInterval: () => 1, clearInterval() {}, onUpdate: () => { notifications++ },
  })
  await monitor.start()
  fail = false
  const check = monitor.check()
  monitor.stop()
  resolveResponse({ ok: true, text: async () => '<script type="module" src="/vue-shell/assets/main-new.js"></script>' })
  await check
  assert.equal(notifications, 0)
})
test('ignores unrelated non-module scripts when determining the current build', async () => {
 const f=fixture();await f.monitor.start()
 f.setHtml('<script src="/vue-shell/assets/main-unrelated.js"></script><script type="module" src="/vue-shell/assets/main-old.js"></script>')
 await f.monitor.check();assert.equal(f.noticed(),0)
 f.setHtml('<script src="/vue-shell/assets/main-unrelated.js"></script><script type="module" src="/vue-shell/assets/main-new.js"></script>')
 await f.monitor.check();assert.equal(f.noticed(),1);f.monitor.stop()
})
