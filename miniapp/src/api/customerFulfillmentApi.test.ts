import { beforeEach, describe, expect, it, vi } from 'vitest'
import { miniRequest } from './client'
import {
  buildCustomerBillDetailPath,
  buildCustomerBillsPath,
  buildCustomerInventoryBatchesPath,
  buildCustomerInventoryPath,
  buildDirectShipCatalogPath,
  buildDirectShipRequestDetailPath,
  buildDirectShipRequestsPath,
  buildProcessingRequestDetailPath,
  buildProcessingRequestsPath,
  buildProcessingCatalogPath,
  cancelDirectShipRequest,
  createDirectShipRequest,
  createProcessingRequest,
  fetchCustomerBillDetail,
  fetchCustomerBills,
  fetchCustomerInventory,
  fetchCustomerInventoryBatches,
  fetchDirectShipCatalog,
  fetchDirectShipRequestDetail,
  fetchDirectShipRequests,
  fetchProcessingRequestDetail,
  fetchProcessingRequests,
  fetchProcessingCatalog,
  previewDirectShipRequest,
  previewProcessingRequest,
} from './customerPortal'

vi.mock('./client', () => ({ miniRequest: vi.fn() }))

beforeEach(() => vi.mocked(miniRequest).mockReset())

describe('customer fulfillment API', () => {
  it('uses customer-safe catalog, inventory and detail routes', () => {
    expect(buildDirectShipCatalogPath()).toBe('/api/mini/direct-ship/catalog')
    expect(buildDirectShipRequestsPath()).toBe('/api/mini/direct-ship/requests')
    expect(buildDirectShipRequestDetailPath(17)).toBe('/api/mini/direct-ship/requests/17')
    expect(buildProcessingRequestsPath()).toBe('/api/mini/processing-requests')
    expect(buildProcessingCatalogPath()).toBe('/api/mini/processing/catalog')
    expect(buildProcessingRequestDetailPath(18)).toBe('/api/mini/processing-requests/18')
    expect(buildCustomerInventoryPath()).toBe('/api/mini/customer-inventory')
    expect(buildCustomerInventoryBatchesPath(911, 60000)).toBe('/api/mini/customer-inventory/911/batches?spec_g=60000')
    expect(buildCustomerBillsPath()).toBe('/api/mini/customer-bills')
    expect(buildCustomerBillDetailPath(19)).toBe('/api/mini/customer-bills/19')
  })

  it('submits one idempotent multi-item direct-ship request and can cancel it', async () => {
    const payload = {
      idempotency_key: 'mini-ds-001',
      recipient_name: '张三',
      recipient_phone: '13800138000',
      province: '云南省',
      city: '普洱市',
      district: '思茅区',
      detail_address: '咖啡路 88 号',
      items: [{ product_id: 911, spec_g: 60000, qty: 2 }],
      note: '',
    }
    vi.mocked(miniRequest).mockResolvedValue({ id: 17, request_no: 'DS-17' })

    await previewDirectShipRequest('token', payload)
    expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/direct-ship/preview', {
      method: 'POST', token: 'token', data: payload,
    })
    await createDirectShipRequest('token', payload)
    expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/direct-ship/requests', {
      method: 'POST', token: 'token', data: payload,
    })
    await cancelDirectShipRequest('token', 17)
    expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/direct-ship/requests/17/cancel', {
      method: 'POST', token: 'token',
    })
  })

  it('uses multi-item BOM preview and submit without client-selected input materials', async () => {
    const payload = { items: [{ product_id: 911, spec_g: 60000, qty: 2 }], note: '客户生产' }
    vi.mocked(miniRequest).mockResolvedValue({ can_submit: true, items: [], materials: [] })

    await previewProcessingRequest('token', payload)
    expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/processing-requests/preview', {
      method: 'POST', token: 'token', data: payload,
    })
    await createProcessingRequest('token', payload)
    expect(miniRequest).toHaveBeenLastCalledWith('/api/mini/processing-requests', {
      method: 'POST', token: 'token', data: payload,
    })
    expect(payload).not.toHaveProperty('input_material_id')
  })

  it('reads shipment, production, inventory and pushed-bill details', async () => {
    vi.mocked(miniRequest).mockResolvedValue({ rows: [] })

    await fetchDirectShipCatalog('token')
    await fetchDirectShipRequests('token')
    await fetchDirectShipRequestDetail('token', 17)
    await fetchProcessingRequests('token')
    await fetchProcessingCatalog('token')
    await fetchProcessingRequestDetail('token', 18)
    await fetchCustomerInventory('token')
    await fetchCustomerInventoryBatches('token', 911, 60000)
    await fetchCustomerBills('token')
    await fetchCustomerBillDetail('token', 19)

    expect(vi.mocked(miniRequest).mock.calls.map(([path]) => path)).toEqual([
      '/api/mini/direct-ship/catalog',
      '/api/mini/direct-ship/requests',
      '/api/mini/direct-ship/requests/17',
      '/api/mini/processing-requests',
      '/api/mini/processing/catalog',
      '/api/mini/processing-requests/18',
      '/api/mini/customer-inventory',
      '/api/mini/customer-inventory/911/batches?spec_g=60000',
      '/api/mini/customer-bills',
      '/api/mini/customer-bills/19',
    ])
  })
})
