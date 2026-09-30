import { apiGet, apiSend } from './client.js'

const base = '/api/product-creator'

export const getProductCreatorModules = () => apiGet(`${base}/modules`)
export const listProductionBomSpecTemplates = () => apiGet('/api/production-bom-spec-templates')
export const getProductionBomSpecTemplateVersion = (templateID, versionID) => apiGet(`/api/production-bom-spec-templates/${templateID}?version_id=${versionID}`)
export const listProductCreatorTemplates = () => apiGet(`${base}/templates`)
export const saveProductCreatorTemplate = (template) => template.id
  ? apiSend(`${base}/templates/${template.id}`, { method: 'PUT', body: template })
  : apiSend(`${base}/templates`, { body: template })
export const validateProductCreatorTemplate = (id) => apiSend(`${base}/templates/${id}/validate`)
export const publishProductCreatorTemplate = (id, revision) => apiSend(`${base}/templates/${id}/publish`, { body: { revision } })
export const disableProductCreatorTemplate = (id, revision) => apiSend(`${base}/templates/${id}/disable`, { body: { revision } })
export const copyProductCreatorTemplate = (id, name) => apiSend(`${base}/templates/${id}/copy`, { body: { name } })
export const listProductCreatorVersions = (id) => apiGet(`${base}/templates/${id}/versions`)
export const listProductCreatorRuns = (id, limit = 20) => apiGet(`${base}/templates/${id}/runs?limit=${limit}`)
export const listRecentProductCreatorRuns = (limit = 5) => apiGet(`${base}/runs/recent?limit=${limit}`)
export const startProductCreatorRun = (templateId) => apiSend(`${base}/runs`, { body: { template_id: templateId } })
export const getProductCreatorRun = (id) => apiGet(`${base}/runs/${id}`)
export const saveProductCreatorRunDraft = (id, revision, inputs, variableValues = null) => apiSend(`${base}/runs/${id}/draft`, {
  method: 'PUT',
  body: { revision, inputs, ...(variableValues ? { variable_values: variableValues } : {}) },
})
export const previewProductCreatorRun = (id, revision) => apiSend(`${base}/runs/${id}/preview`, { body: { revision } })
export const commitProductCreatorRun = (id, revision, idempotencyKey) => apiSend(`${base}/runs/${id}/commit`, {
  body: { revision },
  headers: { 'Idempotency-Key': idempotencyKey },
})
export const executeProductCreatorRunStep = (id, nodeID, revision, action, inputs, idempotencyKey) => apiSend(`${['preview_pricing', 'save_price_draft', 'publish_price'].includes(action) ? `${base}/pricing` : base}/runs/${id}/nodes/${encodeURIComponent(nodeID)}/execute`, {
  body: { revision, action, inputs },
  headers: { 'Idempotency-Key': idempotencyKey },
})
