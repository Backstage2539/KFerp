async function readJson(res) {
  if (res.status === 204) return null
  const data = await res.json().catch(() => ({}))
  if (!res.ok) throw new Error(data.error || '请求失败')
  return data
}

export function apiURL(url) {
  if (typeof window === 'undefined' || !url.startsWith('/')) return url
  return new URL(url, window.location.origin).toString()
}

export async function apiGet(url) {
  const res = await fetch(apiURL(url), {
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
  const res = await fetch(apiURL(url), {
    method,
    headers: { ...baseHeaders, ...headers },
    body: payload,
  })
  return readJson(res)
}
