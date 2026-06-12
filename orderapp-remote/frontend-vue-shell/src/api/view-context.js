import { apiGet, apiSend } from './client.js'

function optionFromViewContextCustomer(option) {
  return {
    id: Number(option?.customer_id || option?.id || 0),
    name: option?.customer_name || option?.label || '',
    company_name: option?.company_name || '',
    contact: option?.contact || '',
    phone: option?.phone || '',
  }
}

function deps(overrides = {}) {
  return {
    get: overrides.get || apiGet,
    send: overrides.send || apiSend,
  }
}

export async function fetchWorkspaceCustomerOptions(overrides = {}) {
  const { get } = deps(overrides)
  try {
    const data = await get('/api/view-context/options?type=customer&limit=200')
    return (data.options || []).map(optionFromViewContextCustomer)
  } catch {
    try {
      const data = await get('/api/customer-fulfillment/customers?limit=200')
      return data.customers || data.items || []
    } catch {
      try {
        const data = await get('/api/customers?limit=200')
        return data.customers || data.items || []
      } catch {
        return []
      }
    }
  }
}

export async function fetchWorkspaceOrderOptions(overrides = {}) {
  const { get } = deps(overrides)
  try {
    const data = await get('/api/view-context/options?type=order&limit=80')
    return data.options || []
  } catch {
    return []
  }
}

export async function fetchViewContextPresets(overrides = {}) {
  const { get } = deps(overrides)
  try {
    const data = await get('/api/view-context/presets')
    return data.presets || []
  } catch {
    return []
  }
}

export function saveViewContextPreset(body, overrides = {}) {
  const { send } = deps(overrides)
  return send('/api/view-context/presets', { body })
}

export function disableViewContextPreset(id, overrides = {}) {
  const { send } = deps(overrides)
  return send(`/api/view-context/presets/${Number(id || 0)}/disable`)
}
