import { completionRetry } from './operation-completion.js'
const forms = new Map()
export function workstationFormCache(kind, jobCardId) {
  const key = `${kind}:${jobCardId}`
  if (!forms.has(key)) forms.set(key, { draft: {}, initialized: false, retry: completionRetry() })
  return forms.get(key)
}
export function clearWorkstationFormCache(kind, jobCardId) { forms.delete(`${kind}:${jobCardId}`) }
