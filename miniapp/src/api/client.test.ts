import { describe, expect, it, vi } from 'vitest'
import {
  buildAPIURL,
  isAuthenticationExpiredRequestError,
  isUnauthorizedRequestError,
  MiniRequestError,
  normalizeAPIBase,
  miniRequest,
  miniUpload,
} from './client'

describe('miniapp API client URL helpers', () => {
  it('uploads files with the miniapp bearer token and parses the JSON asset response', async () => {
    vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
    vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
    const uploadFile = vi.fn((options: any) => options.success({ statusCode: 200, data: JSON.stringify({ asset: { id: 8 } }) }))
    vi.stubGlobal('uni', { uploadFile })
    try {
      await expect(miniUpload('/api/mini/employee/orders/42/payment-voucher', '/tmp/receipt.png', 'employee-token')).resolves.toEqual({ asset: { id: 8 } })
      expect(uploadFile).toHaveBeenCalledWith(expect.objectContaining({
        url: 'https://dev.qacoohee.com/app/api/mini/employee/orders/42/payment-voucher', filePath: '/tmp/receipt.png', name: 'file', header: { Authorization: 'Bearer employee-token' },
      }))
    } finally { vi.unstubAllGlobals(); vi.unstubAllEnvs() }
  })

  it('keeps upload authorization failures distinguishable from invalid responses', async () => {
    vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
    vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
    vi.stubGlobal('uni', { uploadFile: (options: any) => options.success({ statusCode: 401, data: '{"error":"登录已失效"}' }) })
    try {
      await expect(miniUpload('/api/upload', '/tmp/receipt.png', 'expired')).rejects.toMatchObject({ statusCode: 401, message: '登录已失效' })
      vi.stubGlobal('uni', { uploadFile: (options: any) => options.success({ statusCode: 200, data: 'invalid' }) })
      await expect(miniUpload('/api/upload', '/tmp/receipt.png', 'token')).rejects.toThrow('上传响应')
    } finally { vi.unstubAllGlobals(); vi.unstubAllEnvs() }
  })
  it('preserves registration-required API codes for direct-link inline registration', async () => {
    vi.stubEnv('VITE_KFERP_ENVIRONMENT','development')
    vi.stubEnv('VITE_KFERP_API_BASE','https://dev.qacoohee.com/app')
    vi.stubGlobal('uni',{request:({success}:any)=>success({statusCode:401,data:{error:'需要登记',code:'mini_registration_required'}})})
    try {
      await expect(miniRequest('/api/pages/entry')).rejects.toMatchObject({statusCode:401,code:'mini_registration_required'})
    } finally { vi.unstubAllGlobals(); vi.unstubAllEnvs() }
  })
  it('shows the server authorization reason from standard error bodies', async()=>{
    vi.stubEnv('VITE_KFERP_ENVIRONMENT','development');vi.stubEnv('VITE_KFERP_API_BASE','https://dev.qacoohee.com/app')
    vi.stubGlobal('uni',{request:({success}:any)=>success({statusCode:403,data:{message:'只有管理员可以分配'}})})
    try{await expect(miniRequest('/api/mini/employee/sample-requests/1/assign')).rejects.toThrow('只有管理员可以分配')}finally{vi.unstubAllGlobals();vi.unstubAllEnvs()}
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

it('retains the exact current-template revision for employee confirmation',async()=>{
 vi.stubEnv('VITE_KFERP_ENVIRONMENT','development');vi.stubEnv('VITE_KFERP_API_BASE','https://dev.qacoohee.com/app')
 vi.stubGlobal('uni',{request:({success}:any)=>success({statusCode:409,data:{error:'模板已变化',code:'pricing_rule_confirmation_required',current_rule_revision:'rule-v2'}})})
 try {await expect(miniRequest('/api/mini/employee/spec-conversions/quote')).rejects.toMatchObject({statusCode:409,code:'pricing_rule_confirmation_required',current_rule_revision:'rule-v2'})}
 finally {vi.unstubAllGlobals();vi.unstubAllEnvs()}
})
