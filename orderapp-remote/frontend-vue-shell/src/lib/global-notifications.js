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
