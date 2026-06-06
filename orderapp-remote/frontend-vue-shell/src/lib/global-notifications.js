export function notificationDedupeKey(item = {}) {
  const id = Number(item.id || 0)
  if (id > 0) return `id:${id}`
  return [
    item.event_type || '',
    item.source_type || '',
    Number(item.source_id || 0),
    item.title || '',
  ].join('\u0000')
}

export function dedupeNotifications(rows = []) {
  const seen = new Set()
  const out = []
  for (const row of rows || []) {
    const key = notificationDedupeKey(row)
    if (seen.has(key)) continue
    seen.add(key)
    out.push(row)
  }
  return out
}

export function filterDismissedNotifications(rows = [], dismissedIDs = []) {
  const dismissed = new Set((dismissedIDs || [])
    .map((id) => Number(id || 0))
    .filter((id) => id > 0))
  if (!dismissed.size) return [...(rows || [])]
  return (rows || []).filter((row) => !dismissed.has(Number(row?.id || 0)))
}

export function clampNotificationWindowStart(start = 0, total = 0, size = 3) {
  const totalCount = Math.max(0, Number(total || 0))
  const windowSize = Math.max(1, Number(size || 1))
  const maxStart = Math.max(0, totalCount - windowSize)
  const next = Math.trunc(Number(start || 0))
  if (!Number.isFinite(next) || next <= 0) return 0
  return Math.min(next, maxStart)
}

export function notificationWindow(rows = [], start = 0, size = 3) {
  const source = [...(rows || [])]
  const windowSize = Math.max(1, Number(size || 1))
  const safeStart = clampNotificationWindowStart(start, source.length, windowSize)
  return source.slice(safeStart, safeStart + windowSize)
}

export function notificationBackendIDs(rows = []) {
  const seen = new Set()
  const out = []
  for (const row of rows || []) {
    if (row?.local_notice) continue
    const id = Number(row?.id || 0)
    if (id <= 0 || seen.has(id)) continue
    seen.add(id)
    out.push(id)
  }
  return out
}
