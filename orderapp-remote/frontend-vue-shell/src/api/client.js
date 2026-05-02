async function readJson(res) {
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) {
    const err = new Error(data.error || '请求失败')
    err.status = res.status
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

export function apiURL(url) {
  if (url instanceof URL) {
    if (typeof window === 'undefined' || url.origin !== window.location.origin) return url.toString()
    return new URL(`${url.pathname}${url.search}${url.hash}`, window.location.origin).toString()
  }
  if (typeof window === 'undefined' || typeof url !== 'string' || !url.startsWith('/')) return url
  return new URL(url, window.location.origin).toString()
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

export async function apiSend(url, { method = 'POST', body, headers = {} } = {}) {
  const payload = body instanceof FormData || body instanceof URLSearchParams ? body : JSON.stringify(body ?? {})
  const baseHeaders = { Accept: 'application/json' }
  if (!(body instanceof FormData)) {
    baseHeaders['Content-Type'] = body instanceof URLSearchParams ? 'application/x-www-form-urlencoded' : 'application/json'
  }
  const res = await apiFetch(url, {
    method,
    headers: { ...baseHeaders, ...headers },
    body: payload,
  })
  return readJson(res)
}
