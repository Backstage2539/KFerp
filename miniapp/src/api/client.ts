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
  readonly code: string
  readonly current_rule_revision: string

  constructor(message: string, statusCode: number, code = '', currentRuleRevision = '') {
    super(message)
    this.name = 'MiniRequestError'
    this.statusCode = statusCode
    this.code = code
    this.current_rule_revision = currentRuleRevision
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
        const body = res.data as { error?: string; message?: string; code?: string; current_rule_revision?: string }
        reject(new MiniRequestError(body?.error || (typeof body?.message==='string'?body.message:'') || `请求失败（${res.statusCode}），请重试`, res.statusCode, body?.code, body?.current_rule_revision))
      },
      fail: (err) => reject(new Error(err.errMsg || 'network error')),
    })
  })
}

export function miniUpload<T>(path: string, filePath: string, token: string): Promise<T> {
  return new Promise((resolve, reject) => {
    let url = ''
    try { url = buildAPIURL(path) } catch (cause) {
      reject(cause instanceof Error ? cause : new Error('小程序环境配置错误'))
      return
    }
    uni.uploadFile({
      url, filePath, name: 'file', header: { Authorization: `Bearer ${token}` },
      success: response => {
        let body: { error?: string; message?: string; code?: string }
        try { body = JSON.parse(response.data) } catch {
          reject(new Error('上传响应无效，请重试'))
          return
        }
        if (response.statusCode >= 200 && response.statusCode < 300) {
          resolve(body as T)
          return
        }
        reject(new MiniRequestError(body?.error || body?.message || '凭证上传失败，请重试', response.statusCode, body?.code))
      },
      fail: cause => reject(new Error(cause.errMsg || '凭证上传失败，请重试')),
    })
  })
}
