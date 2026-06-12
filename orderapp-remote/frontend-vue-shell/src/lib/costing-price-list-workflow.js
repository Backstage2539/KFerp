export function priceListPricingRuleTrialRequestsForRows(sourceRows = [], options = {}) {
  const {
    customerID = 0,
    cache = {},
    payloadForRow,
    cacheKeyForPayload,
  } = options
  if (typeof payloadForRow !== 'function' || typeof cacheKeyForPayload !== 'function') return []

  const requests = []
  const seen = new Set()
  ;(Array.isArray(sourceRows) ? sourceRows : []).forEach((row) => {
    const payload = payloadForRow(row, { customerID })
    if (!payload) return
    const key = cacheKeyForPayload(payload)
    if (!key || seen.has(key)) return
    const cached = cache[key]
    if (cached?.status === 'loading' || cached?.status === 'success' || cached?.status === 'error') return
    seen.add(key)
    requests.push({ row, key, payload })
  })
  return requests
}
