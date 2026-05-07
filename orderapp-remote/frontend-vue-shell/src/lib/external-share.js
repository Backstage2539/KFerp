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
  const fetchFile = env.fetch || globalThis.fetch
  const FileCtor = env.File || globalThis.File
  const fileURL = absoluteShareURL(resource.file_url, origin)
  const title = resource.title || resource.filename || '分享资源'

  if (!fileURL || typeof fetchFile !== 'function' || typeof FileCtor !== 'function' || typeof nav.share !== 'function') {
    return 'unsupported'
  }

  const response = await fetchFile(fileURL)
  if (!response?.ok || typeof response.blob !== 'function') {
    throw new Error('share file download failed')
  }
  const blob = await response.blob()
  const fileName = normalizeFileName(resource.filename, resource.resource_type)
  const fileType = resource.content_type || blob.type || 'application/octet-stream'
  const file = new FileCtor([blob], fileName, { type: fileType })
  const sharePayload = { title, text: title, files: [file] }

  if (typeof nav.canShare === 'function' && !nav.canShare({ files: [file] })) {
    return 'unsupported'
  }

  try {
    await nav.share(sharePayload)
  } catch (err) {
    if (isUnsupportedShareError(err)) return 'unsupported'
    throw err
  }
  return 'file-shared'
}

function normalizeFileName(raw, resourceType) {
  const value = String(raw || '').trim()
  if (value) return value
  const suffix = String(resourceType || '').includes('image') ? 'png' : 'pdf'
  return `share-resource.${suffix}`
}

function isUnsupportedShareError(err) {
  const name = String(err?.name || '')
  const message = String(err?.message || '')
  return name === 'TypeError' || /file|share|unsupported/i.test(message)
}

function defaultOrigin() {
  if (globalThis.location?.origin) return globalThis.location.origin
  return ''
}
