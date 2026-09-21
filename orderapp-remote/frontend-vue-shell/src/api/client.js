async function readJson(res) {
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.error || data.message || '请求失败')
    err.status = res.status
	err.code = String(data.code || '')
    throw err
  }
  return data
}

const inflightGetRequests = new Map()

function requestDuration(startedAt) {
  return typeof performance !== 'undefined' && typeof performance.now === 'function'
    ? performance.now() - startedAt
    : Date.now() - startedAt
}

function rowCount(payload) {
  if (Array.isArray(payload)) return payload.length
  if (!payload || typeof payload !== 'object') return 0
  for (const key of ['rows', 'items', 'options', 'products']) {
    if (Array.isArray(payload[key])) return payload[key].length
  }
  return 0
}

function requestPath(url) {
  try {
    return new URL(String(url), typeof window !== 'undefined' ? window.location.origin : 'http://localhost').pathname
  } catch {
    return String(url).split('?')[0]
  }
}

function reportSlowRequest(method, url, startedAt, payload, error = null) {
  const durationMs = Math.round(requestDuration(startedAt))
  if (durationMs < 500 || typeof console === 'undefined' || typeof console.warn !== 'function') return
  console.warn('[KFerp slow request]', {
    method,
    path: requestPath(url),
    durationMs,
    rowCount: rowCount(payload),
    status: error?.status || undefined,
  })
}

function readAuthToken() {
  try {
    if (typeof window === 'undefined') return ''
    return window.localStorage?.getItem('auth_token') || ''
  } catch {
    return ''
  }
}

function authHeaders(headers = {}) {
  const token = readAuthToken()
  if (!token || headers.Authorization) return headers
  return { ...headers, Authorization: `Bearer ${token}` }
}

function appBasePath() {
  if (typeof window === 'undefined') return ''
  try {
    const pathname = window.location?.pathname || new URL(window.location?.href || '', window.location?.origin || 'http://localhost').pathname
    return pathname === '/app' || pathname.startsWith('/app/') ? '/app' : ''
  } catch {
    return ''
  }
}

export function appURL(url) {
  if (typeof url !== 'string' || !url.startsWith('/')) return url
  const base = appBasePath()
  if (!base || url === base || url.startsWith(`${base}/`)) return url
  return `${base}${url}`
}

export function apiURL(url) {
  if (url instanceof URL) {
    if (typeof window === 'undefined' || url.origin !== window.location.origin) return url.toString()
    return new URL(appURL(`${url.pathname}${url.search}${url.hash}`), window.location.origin).toString()
  }
  if (typeof window === 'undefined' || typeof url !== 'string' || !url.startsWith('/')) return url
  return new URL(appURL(url), window.location.origin).toString()
}

export async function apiFetch(url, options = {}) {
  const { headers = {}, ...rest } = options
  return fetch(apiURL(url), {
    ...rest,
    headers: authHeaders(headers),
  })
}

export async function apiGet(url) {
  const key = apiURL(url)
  const existing = inflightGetRequests.get(key)
  if (existing) return existing
  const startedAt = typeof performance !== 'undefined' && typeof performance.now === 'function' ? performance.now() : Date.now()
  const request = (async () => {
    try {
      const res = await apiFetch(url, { headers: { Accept: 'application/json' } })
      const payload = await readJson(res)
      reportSlowRequest('GET', key, startedAt, payload)
      return payload
    } catch (error) {
      reportSlowRequest('GET', key, startedAt, null, error)
      throw error
    }
  })()
  inflightGetRequests.set(key, request)
  request.then(() => {
    if (inflightGetRequests.get(key) === request) inflightGetRequests.delete(key)
  }, () => {
    if (inflightGetRequests.get(key) === request) inflightGetRequests.delete(key)
  })
  return request
}

export async function apiSend(url, { method = 'POST', body, headers = {}, signal } = {}) {
  const payload = body instanceof FormData || body instanceof URLSearchParams ? body : JSON.stringify(body ?? {})
  const baseHeaders = { Accept: 'application/json' }
  if (!(body instanceof FormData)) {
    baseHeaders['Content-Type'] = body instanceof URLSearchParams ? 'application/x-www-form-urlencoded' : 'application/json'
  }
  const key = apiURL(url)
  const startedAt = typeof performance !== 'undefined' && typeof performance.now === 'function' ? performance.now() : Date.now()
  try {
    const res = await apiFetch(url, {
      method,
      headers: { ...baseHeaders, ...headers },
      body: payload,
      signal,
    })
    const data = await readJson(res)
    reportSlowRequest(method, key, startedAt, data)
    return data
  } catch (error) {
    reportSlowRequest(method, key, startedAt, null, error)
    throw error
  }
}
