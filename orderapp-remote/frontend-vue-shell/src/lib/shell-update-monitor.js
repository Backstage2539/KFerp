const entryPattern = /(?:^|\/)vue-shell\/assets\/(main-[A-Za-z0-9_-]+\.js)(?:[?#]|$)/

function entryName(value, origin) {
  try { return new URL(value, origin).pathname.match(entryPattern)?.[1] || '' } catch { return '' }
}

function entryFromHtml(html, origin) {
  const scripts = html.matchAll(/<script\b([^>]*)>/gi)
  for (const script of scripts) {
    if (!/\btype\s*=\s*["']module["']/i.test(script[1])) continue
    const src = script[1].match(/\bsrc\s*=\s*["']([^"']+)["']/i)?.[1] || ''
    const entry = entryName(src, origin)
    if (entry) return entry
  }
  return ''
}

// A new bundle requires the user to save their work before they choose to reload.
export function createShellUpdateMonitor({ document, location, fetch, setInterval, clearInterval, onUpdate }) {
  const current = entryName(document.querySelector('script[type="module"][src]')?.src || '', location.origin)
  const shellPath = `${location.pathname === '/app' || location.pathname.startsWith('/app/') ? '/app' : ''}/vue-shell`
  let timer = null
  let stopped = false
  let notified = false
  let inflight = null

  function check() {
    if (stopped || notified || !current || document.visibilityState === 'hidden') return Promise.resolve()
    if (inflight) return inflight
    inflight = (async () => {
      try {
        const response = await fetch(shellPath, { cache: 'no-store', credentials: 'same-origin' })
        if (!response.ok) return
        const next = entryFromHtml(await response.text(), location.origin)
        if (!stopped && next && next !== current && !notified) {
          notified = true
          onUpdate()
        }
      } catch {
        // An offline or failed check must not interrupt the user's work.
      } finally {
        inflight = null
      }
    })()
    return inflight
  }

  function onVisibilityChange() { if (document.visibilityState === 'visible') return check() }
  return {
    start() {
      if (timer !== null) return Promise.resolve()
      stopped = false
      document.addEventListener('visibilitychange', onVisibilityChange)
      timer = setInterval(check, 60000)
      return check()
    },
    stop() {
      stopped = true
      if (timer !== null) clearInterval(timer)
      timer = null
      document.removeEventListener('visibilitychange', onVisibilityChange)
    },
    check,
  }
}
