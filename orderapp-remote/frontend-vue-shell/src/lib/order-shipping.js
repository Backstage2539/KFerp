export function normalizeTrackingInput(raw) {
  return String(raw || '')
    .split(/[\s,;，；、]+/u)
    .map((item) => item.trim())
    .filter(Boolean)
    .filter((item, idx, arr) => arr.indexOf(item) === idx)
}

export function trackingInputSummary(raw) {
  return normalizeTrackingInput(raw).join('\n')
}

export function formatTrackingSummary(raw) {
  const numbers = normalizeTrackingInput(raw)
  if (!numbers.length) return '未回填'
  if (numbers.length === 1) return numbers[0]
  return `${numbers[0]} 等 ${numbers.length} 个单号`
}

export function isOrderShipReady(row = {}) {
  if (row?.is_void) return false
  const shipStatus = String(row?.ship_status || '').trim()
  if (shipStatus.includes('已发货')) return false
  const processStatus = String(row?.process_status || '').trim()
  return processStatus.includes('生产完成') || processStatus === '无需生产' || processStatus === '库存待发货'
}
