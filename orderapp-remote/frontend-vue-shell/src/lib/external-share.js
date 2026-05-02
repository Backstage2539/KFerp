export function buildShareResourcePayload(resourceType, orderID, options = {}) {
  const payload = {
    resource_type: String(resourceType || ''),
    order_id: Number(orderID || 0),
    latest: options.latest !== false,
  }
  const documentID = Number(options.documentID || options.document_id || 0)
  if (documentID > 0) {
    payload.document_id = documentID
  }
  return payload
}

export function absoluteShareURL(rawURL, origin = defaultOrigin()) {
  const value = String(rawURL || '').trim()
  if (!value) return ''
  try {
    return new URL(value, origin).toString()
  } catch {
    return value
  }
}

export async function shareResourceToWechat(resource = {}, env = {}) {
  const origin = env.origin || defaultOrigin()
  const nav = env.navigator || globalThis.navigator || {}
  const shareURL = absoluteShareURL(resource.share_url || resource.file_url, origin)
  const title = resource.title || resource.filename || '分享资源'
  const text = normalizeShareText(resource.share_text, title, shareURL)

  if (shareURL && typeof nav.share === 'function') {
    await nav.share({ title, text, url: shareURL })
    return 'shared'
  }
  if (shareURL && nav.clipboard && typeof nav.clipboard.writeText === 'function') {
    await nav.clipboard.writeText(shareURL)
    return 'copied'
  }
  return 'manual'
}

function normalizeShareText(raw, title, shareURL) {
  const value = String(raw || '').trim()
  if (!value) return shareURL ? `${title}\n${shareURL}` : title
  if (shareURL && value.includes('/share/')) {
    return value.replace(/(^|\s)(\/share\/[^\s]+)/g, `$1${shareURL}`)
  }
  return value
}

function defaultOrigin() {
  if (globalThis.location?.origin) return globalThis.location.origin
  return ''
}
