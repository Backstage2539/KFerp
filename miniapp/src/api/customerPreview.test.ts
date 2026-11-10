import { beforeEach, expect, it, vi } from 'vitest'
import { markPreviewToken, miniRequest, miniUpload, unmarkPreviewToken } from './client'

beforeEach(() => {
  vi.stubEnv('VITE_KFERP_ENVIRONMENT', 'development')
  vi.stubEnv('VITE_KFERP_API_BASE', 'https://dev.qacoohee.com/app')
})
it('blocks preview writes locally while allowing read and explicit exit', async () => {
  const request = vi.fn(options => options.success({statusCode: 200, data: {}}))
  const uploadFile = vi.fn()
  vi.stubGlobal('uni', {request, uploadFile})
  markPreviewToken('preview')
  await expect(miniRequest('/api/mini/fulfillment-orders', {method: 'POST', token: 'preview'})).rejects.toThrow('只读')
  await expect(miniUpload('/upload', '/tmp/x', 'preview')).rejects.toThrow('只读')
  expect(request).not.toHaveBeenCalled()
  expect(uploadFile).not.toHaveBeenCalled()
  await miniRequest('/api/mini/services/orders', {token: 'preview'})
  await miniRequest('/api/mini/customer-preview/exit', {method: 'POST', token: 'preview'})
  expect(request).toHaveBeenCalledTimes(2)
  unmarkPreviewToken('preview')
})
