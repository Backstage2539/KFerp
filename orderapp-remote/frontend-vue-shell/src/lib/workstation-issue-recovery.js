const key = 'kferp:workstation-batch-issue:pending:v1'
const invalid = '无法安全读取领料重试记录，请联系管理员核对库存和操作日志。'
export function issueSessionStorage() {
  try { if (!window.sessionStorage) throw Error(); return window.sessionStorage }
  catch { throw new Error('浏览器存储不可用，无法安全保存领料重试记录。') }
}
function validPayload(p) {
  return p && typeof p.request_id === 'string' && p.request_id.trim() && typeof p.preview_version === 'string' && p.preview_version && Array.isArray(p.job_card_ids) && p.job_card_ids.length && p.job_card_ids.every(id => Number.isSafeInteger(id) && id > 0) && Array.isArray(p.groups) && p.groups.length && p.groups.every(g => typeof g.key === 'string' && g.key && typeof g.version === 'string' && g.version && Number.isFinite(g.quantity) && g.quantity > 0)
}
export function readPendingIssue(storage) {
  let raw
  try { raw = storage.getItem(key) } catch { throw new Error(invalid) }
  if (!raw) return null
  try { const value=JSON.parse(raw); if(value.version!==1 || !validPayload(value.payload)) throw Error(); return value.payload } catch { throw new Error(invalid) }
}
export function savePendingIssue(storage, payload) {
  if (!validPayload(payload)) throw new Error(invalid)
  const old=readPendingIssue(storage)
  if(old && JSON.stringify(old)!==JSON.stringify(payload)) throw new Error('存在另一笔未确认领料，请先恢复原次操作。')
  try { storage.setItem(key,JSON.stringify({version:1,payload})); if(JSON.stringify(readPendingIssue(storage))!==JSON.stringify(payload)) throw Error() } catch { throw new Error('无法保存领料重试记录，本次未提交。') }
}
export function clearPendingIssue(storage, payload) {
  const old=readPendingIssue(storage)
  if(!old || JSON.stringify(old)!==JSON.stringify(payload)) throw new Error(invalid)
  try { storage.removeItem(key); if(storage.getItem(key)!==null) throw Error() } catch { throw new Error('无法清除领料重试记录，请重试确认。') }
}
export function validIssueResult(result,payload) {
  if(!result || result.request_id!==payload.request_id || !Array.isArray(result.groups) || result.groups.length!==payload.groups.length || !Number.isInteger(result.success_count) || !Number.isInteger(result.failed_count)) return false
  const expected=payload.groups.map(g=>g.key).sort(), actual=result.groups.map(g=>g.key).sort()
  const complete = row => {
    if (!row || !['submitted','failed'].includes(row.status) || !['quantity','extra','remaining'].every(field => Number.isSafeInteger(row[field]) && row[field] >= 0) || row.extra > row.quantity || !Array.isArray(row.documents)) return false
    if (row.status === 'failed') return row.quantity === 0 && row.extra === 0 && row.documents.length === 0 && typeof row.error === 'string' && !!row.error.trim()
    return row.quantity > 0 && typeof row.unit === 'string' && !!row.unit.trim() && ['weight','count'].includes(row.quantity_basis) && row.documents.length > 0 && row.documents.every(doc => Number.isSafeInteger(doc.id) && doc.id > 0 && typeof doc.entry_no === 'string' && !!doc.entry_no.trim())
  }
  return JSON.stringify(expected)===JSON.stringify(actual) && result.groups.every(complete) && result.groups.filter(g=>g.status==='submitted').length===result.success_count && result.groups.filter(g=>g.status==='failed').length===result.failed_count
}
