const DEFAULT_API_BASE = 'https://erp.qacoohee.com/app'

export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  token?: string
  data?: UniNamespace.RequestOptions['data']
}

export function normalizeAPIBase(base: string): string {
  const trimmed = base.trim().replace(/\/+$/, '')
  return trimmed || DEFAULT_API_BASE
}

export function configuredAPIBase(): string {
  return normalizeAPIBase(import.meta.env.VITE_KFERP_API_BASE || DEFAULT_API_BASE)
}

export function buildAPIURL(path: string, base = configuredAPIBase()): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${normalizeAPIBase(base)}${normalizedPath}`
}

export function miniRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return new Promise((resolve, reject) => {
    uni.request({
      url: buildAPIURL(path),
      method: options.method || 'GET',
      data: options.data,
      header: {
        ...(options.token ? { Authorization: `Bearer ${options.token}` } : {}),
        'content-type': 'application/json',
      },
      success: (res) => {
        if (res.statusCode >= 200 && res.statusCode < 300) {
          resolve(res.data as T)
          return
        }
        const body = res.data as { error?: string }
        reject(new Error(body?.error || `request failed: ${res.statusCode}`))
      },
      fail: (err) => reject(new Error(err.errMsg || 'network error')),
    })
  })
}
