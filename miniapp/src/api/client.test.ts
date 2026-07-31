import { describe, expect, it } from 'vitest'
import {
  buildAPIURL,
  isAuthenticationExpiredRequestError,
  isUnauthorizedRequestError,
  MiniRequestError,
  normalizeAPIBase,
} from './client'

describe('miniapp API client URL helpers', () => {
  it('normalizes configured API base by trimming whitespace and trailing slashes', () => {
    expect(normalizeAPIBase(' https://erp.example.com/app/// ')).toBe('https://erp.example.com/app')
  })

  it('builds mini API URLs from the configured base and request path', () => {
    expect(buildAPIURL('/api/mini/me', 'https://erp.example.com/app/')).toBe(
      'https://erp.example.com/app/api/mini/me',
    )
  })

  it('preserves HTTP status so pages can distinguish an expired login', () => {
    expect(isUnauthorizedRequestError(new MiniRequestError('请重新登录', 401))).toBe(true)
    expect(isUnauthorizedRequestError(new MiniRequestError('无权访问', 403))).toBe(true)
    expect(isUnauthorizedRequestError(new MiniRequestError('服务异常', 500))).toBe(false)
    expect(isUnauthorizedRequestError(new Error('network error'))).toBe(false)
    expect(isAuthenticationExpiredRequestError(new MiniRequestError('请重新登录', 401))).toBe(true)
    expect(isAuthenticationExpiredRequestError(new MiniRequestError('无权访问', 403))).toBe(false)
  })
})
