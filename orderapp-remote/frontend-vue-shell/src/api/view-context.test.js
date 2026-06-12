import assert from 'node:assert/strict'
import { describe, it } from 'node:test'
import {
  disableViewContextPreset,
  fetchViewContextPresets,
  fetchWorkspaceCustomerOptions,
  fetchWorkspaceOrderOptions,
  saveViewContextPreset,
} from './view-context.js'

describe('view context API helpers', () => {
  it('loads workspace customer options from view-context options first', async () => {
    const calls = []
    const rows = await fetchWorkspaceCustomerOptions({
      get: async (path) => {
        calls.push(path)
        return { options: [{ customer_id: 7, customer_name: '云南客户', company_name: '云南门店' }] }
      },
    })

    assert.deepEqual(calls, ['/api/view-context/options?type=customer&limit=200'])
    assert.deepEqual(rows, [{ id: 7, name: '云南客户', company_name: '云南门店', contact: '', phone: '' }])
  })

  it('falls back through fulfillment customers and customer list endpoints', async () => {
    const calls = []
    const rows = await fetchWorkspaceCustomerOptions({
      get: async (path) => {
        calls.push(path)
        if (path.includes('/api/view-context/options')) throw new Error('view context unavailable')
        if (path.includes('/api/customer-fulfillment/customers')) throw new Error('fulfillment unavailable')
        return { customers: [{ id: 9, name: '兜底客户' }] }
      },
    })

    assert.deepEqual(calls, [
      '/api/view-context/options?type=customer&limit=200',
      '/api/customer-fulfillment/customers?limit=200',
      '/api/customers?limit=200',
    ])
    assert.deepEqual(rows, [{ id: 9, name: '兜底客户' }])
  })

  it('loads order options and presets through named helpers', async () => {
    const calls = []
    const get = async (path) => {
      calls.push(path)
      if (path.includes('type=order')) return { options: [{ order_id: 12, label: 'SO-12' }] }
      return { presets: [{ id: 3, name: '工厂视图' }] }
    }

    assert.deepEqual(await fetchWorkspaceOrderOptions({ get }), [{ order_id: 12, label: 'SO-12' }])
    assert.deepEqual(await fetchViewContextPresets({ get }), [{ id: 3, name: '工厂视图' }])
    assert.deepEqual(calls, ['/api/view-context/options?type=order&limit=80', '/api/view-context/presets'])
  })

  it('saves and disables presets through named helpers', async () => {
    const sends = []
    const send = async (path, options) => {
      sends.push([path, options])
      return { preset: { id: 5 } }
    }

    assert.deepEqual(await saveViewContextPreset({ name: '客户视图' }, { send }), { preset: { id: 5 } })
    await disableViewContextPreset(5, { send })
    assert.deepEqual(sends, [
      ['/api/view-context/presets', { body: { name: '客户视图' } }],
      ['/api/view-context/presets/5/disable', undefined],
    ])
  })
})
