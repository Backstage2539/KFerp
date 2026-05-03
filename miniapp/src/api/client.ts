const API_BASE = 'https://erp.qacoohee.com/app'

export type RequestOptions = {
  method?: 'GET' | 'POST'
  token?: string
  data?: unknown
}

export function miniRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return new Promise((resolve, reject) => {
    uni.request({
      url: `${API_BASE}${path}`,
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
