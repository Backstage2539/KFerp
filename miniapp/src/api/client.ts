import {
  assertMiniappRuntimeSafe,
  configuredMiniappEnvironment,
} from '../config/environment'

export type RequestOptions = {
  method?: 'GET' | 'POST' | 'PUT' | 'DELETE'
  token?: string
  data?: UniNamespace.RequestOptions['data']
}

export class MiniRequestError extends Error {
  readonly statusCode: number

  constructor(message: string, statusCode: number) {
    super(message)
    this.name = 'MiniRequestError'
    this.statusCode = statusCode
  }
}

export function isUnauthorizedRequestError(cause: unknown): boolean {
  return cause instanceof MiniRequestError && (cause.statusCode === 401 || cause.statusCode === 403)
}

export function isAuthenticationExpiredRequestError(cause: unknown): boolean {
  return cause instanceof MiniRequestError && cause.statusCode === 401
}

export function normalizeAPIBase(base: string): string {
  const trimmed = base.trim().replace(/\/+$/, '')
  return trimmed
}

export function configuredAPIBase(): string {
  return assertMiniappRuntimeSafe(configuredMiniappEnvironment()).apiBase
}

export function buildAPIURL(path: string, base = configuredAPIBase()): string {
  const normalizedPath = path.startsWith('/') ? path : `/${path}`
  return `${normalizeAPIBase(base)}${normalizedPath}`
}

export function miniRequest<T>(path: string, options: RequestOptions = {}): Promise<T> {
  return new Promise((resolve, reject) => {
    let url = ''
    try {
      url = buildAPIURL(path)
    } catch (cause) {
      reject(cause instanceof Error ? cause : new Error('小程序环境配置错误'))
      return
    }
    uni.request({
      url,
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
        reject(new MiniRequestError(body?.error || `request failed: ${res.statusCode}`, res.statusCode))
      },
      fail: (err) => reject(new Error(err.errMsg || 'network error')),
    })
  })
}
