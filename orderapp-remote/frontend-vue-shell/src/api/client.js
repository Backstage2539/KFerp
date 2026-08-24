async function readJson(res) {
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.error || '请求失败')
    err.status = res.status
	err.code = String(data.code || '')
    throw err
  }
  return data
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
  const res = await apiFetch(url, {
    headers: { Accept: 'application/json' },
  })
  return readJson(res)
}

export async function apiSend(url, { method = 'POST', body, headers = {}, signal } = {}) {
  const payload = body instanceof FormData || body instanceof URLSearchParams ? body : JSON.stringify(body ?? {})
  const baseHeaders = { Accept: 'application/json' }
  if (!(body instanceof FormData)) {
    baseHeaders['Content-Type'] = body instanceof URLSearchParams ? 'application/x-www-form-urlencoded' : 'application/json'
  }
  const res = await apiFetch(url, {
    method,
    headers: { ...baseHeaders, ...headers },
    body: payload,
    signal,
  })
  return readJson(res)
}
