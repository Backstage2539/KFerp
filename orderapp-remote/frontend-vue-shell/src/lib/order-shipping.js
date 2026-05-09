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
