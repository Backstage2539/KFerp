export function orderConfirmationLabel(status) {
  return ({ pending: '待确认', accepted: '已接单', rejected: '已拒绝', superseded: '已更新' })[status] || '已接单'
}
// Refresh only read models. Callers must keep unsaved editor state separate.
export function visibleOrderRefresh(refresh, env = globalThis) {
  let running = false, stopped = false
  const tick = async () => {
    if (env.document.hidden || running || stopped) return
    running = true
    try { await refresh() } catch { /* Preserve the last successful read; the page's manual refresh reports errors. */ }
    finally { running = false }
  }
  const timer = env.setInterval(tick, 15000)
  env.document.addEventListener('visibilitychange', tick)
  return () => { stopped = true; env.clearInterval(timer); env.document.removeEventListener('visibilitychange', tick) }
}
