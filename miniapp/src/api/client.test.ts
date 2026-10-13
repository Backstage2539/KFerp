import { describe, expect, it, vi } from 'vitest'
import {
  buildAPIURL,
  isAuthenticationExpiredRequestError,
  isUnauthorizedRequestError,
  MiniRequestError,
  normalizeAPIBase,
  miniRequest,
} from './client'

describe('miniapp API client URL helpers', () => {
  it('preserves registration-required API codes for direct-link inline registration', async () => {
    vi.stubEnv('VITE_KFERP_ENVIRONMENT','development')
    vi.stubEnv('VITE_KFERP_API_BASE','https://dev.qacoohee.com/app')
    vi.stubGlobal('uni',{request:({success}:any)=>success({statusCode:401,data:{error:'需要登记',code:'mini_registration_required'}})})
    try {
      await expect(miniRequest('/api/pages/entry')).rejects.toMatchObject({statusCode:401,code:'mini_registration_required'})
    } finally { vi.unstubAllGlobals(); vi.unstubAllEnvs() }
  })
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
